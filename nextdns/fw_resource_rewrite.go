package nextdns

import (
	"context"

	"github.com/LukeEvansTech/terraform-provider-nextdns/internal/nextdns"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.ResourceWithConfigure   = &rewriteResource{}
	_ resource.ResourceWithImportState = &rewriteResource{}
)

// rewriteResource manages a profile's DNS rewrites. The API has no
// replace-all call: rewrites are created and deleted one at a time by id.
type rewriteResource struct {
	client *nextdns.Client
}

type rewriteModel struct {
	ID        types.String        `tfsdk:"id"`
	ProfileID types.String        `tfsdk:"profile_id"`
	Rewrite   []rewriteEntryModel `tfsdk:"rewrite"`
}

type rewriteEntryModel struct {
	Domain  types.String `tfsdk:"domain"`
	Address types.String `tfsdk:"address"`
}

func newRewriteResource() resource.Resource {
	return &rewriteResource{}
}

func (r *rewriteResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_rewrite"
}

func (r *rewriteResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id":         idAttribute(),
			"profile_id": profileIDAttribute(),
		},
		Blocks: map[string]schema.Block{
			"rewrite": schema.SetNestedBlock{
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"domain":  schema.StringAttribute{Required: true},
						"address": schema.StringAttribute{Required: true},
					},
				},
				Validators: []validator.Set{setvalidator.IsRequired()},
			},
		},
	}
}

func (r *rewriteResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFrom(req.ProviderData, &resp.Diagnostics)
}

// Create replaces any live rewrite that matches a declared one, so a
// declared rewrite is never created twice. Rewrites that exist only in the
// API are left alone until the next refresh shows them as drift.
func (r *rewriteResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan rewriteModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	profileID := plan.ProfileID.ValueString()

	existing, err := r.client.Rewrites.List(ctx, &nextdns.ListRewritesRequest{ProfileID: profileID})
	if err != nil {
		resp.Diagnostics.AddError("Error getting rewrites", err.Error())
		return
	}
	declared := rewriteKeys(plan.Rewrite)
	for _, e := range existing {
		if declared[rewriteKey{e.Name, e.Content}] {
			if err := r.client.Rewrites.Delete(ctx, &nextdns.DeleteRewritesRequest{ProfileID: profileID, ID: e.ID}); err != nil {
				resp.Diagnostics.AddError("Error deleting rewrite", err.Error())
				return
			}
		}
	}
	for _, w := range plan.Rewrite {
		if !r.create(ctx, profileID, w, &resp.Diagnostics) {
			return
		}
	}

	plan.ID = plan.ProfileID
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *rewriteResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state rewriteModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	live, err := r.client.Rewrites.List(ctx, &nextdns.ListRewritesRequest{ProfileID: state.ProfileID.ValueString()})
	if err != nil {
		resp.Diagnostics.AddError("Error getting rewrites", err.Error())
		return
	}

	state.ID = state.ProfileID
	state.Rewrite = make([]rewriteEntryModel, 0, len(live))
	for _, e := range live {
		state.Rewrite = append(state.Rewrite, rewriteEntryModel{
			Domain:  types.StringValue(e.Name),
			Address: types.StringValue(e.Content),
		})
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update deletes live rewrites that are no longer declared and creates the
// declared ones that are missing; unchanged rewrites keep their ids.
func (r *rewriteResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan rewriteModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	profileID := plan.ProfileID.ValueString()

	existing, err := r.client.Rewrites.List(ctx, &nextdns.ListRewritesRequest{ProfileID: profileID})
	if err != nil {
		resp.Diagnostics.AddError("Error getting rewrites", err.Error())
		return
	}

	declared := rewriteKeys(plan.Rewrite)
	live := map[rewriteKey]bool{}
	for _, e := range existing {
		k := rewriteKey{e.Name, e.Content}
		live[k] = true
		if declared[k] {
			continue
		}
		if err := r.client.Rewrites.Delete(ctx, &nextdns.DeleteRewritesRequest{ProfileID: profileID, ID: e.ID}); err != nil {
			resp.Diagnostics.AddError("Error deleting rewrite", err.Error())
			return
		}
	}
	for _, w := range plan.Rewrite {
		if live[rewriteKey{w.Domain.ValueString(), w.Address.ValueString()}] {
			continue
		}
		if !r.create(ctx, profileID, w, &resp.Diagnostics) {
			return
		}
	}

	plan.ID = plan.ProfileID
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete removes every rewrite on the profile, declared or not, as the
// SDKv2 implementation did.
func (r *rewriteResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state rewriteModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	profileID := state.ProfileID.ValueString()

	live, err := r.client.Rewrites.List(ctx, &nextdns.ListRewritesRequest{ProfileID: profileID})
	if err != nil {
		resp.Diagnostics.AddError("Error getting rewrites", err.Error())
		return
	}
	for _, e := range live {
		if err := r.client.Rewrites.Delete(ctx, &nextdns.DeleteRewritesRequest{ProfileID: profileID, ID: e.ID}); err != nil {
			resp.Diagnostics.AddError("Error deleting rewrite", err.Error())
			return
		}
	}
}

func (r *rewriteResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	importProfileID(ctx, req, resp)
}

func (r *rewriteResource) create(ctx context.Context, profileID string, w rewriteEntryModel, diags *diag.Diagnostics) bool {
	_, err := r.client.Rewrites.Create(ctx, &nextdns.CreateRewritesRequest{
		ProfileID: profileID,
		Rewrites:  &nextdns.Rewrites{Name: w.Domain.ValueString(), Content: w.Address.ValueString()},
	})
	if err != nil {
		diags.AddError("Error creating rewrite", err.Error())
		return false
	}
	return true
}

// rewriteKey identifies a rewrite by what it does; the API id is not part
// of the configuration.
type rewriteKey struct{ name, content string }

func rewriteKeys(entries []rewriteEntryModel) map[rewriteKey]bool {
	out := make(map[rewriteKey]bool, len(entries))
	for _, w := range entries {
		out[rewriteKey{w.Domain.ValueString(), w.Address.ValueString()}] = true
	}
	return out
}

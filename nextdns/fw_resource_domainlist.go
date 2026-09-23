package nextdns

import (
	"context"
	"fmt"

	"github.com/LukeEvansTech/terraform-provider-nextdns/internal/nextdns"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.ResourceWithConfigure   = &domainListResource{}
	_ resource.ResourceWithImportState = &domainListResource{}
)

// domainListResource implements nextdns_allowlist and nextdns_denylist,
// which have the same shape: the whole list is replaced with one PUT.
type domainListResource struct {
	client *nextdns.Client
	kind   domainListKind
}

// domainListKind adapts the shared implementation to one list endpoint.
type domainListKind struct {
	typeSuffix string // "_allowlist"
	noun       string // "allow list", used in diagnostics
	put        func(ctx context.Context, c *nextdns.Client, profileID string, entries []domainEntryModel) error
	list       func(ctx context.Context, c *nextdns.Client, profileID string) ([]domainEntryModel, error)
}

type domainListModel struct {
	ID        types.String       `tfsdk:"id"`
	ProfileID types.String       `tfsdk:"profile_id"`
	Domain    []domainEntryModel `tfsdk:"domain"`
}

type domainEntryModel struct {
	ID     types.String `tfsdk:"id"`
	Active types.Bool   `tfsdk:"active"`
}

func newAllowlistResource() resource.Resource {
	return &domainListResource{kind: domainListKind{
		typeSuffix: "_allowlist",
		noun:       "allow list",
		put: func(ctx context.Context, c *nextdns.Client, profileID string, entries []domainEntryModel) error {
			list := make([]*nextdns.Allowlist, 0, len(entries))
			for _, e := range entries {
				list = append(list, &nextdns.Allowlist{ID: e.ID.ValueString(), Active: e.Active.ValueBool()})
			}
			return c.Allowlist.Create(ctx, &nextdns.CreateAllowlistRequest{ProfileID: profileID, Allowlist: list})
		},
		list: func(ctx context.Context, c *nextdns.Client, profileID string) ([]domainEntryModel, error) {
			got, err := c.Allowlist.List(ctx, &nextdns.ListAllowlistRequest{ProfileID: profileID})
			if err != nil {
				return nil, err
			}
			out := make([]domainEntryModel, 0, len(got))
			for _, e := range got {
				out = append(out, domainEntryModel{ID: types.StringValue(e.ID), Active: types.BoolValue(e.Active)})
			}
			return out, nil
		},
	}}
}

func newDenylistResource() resource.Resource {
	return &domainListResource{kind: domainListKind{
		typeSuffix: "_denylist",
		noun:       "deny list",
		put: func(ctx context.Context, c *nextdns.Client, profileID string, entries []domainEntryModel) error {
			list := make([]*nextdns.Denylist, 0, len(entries))
			for _, e := range entries {
				list = append(list, &nextdns.Denylist{ID: e.ID.ValueString(), Active: e.Active.ValueBool()})
			}
			return c.Denylist.Create(ctx, &nextdns.CreateDenylistRequest{ProfileID: profileID, Denylist: list})
		},
		list: func(ctx context.Context, c *nextdns.Client, profileID string) ([]domainEntryModel, error) {
			got, err := c.Denylist.List(ctx, &nextdns.ListDenylistRequest{ProfileID: profileID})
			if err != nil {
				return nil, err
			}
			out := make([]domainEntryModel, 0, len(got))
			for _, e := range got {
				out = append(out, domainEntryModel{ID: types.StringValue(e.ID), Active: types.BoolValue(e.Active)})
			}
			return out, nil
		},
	}}
}

func (r *domainListResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + r.kind.typeSuffix
}

func (r *domainListResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id":         idAttribute(),
			"profile_id": profileIDAttribute(),
		},
		Blocks: map[string]schema.Block{
			"domain": schema.SetNestedBlock{
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"id":     schema.StringAttribute{Required: true},
						"active": schema.BoolAttribute{Required: true},
					},
				},
				Validators: []validator.Set{setvalidator.IsRequired()},
			},
		},
	}
}

func (r *domainListResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFrom(req.ProviderData, &resp.Diagnostics)
}

func (r *domainListResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	r.write(ctx, req.Plan, &resp.State, &resp.Diagnostics, "creating")
}

func (r *domainListResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	r.write(ctx, req.Plan, &resp.State, &resp.Diagnostics, "updating")
}

// write replaces the live list with the planned one. State is the plan: the
// PUT is a full replace, so the list is exactly what was sent.
func (r *domainListResource) write(ctx context.Context, plan tfsdk.Plan, state *tfsdk.State, diags *diag.Diagnostics, verb string) {
	var m domainListModel
	diags.Append(plan.Get(ctx, &m)...)
	if diags.HasError() {
		return
	}

	if err := r.kind.put(ctx, r.client, m.ProfileID.ValueString(), m.Domain); err != nil {
		diags.AddError(fmt.Sprintf("Error %s %s", verb, r.kind.noun), err.Error())
		return
	}

	m.ID = m.ProfileID
	diags.Append(state.Set(ctx, &m)...)
}

func (r *domainListResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m domainListModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}

	entries, err := r.kind.list(ctx, r.client, m.ProfileID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error getting "+r.kind.noun, err.Error())
		return
	}

	m.ID = m.ProfileID
	m.Domain = entries
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

func (r *domainListResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m domainListModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.kind.put(ctx, r.client, m.ProfileID.ValueString(), nil); err != nil {
		resp.Diagnostics.AddError("Error deleting "+r.kind.noun, err.Error())
	}
}

func (r *domainListResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	importProfileID(ctx, req, resp)
}

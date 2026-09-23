package nextdns

import (
	"context"

	"github.com/LukeEvansTech/terraform-provider-nextdns/internal/nextdns"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.ResourceWithConfigure   = &privacyResource{}
	_ resource.ResourceWithImportState = &privacyResource{}
)

type privacyResource struct {
	client *nextdns.Client
}

type privacyModel struct {
	ID                types.String `tfsdk:"id"`
	ProfileID         types.String `tfsdk:"profile_id"`
	AllowAffiliate    types.Bool   `tfsdk:"allow_affiliate"`
	DisguisedTrackers types.Bool   `tfsdk:"disguised_trackers"`
	Blocklists        types.List   `tfsdk:"blocklists"`
	Natives           types.List   `tfsdk:"natives"`
}

func newPrivacyResource() resource.Resource {
	return &privacyResource{}
}

func (r *privacyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_privacy"
}

func (r *privacyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id":         idAttribute(),
			"profile_id": profileIDAttribute(),
			"allow_affiliate": schema.BoolAttribute{
				Description: "Allow affiliate & tracking links.",
				Required:    true,
			},
			"disguised_trackers": schema.BoolAttribute{
				Description: "Block disguised third-party trackers.",
				Required:    true,
			},
			"blocklists": schema.ListAttribute{
				Description: "Blocklists.",
				ElementType: types.StringType,
				Optional:    true,
			},
			"natives": schema.ListAttribute{
				Description: "Native tracking protection.",
				ElementType: types.StringType,
				Optional:    true,
			},
		},
	}
}

func (r *privacyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFrom(req.ProviderData, &resp.Diagnostics)
}

func (r *privacyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	r.write(ctx, req.Plan, &resp.State, &resp.Diagnostics)
}

func (r *privacyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	r.write(ctx, req.Plan, &resp.State, &resp.Diagnostics)
}

// write sends the lists to their own endpoints (a full replace; an omitted
// list clears it), then PATCHes the switches, as the SDKv2 resource did.
func (r *privacyResource) write(ctx context.Context, plan tfsdk.Plan, state *tfsdk.State, diags *diag.Diagnostics) {
	var m privacyModel
	diags.Append(plan.Get(ctx, &m)...)
	if diags.HasError() {
		return
	}
	profileID := m.ProfileID.ValueString()

	privacy := &nextdns.Privacy{
		AllowAffiliate:    m.AllowAffiliate.ValueBool(),
		DisguisedTrackers: m.DisguisedTrackers.ValueBool(),
		Blocklists:        []*nextdns.PrivacyBlocklists{},
		Natives:           []*nextdns.PrivacyNatives{},
	}
	for _, id := range stringIDs(ctx, m.Blocklists, diags) {
		privacy.Blocklists = append(privacy.Blocklists, &nextdns.PrivacyBlocklists{ID: id})
	}
	for _, id := range stringIDs(ctx, m.Natives, diags) {
		privacy.Natives = append(privacy.Natives, &nextdns.PrivacyNatives{ID: id})
	}
	if diags.HasError() {
		return
	}

	if err := r.client.PrivacyBlocklists.Create(ctx, &nextdns.CreatePrivacyBlocklistsRequest{
		ProfileID: profileID, PrivacyBlocklists: privacy.Blocklists,
	}); err != nil {
		diags.AddError("Error writing privacy blocklists", err.Error())
		return
	}
	if err := r.client.PrivacyNatives.Create(ctx, &nextdns.CreatePrivacyNativesRequest{
		ProfileID: profileID, PrivacyNatives: privacy.Natives,
	}); err != nil {
		diags.AddError("Error writing privacy natives", err.Error())
		return
	}
	if err := r.client.Privacy.Update(ctx, &nextdns.UpdatePrivacyRequest{ProfileID: profileID, Privacy: privacy}); err != nil {
		diags.AddError("Error writing privacy settings", err.Error())
		return
	}

	m.ID = m.ProfileID
	diags.Append(state.Set(ctx, &m)...)
}

func (r *privacyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m privacyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}

	privacy, err := r.client.Privacy.Get(ctx, &nextdns.GetPrivacyRequest{ProfileID: m.ProfileID.ValueString()})
	if err != nil {
		resp.Diagnostics.AddError("Error getting privacy settings", err.Error())
		return
	}

	blocklists := make([]string, 0, len(privacy.Blocklists))
	for _, b := range privacy.Blocklists {
		blocklists = append(blocklists, b.ID)
	}
	natives := make([]string, 0, len(privacy.Natives))
	for _, n := range privacy.Natives {
		natives = append(natives, n.ID)
	}

	m.ID = m.ProfileID
	m.AllowAffiliate = types.BoolValue(privacy.AllowAffiliate)
	m.DisguisedTrackers = types.BoolValue(privacy.DisguisedTrackers)
	m.Blocklists = stringListFromAPI(blocklists, m.Blocklists)
	m.Natives = stringListFromAPI(natives, m.Natives)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

// Delete clears both lists and resets the switches to false.
func (r *privacyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m privacyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	profileID := m.ProfileID.ValueString()

	if err := r.client.PrivacyBlocklists.Create(ctx, &nextdns.CreatePrivacyBlocklistsRequest{
		ProfileID: profileID, PrivacyBlocklists: []*nextdns.PrivacyBlocklists{},
	}); err != nil {
		resp.Diagnostics.AddError("Error deleting privacy blocklists", err.Error())
		return
	}
	if err := r.client.PrivacyNatives.Create(ctx, &nextdns.CreatePrivacyNativesRequest{
		ProfileID: profileID, PrivacyNatives: []*nextdns.PrivacyNatives{},
	}); err != nil {
		resp.Diagnostics.AddError("Error deleting privacy natives", err.Error())
		return
	}
	if err := r.client.Privacy.Update(ctx, &nextdns.UpdatePrivacyRequest{ProfileID: profileID, Privacy: &nextdns.Privacy{}}); err != nil {
		resp.Diagnostics.AddError("Error deleting privacy settings", err.Error())
	}
}

func (r *privacyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	importProfileID(ctx, req, resp)
}

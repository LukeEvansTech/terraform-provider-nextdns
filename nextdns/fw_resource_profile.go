package nextdns

import (
	"context"
	"fmt"

	"github.com/LukeEvansTech/terraform-provider-nextdns/internal/nextdns"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.ResourceWithConfigure   = &profileResource{}
	_ resource.ResourceWithImportState = &profileResource{}
)

type profileResource struct {
	client *nextdns.Client
}

type profileModel struct {
	ID        types.String `tfsdk:"id"`
	ProfileID types.String `tfsdk:"profile_id"`
	Name      types.String `tfsdk:"name"`
}

func newProfileResource() resource.Resource {
	return &profileResource{}
}

func (r *profileResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_profile"
}

func (r *profileResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": idAttribute(),
			"profile_id": schema.StringAttribute{
				Description:   "The profile identifier to target the resource.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Description: "Profile name.",
				Required:    true,
			},
		},
	}
}

func (r *profileResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFrom(req.ProviderData, &resp.Diagnostics)
}

func (r *profileResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan profileModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id, err := r.client.Profiles.Create(ctx, &nextdns.CreateProfileRequest{Name: plan.Name.ValueString()})
	if err != nil {
		resp.Diagnostics.AddError("Error creating profile", err.Error())
		return
	}

	plan.ID = types.StringValue(id)
	plan.ProfileID = types.StringValue(id)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *profileResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state profileModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	profile, err := r.client.Profiles.Get(ctx, &nextdns.GetProfileRequest{ProfileID: state.ProfileID.ValueString()})
	if err != nil {
		resp.Diagnostics.AddError("Error getting profile", err.Error())
		return
	}

	state.ID = state.ProfileID
	state.Name = types.StringValue(profile.Name)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *profileResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan profileModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.Profiles.Update(ctx, &nextdns.UpdateProfileRequest{
		ProfileID: plan.ProfileID.ValueString(),
		Profile:   &nextdns.Profile{Name: plan.Name.ValueString()},
	})
	if err != nil {
		resp.Diagnostics.AddError("Error updating profile", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *profileResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state profileModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.Profiles.Delete(ctx, &nextdns.DeleteProfileRequest{ProfileID: state.ProfileID.ValueString()})
	if err != nil {
		resp.Diagnostics.AddError("Error deleting profile", fmt.Sprintf("profile %s: %s", state.ProfileID.ValueString(), err))
	}
}

func (r *profileResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	importProfileID(ctx, req, resp)
}

// importProfileID imports any profile-scoped resource: the import ID is the
// profile ID, and both id and profile_id carry it, as under SDKv2.
func importProfileID(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("profile_id"), req.ID)...)
}

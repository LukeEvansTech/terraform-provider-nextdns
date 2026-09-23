package nextdns

import (
	"fmt"

	"github.com/LukeEvansTech/terraform-provider-nextdns/internal/nextdns"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

// idAttribute is the id every resource carried implicitly under SDKv2. It
// always equals the profile ID.
func idAttribute() schema.StringAttribute {
	return schema.StringAttribute{
		Description:   "The profile identifier; the same value as `profile_id`.",
		Computed:      true,
		PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
	}
}

// profileIDAttribute is the profile_id every profile-scoped resource takes.
// As under SDKv2 it is not ForceNew: changing it writes the new profile and
// leaves the old one as it was.
func profileIDAttribute() schema.StringAttribute {
	return schema.StringAttribute{
		Description: "The profile identifier to target the resource.",
		Required:    true,
	}
}

// clientFrom unpacks the API client the provider's Configure produced. It
// returns nil without a diagnostic when the provider is not configured yet,
// which the framework does during validation.
func clientFrom(data any, diags *diag.Diagnostics) *nextdns.Client {
	if data == nil {
		return nil
	}
	client, ok := data.(*nextdns.Client)
	if !ok {
		diags.AddError("Unexpected provider data", fmt.Sprintf("expected *nextdns.Client, got %T", data))
		return nil
	}
	return client
}

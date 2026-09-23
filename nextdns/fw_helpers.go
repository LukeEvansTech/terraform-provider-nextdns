package nextdns

import (
	"context"
	"fmt"

	"github.com/LukeEvansTech/terraform-provider-nextdns/internal/nextdns"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
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

// stringIDs returns the elements of a list of strings; null gives nil.
func stringIDs(ctx context.Context, l types.List, diags *diag.Diagnostics) []string {
	if l.IsNull() || l.IsUnknown() {
		return nil
	}
	var out []string
	diags.Append(l.ElementsAs(ctx, &out, false)...)
	return out
}

// stringListFromAPI turns the ids the API returned into a list value for
// state. An empty result keeps the prior value when that was null or empty,
// so an omitted list stays null and an explicit [] stays []: the framework
// plans a change between the two, where SDKv2 treated them as the same.
func stringListFromAPI(ids []string, prior types.List) types.List {
	if len(ids) == 0 && (prior.IsNull() || (!prior.IsUnknown() && len(prior.Elements()) == 0)) {
		return prior
	}
	elems := make([]attr.Value, 0, len(ids))
	for _, id := range ids {
		elems = append(elems, types.StringValue(id))
	}
	return types.ListValueMust(types.StringType, elems)
}

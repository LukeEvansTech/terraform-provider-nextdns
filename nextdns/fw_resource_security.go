package nextdns

import (
	"context"

	"github.com/LukeEvansTech/terraform-provider-nextdns/internal/nextdns"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.ResourceWithConfigure   = &securityResource{}
	_ resource.ResourceWithImportState = &securityResource{}
)

type securityResource struct {
	client *nextdns.Client
}

type securityModel struct {
	ID                      types.String `tfsdk:"id"`
	ProfileID               types.String `tfsdk:"profile_id"`
	ThreatIntelligenceFeeds types.Bool   `tfsdk:"threat_intelligence_feeds"`
	AiThreatDetection       types.Bool   `tfsdk:"ai_threat_detection"`
	GoogleSafeBrowsing      types.Bool   `tfsdk:"google_safe_browsing"`
	CryptoJacking           types.Bool   `tfsdk:"crypto_jacking"`
	DNSRebinding            types.Bool   `tfsdk:"dns_rebinding"`
	IdnHomographs           types.Bool   `tfsdk:"idn_homographs"`
	TypoSquatting           types.Bool   `tfsdk:"typo_squatting"`
	Dga                     types.Bool   `tfsdk:"dga"`
	Nrd                     types.Bool   `tfsdk:"nrd"`
	DDNS                    types.Bool   `tfsdk:"ddns"`
	Parking                 types.Bool   `tfsdk:"parking"`
	Csam                    types.Bool   `tfsdk:"csam"`
	Tlds                    types.List   `tfsdk:"tlds"`

	FreeHostingDomains       types.Bool `tfsdk:"free_hosting_domains"`
	TunnelingEndpoints       types.Bool `tfsdk:"tunneling_endpoints"`
	DataDropServices         types.Bool `tfsdk:"data_drop_services"`
	ResidentialHosting       types.Bool `tfsdk:"residential_hosting"`
	UntrustedCertificates    types.Bool `tfsdk:"untrusted_certificates"`
	FastFluxNetworks         types.Bool `tfsdk:"fast_flux_networks"`
	DNSDataExfiltration      types.Bool `tfsdk:"dns_data_exfiltration"`
	DNSPayloadDelivery       types.Bool `tfsdk:"dns_payload_delivery"`
	DecentralizedWebGateways types.Bool `tfsdk:"decentralized_web_gateways"`
	HighRiskTlds             types.Bool `tfsdk:"high_risk_tlds"`
	NewlyActiveDomains       types.Bool `tfsdk:"newly_active_domains"`
}

// securitySwitch pairs an extended switch in the model with its API field,
// so the tri-state handling is written once.
type securitySwitch struct {
	attr *types.Bool
	api  **bool
}

func (m *securityModel) extended(s *nextdns.Security) []securitySwitch {
	return []securitySwitch{
		{&m.FreeHostingDomains, &s.FreeHostingDomains},
		{&m.TunnelingEndpoints, &s.TunnelingEndpoints},
		{&m.DataDropServices, &s.DataDropServices},
		{&m.ResidentialHosting, &s.ResidentialHosting},
		{&m.UntrustedCertificates, &s.UntrustedCertificates},
		{&m.FastFluxNetworks, &s.FastFluxNetworks},
		{&m.DNSDataExfiltration, &s.DNSDataExfiltration},
		{&m.DNSPayloadDelivery, &s.DNSPayloadDelivery},
		{&m.DecentralizedWebGateways, &s.DecentralizedWebGateways},
		{&m.HighRiskTlds, &s.HighRiskTlds},
		{&m.NewlyActiveDomains, &s.NewlyActiveDomains},
	}
}

func newSecurityResource() resource.Resource {
	return &securityResource{}
}

func (r *securityResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_security"
}

func (r *securityResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	required := func(description string) schema.BoolAttribute {
		return schema.BoolAttribute{Description: description, Required: true}
	}
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id":                        idAttribute(),
			"profile_id":                profileIDAttribute(),
			"threat_intelligence_feeds": required("Threat intelligence feeds."),
			"ai_threat_detection":       required("AI-Driven threat detection."),
			"google_safe_browsing":      required("Google safe browsing."),
			"crypto_jacking":            required("Cryptojacking protection."),
			"dns_rebinding":             required("DNS rebinding protection."),
			"idn_homographs":            required("IDN homograph attacks protection."),
			"typo_squatting":            required("Typosquatting protection."),
			"dga":                       required("Domain generation algorithms (DGAs) protection."),
			"nrd":                       required("Block newly registered domains (NRDs)."),
			"ddns":                      required("Block dynamic DNS hostnames."),
			"parking":                   required("Block parked domains."),
			"csam":                      required("Block child sexual abuse material."),
			"tlds": schema.ListAttribute{
				Description: "Block top-level domains (TLDs).",
				ElementType: types.StringType,
				Optional:    true,
			},

			// Extended switches (v0.3.0). Optional + Computed: omitting one keeps
			// the live value and records it in state; an explicit true/false is
			// written; one the API stops returning keeps its last known value.
			"free_hosting_domains":       extendedSwitch("Block domains on free hosting providers (API: `freeHostingDomains`)."),
			"tunneling_endpoints":        extendedSwitch("Block tunneling and DNS-over-anything endpoints (API: `tunnelingEndpoints`)."),
			"data_drop_services":         extendedSwitch("Block data drop / paste services (API: `dataDropServices`)."),
			"residential_hosting":        extendedSwitch("Block domains served from residential IP space (API: `residentialHosting`)."),
			"untrusted_certificates":     extendedSwitch("Block domains presenting untrusted certificates (API: `untrustedCertificates`)."),
			"fast_flux_networks":         extendedSwitch("Block fast-flux networks (API: `fastFluxNetworks`). Hidden in the dashboard for some profiles; the API returns it, but enforcement is unverified."),
			"dns_data_exfiltration":      extendedSwitch("Block DNS data exfiltration (API: `dnsDataExfiltration`). Hidden in the dashboard for some profiles; the API returns it, but enforcement is unverified."),
			"dns_payload_delivery":       extendedSwitch("Block DNS payload delivery (API: `dnsPayloadDelivery`)."),
			"decentralized_web_gateways": extendedSwitch("Block decentralized web (IPFS, ENS) gateways (API: `decentralizedWebGateways`)."),
			"high_risk_tlds":             extendedSwitch("Block high-risk TLDs (API: `highRiskTlds`)."),
			"newly_active_domains":       extendedSwitch("Block newly active domains (API: `newlyActiveDomains`). Returned by the API since September 2026."),
		},
	}
}

// extendedSwitch is the schema for a tri-state switch: Optional + Computed,
// with the prior state carried into the plan when the attribute is omitted,
// so omission never produces a diff.
func extendedSwitch(description string) schema.BoolAttribute {
	return schema.BoolAttribute{
		Description:   description,
		Optional:      true,
		Computed:      true,
		PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
	}
}

func (r *securityResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFrom(req.ProviderData, &resp.Diagnostics)
}

func (r *securityResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	r.write(ctx, req.Config, req.Plan, &resp.State, &resp.Diagnostics)
}

func (r *securityResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	r.write(ctx, req.Config, req.Plan, &resp.State, &resp.Diagnostics)
}

// write PUTs the TLD list and PATCHes the switches. An extended switch is
// sent only when the configuration sets it, which is why the config is read
// and not just the plan: in the plan an omitted switch already carries the
// prior state's value.
func (r *securityResource) write(ctx context.Context, config tfsdk.Config, plan tfsdk.Plan, state *tfsdk.State, diags *diag.Diagnostics) {
	var m, cfg securityModel
	diags.Append(plan.Get(ctx, &m)...)
	diags.Append(config.Get(ctx, &cfg)...)
	if diags.HasError() {
		return
	}
	profileID := m.ProfileID.ValueString()

	sec := &nextdns.Security{
		ThreatIntelligenceFeeds: m.ThreatIntelligenceFeeds.ValueBool(),
		AiThreatDetection:       m.AiThreatDetection.ValueBool(),
		GoogleSafeBrowsing:      m.GoogleSafeBrowsing.ValueBool(),
		Cryptojacking:           m.CryptoJacking.ValueBool(),
		DNSRebinding:            m.DNSRebinding.ValueBool(),
		IdnHomographs:           m.IdnHomographs.ValueBool(),
		Typosquatting:           m.TypoSquatting.ValueBool(),
		Dga:                     m.Dga.ValueBool(),
		Nrd:                     m.Nrd.ValueBool(),
		DDNS:                    m.DDNS.ValueBool(),
		Parking:                 m.Parking.ValueBool(),
		Csam:                    m.Csam.ValueBool(),
		Tlds:                    []*nextdns.SecurityTlds{},
	}
	for _, id := range stringIDs(ctx, m.Tlds, diags) {
		sec.Tlds = append(sec.Tlds, &nextdns.SecurityTlds{ID: id})
	}
	for _, sw := range cfg.extended(sec) {
		*sw.api = boolPointer(*sw.attr)
	}
	if diags.HasError() {
		return
	}

	if err := r.client.SecurityTlds.Create(ctx, &nextdns.CreateSecurityTldsRequest{ProfileID: profileID, SecurityTlds: sec.Tlds}); err != nil {
		diags.AddError("Error writing security TLDs", err.Error())
		return
	}
	if err := r.client.Security.Update(ctx, &nextdns.UpdateSecurityRequest{ProfileID: profileID, Security: sec}); err != nil {
		diags.AddError("Error writing security settings", err.Error())
		return
	}

	// Switches still unknown were omitted on create: record what the API
	// holds, or null when it does not report them.
	if anyUnknown(m.extended(&nextdns.Security{})) {
		live, err := r.client.Security.Get(ctx, &nextdns.GetSecurityRequest{ProfileID: profileID})
		if err != nil {
			diags.AddError("Error reading back security settings", err.Error())
			return
		}
		for _, sw := range m.extended(live) {
			if sw.attr.IsUnknown() {
				*sw.attr = types.BoolPointerValue(*sw.api)
			}
		}
	}

	m.ID = m.ProfileID
	diags.Append(state.Set(ctx, &m)...)
}

func (r *securityResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m securityModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}

	sec, err := r.client.Security.Get(ctx, &nextdns.GetSecurityRequest{ProfileID: m.ProfileID.ValueString()})
	if err != nil {
		resp.Diagnostics.AddError("Error getting security settings", err.Error())
		return
	}

	m.ID = m.ProfileID
	m.ThreatIntelligenceFeeds = types.BoolValue(sec.ThreatIntelligenceFeeds)
	m.AiThreatDetection = types.BoolValue(sec.AiThreatDetection)
	m.GoogleSafeBrowsing = types.BoolValue(sec.GoogleSafeBrowsing)
	m.CryptoJacking = types.BoolValue(sec.Cryptojacking)
	m.DNSRebinding = types.BoolValue(sec.DNSRebinding)
	m.IdnHomographs = types.BoolValue(sec.IdnHomographs)
	m.TypoSquatting = types.BoolValue(sec.Typosquatting)
	m.Dga = types.BoolValue(sec.Dga)
	m.Nrd = types.BoolValue(sec.Nrd)
	m.DDNS = types.BoolValue(sec.DDNS)
	m.Parking = types.BoolValue(sec.Parking)
	m.Csam = types.BoolValue(sec.Csam)

	tlds := make([]string, 0, len(sec.Tlds))
	for _, t := range sec.Tlds {
		tlds = append(tlds, t.ID)
	}
	m.Tlds = stringListFromAPI(tlds, m.Tlds)

	// A switch the API does not return keeps its last known value.
	for _, sw := range m.extended(sec) {
		if *sw.api != nil {
			*sw.attr = types.BoolValue(**sw.api)
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

// Delete clears the TLD list and PATCHes an empty security object, which
// sets the twelve original switches to false and leaves the extended ones
// as they are.
func (r *securityResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m securityModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	profileID := m.ProfileID.ValueString()

	if err := r.client.SecurityTlds.Create(ctx, &nextdns.CreateSecurityTldsRequest{
		ProfileID: profileID, SecurityTlds: []*nextdns.SecurityTlds{},
	}); err != nil {
		resp.Diagnostics.AddError("Error deleting security TLDs", err.Error())
		return
	}
	if err := r.client.Security.Update(ctx, &nextdns.UpdateSecurityRequest{ProfileID: profileID, Security: &nextdns.Security{}}); err != nil {
		resp.Diagnostics.AddError("Error deleting security settings", err.Error())
	}
}

func (r *securityResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	importProfileID(ctx, req, resp)
}

// boolPointer returns nil for a null or unknown value, else a pointer.
func boolPointer(v types.Bool) *bool {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	b := v.ValueBool()
	return &b
}

func anyUnknown(switches []securitySwitch) bool {
	for _, sw := range switches {
		if sw.attr.IsUnknown() {
			return true
		}
	}
	return false
}

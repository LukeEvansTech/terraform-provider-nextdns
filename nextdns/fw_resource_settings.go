package nextdns

import (
	"context"
	"time"

	"github.com/LukeEvansTech/terraform-provider-nextdns/internal/nextdns"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.ResourceWithConfigure   = &settingsResource{}
	_ resource.ResourceWithImportState = &settingsResource{}
)

type settingsResource struct {
	client *nextdns.Client
}

type settingsModel struct {
	ID          types.String               `tfsdk:"id"`
	ProfileID   types.String               `tfsdk:"profile_id"`
	Logs        []settingsLogsModel        `tfsdk:"logs"`
	BlockPage   []settingsBlockPageModel   `tfsdk:"block_page"`
	Performance []settingsPerformanceModel `tfsdk:"performance"`
	Web3        types.Bool                 `tfsdk:"web3"`
	Bav         types.Bool                 `tfsdk:"bypass_age_verification"`
}

type settingsLogsModel struct {
	Enabled   types.Bool                 `tfsdk:"enabled"`
	Privacy   []settingsLogsPrivacyModel `tfsdk:"privacy"`
	Retention types.String               `tfsdk:"retention"`
	Location  types.String               `tfsdk:"location"`
}

type settingsLogsPrivacyModel struct {
	LogClientsIP types.Bool `tfsdk:"log_clients_ip"`
	LogDomains   types.Bool `tfsdk:"log_domains"`
}

type settingsBlockPageModel struct {
	Enabled types.Bool `tfsdk:"enabled"`
}

type settingsPerformanceModel struct {
	Ecs             types.Bool `tfsdk:"ecs"`
	CacheBoost      types.Bool `tfsdk:"cache_boost"`
	CnameFlattening types.Bool `tfsdk:"cname_flattening"`
}

// retentionPeriods maps the retention labels the schema accepts to the
// seconds the API stores. A month is 30 days and a year 365, as the API
// counts them.
var retentionPeriods = []struct {
	label string
	d     time.Duration
}{
	{"1 hour", time.Hour},
	{"6 hours", 6 * time.Hour},
	{"1 day", 24 * time.Hour},
	{"1 week", 7 * 24 * time.Hour},
	{"1 month", 30 * 24 * time.Hour},
	{"3 months", 90 * 24 * time.Hour},
	{"6 months", 180 * 24 * time.Hour},
	{"1 year", 365 * 24 * time.Hour},
	{"2 years", 2 * 365 * 24 * time.Hour},
}

func newSettingsResource() resource.Resource {
	return &settingsResource{}
}

func (r *settingsResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_settings"
}

func (r *settingsResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	labels := make([]string, 0, len(retentionPeriods))
	for _, p := range retentionPeriods {
		labels = append(labels, p.label)
	}
	boolRequired := func(description string) schema.BoolAttribute {
		return schema.BoolAttribute{Description: description, Required: true}
	}
	requiredBlock := []validator.List{listvalidator.IsRequired()}

	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id":         idAttribute(),
			"profile_id": profileIDAttribute(),
			"web3":       boolRequired("Web3."),
			"bypass_age_verification": schema.BoolAttribute{
				Description:   "Bypass Age Verification (API: `bav`). Optional + Computed: omitting it keeps the live value and records it in state; an explicit true/false is written.",
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
		},
		Blocks: map[string]schema.Block{
			"logs": schema.ListNestedBlock{
				Description: "Logs.",
				Validators:  requiredBlock,
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"enabled": boolRequired("Enable logs."),
						"retention": schema.StringAttribute{
							Description: "Retention period for logs.",
							Required:    true,
							Validators:  []validator.String{stringvalidator.OneOf(labels...)},
						},
						"location": schema.StringAttribute{
							Description: "Location of the logs.",
							Required:    true,
						},
					},
					Blocks: map[string]schema.Block{
						"privacy": schema.ListNestedBlock{
							Validators: requiredBlock,
							NestedObject: schema.NestedBlockObject{
								Attributes: map[string]schema.Attribute{
									"log_clients_ip": boolRequired("Log clients IP."),
									"log_domains":    boolRequired("Log domains."),
								},
							},
						},
					},
				},
			},
			"block_page": schema.ListNestedBlock{
				Description: "Block Page.",
				Validators:  requiredBlock,
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"enabled": boolRequired("Enable block page."),
					},
				},
			},
			"performance": schema.ListNestedBlock{
				Description: "Performance.",
				Validators:  requiredBlock,
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"ecs":              boolRequired("Anonymized EDNS Client Subnet."),
						"cache_boost":      boolRequired("Cache Boost."),
						"cname_flattening": boolRequired("CNAME Flattening."),
					},
				},
			},
		},
	}
}

func (r *settingsResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFrom(req.ProviderData, &resp.Diagnostics)
}

func (r *settingsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	r.write(ctx, req.Config, req.Plan, &resp.State, &resp.Diagnostics)
}

func (r *settingsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	r.write(ctx, req.Config, req.Plan, &resp.State, &resp.Diagnostics)
}

// write PATCHes logs, block page and performance at their own paths, then
// the settings object, as the SDKv2 resource did. bav is sent only when the
// configuration sets it. Only the first element of each block is used.
func (r *settingsResource) write(ctx context.Context, config tfsdk.Config, plan tfsdk.Plan, state *tfsdk.State, diags *diag.Diagnostics) {
	var m, cfg settingsModel
	diags.Append(plan.Get(ctx, &m)...)
	diags.Append(config.Get(ctx, &cfg)...)
	if diags.HasError() {
		return
	}
	if len(m.Logs) == 0 || len(m.Logs[0].Privacy) == 0 || len(m.BlockPage) == 0 || len(m.Performance) == 0 {
		diags.AddError("Incomplete settings", "logs (with privacy), block_page and performance are all required.")
		return
	}
	profileID := m.ProfileID.ValueString()
	logs, privacy := m.Logs[0], m.Logs[0].Privacy[0]

	settings := &nextdns.Settings{
		Logs: &nextdns.SettingsLogs{
			Enabled: logs.Enabled.ValueBool(),
			Drop: &nextdns.SettingsLogsDrop{
				IP:     !privacy.LogClientsIP.ValueBool(),
				Domain: !privacy.LogDomains.ValueBool(),
			},
			Retention: retentionSeconds(logs.Retention.ValueString()),
			Location:  logs.Location.ValueString(),
		},
		BlockPage: &nextdns.SettingsBlockPage{Enabled: m.BlockPage[0].Enabled.ValueBool()},
		Performance: &nextdns.SettingsPerformance{
			Ecs:             m.Performance[0].Ecs.ValueBool(),
			CacheBoost:      m.Performance[0].CacheBoost.ValueBool(),
			CnameFlattening: m.Performance[0].CnameFlattening.ValueBool(),
		},
		Web3: m.Web3.ValueBool(),
		Bav:  boolPointer(cfg.Bav),
	}

	if err := r.client.SettingsLogs.Update(ctx, &nextdns.UpdateSettingsLogsRequest{ProfileID: profileID, SettingsLogs: settings.Logs}); err != nil {
		diags.AddError("Error writing logs settings", err.Error())
		return
	}
	if err := r.client.SettingsBlockPage.Update(ctx, &nextdns.UpdateSettingsBlockPageRequest{ProfileID: profileID, SettingsBlockPage: settings.BlockPage}); err != nil {
		diags.AddError("Error writing block page settings", err.Error())
		return
	}
	if err := r.client.SettingsPerformance.Update(ctx, &nextdns.UpdateSettingsPerformanceRequest{ProfileID: profileID, SettingsPerformance: settings.Performance}); err != nil {
		diags.AddError("Error writing performance settings", err.Error())
		return
	}
	if err := r.client.Settings.Update(ctx, &nextdns.UpdateSettingsRequest{ProfileID: profileID, Settings: settings}); err != nil {
		diags.AddError("Error writing settings", err.Error())
		return
	}

	if m.Bav.IsUnknown() {
		live, err := r.client.Settings.Get(ctx, &nextdns.GetSettingsRequest{ProfileID: profileID})
		if err != nil {
			diags.AddError("Error reading back settings", err.Error())
			return
		}
		m.Bav = types.BoolPointerValue(live.Bav)
	}

	m.ID = m.ProfileID
	diags.Append(state.Set(ctx, &m)...)
}

func (r *settingsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m settingsModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}

	s, err := r.client.Settings.Get(ctx, &nextdns.GetSettingsRequest{ProfileID: m.ProfileID.ValueString()})
	if err != nil {
		resp.Diagnostics.AddError("Error getting settings", err.Error())
		return
	}
	logs := s.Logs
	if logs == nil {
		logs = &nextdns.SettingsLogs{}
	}
	drop := logs.Drop
	if drop == nil {
		drop = &nextdns.SettingsLogsDrop{}
	}
	blockPage := s.BlockPage
	if blockPage == nil {
		blockPage = &nextdns.SettingsBlockPage{}
	}
	perf := s.Performance
	if perf == nil {
		perf = &nextdns.SettingsPerformance{}
	}

	m.ID = m.ProfileID
	m.Logs = []settingsLogsModel{{
		Enabled: types.BoolValue(logs.Enabled),
		Privacy: []settingsLogsPrivacyModel{{
			LogClientsIP: types.BoolValue(!drop.IP),
			LogDomains:   types.BoolValue(!drop.Domain),
		}},
		Retention: types.StringValue(retentionLabel(logs.Retention)),
		Location:  types.StringValue(logs.Location),
	}}
	m.BlockPage = []settingsBlockPageModel{{Enabled: types.BoolValue(blockPage.Enabled)}}
	m.Performance = []settingsPerformanceModel{{
		Ecs:             types.BoolValue(perf.Ecs),
		CacheBoost:      types.BoolValue(perf.CacheBoost),
		CnameFlattening: types.BoolValue(perf.CnameFlattening),
	}}
	m.Web3 = types.BoolValue(s.Web3)
	// bav the API does not return keeps its last known value.
	if s.Bav != nil {
		m.Bav = types.BoolValue(*s.Bav)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

// Delete PATCHes zero values everywhere, as the SDKv2 resource did.
func (r *settingsResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m settingsModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	profileID := m.ProfileID.ValueString()

	if err := r.client.SettingsLogs.Update(ctx, &nextdns.UpdateSettingsLogsRequest{ProfileID: profileID, SettingsLogs: &nextdns.SettingsLogs{}}); err != nil {
		resp.Diagnostics.AddError("Error deleting logs settings", err.Error())
		return
	}
	if err := r.client.SettingsBlockPage.Update(ctx, &nextdns.UpdateSettingsBlockPageRequest{ProfileID: profileID, SettingsBlockPage: &nextdns.SettingsBlockPage{}}); err != nil {
		resp.Diagnostics.AddError("Error deleting block page settings", err.Error())
		return
	}
	if err := r.client.SettingsPerformance.Update(ctx, &nextdns.UpdateSettingsPerformanceRequest{ProfileID: profileID, SettingsPerformance: &nextdns.SettingsPerformance{}}); err != nil {
		resp.Diagnostics.AddError("Error deleting performance settings", err.Error())
		return
	}
	if err := r.client.Settings.Update(ctx, &nextdns.UpdateSettingsRequest{ProfileID: profileID, Settings: &nextdns.Settings{}}); err != nil {
		resp.Diagnostics.AddError("Error deleting settings", err.Error())
	}
}

func (r *settingsResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	importProfileID(ctx, req, resp)
}

// retentionSeconds converts a retention label to seconds; unknown is 0.
func retentionSeconds(label string) int {
	for _, p := range retentionPeriods {
		if p.label == label {
			return int(p.d.Seconds())
		}
	}
	return 0
}

// retentionLabel converts seconds to a retention label; unknown is "".
func retentionLabel(seconds int) string {
	for _, p := range retentionPeriods {
		if int(p.d.Seconds()) == seconds {
			return p.label
		}
	}
	return ""
}

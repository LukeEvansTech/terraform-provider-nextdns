package nextdns

import (
	"context"
	"regexp"

	"github.com/LukeEvansTech/terraform-provider-nextdns/internal/nextdns"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.ResourceWithConfigure   = &parentalControlResource{}
	_ resource.ResourceWithImportState = &parentalControlResource{}
)

type parentalControlResource struct {
	client *nextdns.Client
}

type parentalControlModel struct {
	ID                    types.String              `tfsdk:"id"`
	ProfileID             types.String              `tfsdk:"profile_id"`
	BlockBypass           types.Bool                `tfsdk:"block_bypass"`
	SafeSearch            types.Bool                `tfsdk:"safe_search"`
	YoutubeRestrictedMode types.Bool                `tfsdk:"youtube_restricted_mode"`
	Category              []parentalEntryModel      `tfsdk:"category"`
	Service               []parentalEntryModel      `tfsdk:"service"`
	Recreation            []parentalRecreationModel `tfsdk:"recreation"`
}

type parentalEntryModel struct {
	ID         types.String `tfsdk:"id"`
	Active     types.Bool   `tfsdk:"active"`
	Recreation types.Bool   `tfsdk:"recreation"`
}

type parentalRecreationModel struct {
	Timezone  types.String            `tfsdk:"timezone"`
	Monday    []parentalIntervalModel `tfsdk:"monday"`
	Tuesday   []parentalIntervalModel `tfsdk:"tuesday"`
	Wednesday []parentalIntervalModel `tfsdk:"wednesday"`
	Thursday  []parentalIntervalModel `tfsdk:"thursday"`
	Friday    []parentalIntervalModel `tfsdk:"friday"`
	Saturday  []parentalIntervalModel `tfsdk:"saturday"`
	Sunday    []parentalIntervalModel `tfsdk:"sunday"`
}

type parentalIntervalModel struct {
	Start types.String `tfsdk:"start"`
	End   types.String `tfsdk:"end"`
}

// days pairs each weekday block with its API field.
func (m *parentalRecreationModel) days(t *nextdns.ParentalControlRecreationTimes) []struct {
	block *[]parentalIntervalModel
	api   **nextdns.ParentalControlRecreationInterval
} {
	type day = struct {
		block *[]parentalIntervalModel
		api   **nextdns.ParentalControlRecreationInterval
	}
	return []day{
		{&m.Monday, &t.Monday},
		{&m.Tuesday, &t.Tuesday},
		{&m.Wednesday, &t.Wednesday},
		{&m.Thursday, &t.Thursday},
		{&m.Friday, &t.Friday},
		{&m.Saturday, &t.Saturday},
		{&m.Sunday, &t.Sunday},
	}
}

var recreationTimeFormat = regexp.MustCompile(`^([0-1][0-9]|2[0-3]):[0-5][0-9]:00$`)

func newParentalControlResource() resource.Resource {
	return &parentalControlResource{}
}

func (r *parentalControlResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_parental_control"
}

func (r *parentalControlResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	entry := schema.NestedBlockObject{
		Attributes: map[string]schema.Attribute{
			"id":         schema.StringAttribute{Required: true},
			"active":     schema.BoolAttribute{Required: true},
			"recreation": schema.BoolAttribute{Required: true},
		},
	}
	timeOfDay := schema.StringAttribute{
		Required:   true,
		Validators: []validator.String{stringvalidator.RegexMatches(recreationTimeFormat, "Must be in HH:MM:00 format")},
	}
	interval := schema.ListNestedBlock{
		NestedObject: schema.NestedBlockObject{
			Attributes: map[string]schema.Attribute{"start": timeOfDay, "end": timeOfDay},
		},
	}

	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id":         idAttribute(),
			"profile_id": profileIDAttribute(),
			"block_bypass": schema.BoolAttribute{
				Description: "Block bypass methods.",
				Required:    true,
			},
			"safe_search": schema.BoolAttribute{
				Description: "Safe search.",
				Required:    true,
			},
			"youtube_restricted_mode": schema.BoolAttribute{
				Description: "YouTube restricted mode.",
				Required:    true,
			},
		},
		Blocks: map[string]schema.Block{
			"category": schema.SetNestedBlock{
				Description:  "Restrict access to specific categories of websites and apps.",
				NestedObject: entry,
			},
			"service": schema.SetNestedBlock{
				Description:  "Restrict access to specific websites, apps and games.",
				NestedObject: entry,
			},
			"recreation": schema.ListNestedBlock{
				Description: "Period for each day of the week during which some of the websites, apps, games or categories will not be blocked.",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"timezone": schema.StringAttribute{Required: true},
					},
					Blocks: map[string]schema.Block{
						"monday":    interval,
						"tuesday":   interval,
						"wednesday": interval,
						"thursday":  interval,
						"friday":    interval,
						"saturday":  interval,
						"sunday":    interval,
					},
				},
			},
		},
	}
}

func (r *parentalControlResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFrom(req.ProviderData, &resp.Diagnostics)
}

func (r *parentalControlResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	r.write(ctx, req.Plan, &resp.State, &resp.Diagnostics)
}

func (r *parentalControlResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	r.write(ctx, req.Plan, &resp.State, &resp.Diagnostics)
}

// write PUTs services and categories to their own endpoints, then PATCHes
// the switches and recreation. The PATCH must not carry services or
// categories (the cozy-corner fix). With no recreation block an empty
// recreation object is sent, as under SDKv2.
func (r *parentalControlResource) write(ctx context.Context, plan tfsdk.Plan, state *tfsdk.State, diags *diag.Diagnostics) {
	var m parentalControlModel
	diags.Append(plan.Get(ctx, &m)...)
	if diags.HasError() {
		return
	}
	profileID := m.ProfileID.ValueString()

	services := make([]*nextdns.ParentalControlServices, 0, len(m.Service))
	for _, e := range m.Service {
		services = append(services, &nextdns.ParentalControlServices{
			ID: e.ID.ValueString(), Active: e.Active.ValueBool(), Recreation: e.Recreation.ValueBool(),
		})
	}
	categories := make([]*nextdns.ParentalControlCategories, 0, len(m.Category))
	for _, e := range m.Category {
		categories = append(categories, &nextdns.ParentalControlCategories{
			ID: e.ID.ValueString(), Active: e.Active.ValueBool(), Recreation: e.Recreation.ValueBool(),
		})
	}

	recreation := &nextdns.ParentalControlRecreation{}
	if len(m.Recreation) > 0 {
		rec := m.Recreation[0]
		times := &nextdns.ParentalControlRecreationTimes{}
		for _, d := range rec.days(times) {
			if len(*d.block) > 0 {
				*d.api = &nextdns.ParentalControlRecreationInterval{
					Start: (*d.block)[0].Start.ValueString(),
					End:   (*d.block)[0].End.ValueString(),
				}
			}
		}
		recreation = &nextdns.ParentalControlRecreation{Times: times, Timezone: rec.Timezone.ValueString()}
	}

	if err := r.client.ParentalControlServices.Create(ctx, &nextdns.CreateParentalControlServicesRequest{
		ProfileID: profileID, ParentalControlServices: services,
	}); err != nil {
		diags.AddError("Error writing parental control services", err.Error())
		return
	}
	if err := r.client.ParentalControlCategories.Create(ctx, &nextdns.CreateParentalControlCategoriesRequest{
		ProfileID: profileID, ParentalControlCategories: categories,
	}); err != nil {
		diags.AddError("Error writing parental control categories", err.Error())
		return
	}
	if err := r.client.ParentalControl.Update(ctx, &nextdns.UpdateParentalControlRequest{
		ProfileID: profileID,
		ParentalControl: &nextdns.ParentalControl{
			SafeSearch:            m.SafeSearch.ValueBool(),
			YoutubeRestrictedMode: m.YoutubeRestrictedMode.ValueBool(),
			BlockBypass:           m.BlockBypass.ValueBool(),
			Recreation:            recreation,
		},
	}); err != nil {
		diags.AddError("Error writing parental control settings", err.Error())
		return
	}

	m.ID = m.ProfileID
	diags.Append(state.Set(ctx, &m)...)
}

func (r *parentalControlResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m parentalControlModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}

	pc, err := r.client.ParentalControl.Get(ctx, &nextdns.GetParentalControlRequest{ProfileID: m.ProfileID.ValueString()})
	if err != nil {
		resp.Diagnostics.AddError("Error getting parental control settings", err.Error())
		return
	}

	m.ID = m.ProfileID
	m.BlockBypass = types.BoolValue(pc.BlockBypass)
	m.SafeSearch = types.BoolValue(pc.SafeSearch)
	m.YoutubeRestrictedMode = types.BoolValue(pc.YoutubeRestrictedMode)

	m.Service = make([]parentalEntryModel, 0, len(pc.Services))
	for _, s := range pc.Services {
		m.Service = append(m.Service, parentalEntryModel{
			ID: types.StringValue(s.ID), Active: types.BoolValue(s.Active), Recreation: types.BoolValue(s.Recreation),
		})
	}
	m.Category = make([]parentalEntryModel, 0, len(pc.Categories))
	for _, c := range pc.Categories {
		m.Category = append(m.Category, parentalEntryModel{
			ID: types.StringValue(c.ID), Active: types.BoolValue(c.Active), Recreation: types.BoolValue(c.Recreation),
		})
	}

	// The API always returns a recreation object; one it does not return
	// leaves state as it was, as under SDKv2.
	if pc.Recreation != nil {
		rec := parentalRecreationModel{Timezone: types.StringValue(pc.Recreation.Timezone)}
		times := pc.Recreation.Times
		if times == nil {
			times = &nextdns.ParentalControlRecreationTimes{}
		}
		for _, d := range rec.days(times) {
			*d.block = []parentalIntervalModel{}
			if *d.api != nil {
				*d.block = []parentalIntervalModel{{
					Start: types.StringValue((*d.api).Start),
					End:   types.StringValue((*d.api).End),
				}}
			}
		}
		m.Recreation = []parentalRecreationModel{rec}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

// Delete clears services and categories and PATCHes an empty object.
func (r *parentalControlResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m parentalControlModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	profileID := m.ProfileID.ValueString()

	if err := r.client.ParentalControlServices.Create(ctx, &nextdns.CreateParentalControlServicesRequest{
		ProfileID: profileID, ParentalControlServices: []*nextdns.ParentalControlServices{},
	}); err != nil {
		resp.Diagnostics.AddError("Error deleting parental control services", err.Error())
		return
	}
	if err := r.client.ParentalControlCategories.Create(ctx, &nextdns.CreateParentalControlCategoriesRequest{
		ProfileID: profileID, ParentalControlCategories: []*nextdns.ParentalControlCategories{},
	}); err != nil {
		resp.Diagnostics.AddError("Error deleting parental control categories", err.Error())
		return
	}
	if err := r.client.ParentalControl.Update(ctx, &nextdns.UpdateParentalControlRequest{
		ProfileID: profileID, ParentalControl: &nextdns.ParentalControl{},
	}); err != nil {
		resp.Diagnostics.AddError("Error deleting parental control settings", err.Error())
	}
}

func (r *parentalControlResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	importProfileID(ctx, req, resp)
}

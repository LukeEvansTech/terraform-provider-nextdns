package nextdns

import (
	"context"

	"github.com/LukeEvansTech/terraform-provider-nextdns/internal/nextdns"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ datasource.DataSourceWithConfigure = &setupEndpointDataSource{}
	_ datasource.DataSourceWithConfigure = &setupLinkedIPDataSource{}
)

// emptyStringList makes an empty API list an empty list in state rather
// than null, so length() and for expressions keep working on it.
var emptyStringList = types.ListValueMust(types.StringType, []attr.Value{})

// dataSourceID is the id both data sources carried implicitly under SDKv2.
func dataSourceID() schema.StringAttribute {
	return schema.StringAttribute{
		Description: "The profile identifier; the same value as `profile_id`.",
		Computed:    true,
	}
}

func dataSourceProfileID() schema.StringAttribute {
	return schema.StringAttribute{
		Description: "The profile identifier to target the resource.",
		Required:    true,
	}
}

type setupEndpointDataSource struct {
	client *nextdns.Client
}

type setupEndpointModel struct {
	ID        types.String `tfsdk:"id"`
	ProfileID types.String `tfsdk:"profile_id"`
	DoH       types.String `tfsdk:"doh"`
	DoT       types.String `tfsdk:"dot"`
	IPv4      types.List   `tfsdk:"ipv4"`
	IPv6      types.List   `tfsdk:"ipv6"`
	DNSCrypt  types.String `tfsdk:"dnscrypt"`
}

func newSetupEndpointDataSource() datasource.DataSource {
	return &setupEndpointDataSource{}
}

func (d *setupEndpointDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_setup_endpoint"
}

func (d *setupEndpointDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id":         dataSourceID(),
			"profile_id": dataSourceProfileID(),
			"doh": schema.StringAttribute{
				Description: "The DNS over HTTPS address the profile is reachable at.",
				Computed:    true,
			},
			"dot": schema.StringAttribute{
				Description: "The DNS over TLS address the profile is reachable at.",
				Computed:    true,
			},
			"ipv4": schema.ListAttribute{
				Description: "The IPv4 addresses the profile is reachable at.",
				ElementType: types.StringType,
				Computed:    true,
			},
			"ipv6": schema.ListAttribute{
				Description: "The IPv6 addresses the profile is reachable at.",
				ElementType: types.StringType,
				Computed:    true,
			},
			"dnscrypt": schema.StringAttribute{
				Description: "The DNS Stamps from the profile.",
				Computed:    true,
			},
		},
	}
}

func (d *setupEndpointDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFrom(req.ProviderData, &resp.Diagnostics)
}

func (d *setupEndpointDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var m setupEndpointModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	profileID := m.ProfileID.ValueString()

	setup, err := d.client.Setup.Get(ctx, &nextdns.GetSetupRequest{ProfileID: profileID})
	if err != nil {
		resp.Diagnostics.AddError("Error getting setup endpoint settings", err.Error())
		return
	}

	m.ID = m.ProfileID
	m.DoH = types.StringValue(DNSOverHTTPSAddress(profileID))
	m.DoT = types.StringValue(DNSOverTLSAddress(profileID))
	m.IPv4 = stringListFromAPI(setup.Ipv4, emptyStringList)
	m.IPv6 = stringListFromAPI(setup.Ipv6, emptyStringList)
	m.DNSCrypt = types.StringValue(setup.Dnscrypt)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

type setupLinkedIPDataSource struct {
	client *nextdns.Client
}

type setupLinkedIPModel struct {
	ID          types.String `tfsdk:"id"`
	ProfileID   types.String `tfsdk:"profile_id"`
	Servers     types.List   `tfsdk:"servers"`
	IP          types.String `tfsdk:"ip"`
	DDNS        types.String `tfsdk:"ddns"`
	UpdateToken types.String `tfsdk:"update_token"`
}

func newSetupLinkedIPDataSource() datasource.DataSource {
	return &setupLinkedIPDataSource{}
}

func (d *setupLinkedIPDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_setup_linkedip"
}

func (d *setupLinkedIPDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id":         dataSourceID(),
			"profile_id": dataSourceProfileID(),
			"servers": schema.ListAttribute{
				Description: "The DNS servers available for the profile.",
				ElementType: types.StringType,
				Computed:    true,
			},
			"ip": schema.StringAttribute{
				Description: "The IP linked to the profile.",
				Computed:    true,
			},
			"ddns": schema.StringAttribute{
				Description: "The DDNS configuration for the linked IP.",
				Computed:    true,
			},
			"update_token": schema.StringAttribute{
				Description: "The update token to use to update the linked IP.",
				Computed:    true,
			},
		},
	}
}

func (d *setupLinkedIPDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFrom(req.ProviderData, &resp.Diagnostics)
}

func (d *setupLinkedIPDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var m setupLinkedIPModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}

	setup, err := d.client.SetupLinkedIP.Get(ctx, &nextdns.GetSetupLinkedIPRequest{ProfileID: m.ProfileID.ValueString()})
	if err != nil {
		resp.Diagnostics.AddError("Error getting setup linked IP settings", err.Error())
		return
	}

	m.ID = m.ProfileID
	m.Servers = stringListFromAPI(setup.Servers, emptyStringList)
	m.IP = types.StringValue(setup.IP)
	m.DDNS = types.StringValue(setup.Ddns)
	m.UpdateToken = types.StringValue(setup.UpdateToken)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

package nextdns

import (
	"context"
	"os"

	"github.com/LukeEvansTech/terraform-provider-nextdns/internal/nextdns"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// frameworkProvider is the Plugin Framework half of the provider. Its schema
// and configuration behaviour must match the SDKv2 Provider() exactly while
// both are muxed.
type frameworkProvider struct{}

type frameworkProviderModel struct {
	APIKey  types.String `tfsdk:"api_key"`
	BaseURL types.String `tfsdk:"base_url"`
}

// NewFrameworkProvider returns the Plugin Framework provider.
func NewFrameworkProvider() provider.Provider {
	return &frameworkProvider{}
}

func (p *frameworkProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "nextdns"
}

func (p *frameworkProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"api_key": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "NextDNS API Key. Falls back to the `NEXTDNS_API_KEY` environment variable.",
			},
			"base_url": schema.StringAttribute{
				Optional:    true,
				Description: "Base URL of the NextDNS API. Defaults to `https://api.nextdns.io/`; falls back to the `NEXTDNS_BASE_URL` environment variable. Only useful for testing against a stub.",
			},
		},
	}
}

func (p *frameworkProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var cfg frameworkProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiKey := os.Getenv("NEXTDNS_API_KEY")
	if v := cfg.APIKey.ValueString(); v != "" {
		apiKey = v
	}
	if apiKey == "" {
		resp.Diagnostics.AddError(
			"Missing NextDNS API key",
			"NextDNS API key must be provided in the provider block or NEXTDNS_API_KEY environment variable.",
		)
		return
	}

	opts := []nextdns.ClientOption{nextdns.WithAPIKey(apiKey)}
	baseURL := os.Getenv("NEXTDNS_BASE_URL")
	if v := cfg.BaseURL.ValueString(); v != "" {
		baseURL = v
	}
	if baseURL != "" {
		opts = append(opts, nextdns.WithBaseURL(baseURL))
	}

	client, err := nextdns.New(opts...)
	if err != nil {
		resp.Diagnostics.AddError("Unable to create NextDNS client", err.Error())
		return
	}

	resp.ResourceData = client
	resp.DataSourceData = client
}

func (p *frameworkProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		newAllowlistResource,
		newDenylistResource,
		newParentalControlResource,
		newPrivacyResource,
		newProfileResource,
		newRewriteResource,
		newSecurityResource,
		newSettingsResource,
	}
}

func (p *frameworkProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return nil
}

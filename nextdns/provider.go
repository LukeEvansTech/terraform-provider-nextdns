package nextdns

import (
	"context"
	"os"

	"github.com/LukeEvansTech/terraform-provider-nextdns/internal/nextdns"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func Provider() *schema.Provider {
	return &schema.Provider{
		Schema: map[string]*schema.Schema{
			"api_key": {
				Type:        schema.TypeString,
				Optional:    true,
				Sensitive:   true,
				Description: "NextDNS API Key. Falls back to the `NEXTDNS_API_KEY` environment variable.",
			},
			"base_url": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Base URL of the NextDNS API. Defaults to `https://api.nextdns.io/`; falls back to the `NEXTDNS_BASE_URL` environment variable. Only useful for testing against a stub.",
			},
		},
		DataSourcesMap: map[string]*schema.Resource{
			"nextdns_setup_endpoint": dataSourceNextDNSSetupEndpoint(),
			"nextdns_setup_linkedip": dataSourceNextDNSSetupLinkedIP(),
		},
		ResourcesMap: map[string]*schema.Resource{
			"nextdns_allowlist":        resourceNextDNSAllowlist(),
			"nextdns_denylist":         resourceNextDNSDenylist(),
			"nextdns_parental_control": resourceNextDNSParentalControl(),
			"nextdns_privacy":          resourceNextDNSPrivacy(),
			"nextdns_profile":          resourceNextDNSProfile(),
			"nextdns_rewrite":          resourceNextDNSRewrite(),
			"nextdns_security":         resourceNextDNSSecurity(),
			"nextdns_settings":         resourceNextDNSSettings(),
		},
		ConfigureContextFunc: configure,
	}
}

// nolint:revive
func configure(ctx context.Context, d *schema.ResourceData) (interface{}, diag.Diagnostics) {
	apiKey := os.Getenv("NEXTDNS_API_KEY")

	if key, ok := d.Get("api_key").(string); ok && len(key) > 0 {
		apiKey = key
	}

	if len(apiKey) == 0 {
		return nil, diag.Errorf(
			"NextDNS API key must be provided in the provider block or NEXTDNS_API_KEY environment variable.",
		)
	}

	opts := []nextdns.ClientOption{nextdns.WithAPIKey(apiKey)}

	baseURL := os.Getenv("NEXTDNS_BASE_URL")
	if u, ok := d.Get("base_url").(string); ok && len(u) > 0 {
		baseURL = u
	}
	if len(baseURL) > 0 {
		opts = append(opts, nextdns.WithBaseURL(baseURL))
	}

	client, err := nextdns.New(opts...)
	if err != nil {
		return nil, diag.FromErr(err)
	}

	return client, nil
}

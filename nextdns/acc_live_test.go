package nextdns

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/LukeEvansTech/terraform-provider-nextdns/internal/nextdns"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

// Live acceptance test. Opt-in only: it runs when TF_ACC=1 and
// NEXTDNS_API_KEY are both set, creates a throwaway profile named
// "tf-acc-<random>" in that account, writes every v0.3.0 attribute to it
// (including the two that some dashboards hide), reads them back through
// the raw API, and destroys the profile at the end. It never touches any
// pre-existing profile.
//
//	TF_ACC=1 NEXTDNS_API_KEY=... go test ./nextdns/ -run TestAccLive -v
func TestAccLive_ExtendedAttributesRoundTrip(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("TF_ACC not set; live acceptance test skipped")
	}
	apiKey := os.Getenv("NEXTDNS_API_KEY")
	if apiKey == "" {
		t.Skip("NEXTDNS_API_KEY not set; live acceptance test skipped")
	}

	name := acctest.RandomWithPrefix("tf-acc")
	config := func(bav bool, highRisk bool) string {
		return fmt.Sprintf(`
provider "nextdns" {}

resource "nextdns_profile" "acc" {
  name = %q
}

resource "nextdns_security" "acc" {
  profile_id                = nextdns_profile.acc.id
  threat_intelligence_feeds = true
  ai_threat_detection       = true
  google_safe_browsing      = false
  crypto_jacking            = true
  dns_rebinding             = true
  idn_homographs            = true
  typo_squatting            = true
  dga                       = true
  nrd                       = true
  ddns                      = true
  parking                   = true
  csam                      = true
  tlds                      = ["autos"]

  free_hosting_domains       = false
  tunneling_endpoints        = false
  data_drop_services         = false
  residential_hosting        = false
  untrusted_certificates     = false
  fast_flux_networks         = false
  dns_data_exfiltration      = false
  dns_payload_delivery       = false
  decentralized_web_gateways = false
  high_risk_tlds             = %t
}

resource "nextdns_settings" "acc" {
  profile_id              = nextdns_profile.acc.id
  web3                    = true
  bypass_age_verification = %t

  logs {
    enabled   = true
    location  = "ch"
    retention = "1 hour"
    privacy {
      log_clients_ip = true
      log_domains    = true
    }
  }
  block_page {
    enabled = false
  }
  performance {
    ecs              = true
    cache_boost      = true
    cname_flattening = false
  }
}
`, name, highRisk, bav)
	}

	// readBack fetches the live values through the raw client, independently
	// of the provider's own state, so the check is not the writer testifying
	// for itself.
	readBack := func(wantHighRisk, wantBav bool) resource.TestCheckFunc {
		return func(s *terraformState) error {
			rs, ok := s.RootModule().Resources["nextdns_profile.acc"]
			if !ok {
				return fmt.Errorf("profile not in state")
			}
			client, err := nextdns.New(nextdns.WithAPIKey(apiKey))
			if err != nil {
				return err
			}
			ctx := context.Background()
			sec, err := client.Security.Get(ctx, &nextdns.GetSecurityRequest{ProfileID: rs.Primary.ID})
			if err != nil {
				return err
			}
			if sec.HighRiskTlds == nil || *sec.HighRiskTlds != wantHighRisk {
				return fmt.Errorf("live highRiskTlds = %v, want %t", sec.HighRiskTlds, wantHighRisk)
			}
			for k, v := range map[string]*bool{
				"freeHostingDomains": sec.FreeHostingDomains, "tunnelingEndpoints": sec.TunnelingEndpoints,
				"dataDropServices": sec.DataDropServices, "residentialHosting": sec.ResidentialHosting,
				"untrustedCertificates": sec.UntrustedCertificates, "fastFluxNetworks": sec.FastFluxNetworks,
				"dnsDataExfiltration": sec.DNSDataExfiltration, "dnsPayloadDelivery": sec.DNSPayloadDelivery,
				"decentralizedWebGateways": sec.DecentralizedWebGateways,
			} {
				if v == nil {
					t.Logf("live API did not return %s for the throwaway profile", k)
				} else if *v {
					return fmt.Errorf("live %s = true, want false", k)
				}
			}
			set, err := client.Settings.Get(ctx, &nextdns.GetSettingsRequest{ProfileID: rs.Primary.ID})
			if err != nil {
				return err
			}
			if set.Bav == nil || *set.Bav != wantBav {
				return fmt.Errorf("live bav = %v, want %t", set.Bav, wantBav)
			}
			return nil
		}
	}

	resource.Test(t, resource.TestCase{
		ProviderFactories: providerFactories(),
		Steps: []resource.TestStep{
			{
				Config: config(true, false),
				Check:  readBack(false, true),
			},
			{
				Config: config(false, true),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("nextdns_security.acc", plancheck.ResourceActionUpdate),
						plancheck.ExpectResourceAction("nextdns_settings.acc", plancheck.ResourceActionUpdate),
					},
				},
				Check: readBack(true, false),
			},
			{
				Config: config(false, true),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

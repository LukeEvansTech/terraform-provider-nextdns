// editorconfig-checker-disable-file: HCL in raw strings is space-indented.
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
// fast_flux_networks and dns_data_exfiltration are hidden in the dashboard
// for some profiles, so a write to them is the thing in doubt. They are
// flipped true, read back, then flipped false and read back again: writing
// false alone would pass whether or not the API honours the field.
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
	config := func(bav, highRisk, hidden bool) string {
		return fmt.Sprintf(`
provider "nextdns" {}

resource "nextdns_profile" "acc" {
  name = %[1]q
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
  fast_flux_networks         = %[4]t
  dns_data_exfiltration      = %[4]t
  dns_payload_delivery       = false
  decentralized_web_gateways = false
  high_risk_tlds             = %[2]t
}

resource "nextdns_settings" "acc" {
  profile_id              = nextdns_profile.acc.id
  web3                    = true
  bypass_age_verification = %[3]t

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
`, name, highRisk, bav, hidden)
	}

	// readBack fetches the live values through the raw client, independently
	// of the provider's own state, so the check is not the writer testifying
	// for itself.
	readBack := func(wantHighRisk, wantBav, wantHidden bool) resource.TestCheckFunc {
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
			if err := checkLiveSwitches(t, sec, wantHidden); err != nil {
				return err
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
		ProtoV5ProviderFactories: protoV5ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: config(true, false, true),
				Check:  readBack(false, true, true),
			},
			{
				Config: config(false, true, false),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("nextdns_security.acc", plancheck.ResourceActionUpdate),
						plancheck.ExpectResourceAction("nextdns_settings.acc", plancheck.ResourceActionUpdate),
					},
				},
				Check: readBack(true, false, false),
			},
			{
				Config: config(false, true, false),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

// checkLiveSwitches asserts the live security switches on the throwaway
// profile: the two dashboard-hidden ones must be present and equal
// wantHidden, and the other extended switches must not be true.
func checkLiveSwitches(t *testing.T, sec *nextdns.Security, wantHidden bool) error {
	t.Helper()
	for k, v := range map[string]*bool{
		"fastFluxNetworks": sec.FastFluxNetworks, "dnsDataExfiltration": sec.DNSDataExfiltration,
	} {
		if v == nil {
			return fmt.Errorf("live API did not return %s", k)
		}
		if *v != wantHidden {
			return fmt.Errorf("live %s = %t, want %t", k, *v, wantHidden)
		}
	}
	for k, v := range map[string]*bool{
		"freeHostingDomains": sec.FreeHostingDomains, "tunnelingEndpoints": sec.TunnelingEndpoints,
		"dataDropServices": sec.DataDropServices, "residentialHosting": sec.ResidentialHosting,
		"untrustedCertificates": sec.UntrustedCertificates, "dnsPayloadDelivery": sec.DNSPayloadDelivery,
		"decentralizedWebGateways": sec.DecentralizedWebGateways,
	} {
		if v == nil {
			t.Logf("live API did not return %s for the throwaway profile", k)
		} else if *v {
			return fmt.Errorf("live %s = true, want false", k)
		}
	}
	return nil
}

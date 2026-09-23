// editorconfig-checker-disable-file: HCL in raw strings is space-indented.
package nextdns

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

// seedSettings mirrors the live shape of both managed profiles.
func seedSettings(f *fakeAPI, bav string) {
	f.seed("abc123", "settings", `{"web3": true`+bav+`}`)
	f.seed("abc123", "settings/logs", `{"enabled": true, "drop": {"ip": false, "domain": false}, "retention": 7776000, "location": "ch"}`)
	f.seed("abc123", "settings/blockPage", `{"enabled": false}`)
	f.seed("abc123", "settings/performance", `{"ecs": true, "cacheBoost": true, "cnameFlattening": false}`)
}

func settingsConfig(f *fakeAPI, extra string) string {
	return providerBlock(f) + fmt.Sprintf(`
resource "nextdns_settings" "this" {
  profile_id = "abc123"
  web3       = true
%s
  logs {
    enabled   = true
    location  = "ch"
    retention = "3 months"
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
`, extra)
}

func TestSettings_OmittedBavPreservesLive(t *testing.T) {
	f := newFakeAPI(t)
	seedSettings(f, `, "bav": true`)

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: protoV5ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: settingsConfig(f, ""),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("nextdns_settings.this", "bypass_age_verification", "true"),
					resource.TestCheckResourceAttr("nextdns_settings.this", "web3", "true"),
					func(_ *terraformState) error {
						reqs := f.requests("PATCH", "/settings")
						if len(reqs) == 0 {
							return fmt.Errorf("expected a PATCH /settings")
						}
						for _, r := range reqs {
							if _, present := r.Body["bav"]; present {
								return fmt.Errorf("PATCH /settings sent bav although omitted: %v", r.Body)
							}
						}
						if v, _ := f.get("abc123", "settings", "bav"); v != true {
							return fmt.Errorf("API bav changed to %v", v)
						}
						return nil
					},
				),
			},
			{
				Config: settingsConfig(f, ""),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

func TestSettings_ExplicitBavIsWrittenBothWays(t *testing.T) {
	f := newFakeAPI(t)
	seedSettings(f, `, "bav": true`)

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: protoV5ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: settingsConfig(f, `  bypass_age_verification = false`),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("nextdns_settings.this", "bypass_age_verification", "false"),
					func(_ *terraformState) error {
						if v, _ := f.get("abc123", "settings", "bav"); v != false {
							return fmt.Errorf("API bav = %v, want false", v)
						}
						return nil
					},
				),
			},
			{
				Config: settingsConfig(f, `  bypass_age_verification = true`),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("nextdns_settings.this", plancheck.ResourceActionUpdate),
					},
				},
				Check: func(_ *terraformState) error {
					if v, _ := f.get("abc123", "settings", "bav"); v != true {
						return fmt.Errorf("API bav = %v, want true", v)
					}
					return nil
				},
			},
			{
				Config: settingsConfig(f, `  bypass_age_verification = true`),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

func TestSettings_BavAbsentFromResponseKeepsState(t *testing.T) {
	f := newFakeAPI(t)
	seedSettings(f, `, "bav": true`)

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: protoV5ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: settingsConfig(f, `  bypass_age_verification = true`),
				Check:  resource.TestCheckResourceAttr("nextdns_settings.this", "bypass_age_verification", "true"),
			},
			{
				PreConfig: func() { f.hide("abc123", "settings", "bav") },
				Config:    settingsConfig(f, `  bypass_age_verification = true`),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
				Check: resource.TestCheckResourceAttr("nextdns_settings.this", "bypass_age_verification", "true"),
			},
		},
	})
}

func TestSettings_ImportReadsBav(t *testing.T) {
	f := newFakeAPI(t)
	seedSettings(f, `, "bav": true`)

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: protoV5ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config:             settingsConfig(f, ""),
				ResourceName:       "nextdns_settings.this",
				ImportState:        true,
				ImportStateId:      "abc123",
				ImportStatePersist: true,
				ImportStateCheck: func(states []*instanceState) error {
					if got := states[0].Attributes["bypass_age_verification"]; got != "true" {
						return fmt.Errorf("bypass_age_verification = %q, want true", got)
					}
					return nil
				},
			},
		},
	})
}

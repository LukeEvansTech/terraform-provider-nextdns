// editorconfig-checker-disable-file: HCL in raw strings is space-indented.
package nextdns

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

func privacyConfig(f *fakeAPI, lists string) string {
	return providerBlock(f) + fmt.Sprintf(`
resource "nextdns_privacy" "this" {
  profile_id         = "abc123"
  allow_affiliate    = false
  disguised_trackers = true
%s
}
`, lists)
}

func seedPrivacy(f *fakeAPI) {
	f.seed("abc123", "privacy", `{"disguisedTrackers": false, "allowAffiliate": true, "rotatingAdDomains": true}`)
	f.seed("abc123", "privacy/blocklists", `[]`)
	f.seed("abc123", "privacy/natives", `[]`)
}

// An omitted list, an explicit empty list and a populated one each plan
// clean on the second run, and moving between them updates in place.
func TestPrivacy_ListForms(t *testing.T) {
	f := newFakeAPI(t)
	seedPrivacy(f)

	empty := resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}
	update := resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
		plancheck.ExpectResourceAction("nextdns_privacy.this", plancheck.ResourceActionUpdate),
	}}

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: protoV5ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: privacyConfig(f, ""),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckNoResourceAttr("nextdns_privacy.this", "blocklists"),
					func(_ *terraformState) error {
						if v, _ := f.get("abc123", "privacy", "disguisedTrackers"); v != true {
							return fmt.Errorf("API disguisedTrackers = %v, want true", v)
						}
						return nil
					},
				),
			},
			{Config: privacyConfig(f, ""), ConfigPlanChecks: empty},
			{Config: privacyConfig(f, `  natives = []`), ConfigPlanChecks: update},
			{Config: privacyConfig(f, `  natives = []`), ConfigPlanChecks: empty},
			{
				Config:           privacyConfig(f, `  natives = ["apple", "windows"]`),
				ConfigPlanChecks: update,
				Check: func(_ *terraformState) error {
					l := f.list("abc123", "privacy/natives")
					if len(l) != 2 {
						return fmt.Errorf("natives = %v, want two entries", l)
					}
					return nil
				},
			},
			{Config: privacyConfig(f, `  natives = ["apple", "windows"]`), ConfigPlanChecks: empty},
		},
	})
}

func TestPrivacy_OmittedRotatingAdDomainsPreservesLive(t *testing.T) {
	f := newFakeAPI(t)
	seedPrivacy(f)

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: protoV5ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: privacyConfig(f, ""),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("nextdns_privacy.this", "rotating_ad_domains", "true"),
					func(_ *terraformState) error {
						reqs := f.requests("PATCH", "/privacy")
						if len(reqs) == 0 {
							return fmt.Errorf("expected a PATCH /privacy")
						}
						for _, r := range reqs {
							if _, present := r.Body["rotatingAdDomains"]; present {
								return fmt.Errorf("PATCH /privacy sent rotatingAdDomains although omitted: %v", r.Body)
							}
						}
						if v, _ := f.get("abc123", "privacy", "rotatingAdDomains"); v != true {
							return fmt.Errorf("API rotatingAdDomains changed to %v", v)
						}
						return nil
					},
				),
			},
			{
				Config: privacyConfig(f, ""),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

func TestPrivacy_ExplicitRotatingAdDomainsIsWrittenBothWays(t *testing.T) {
	f := newFakeAPI(t)
	seedPrivacy(f)

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: protoV5ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: privacyConfig(f, `  rotating_ad_domains = false`),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("nextdns_privacy.this", "rotating_ad_domains", "false"),
					func(_ *terraformState) error {
						if v, _ := f.get("abc123", "privacy", "rotatingAdDomains"); v != false {
							return fmt.Errorf("API rotatingAdDomains = %v, want false", v)
						}
						return nil
					},
				),
			},
			{
				Config: privacyConfig(f, `  rotating_ad_domains = true`),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("nextdns_privacy.this", plancheck.ResourceActionUpdate),
					},
				},
				Check: func(_ *terraformState) error {
					if v, _ := f.get("abc123", "privacy", "rotatingAdDomains"); v != true {
						return fmt.Errorf("API rotatingAdDomains = %v, want true", v)
					}
					return nil
				},
			},
			{
				Config: privacyConfig(f, `  rotating_ad_domains = true`),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

func TestPrivacy_RotatingAdDomainsAbsentFromResponseKeepsState(t *testing.T) {
	f := newFakeAPI(t)
	seedPrivacy(f)

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: protoV5ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: privacyConfig(f, `  rotating_ad_domains = true`),
				Check:  resource.TestCheckResourceAttr("nextdns_privacy.this", "rotating_ad_domains", "true"),
			},
			{
				PreConfig: func() { f.hide("abc123", "privacy", "rotatingAdDomains") },
				Config:    privacyConfig(f, `  rotating_ad_domains = true`),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
				Check: resource.TestCheckResourceAttr("nextdns_privacy.this", "rotating_ad_domains", "true"),
			},
		},
	})
}

func TestPrivacy_ImportReadsRotatingAdDomains(t *testing.T) {
	f := newFakeAPI(t)
	seedPrivacy(f)

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: protoV5ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config:             privacyConfig(f, ""),
				ResourceName:       "nextdns_privacy.this",
				ImportState:        true,
				ImportStateId:      "abc123",
				ImportStatePersist: true,
				ImportStateCheck: func(states []*instanceState) error {
					if got := states[0].Attributes["rotating_ad_domains"]; got != "true" {
						return fmt.Errorf("rotating_ad_domains = %q, want true", got)
					}
					return nil
				},
			},
		},
	})
}

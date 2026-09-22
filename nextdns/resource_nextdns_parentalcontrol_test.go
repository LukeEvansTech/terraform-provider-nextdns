package nextdns

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

// Regression for the ported cozy-corner fix: the profile-level PATCH must
// not carry services/categories (they have their own endpoints), and a
// recreation block with only a timezone (times: null) must not panic.
func TestParentalControl_PatchOmitsServicesAndCategoriesAndTimesMayBeNull(t *testing.T) {
	f := newFakeAPI(t)
	f.seed("abc123", "parentalControl", `{"safeSearch": false, "youtubeRestrictedMode": false, "blockBypass": true, "recreation": {"times": null, "timezone": ""}}`)
	f.seed("abc123", "parentalControl/services", `[]`)
	f.seed("abc123", "parentalControl/categories", `[]`)

	config := providerBlock(f) + `
resource "nextdns_parental_control" "this" {
  profile_id              = "abc123"
  safe_search             = false
  youtube_restricted_mode = false
  block_bypass            = true

  service {
    id         = "tiktok"
    active     = true
    recreation = false
  }

  recreation {
    timezone = ""
  }
}
`
	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: providerFactories(),
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("nextdns_parental_control.this", "block_bypass", "true"),
					resource.TestCheckResourceAttr("nextdns_parental_control.this", "service.#", "1"),
					func(s *terraformState) error {
						reqs := f.requests("PATCH", "/parentalControl")
						if len(reqs) == 0 {
							return fmt.Errorf("expected a PATCH /parentalControl")
						}
						for _, r := range reqs {
							for _, k := range []string{"services", "categories"} {
								if _, present := r.Body[k]; present {
									return fmt.Errorf("PATCH /parentalControl carried %q: %v", k, r.Body)
								}
							}
						}
						if len(f.requests("PUT", "/parentalControl/services")) == 0 {
							return fmt.Errorf("services were not written through their own endpoint")
						}
						return nil
					},
				),
			},
			{
				Config: config,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

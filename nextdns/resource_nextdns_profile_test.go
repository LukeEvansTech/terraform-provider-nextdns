// editorconfig-checker-disable-file: HCL in raw strings is space-indented.
package nextdns

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

func profileConfig(f *fakeAPI, name string) string {
	return providerBlock(f) + fmt.Sprintf(`
resource "nextdns_profile" "this" {
  name = %q
}
`, name)
}

// Create, rename in place, import, and destroy against the fake API.
func TestProfile_Lifecycle(t *testing.T) {
	f := newFakeAPI(t)

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: protoV5ProviderFactories(),
		CheckDestroy: func(_ *terraformState) error {
			if len(f.requests("DELETE", "/profiles/fake01")) != 1 {
				return fmt.Errorf("expected one DELETE /profiles/fake01")
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: profileConfig(f, "first"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("nextdns_profile.this", "id", "fake01"),
					resource.TestCheckResourceAttr("nextdns_profile.this", "profile_id", "fake01"),
					resource.TestCheckResourceAttr("nextdns_profile.this", "name", "first"),
				),
			},
			{
				Config: profileConfig(f, "second"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("nextdns_profile.this", plancheck.ResourceActionUpdate),
					},
				},
				Check: func(_ *terraformState) error {
					reqs := f.requests("PATCH", "/profiles/fake01")
					if len(reqs) != 1 || reqs[0].Body["name"] != "second" {
						return fmt.Errorf("expected one PATCH renaming to second, got %v", reqs)
					}
					return nil
				},
			},
			{
				Config:            profileConfig(f, "second"),
				ResourceName:      "nextdns_profile.this",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

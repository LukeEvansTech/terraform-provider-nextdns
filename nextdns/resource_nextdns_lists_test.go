// editorconfig-checker-disable-file: HCL in raw strings is space-indented.
package nextdns

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

func allowlistConfig(f *fakeAPI, domains ...string) string {
	cfg := providerBlock(f) + `
resource "nextdns_allowlist" "this" {
  profile_id = "abc123"
`
	for _, d := range domains {
		cfg += fmt.Sprintf(`  domain {
    id     = %q
    active = true
  }
`, d)
	}
	return cfg + "}\n"
}

// The whole list is PUT on every write, a hand-added entry shows up as drift
// and is removed by the next apply, and destroy empties the list.
func TestAllowlist_ReplaceDriftAndDestroy(t *testing.T) {
	f := newFakeAPI(t)
	f.seed("abc123", "allowlist", `[]`)

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: protoV5ProviderFactories(),
		CheckDestroy: func(_ *terraformState) error {
			puts := f.requests("PUT", "/allowlist")
			if l := f.list("abc123", "allowlist"); len(l) != 0 {
				return fmt.Errorf("allowlist not emptied on destroy: %v (%d PUTs)", l, len(puts))
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: allowlistConfig(f, "a.example", "b.example"),
				Check:  resource.TestCheckResourceAttr("nextdns_allowlist.this", "domain.#", "2"),
			},
			{
				PreConfig: func() {
					f.seed("abc123", "allowlist", `[{"id":"a.example","active":true},{"id":"b.example","active":true},{"id":"by-hand.example","active":true}]`)
				},
				Config: allowlistConfig(f, "a.example", "b.example"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("nextdns_allowlist.this", plancheck.ResourceActionUpdate),
					},
				},
				Check: func(_ *terraformState) error {
					if l := f.list("abc123", "allowlist"); len(l) != 2 {
						return fmt.Errorf("hand-added entry survived the apply: %v", l)
					}
					return nil
				},
			},
			{
				Config:            allowlistConfig(f, "a.example", "b.example"),
				ResourceName:      "nextdns_allowlist.this",
				ImportState:       true,
				ImportStateId:     "abc123",
				ImportStateVerify: true,
			},
		},
	})
}

func TestDenylist_ActiveFlagRoundTrip(t *testing.T) {
	f := newFakeAPI(t)
	f.seed("abc123", "denylist", `[]`)
	config := providerBlock(f) + `
resource "nextdns_denylist" "this" {
  profile_id = "abc123"
  domain {
    id     = "mask.icloud.com"
    active = false
  }
}
`
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: protoV5ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: func(_ *terraformState) error {
					l := f.list("abc123", "denylist")
					if len(l) != 1 || l[0].(map[string]any)["active"] != false {
						return fmt.Errorf("denylist = %v, want one inactive entry", l)
					}
					return nil
				},
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

func rewriteConfig(f *fakeAPI, rewrites ...[2]string) string {
	cfg := providerBlock(f) + `
resource "nextdns_rewrite" "this" {
  profile_id = "abc123"
`
	for _, w := range rewrites {
		cfg += fmt.Sprintf(`  rewrite {
    domain  = %q
    address = %q
  }
`, w[0], w[1])
	}
	return cfg + "}\n"
}

// Update touches only what changed: the kept rewrite is neither deleted nor
// recreated.
func TestRewrite_UpdateIsIncremental(t *testing.T) {
	f := newFakeAPI(t)
	f.seed("abc123", "rewrites", `[]`)

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: protoV5ProviderFactories(),
		CheckDestroy: func(_ *terraformState) error {
			if l := f.list("abc123", "rewrites"); len(l) != 0 {
				return fmt.Errorf("rewrites left after destroy: %v", l)
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: rewriteConfig(f, [2]string{"keep.example", "10.0.0.1"}, [2]string{"drop.example", "10.0.0.2"}),
				Check:  resource.TestCheckResourceAttr("nextdns_rewrite.this", "rewrite.#", "2"),
			},
			{
				Config: rewriteConfig(f, [2]string{"keep.example", "10.0.0.1"}, [2]string{"add.example", "10.0.0.3"}),
				Check: func(_ *terraformState) error {
					dels := f.requests("DELETE", "/rewrites/rw02")
					if len(dels) != 1 {
						return fmt.Errorf("expected drop.example (rw02) deleted once, got %d", len(dels))
					}
					if n := len(f.requests("DELETE", "/rewrites/rw01")); n != 0 {
						return fmt.Errorf("kept rewrite rw01 was deleted %d times", n)
					}
					names := map[string]bool{}
					for _, e := range f.list("abc123", "rewrites") {
						names[e.(map[string]any)["name"].(string)] = true
					}
					if len(names) != 2 || !names["keep.example"] || !names["add.example"] {
						return fmt.Errorf("live rewrites = %v", names)
					}
					return nil
				},
			},
			{
				Config: rewriteConfig(f, [2]string{"keep.example", "10.0.0.1"}, [2]string{"add.example", "10.0.0.3"}),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

// SDKv2 declared the domain and rewrite blocks Required (min_items 1 in the
// schema); the framework enforces the same with a validator.
func TestLists_BlockIsRequired(t *testing.T) {
	f := newFakeAPI(t)
	for _, cfg := range []string{
		`resource "nextdns_allowlist" "this" { profile_id = "abc123" }`,
		`resource "nextdns_denylist" "this" { profile_id = "abc123" }`,
		`resource "nextdns_rewrite" "this" { profile_id = "abc123" }`,
	} {
		resource.UnitTest(t, resource.TestCase{
			ProtoV5ProviderFactories: protoV5ProviderFactories(),
			Steps: []resource.TestStep{{
				Config:      providerBlock(f) + cfg,
				ExpectError: regexp.MustCompile(`must have a configuration value`),
			}},
		})
	}
}

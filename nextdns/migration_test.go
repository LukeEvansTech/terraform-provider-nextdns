// editorconfig-checker-disable-file: HCL in raw strings is space-indented.
package nextdns

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

// lastSDKv2Release is the last release built entirely on SDKv2. Every
// resource is created with it first, then planned with the provider under
// test: an empty plan proves the framework implementation reads state the
// SDKv2 one wrote, and agrees with it about the live values.
const lastSDKv2Release = "0.3.0"

// migrationTest applies config with the last SDKv2 release (installed from
// the Pages mirror, see TestMain), then expects the provider under test to
// plan no changes against the same config, state and fake API.
func migrationTest(t *testing.T, config string) {
	t.Helper()
	// Serve the provider under test at the released provider's address, so
	// state written by one is planned by the other without a replace. The
	// other tests keep the default namespace, which their configurations
	// resolve to implicitly.
	t.Setenv("TF_ACC_PROVIDER_NAMESPACE", "lukeevanstech")
	resource.UnitTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				ExternalProviders: map[string]resource.ExternalProvider{
					"nextdns": {
						Source:            "registry.opentofu.org/lukeevanstech/nextdns",
						VersionConstraint: lastSDKv2Release,
					},
				},
				Config: config,
			},
			{
				ProtoV5ProviderFactories: protoV5ProviderFactories(),
				Config:                   config,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

func TestMigration_Profile(t *testing.T) {
	f := newFakeAPI(t)
	migrationTest(t, providerBlock(f)+`
resource "nextdns_profile" "this" {
  name = "migration"
}
`)
}

func TestMigration_Allowlist(t *testing.T) {
	f := newFakeAPI(t)
	f.seed("abc123", "allowlist", `[]`)
	migrationTest(t, providerBlock(f)+`
resource "nextdns_allowlist" "this" {
  profile_id = "abc123"
  domain {
    id     = "example.com"
    active = true
  }
  domain {
    id     = "example.org"
    active = false
  }
}
`)
}

func TestMigration_Denylist(t *testing.T) {
	f := newFakeAPI(t)
	f.seed("abc123", "denylist", `[]`)
	migrationTest(t, providerBlock(f)+`
resource "nextdns_denylist" "this" {
  profile_id = "abc123"
  domain {
    id     = "mask.icloud.com"
    active = true
  }
}
`)
}

func TestMigration_Rewrite(t *testing.T) {
	f := newFakeAPI(t)
	f.seed("abc123", "rewrites", `[]`)
	migrationTest(t, providerBlock(f)+`
resource "nextdns_rewrite" "this" {
  profile_id = "abc123"
  rewrite {
    domain  = "nas.example.com"
    address = "10.0.0.10"
  }
  rewrite {
    domain  = "printer.example.com"
    address = "10.0.0.11"
  }
}
`)
}

func TestMigration_Privacy(t *testing.T) {
	f := newFakeAPI(t)
	f.seed("abc123", "privacy", `{"disguisedTrackers": true, "allowAffiliate": false}`)
	f.seed("abc123", "privacy/blocklists", `[]`)
	f.seed("abc123", "privacy/natives", `[]`)
	migrationTest(t, providerBlock(f)+`
resource "nextdns_privacy" "this" {
  profile_id         = "abc123"
  allow_affiliate    = false
  disguised_trackers = true
  blocklists         = ["hagezi-multi-pro"]
  natives            = ["windows", "apple"]
}
`)
}

// Privacy with both lists omitted: the API returns empty lists, which the
// SDKv2 provider recorded in its own way; the framework must not plan a
// change from it.
func TestMigration_PrivacyListsOmitted(t *testing.T) {
	f := newFakeAPI(t)
	f.seed("abc123", "privacy", `{"disguisedTrackers": true, "allowAffiliate": false}`)
	f.seed("abc123", "privacy/blocklists", `[]`)
	f.seed("abc123", "privacy/natives", `[]`)
	migrationTest(t, providerBlock(f)+`
resource "nextdns_privacy" "this" {
  profile_id         = "abc123"
  allow_affiliate    = false
  disguised_trackers = true
}
`)
}

func TestMigration_Security(t *testing.T) {
	f := newFakeAPI(t)
	f.seed("abc123", "security", securityLive)
	f.seed("abc123", "security/tlds", `[]`)
	migrationTest(t, securityConfig(f, `  data_drop_services = false
  high_risk_tlds     = false`))
}

// Security with every extended switch omitted, and no tlds: state holds
// only what the API reported.
func TestMigration_SecurityExtendedOmitted(t *testing.T) {
	f := newFakeAPI(t)
	f.seed("abc123", "security", securityLive)
	f.seed("abc123", "security", `{"highRiskTlds": true}`)
	f.seed("abc123", "security/tlds", `[]`)
	migrationTest(t, providerBlock(f)+`
resource "nextdns_security" "this" {
  profile_id                = "abc123"
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
}
`)
}

func TestMigration_Settings(t *testing.T) {
	f := newFakeAPI(t)
	seedSettings(f, `, "bav": true`)
	migrationTest(t, settingsConfig(f, `  bypass_age_verification = true`))
}

func TestMigration_SettingsBavOmitted(t *testing.T) {
	f := newFakeAPI(t)
	seedSettings(f, `, "bav": true`)
	migrationTest(t, settingsConfig(f, ""))
}

// Mirrors the production shape: services, no categories, and the empty
// recreation block the API always returns.
func TestMigration_ParentalControl(t *testing.T) {
	f := newFakeAPI(t)
	f.seed("abc123", "parentalControl", `{"safeSearch": false, "youtubeRestrictedMode": false, "blockBypass": true, "recreation": {"times": null, "timezone": ""}}`)
	f.seed("abc123", "parentalControl/services", `[]`)
	f.seed("abc123", "parentalControl/categories", `[]`)
	migrationTest(t, providerBlock(f)+`
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
  service {
    id         = "fortnite"
    active     = true
    recreation = true
  }

  recreation {
    timezone = ""
  }
}
`)
}

// Parental control with categories and a full recreation schedule.
func TestMigration_ParentalControlSchedule(t *testing.T) {
	f := newFakeAPI(t)
	f.seed("abc123", "parentalControl", `{"safeSearch": true, "youtubeRestrictedMode": true, "blockBypass": false}`)
	f.seed("abc123", "parentalControl/services", `[]`)
	f.seed("abc123", "parentalControl/categories", `[]`)
	migrationTest(t, providerBlock(f)+`
resource "nextdns_parental_control" "this" {
  profile_id              = "abc123"
  safe_search             = true
  youtube_restricted_mode = true
  block_bypass            = false

  category {
    id         = "gambling"
    active     = true
    recreation = false
  }

  recreation {
    timezone = "Europe/London"
    monday {
      start = "18:00:00"
      end   = "20:30:00"
    }
    saturday {
      start = "09:00:00"
      end   = "21:00:00"
    }
  }
}
`)
}

func TestMigration_DataSources(t *testing.T) {
	f := newFakeAPI(t)
	f.seed("abc123", "setup", `{"ipv4": ["45.90.28.1", "45.90.30.1"], "ipv6": ["2a07:a8c0::ab:c123"], "dnscrypt": "sdns://stamp"}`)
	f.seed("abc123", "setup/linkedip", `{"servers": ["45.90.28.1"], "ip": "192.0.2.1", "ddns": "", "updateToken": "not-a-real-token"}`)
	migrationTest(t, providerBlock(f)+`
data "nextdns_setup_endpoint" "this" {
  profile_id = "abc123"
}

data "nextdns_setup_linkedip" "this" {
  profile_id = "abc123"
}
`)
}

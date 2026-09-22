package nextdns

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

const securityLive = `{
  "threatIntelligenceFeeds": true, "aiThreatDetection": true, "googleSafeBrowsing": false,
  "cryptojacking": true, "dnsRebinding": true, "idnHomographs": true, "typosquatting": true,
  "dga": true, "nrd": true, "ddns": true, "parking": true, "csam": true,
  "freeHostingDomains": false, "tunnelingEndpoints": false, "dataDropServices": false,
  "residentialHosting": false, "untrustedCertificates": false, "fastFluxNetworks": false,
  "dnsDataExfiltration": false, "dnsPayloadDelivery": false, "decentralizedWebGateways": false,
  "highRiskTlds": false
}`

var extendedSecurityAttrs = map[string]string{
	"free_hosting_domains":       "freeHostingDomains",
	"tunneling_endpoints":        "tunnelingEndpoints",
	"data_drop_services":         "dataDropServices",
	"residential_hosting":        "residentialHosting",
	"untrusted_certificates":     "untrustedCertificates",
	"fast_flux_networks":         "fastFluxNetworks",
	"dns_data_exfiltration":      "dnsDataExfiltration",
	"dns_payload_delivery":       "dnsPayloadDelivery",
	"decentralized_web_gateways": "decentralizedWebGateways",
	"high_risk_tlds":             "highRiskTlds",
}

func securityConfig(f *fakeAPI, extra string) string {
	return providerBlock(f) + fmt.Sprintf(`
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
  tlds                      = ["autos", "bid"]
%s
}
`, extra)
}

// Omitted extended attributes: nothing is sent for them, state records the
// live values, and a second plan is empty.
func TestSecurity_OmittedExtendedAttributesPreserveLive(t *testing.T) {
	f := newFakeAPI(t)
	f.seed("abc123", "security", securityLive)
	f.seed("abc123", "security/tlds", `[]`)

	checks := []resource.TestCheckFunc{
		resource.TestCheckResourceAttr("nextdns_security.this", "threat_intelligence_feeds", "true"),
	}
	for attr := range extendedSecurityAttrs {
		checks = append(checks, resource.TestCheckResourceAttr("nextdns_security.this", attr, "false"))
	}
	checks = append(checks, func(_ *terraformState) error {
		for _, r := range f.requests("PATCH", "/security") {
			for _, apiKey := range extendedSecurityAttrs {
				if _, present := r.Body[apiKey]; present {
					return fmt.Errorf("PATCH /security sent %q although the attribute was omitted", apiKey)
				}
			}
		}
		if len(f.requests("PATCH", "/security")) == 0 {
			return fmt.Errorf("expected at least one PATCH /security")
		}
		return nil
	})

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: providerFactories(),
		Steps: []resource.TestStep{
			{
				Config: securityConfig(f, ""),
				Check:  resource.ComposeTestCheckFunc(checks...),
			},
			{
				Config: securityConfig(f, ""),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

// Explicit false for a switch the API holds as true is written as false.
func TestSecurity_ExplicitFalseIsWritten(t *testing.T) {
	f := newFakeAPI(t)
	f.seed("abc123", "security", securityLive)
	f.seed("abc123", "security", `{"threatIntelligenceFeeds": true, "highRiskTlds": true, "freeHostingDomains": true}`)
	f.seed("abc123", "security/tlds", `[]`)

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: providerFactories(),
		Steps: []resource.TestStep{
			{
				Config: securityConfig(f, `  high_risk_tlds = false
  free_hosting_domains = false`),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("nextdns_security.this", "high_risk_tlds", "false"),
					resource.TestCheckResourceAttr("nextdns_security.this", "free_hosting_domains", "false"),
					func(_ *terraformState) error {
						v, ok := f.get("abc123", "security", "highRiskTlds")
						if !ok || v != false {
							return fmt.Errorf("API highRiskTlds = %v (present %v), want false", v, ok)
						}
						reqs := f.requests("PATCH", "/security")
						if len(reqs) == 0 {
							return fmt.Errorf("no PATCH /security recorded")
						}
						last := reqs[len(reqs)-1].Body
						if last["highRiskTlds"] != false || last["freeHostingDomains"] != false {
							return fmt.Errorf("last PATCH body missing explicit false: %v", last)
						}
						if _, present := last["tunnelingEndpoints"]; present {
							return fmt.Errorf("PATCH sent tunnelingEndpoints although omitted")
						}
						return nil
					},
				),
			},
		},
	})
}

// Explicit true is written and read back; flipping it later updates in place.
func TestSecurity_ExplicitTrueRoundTrip(t *testing.T) {
	f := newFakeAPI(t)
	f.seed("abc123", "security", securityLive)
	f.seed("abc123", "security/tlds", `[]`)

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: providerFactories(),
		Steps: []resource.TestStep{
			{
				Config: securityConfig(f, `  tunneling_endpoints = true`),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("nextdns_security.this", "tunneling_endpoints", "true"),
					func(_ *terraformState) error {
						v, _ := f.get("abc123", "security", "tunnelingEndpoints")
						if v != true {
							return fmt.Errorf("API tunnelingEndpoints = %v, want true", v)
						}
						return nil
					},
				),
			},
			{
				Config: securityConfig(f, `  tunneling_endpoints = false`),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("nextdns_security.this", plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.TestCheckResourceAttr("nextdns_security.this", "tunneling_endpoints", "false"),
			},
		},
	})
}

// A switch the API stops returning keeps its last known state and does not
// produce a diff, whether the attribute is omitted or set to its old value.
func TestSecurity_AbsentFromResponseKeepsStateWithoutDrift(t *testing.T) {
	f := newFakeAPI(t)
	f.seed("abc123", "security", securityLive)
	f.seed("abc123", "security/tlds", `[]`)

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: providerFactories(),
		Steps: []resource.TestStep{
			{
				// State holds true for both so that a wrong "write false when
				// absent" would be visible as a diff or a changed attribute.
				Config: securityConfig(f, `  fast_flux_networks = true
  dns_data_exfiltration = true`),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("nextdns_security.this", "fast_flux_networks", "true"),
					resource.TestCheckResourceAttr("nextdns_security.this", "dns_data_exfiltration", "true"),
				),
			},
			{
				PreConfig: func() {
					f.hide("abc123", "security", "fastFluxNetworks")
					f.hide("abc123", "security", "dnsDataExfiltration")
				},
				Config: securityConfig(f, `  fast_flux_networks = true`),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("nextdns_security.this", "fast_flux_networks", "true"),
					resource.TestCheckResourceAttr("nextdns_security.this", "dns_data_exfiltration", "true"),
				),
			},
			{
				Config: securityConfig(f, ""),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("nextdns_security.this", "fast_flux_networks", "true"),
					resource.TestCheckResourceAttr("nextdns_security.this", "dns_data_exfiltration", "true"),
				),
			},
		},
	})
}

// Import populates the extended attributes from the API.
func TestSecurity_ImportReadsExtendedAttributes(t *testing.T) {
	f := newFakeAPI(t)
	f.seed("abc123", "security", securityLive)
	f.seed("abc123", "security", `{"highRiskTlds": true}`)
	f.seed("abc123", "security/tlds", `[{"id":"autos"},{"id":"bid"}]`)

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: providerFactories(),
		Steps: []resource.TestStep{
			{
				Config:             securityConfig(f, ""),
				ResourceName:       "nextdns_security.this",
				ImportState:        true,
				ImportStateId:      "abc123",
				ImportStatePersist: true,
				ImportStateCheck: func(states []*instanceState) error {
					if len(states) != 1 {
						return fmt.Errorf("expected 1 state, got %d", len(states))
					}
					if got := states[0].Attributes["high_risk_tlds"]; got != "true" {
						return fmt.Errorf("high_risk_tlds = %q, want true", got)
					}
					if got := states[0].Attributes["free_hosting_domains"]; got != "false" {
						return fmt.Errorf("free_hosting_domains = %q, want false", got)
					}
					return nil
				},
			},
		},
	})
}

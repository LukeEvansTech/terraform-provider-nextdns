// editorconfig-checker-disable-file: HCL in raw strings is space-indented.
package nextdns

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestDataSources_Values(t *testing.T) {
	f := newFakeAPI(t)
	f.seed("abc123", "setup", `{"ipv4": ["45.90.28.1", "45.90.30.1"], "ipv6": [], "dnscrypt": "sdns://stamp"}`)
	f.seed("abc123", "setup/linkedip", `{"servers": ["45.90.28.1"], "ip": "192.0.2.1", "ddns": "", "updateToken": "not-a-real-token"}`)

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: protoV5ProviderFactories(),
		Steps: []resource.TestStep{{
			Config: providerBlock(f) + `
data "nextdns_setup_endpoint" "this" {
  profile_id = "abc123"
}

data "nextdns_setup_linkedip" "this" {
  profile_id = "abc123"
}

output "ipv6_count" {
  value = length(data.nextdns_setup_endpoint.this.ipv6)
}
`,
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr("data.nextdns_setup_endpoint.this", "id", "abc123"),
				resource.TestCheckResourceAttr("data.nextdns_setup_endpoint.this", "doh", "https://dns.nextdns.io/abc123"),
				resource.TestCheckResourceAttr("data.nextdns_setup_endpoint.this", "dot", "abc123.dns.nextdns.io"),
				resource.TestCheckResourceAttr("data.nextdns_setup_endpoint.this", "ipv4.#", "2"),
				resource.TestCheckResourceAttr("data.nextdns_setup_endpoint.this", "dnscrypt", "sdns://stamp"),
				resource.TestCheckOutput("ipv6_count", "0"),
				resource.TestCheckResourceAttr("data.nextdns_setup_linkedip.this", "ip", "192.0.2.1"),
				resource.TestCheckResourceAttr("data.nextdns_setup_linkedip.this", "servers.0", "45.90.28.1"),
			),
		}},
	})
}

// update_token authorises changing the profile's linked IP, so it is marked
// sensitive: an output that exposes it must say so, and plan output redacts it.
func TestDataSources_LinkedIPUpdateTokenIsSensitive(t *testing.T) {
	f := newFakeAPI(t)
	f.seed("abc123", "setup/linkedip", `{"servers": ["45.90.28.1"], "ip": "192.0.2.1", "ddns": "", "updateToken": "not-a-real-token"}`)
	config := func(sensitive string) string {
		return providerBlock(f) + `
data "nextdns_setup_linkedip" "this" {
  profile_id = "abc123"
}

output "token" {
  value = data.nextdns_setup_linkedip.this.update_token
` + sensitive + `}
`
	}

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: protoV5ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config:      config(""),
				ExpectError: regexp.MustCompile(`(?i)output refers to sensitive values`),
			},
			{
				Config: config("  sensitive = true\n"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.nextdns_setup_linkedip.this", "update_token", "not-a-real-token"),
					resource.TestCheckOutput("token", "not-a-real-token"),
				),
			},
		},
	})
}

resource "nextdns_allowlist" "this" {
  profile_id = nextdns_profile.this.id

  domain {
    id     = "nextdns.io"
    active = true
  }

  domain {
    id     = "example.com"
    active = false
  }
}

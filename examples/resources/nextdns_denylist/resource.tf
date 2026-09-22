resource "nextdns_denylist" "this" {
  profile_id = nextdns_profile.this.id

  domain {
    id     = "mask.icloud.com"
    active = true
  }

  domain {
    id     = "mask-h2.icloud.com"
    active = true
  }
}

resource "nextdns_rewrite" "this" {
  profile_id = nextdns_profile.this.id

  rewrite {
    domain  = "nas.home.example"
    address = "192.0.2.10"
  }
}

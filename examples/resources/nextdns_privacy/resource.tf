resource "nextdns_privacy" "this" {
  profile_id = nextdns_profile.this.id

  disguised_trackers = false
  allow_affiliate    = true

  blocklists = ["hagezi-multi-pro"]
  natives    = ["windows", "apple", "sonos", "roku"]
}

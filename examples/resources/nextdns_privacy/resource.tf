resource "nextdns_privacy" "this" {
  profile_id = nextdns_profile.this.id

  disguised_trackers = false
  allow_affiliate    = true

  # Optional + Computed: omit to keep the live value.
  rotating_ad_domains = false

  blocklists = ["hagezi-multi-pro"]
  natives    = ["windows", "apple", "sonos", "roku"]
}

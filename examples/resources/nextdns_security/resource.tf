resource "nextdns_security" "this" {
  profile_id = nextdns_profile.this.id

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

  tlds = ["autos", "bid", "loan"]

  # Extended switches (v0.3.0). Each is Optional + Computed: leave one out to
  # keep whatever the profile currently has, set it to write a value.
  free_hosting_domains       = false
  tunneling_endpoints        = false
  data_drop_services         = false
  residential_hosting        = false
  untrusted_certificates     = false
  dns_payload_delivery       = false
  decentralized_web_gateways = false
  high_risk_tlds             = false
  newly_active_domains       = false

  # fast_flux_networks and dns_data_exfiltration are returned by the API but
  # hidden in the dashboard for some profiles; omit them until a write has
  # been proven to take effect on yours.
}

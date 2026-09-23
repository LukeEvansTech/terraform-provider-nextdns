data "nextdns_setup_endpoint" "this" {
  profile_id = nextdns_profile.this.id
}

output "doh" {
  value = data.nextdns_setup_endpoint.this.doh
}

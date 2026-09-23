data "nextdns_setup_linkedip" "this" {
  profile_id = nextdns_profile.this.id
}

output "servers" {
  value = data.nextdns_setup_linkedip.this.servers
}

# Changelog

## v0.4.0 (2026-09-23)

The provider is rewritten on the Terraform Plugin Framework. Configuration, state and behaviour are unchanged: every resource created with v0.3.0 plans no changes under this version, which the migration tests and a plan against the live profiles both show.

- Changed: every resource and data source is implemented with `terraform-plugin-framework`, served over protocol 5 as before. SDKv2 and the interim `terraform-plugin-mux` are gone.
- Changed: `id` is computed only; it was optional and computed. Required blocks enforce their minimum with a validator instead of `min_items`.
- Changed: data-source list attributes are an empty list, never null, when the API returns none.
- Added: `nextdns_security.newly_active_domains` (API `security.newlyActiveDomains`), which the API began returning in September 2026. Optional + Computed like the other extended switches, and covered by the live acceptance test, which writes it true and then false.
- Added: migration tests: each resource is applied with v0.3.0, installed from the Pages mirror, and must then plan no changes under the provider being built.

## v0.3.0 (2026-09-23)

First release of the `lukeevanstech/nextdns` fork. Schema is upstream v0.2.0 plus the attributes below; nothing upstream was renamed or removed.

### Added

- `nextdns_security`: `free_hosting_domains`, `tunneling_endpoints`, `data_drop_services`, `residential_hosting`, `untrusted_certificates`, `fast_flux_networks`, `dns_data_exfiltration`, `dns_payload_delivery`, `decentralized_web_gateways`, `high_risk_tlds`. Optional + Computed; omitted means preserve, explicit false is written, absent-from-API keeps state.
- `nextdns_settings`: `bypass_age_verification` (API `settings.bav`), same semantics.
- Provider `base_url` attribute (`NEXTDNS_BASE_URL`) for testing against a stub.
- Test suite: client serialisation tests, provider behaviour tests driven by OpenTofu against an in-memory NextDNS stub, opt-in live acceptance test.
- `cmd/mirror-index`: renders the provider network-mirror protocol for a release; the release workflow publishes it to GitHub Pages.

### Fixed

- Parental control: services and categories are no longer resent in the profile PATCH, and a recreation block with `times: null` no longer panics (ported from cozy-corner).

### Changed

- Module path `github.com/LukeEvansTech/terraform-provider-nextdns`; the `nextdns-go` v0.5.0 client is vendored as `internal/nextdns` (with `pkg/errors` replaced by `fmt.Errorf`).
- Go 1.26, terraform-plugin-sdk/v2 2.40.1.
- Distribution moves from the Terraform registry to GitHub Releases plus a GitHub Pages network mirror; archives are unsigned.

## v0.2.0 and earlier

See the upstream [releases](https://github.com/amalucelli/terraform-provider-nextdns/releases).

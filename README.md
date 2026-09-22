# terraform-provider-nextdns

[NextDNS](https://nextdns.io/) provider for [OpenTofu](https://opentofu.org/) and [Terraform](https://terraform.io).

This is the `lukeevanstech/nextdns` fork of [`amalucelli/terraform-provider-nextdns`](https://github.com/amalucelli/terraform-provider-nextdns), which has not had a release since v0.2.0 (February 2024). The fork:

- adds the eleven profile switches the API exposes but upstream cannot manage: ten security switches (`free_hosting_domains`, `tunneling_endpoints`, `data_drop_services`, `residential_hosting`, `untrusted_certificates`, `fast_flux_networks`, `dns_data_exfiltration`, `dns_payload_delivery`, `decentralized_web_gateways`, `high_risk_tlds`) and `bypass_age_verification` (`settings.bav`);
- keeps every upstream resource, attribute and identifier unchanged, so an existing state moves across with `state replace-provider` and a no-change plan;
- folds the [`nextdns-go`](https://github.com/amalucelli/nextdns-go) client in as `internal/nextdns` (upstream last released March 2023);
- ports the parental-control fix from [`cozy-corner`](https://github.com/cozy-corner/terraform-provider-nextdns) (services and categories are no longer resent in the profile PATCH; a recreation block with only a timezone no longer panics);
- adds a test suite driven by OpenTofu against an in-memory stub of the NextDNS API, plus an opt-in live acceptance test.

Upstream's history is preserved in this repository.

## Installation

The provider is distributed from [GitHub Releases](https://github.com/LukeEvansTech/terraform-provider-nextdns/releases) through a [provider network mirror](https://opentofu.org/docs/cli/config/config-file/#provider-installation) on GitHub Pages. It is not on a registry. Point the CLI at the mirror for this one provider:

```hcl
# .tofurc — select it with TF_CLI_CONFIG_FILE=.tofurc
provider_installation {
  network_mirror {
    url     = "https://lukeevanstech.github.io/terraform-provider-nextdns/"
    include = ["lukeevanstech/nextdns"]
  }
  direct {
    exclude = ["lukeevanstech/nextdns"]
  }
}
```

The mirror publishes under both `registry.opentofu.org` and `registry.terraform.io`, so the same file works for either engine.

```hcl
terraform {
  required_providers {
    nextdns = {
      source  = "lukeevanstech/nextdns"
      version = "~> 0.3"
    }
  }
}

provider "nextdns" {
  api_key = var.nextdns_api_key # or NEXTDNS_API_KEY
}
```

## The eleven new attributes

All eleven are `Optional + Computed` booleans with the same three-way behaviour:

| Written in configuration | What the provider does |
| --- | --- |
| omitted | sends nothing for it; the live value is preserved and recorded in state |
| `false` or `true` | writes that value |
| (API stops returning it) | state keeps the last known value; no diff |

`fast_flux_networks` and `dns_data_exfiltration` are returned by the API for every profile inspected but hidden in the dashboard for some. The API accepts them on read; whether a write takes effect on a given profile has not been verified. Leave them omitted until it has been on yours.

## Migrating from `amalucelli/nextdns`

1. Install the mirror configuration above and change `source` to `lukeevanstech/nextdns`, `version` to `~> 0.3`.
2. `tofu init -upgrade`
3. `tofu state replace-provider registry.opentofu.org/amalucelli/nextdns registry.opentofu.org/lukeevanstech/nextdns` (use `registry.terraform.io` on Terraform).
4. `tofu plan` should report no changes. Then author the new attributes at your leisure.

Rollback is the reverse `state replace-provider` plus reverting the source, version and mirror configuration.

## Documentation

Generated from the provider schema into [`docs/`](docs/) by `tools/gen-docs.sh` (`go generate ./...`). Full worked example in [`examples/main.tf`](examples/main.tf).

## Development

Tooling is pinned in `.mise.toml` (Go, OpenTofu, GoReleaser, golangci-lint). `tfplugindocs` is a `go.mod` tool.

```sh
mise install
go test ./...                          # stub-driven suite; TestMain points plugin-testing at the tofu on PATH
mise exec -- golangci-lint run ./...
go generate ./...                      # regenerate docs/
mise exec -- goreleaser release --snapshot --clean   # local multi-platform build into dist/
go run ./cmd/mirror-index -dist dist -version 0.3.0 -out site -copy-archives   # mirror layout, usable as a filesystem_mirror
```

Live acceptance test (creates and destroys a throwaway profile in your account; never touches an existing one):

```sh
TF_ACC=1 NEXTDNS_API_KEY=... go test ./nextdns/ -run TestAccLive -v
```

## Release

Pushing a `v*` tag builds unsigned multi-platform archives with GoReleaser, publishes them as a GitHub Release, renders the network-mirror JSON with `cmd/mirror-index` and deploys it to GitHub Pages. There is no GPG signing and no registry submission.

## Licence

MPL-2.0, as upstream. The vendored client's licence is in `internal/nextdns/LICENSE-nextdns-go.md`.

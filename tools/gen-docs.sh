#!/usr/bin/env bash
# Regenerates docs/ from the provider schema with tfplugindocs.
#
# tfplugindocs normally builds the provider and asks the Terraform CLI for the
# schema. This repository targets OpenTofu, so the schema is dumped with tofu
# via a dev_overrides CLI config and handed over with --providers-schema.
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

tofu_bin="${TOFU:-$(command -v tofu)}"

( cd "$root" && go build -o "$work/terraform-provider-nextdns" . )

cat > "$work/cli.tfrc" <<CFG
provider_installation {
  dev_overrides {
    "lukeevanstech/nextdns" = "$work"
  }
  direct {}
}
CFG

mkdir -p "$work/cfg"
cat > "$work/cfg/main.tf" <<'TF'
terraform {
  required_providers {
    nextdns = {
      source = "lukeevanstech/nextdns"
    }
  }
}
TF

( cd "$work/cfg" && TF_CLI_CONFIG_FILE="$work/cli.tfrc" "$tofu_bin" providers schema -json > "$work/schema.raw.json" 2>/dev/null )

# tfplugindocs looks the provider up by its short name (or under the hashicorp
# namespace); OpenTofu keys the schema under registry.opentofu.org. Re-key it.
python3 - "$work/schema.raw.json" "$work/schema.json" <<'PY'
import json, sys
doc = json.load(open(sys.argv[1]))
schemas = doc.get("provider_schemas", {})
rekeyed = {}
for key, value in schemas.items():
    host, ns, typ = key.split("/")
    rekeyed[typ] = value  # tfplugindocs matches the bare short name
doc["provider_schemas"] = rekeyed
json.dump(doc, open(sys.argv[2], "w"))
PY

( cd "$root" && go tool tfplugindocs generate \
    --provider-name nextdns \
    --rendered-provider-name NextDNS \
    --providers-schema "$work/schema.json" )

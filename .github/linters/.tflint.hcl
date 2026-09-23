# examples/ holds documentation snippets rendered into docs/ by tfplugindocs.
# Per-resource files deliberately carry no terraform {} block, so the two
# rules that demand one are off; everything else in the bundled terraform
# ruleset still applies.
plugin "terraform" {
  enabled = true
  preset  = "recommended"
}

rule "terraform_required_providers" {
  enabled = false
}

rule "terraform_required_version" {
  enabled = false
}

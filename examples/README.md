# Examples

`main.tf` is a complete worked example of every resource and data source. The
`provider/`, `resources/` and `data-sources/` directories hold the per-page
snippets `tfplugindocs` embeds into `docs/`.

To run `main.tf`, add a `providers.tf` (gitignored) with the provider block and
the mirror configuration from the top-level README, then:

```sh
TF_CLI_CONFIG_FILE=.tofurc tofu init
tofu apply
```

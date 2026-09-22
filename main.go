package main

import (
	"github.com/LukeEvansTech/terraform-provider-nextdns/nextdns"
	"github.com/hashicorp/terraform-plugin-sdk/v2/plugin"
)

func main() {
	plugin.Serve(&plugin.ServeOpts{
		ProviderFunc: nextdns.Provider,
	})
}

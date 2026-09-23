// Regenerate docs/ from the provider schema (see tools/gen-docs.sh).
//go:generate ./tools/gen-docs.sh

package main

import (
	"context"
	"flag"
	"log"

	"github.com/LukeEvansTech/terraform-provider-nextdns/nextdns"
	"github.com/hashicorp/terraform-plugin-go/tfprotov5/tf5server"
)

// providerAddress is used for debug-mode reattach only; installation goes
// through the network mirror, which serves both registry hostnames.
const providerAddress = "registry.opentofu.org/lukeevanstech/nextdns"

func main() {
	debug := flag.Bool("debug", false, "run with support for debuggers such as delve")
	flag.Parse()

	server, err := nextdns.ProviderServer(context.Background())
	if err != nil {
		log.Fatal(err)
	}

	var opts []tf5server.ServeOpt
	if *debug {
		opts = append(opts, tf5server.WithManagedDebug())
	}

	if err := tf5server.Serve(providerAddress, server, opts...); err != nil {
		log.Fatal(err)
	}
}

package nextdns

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
	"github.com/hashicorp/terraform-plugin-mux/tf5muxserver"
)

// ProviderServer serves the Plugin Framework provider and the remaining
// SDKv2 resources behind one protocol 5 server. Each resource type lives in
// exactly one of the two; the mux routes by type name and refuses to start
// if both declare the same one, or if their provider schemas differ.
func ProviderServer(ctx context.Context) (func() tfprotov5.ProviderServer, error) {
	mux, err := tf5muxserver.NewMuxServer(ctx,
		providerserver.NewProtocol5(NewFrameworkProvider()),
		Provider().GRPCProvider,
	)
	if err != nil {
		return nil, err
	}
	return mux.ProviderServer, nil
}

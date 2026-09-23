package nextdns

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
)

// ProviderServer serves the provider over protocol 5, the protocol the
// SDKv2 releases spoke, so nothing changes for OpenTofu or for state.
func ProviderServer(_ context.Context) (func() tfprotov5.ProviderServer, error) {
	return providerserver.NewProtocol5(NewFrameworkProvider()), nil
}

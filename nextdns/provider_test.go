// editorconfig-checker-disable-file: HCL in raw strings is space-indented.
package nextdns

import (
	"fmt"
	"os"
	"os/exec"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// TestMain points terraform-plugin-testing at OpenTofu when no binary has
// been chosen explicitly, so the suite runs against the engine the
// configuration repositories actually use.
func TestMain(m *testing.M) {
	if os.Getenv("TF_ACC_TERRAFORM_PATH") == "" {
		if p, err := exec.LookPath("tofu"); err == nil {
			_ = os.Setenv("TF_ACC_TERRAFORM_PATH", p)
			if os.Getenv("TF_ACC_PROVIDER_HOST") == "" {
				_ = os.Setenv("TF_ACC_PROVIDER_HOST", "registry.opentofu.org")
			}
		}
	}
	os.Exit(m.Run())
}

func TestProviderInternalValidate(t *testing.T) {
	if err := Provider().InternalValidate(); err != nil {
		t.Fatal(err)
	}
}

// providerFactories wires the provider under test to a fake API.
func providerFactories() map[string]func() (*schema.Provider, error) {
	return map[string]func() (*schema.Provider, error){
		"nextdns": func() (*schema.Provider, error) { return Provider(), nil },
	}
}

// providerBlock renders the provider configuration for a fake API.
func providerBlock(f *fakeAPI) string {
	return fmt.Sprintf(`
provider "nextdns" {
  api_key  = "test-key"
  base_url = %q
}
`, f.Server.URL+"/")
}

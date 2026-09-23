// editorconfig-checker-disable-file: HCL in raw strings is space-indented.
package nextdns

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
)

// mirrorURL is the provider's own network mirror on GitHub Pages; the
// migration tests install the last SDKv2 release from it.
const mirrorURL = "https://lukeevanstech.github.io/terraform-provider-nextdns/"

// TestMain points terraform-plugin-testing at OpenTofu when no binary has
// been chosen explicitly, so the suite runs against the engine the
// configuration repositories actually use. Unless a CLI config is already
// set, it also writes one that installs released versions of
// lukeevanstech/nextdns from the Pages mirror (see migrationTest).
func TestMain(m *testing.M) {
	if os.Getenv("TF_ACC_TERRAFORM_PATH") == "" {
		if p, err := exec.LookPath("tofu"); err == nil {
			_ = os.Setenv("TF_ACC_TERRAFORM_PATH", p)
			if os.Getenv("TF_ACC_PROVIDER_HOST") == "" {
				_ = os.Setenv("TF_ACC_PROVIDER_HOST", "registry.opentofu.org")
			}
		}
	}
	cleanup := func() {}
	if os.Getenv("TF_CLI_CONFIG_FILE") == "" {
		dir, err := os.MkdirTemp("", "nextdns-tofurc-")
		if err == nil {
			rc := filepath.Join(dir, ".tofurc")
			body := fmt.Sprintf(`provider_installation {
  network_mirror {
    url     = %q
    include = ["registry.opentofu.org/lukeevanstech/nextdns"]
  }
  direct {
    exclude = ["registry.opentofu.org/lukeevanstech/nextdns"]
  }
}
`, mirrorURL)
			if os.WriteFile(rc, []byte(body), 0o600) == nil {
				_ = os.Setenv("TF_CLI_CONFIG_FILE", rc)
			}
			cleanup = func() { _ = os.RemoveAll(dir) }
		}
	}
	code := m.Run()
	cleanup()
	os.Exit(code)
}

func TestProviderInternalValidate(t *testing.T) {
	if err := Provider().InternalValidate(); err != nil {
		t.Fatal(err)
	}
}

// The mux refuses to start when the two halves disagree on the provider
// schema or both claim a type, so starting it and fetching the schema is
// the check that the split is consistent.
func TestProviderServerSchema(t *testing.T) {
	ctx := context.Background()
	factory, err := ProviderServer(ctx)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := factory().GetProviderSchema(ctx, &tfprotov5.GetProviderSchemaRequest{})
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range resp.Diagnostics {
		if d.Severity == tfprotov5.DiagnosticSeverityError {
			t.Errorf("schema diagnostic: %s: %s", d.Summary, d.Detail)
		}
	}
	want := []string{
		"nextdns_allowlist", "nextdns_denylist", "nextdns_parental_control", "nextdns_privacy",
		"nextdns_profile", "nextdns_rewrite", "nextdns_security", "nextdns_settings",
	}
	for _, name := range want {
		if _, ok := resp.ResourceSchemas[name]; !ok {
			t.Errorf("resource %s missing from the muxed schema", name)
		}
	}
	for _, name := range []string{"nextdns_setup_endpoint", "nextdns_setup_linkedip"} {
		if _, ok := resp.DataSourceSchemas[name]; !ok {
			t.Errorf("data source %s missing from the muxed schema", name)
		}
	}
}

// protoV5ProviderFactories serves the muxed provider under test.
func protoV5ProviderFactories() map[string]func() (tfprotov5.ProviderServer, error) {
	return map[string]func() (tfprotov5.ProviderServer, error){
		"nextdns": func() (tfprotov5.ProviderServer, error) {
			factory, err := ProviderServer(context.Background())
			if err != nil {
				return nil, err
			}
			return factory(), nil
		},
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

package main

import (
	"archive/zip"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/mod/sumdb/dirhash"
)

func makeZip(t *testing.T, path, binaryName, content string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	w, err := zw.Create(binaryName)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestRunWritesProtocolDocumentsForEveryHost(t *testing.T) {
	dist := t.TempDir()
	out := t.TempDir()
	makeZip(t, filepath.Join(dist, "terraform-provider-nextdns_0.3.0_darwin_arm64.zip"), "terraform-provider-nextdns_v0.3.0", "darwin")
	makeZip(t, filepath.Join(dist, "terraform-provider-nextdns_0.3.0_linux_amd64.zip"), "terraform-provider-nextdns_v0.3.0", "linux")
	makeZip(t, filepath.Join(dist, "terraform-provider-nextdns_0.2.9_linux_amd64.zip"), "terraform-provider-nextdns_v0.2.9", "old") // other version: ignored
	if err := os.WriteFile(filepath.Join(dist, "terraform-provider-nextdns_0.3.0_SHA256SUMS"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	err := run(options{
		dist: dist, version: "0.3.0", out: out,
		releaseURL: "https://github.com/LukeEvansTech/terraform-provider-nextdns/releases/download/v0.3.0/",
		namespace:  "lukeevanstech", typ: "nextdns",
		hosts: []string{"registry.opentofu.org", "registry.terraform.io"},
	})
	if err != nil {
		t.Fatal(err)
	}

	wantH1, err := dirhash.HashZip(filepath.Join(dist, "terraform-provider-nextdns_0.3.0_darwin_arm64.zip"), dirhash.Hash1)
	if err != nil {
		t.Fatal(err)
	}

	for _, host := range []string{"registry.opentofu.org", "registry.terraform.io"} {
		dir := filepath.Join(out, host, "lukeevanstech", "nextdns")

		var idx indexDoc
		mustReadJSON(t, filepath.Join(dir, "index.json"), &idx)
		if _, ok := idx.Versions["0.3.0"]; !ok || len(idx.Versions) != 1 {
			t.Fatalf("%s index.json versions = %v", host, idx.Versions)
		}

		var v versionDoc
		mustReadJSON(t, filepath.Join(dir, "0.3.0.json"), &v)
		if len(v.Archives) != 2 {
			t.Fatalf("%s 0.3.0.json archives = %v", host, v.Archives)
		}
		a := v.Archives["darwin_arm64"]
		if a.URL != "https://github.com/LukeEvansTech/terraform-provider-nextdns/releases/download/v0.3.0/terraform-provider-nextdns_0.3.0_darwin_arm64.zip" {
			t.Fatalf("url = %q", a.URL)
		}
		if len(a.Hashes) != 1 || a.Hashes[0] != wantH1 {
			t.Fatalf("hashes = %v, want [%s]", a.Hashes, wantH1)
		}
		if !hasPrefix(a.Hashes[0], "h1:") {
			t.Fatalf("hash %q is not h1 form", a.Hashes[0])
		}
	}
}

func TestRunMergesExistingIndexAndCopiesArchives(t *testing.T) {
	dist := t.TempDir()
	out := t.TempDir()
	makeZip(t, filepath.Join(dist, "terraform-provider-nextdns_0.3.1_linux_amd64.zip"), "terraform-provider-nextdns_v0.3.1", "linux")
	dir := filepath.Join(out, "registry.opentofu.org", "lukeevanstech", "nextdns")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "index.json"), []byte(`{"versions":{"0.3.0":{}}}`), 0o600); err != nil {
		t.Fatal(err)
	}

	err := run(options{
		dist: dist, version: "0.3.1", out: out, copyArchives: true,
		namespace: "lukeevanstech", typ: "nextdns", hosts: []string{"registry.opentofu.org"},
	})
	if err != nil {
		t.Fatal(err)
	}

	var idx indexDoc
	mustReadJSON(t, filepath.Join(dir, "index.json"), &idx)
	if len(idx.Versions) != 2 {
		t.Fatalf("index versions = %v, want 0.3.0 and 0.3.1", idx.Versions)
	}
	var v versionDoc
	mustReadJSON(t, filepath.Join(dir, "0.3.1.json"), &v)
	if v.Archives["linux_amd64"].URL != "terraform-provider-nextdns_0.3.1_linux_amd64.zip" {
		t.Fatalf("relative url expected, got %q", v.Archives["linux_amd64"].URL)
	}
	if _, err := os.Stat(filepath.Join(dir, "terraform-provider-nextdns_0.3.1_linux_amd64.zip")); err != nil {
		t.Fatalf("archive not copied: %v", err)
	}
}

func TestRunRejectsMissingInputs(t *testing.T) {
	if err := run(options{dist: t.TempDir(), out: t.TempDir()}); err == nil {
		t.Fatal("expected error without -version")
	}
	if err := run(options{dist: t.TempDir(), out: t.TempDir(), version: "0.3.0"}); err == nil {
		t.Fatal("expected error without -release-url")
	}
	if err := run(options{dist: t.TempDir(), out: t.TempDir(), version: "0.3.0", releaseURL: "https://x/", typ: "nextdns", hosts: []string{"h"}}); err == nil {
		t.Fatal("expected error with no archives")
	}
}

func mustReadJSON(t *testing.T, path string, v any) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, v); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
}

func hasPrefix(s, p string) bool { return len(s) >= len(p) && s[:len(p)] == p }

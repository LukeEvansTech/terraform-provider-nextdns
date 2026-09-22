// Command mirror-index renders the Terraform/OpenTofu provider network
// mirror protocol for one release of this provider.
//
// Given the GoReleaser dist directory (or any directory holding the
// terraform-provider-<type>_<version>_<os>_<arch>.zip archives) it writes,
// for every registry hostname requested:
//
//	<out>/<host>/<namespace>/<type>/index.json     - all versions (merged with any existing file)
//	<out>/<host>/<namespace>/<type>/<version>.json - per-platform archive URL + h1 hash
//
// With -copy-archives the zips are also copied next to the JSON so the same
// directory doubles as a filesystem_mirror (packed layout) for local use.
// Without it, archive URLs point at -release-url, normally the GitHub
// Release assets, and the site stays small enough for GitHub Pages.
//
// Protocol reference: https://opentofu.org/docs/internals/provider-network-mirror-protocol/
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"golang.org/x/mod/sumdb/dirhash"
)

type options struct {
	dist         string
	version      string
	releaseURL   string
	out          string
	namespace    string
	typ          string
	hosts        []string
	copyArchives bool
}

// archive is one platform build.
type archive struct {
	file string
	os   string
	arch string
	h1   string
}

type versionDoc struct {
	Archives map[string]archiveDoc `json:"archives"`
}

type archiveDoc struct {
	URL    string   `json:"url"`
	Hashes []string `json:"hashes"`
}

type indexDoc struct {
	Versions map[string]struct{} `json:"versions"`
}

func main() {
	var o options
	var hosts string
	flag.StringVar(&o.dist, "dist", "dist", "directory holding the provider zip archives")
	flag.StringVar(&o.version, "version", "", "version without leading v, e.g. 0.3.0 (required)")
	flag.StringVar(&o.releaseURL, "release-url", "", "base URL the archives are served from, e.g. https://github.com/OWNER/REPO/releases/download/v0.3.0 (required unless -copy-archives)")
	flag.StringVar(&o.out, "out", "site", "output directory (the mirror root)")
	flag.StringVar(&o.namespace, "namespace", "lukeevanstech", "provider namespace")
	flag.StringVar(&o.typ, "type", "nextdns", "provider type")
	flag.StringVar(&hosts, "hosts", "registry.opentofu.org,registry.terraform.io", "comma-separated registry hostnames to publish under")
	flag.BoolVar(&o.copyArchives, "copy-archives", false, "copy the zips into the site and use relative URLs (filesystem_mirror compatible)")
	flag.Parse()
	o.hosts = strings.Split(hosts, ",")

	if err := run(o); err != nil {
		fmt.Fprintln(os.Stderr, "mirror-index:", err)
		os.Exit(1)
	}
}

var archiveRe = regexp.MustCompile(`^terraform-provider-([a-z0-9-]+)_([0-9][^_]*)_([a-z0-9]+)_([a-z0-9]+)\.zip$`)

func run(o options) error {
	if o.version == "" {
		return errors.New("-version is required")
	}
	if o.releaseURL == "" && !o.copyArchives {
		return errors.New("-release-url is required unless -copy-archives is set")
	}
	archives, err := findArchives(o.dist, o.typ, o.version)
	if err != nil {
		return err
	}
	if len(archives) == 0 {
		return fmt.Errorf("no terraform-provider-%s_%s_<os>_<arch>.zip archives in %s", o.typ, o.version, o.dist)
	}

	for _, host := range o.hosts {
		host = strings.TrimSpace(host)
		if host == "" {
			continue
		}
		dir := filepath.Join(o.out, host, o.namespace, o.typ)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}

		vdoc := versionDoc{Archives: map[string]archiveDoc{}}
		for _, a := range archives {
			url := strings.TrimRight(o.releaseURL, "/") + "/" + a.file
			if o.copyArchives {
				if err := copyFile(filepath.Join(o.dist, a.file), filepath.Join(dir, a.file)); err != nil {
					return err
				}
				url = a.file
			}
			vdoc.Archives[a.os+"_"+a.arch] = archiveDoc{URL: url, Hashes: []string{a.h1}}
		}
		if err := writeJSON(filepath.Join(dir, o.version+".json"), vdoc); err != nil {
			return err
		}

		idx := indexDoc{Versions: map[string]struct{}{}}
		if existing, err := os.ReadFile(filepath.Join(dir, "index.json")); err == nil {
			if err := json.Unmarshal(existing, &idx); err != nil {
				return fmt.Errorf("existing index.json in %s is not valid: %w", dir, err)
			}
			if idx.Versions == nil {
				idx.Versions = map[string]struct{}{}
			}
		}
		idx.Versions[o.version] = struct{}{}
		if err := writeJSON(filepath.Join(dir, "index.json"), idx); err != nil {
			return err
		}
	}
	return nil
}

func findArchives(dist, typ, version string) ([]archive, error) {
	entries, err := os.ReadDir(dist)
	if err != nil {
		return nil, err
	}
	var out []archive
	for _, e := range entries {
		m := archiveRe.FindStringSubmatch(e.Name())
		if m == nil || m[1] != typ || m[2] != version {
			continue
		}
		h1, err := dirhash.HashZip(filepath.Join(dist, e.Name()), dirhash.Hash1)
		if err != nil {
			return nil, fmt.Errorf("hashing %s: %w", e.Name(), err)
		}
		out = append(out, archive{file: e.Name(), os: m[3], arch: m[4], h1: h1})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].file < out[j].file })
	return out, nil
}

func writeJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

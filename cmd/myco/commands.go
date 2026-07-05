package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lockyc/mycelium/internal/audit"
	"github.com/lockyc/mycelium/internal/catalog"
	"github.com/lockyc/mycelium/internal/hub"
	"github.com/lockyc/mycelium/internal/scan"
	"github.com/lockyc/mycelium/internal/transport"
)

func splitRoots(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func runScan(args []string) error {
	fs := flag.NewFlagSet("scan", flag.ContinueOnError)
	roots := fs.String("roots", "", "comma-separated repo roots")
	node := fs.String("node", "", "node id")
	source := fs.String("source", "local-checkout", "source type")
	fallbackHost := fs.String("fallback-host", "", "host for repos with no origin remote")
	excludeOwners := fs.String("exclude-owners", "", "comma-separated owner dirs to skip (e.g. vendor)")
	ref := fs.String("ref", "", "git ref to read sidecars from; falls back to HEAD per-repo when absent (e.g. dev)")
	out := fs.String("out", "", "manifest output path")
	push := fs.String("push", "", "hub URL to POST the manifest to (optional)")
	tokenFile := fs.String("token-file", "", "file holding the hub bearer token")
	if err := fs.Parse(args); err != nil {
		return err
	}
	m, orphans, err := scan.Scan(splitRoots(*roots), scan.Options{
		Node:          *node,
		Source:        *source,
		Now:           time.Now().UTC().Format(time.RFC3339),
		FallbackHost:  *fallbackHost,
		ExcludeOwners: splitRoots(*excludeOwners),
		Ref:           *ref,
	})
	if err != nil {
		return err
	}
	for _, o := range orphans {
		fmt.Fprintln(os.Stderr, "warning: orphan (no committed catalog.toml):", o)
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	if *out == "" {
		fmt.Println(string(data))
	} else {
		if err := os.WriteFile(*out, data, 0o644); err != nil {
			return err
		}
	}
	if *push != "" {
		token := ""
		if *tokenFile != "" {
			b, err := os.ReadFile(*tokenFile)
			if err != nil {
				return err
			}
			token = strings.TrimSpace(string(b))
		}
		if err := transport.Push(*push, token, m); err != nil {
			return err
		}
		fmt.Fprintln(os.Stderr, "pushed manifest to", *push)
	}
	return nil
}

func runBuild(args []string) error {
	fs := flag.NewFlagSet("build", flag.ContinueOnError)
	manifests := fs.String("manifests", "", "dir of manifest .json files")
	overlayPath := fs.String("overlay", "", "overlay.toml path (optional)")
	out := fs.String("out", ".", "output dir")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *manifests == "" {
		return fmt.Errorf("build: --manifests <dir> is required")
	}
	return hub.Build(*manifests, *overlayPath, *out)
}

func runValidate(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: myco validate <catalog.toml>")
	}
	data, err := os.ReadFile(args[0])
	if err != nil {
		return err
	}
	sc, err := catalog.ParseSidecar(data)
	if err != nil {
		return err
	}
	fmt.Printf("ok: %s — %s\n", sc.Name, sc.Summary)
	return nil
}

func runAudit(args []string) error {
	fs := flag.NewFlagSet("audit", flag.ContinueOnError)
	catDir := fs.String("catalog", ".", "catalog dir (with catalog.json)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	data, err := os.ReadFile(filepath.Join(*catDir, "catalog.json"))
	if err != nil {
		return err
	}
	var cat catalog.Catalog
	if err := json.Unmarshal(data, &cat); err != nil {
		return err
	}
	var prev []string
	if pd, err := os.ReadFile(filepath.Join(*catDir, "previous.json")); err == nil {
		if err := json.Unmarshal(pd, &prev); err != nil {
			fmt.Fprintln(os.Stderr, "warning: ignoring unreadable previous.json:", err)
		}
	}
	findings := audit.Audit(cat, nil, nil, prev)
	// persist current ids for next run's staleness check
	var ids []string
	for _, c := range cat.Components {
		ids = append(ids, c.ID)
	}
	if idsData, err := json.Marshal(ids); err == nil {
		if err := os.WriteFile(filepath.Join(*catDir, "previous.json"), idsData, 0o644); err != nil {
			fmt.Fprintln(os.Stderr, "warning: could not persist previous.json (staleness check disabled next run):", err)
		}
	}
	for _, f := range findings {
		fmt.Printf("%s: %s\n", f.Kind, f.Detail)
	}
	if len(findings) > 0 {
		return fmt.Errorf("%d audit finding(s)", len(findings))
	}
	fmt.Println("audit: clean")
	return nil
}

func runServe(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	manifests := fs.String("manifests", "", "dir of manifest .json files")
	overlayPath := fs.String("overlay", "", "overlay.toml path (optional)")
	catDir := fs.String("catalog", ".", "catalog output/serve dir")
	ingestToken := fs.String("ingest-token-file", "", "file holding the ingest bearer token")
	addr := fs.String("addr", ":8080", "listen address")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *manifests == "" {
		return fmt.Errorf("serve: --manifests <dir> is required")
	}
	token := ""
	if *ingestToken != "" {
		b, err := os.ReadFile(*ingestToken)
		if err != nil {
			return err
		}
		token = strings.TrimSpace(string(b))
	}
	fmt.Println("serving catalog + ingest on", *addr)
	return hub.Serve(*manifests, *overlayPath, *catDir, token, *addr)
}

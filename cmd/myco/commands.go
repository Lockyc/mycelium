package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lockyc/mycelium/internal/audit"
	"github.com/lockyc/mycelium/internal/catalog"
	"github.com/lockyc/mycelium/internal/scan"
	"github.com/lockyc/mycelium/internal/serve"
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
	out := fs.String("out", "", "manifest output path")
	if err := fs.Parse(args); err != nil {
		return err
	}
	m, orphans, err := scan.Scan(splitRoots(*roots), *node, *source, time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		return err
	}
	for _, o := range orphans {
		fmt.Fprintln(os.Stderr, "warning: orphan (no catalog.toml):", o)
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	if *out == "" {
		fmt.Println(string(data))
		return nil
	}
	return os.WriteFile(*out, data, 0o644)
}

func loadManifests(dir string) ([]catalog.Manifest, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var ms []catalog.Manifest
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		var m catalog.Manifest
		if err := json.Unmarshal(data, &m); err != nil {
			return nil, err
		}
		ms = append(ms, m)
	}
	return ms, nil
}

func runBuild(args []string) error {
	fs := flag.NewFlagSet("build", flag.ContinueOnError)
	manifests := fs.String("manifests", "", "dir of manifest .json files")
	overlayPath := fs.String("overlay", "", "overlay.toml path (optional)")
	out := fs.String("out", ".", "output dir")
	if err := fs.Parse(args); err != nil {
		return err
	}
	ms, err := loadManifests(*manifests)
	if err != nil {
		return err
	}
	var ov catalog.Overlay
	if *overlayPath != "" {
		data, err := os.ReadFile(*overlayPath)
		if err != nil {
			return err
		}
		if ov, err = catalog.ParseOverlay(data); err != nil {
			return err
		}
	}
	cat := catalog.Merge(ms, ov)
	if err := os.MkdirAll(*out, 0o755); err != nil {
		return err
	}
	jsonData, err := catalog.RenderJSON(cat)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(*out, "catalog.json"), jsonData, 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(*out, "CATALOG.md"), []byte(catalog.RenderMarkdown(cat)), 0o644)
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
	addr := fs.String("addr", ":8080", "listen address")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := runBuild([]string{"--manifests", *manifests, "--overlay", *overlayPath, "--out", *catDir}); err != nil {
		return err
	}
	fmt.Println("serving catalog on", *addr)
	return http.ListenAndServe(*addr, serve.Handler(*catDir))
}

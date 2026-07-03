package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestBuildProducesCatalog(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "widgets")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, a := range [][]string{{"init", "-q"}, {"remote", "add", "origin", "git@github.com:acme/widgets.git"},
		{"-c", "user.email=t@t", "-c", "user.name=t", "commit", "--allow-empty", "-q", "-m", "x"}} {
		c := exec.Command("git", a...)
		c.Dir = repo
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", a, err, out)
		}
	}
	if err := os.WriteFile(filepath.Join(repo, "catalog.toml"), []byte("name=\"widgets\"\nsummary=\"cur\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	manDir := t.TempDir()
	outDir := t.TempDir()
	if err := runScan([]string{"--roots", root, "--node", "test", "--out", filepath.Join(manDir, "m.json")}); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if err := runBuild([]string{"--manifests", manDir, "--out", outDir}); err != nil {
		t.Fatalf("build: %v", err)
	}
	if _, err := os.Stat(filepath.Join(outDir, "CATALOG.md")); err != nil {
		t.Fatalf("no CATALOG.md: %v", err)
	}
	if _, err := os.Stat(filepath.Join(outDir, "catalog.json")); err != nil {
		t.Fatalf("no catalog.json: %v", err)
	}
}

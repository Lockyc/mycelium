package main

import (
	"encoding/json"
	"errors"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lockyc/mycelium/internal/graph"
	"github.com/lockyc/mycelium/internal/hub"
)

func TestBuildProducesArtifacts(t *testing.T) {
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
	if err := os.WriteFile(filepath.Join(repo, "mycelium.toml"), []byte("name=\"widgets\"\nsummary=\"cur\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	manDir := t.TempDir()
	outDir := t.TempDir()
	if err := runScan([]string{"--roots", root, "--node", "test", "--out", filepath.Join(manDir, "m.json")}); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if err := runBuild([]string{"--manifests", manDir, "--dir", outDir}); err != nil {
		t.Fatalf("build: %v", err)
	}
	if _, err := os.Stat(filepath.Join(outDir, "MAP.md")); err != nil {
		t.Fatalf("no MAP.md: %v", err)
	}
	if _, err := os.Stat(filepath.Join(outDir, "graph.json")); err != nil {
		t.Fatalf("no graph.json: %v", err)
	}
}

// TestBuildThenAudit exercises the build -> audit round-trip: runAudit must
// be able to read whatever artifact runBuild actually writes. This is the
// regression gate for the artifact-name rename — it fails with a
// file-not-found error if runAudit reads a filename runBuild doesn't write.
func TestBuildThenAudit(t *testing.T) {
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
	if err := os.WriteFile(filepath.Join(repo, "mycelium.toml"), []byte("name=\"widgets\"\nsummary=\"cur\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	manDir := t.TempDir()
	outDir := t.TempDir()
	if err := runScan([]string{"--roots", root, "--node", "test", "--out", filepath.Join(manDir, "m.json")}); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if err := runBuild([]string{"--manifests", manDir, "--dir", outDir}); err != nil {
		t.Fatalf("build: %v", err)
	}

	err := runAudit([]string{"--dir", outDir})
	if err != nil && errors.Is(err, os.ErrNotExist) {
		t.Fatalf("runAudit could not read the build artifact: %v", err)
	}
	// A non-nil, non-ErrNotExist error here just means the audit found
	// findings (e.g. an unreachable orphan) — that's expected output, not
	// a failure of this test, which only asserts the artifact was readable.
}

func TestValidateLintsEnums(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "good.toml")
	bad := filepath.Join(dir, "bad.toml")
	os.WriteFile(good, []byte("name=\"x\"\nsummary=\"s\"\nkind=\"service\"\n"), 0o644)
	os.WriteFile(bad, []byte("name=\"x\"\nsummary=\"s\"\nkind=\"servce\"\n"), 0o644)
	if err := runValidate([]string{good}); err != nil {
		t.Fatalf("valid sidecar rejected: %v", err)
	}
	if err := runValidate([]string{bad}); err == nil {
		t.Fatal("kind typo passed validate")
	}
}

// One broken sidecar on a node must not stall the node: `scan --push` still
// delivers every other repo to the hub, and `myco audit` over the rebuilt graph
// reports the broken one as an invalid-sidecar finding.
func TestScanPushSurvivesBrokenSidecarAndAuditReportsIt(t *testing.T) {
	root := t.TempDir()
	mkRepo := func(name, sidecar string) {
		repo := filepath.Join(root, "acme", name)
		if err := os.MkdirAll(repo, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(repo, "mycelium.toml"), []byte(sidecar), 0o644); err != nil {
			t.Fatal(err)
		}
		for _, a := range [][]string{{"init", "-q"}, {"remote", "add", "origin", "git@github.com:acme/" + name + ".git"},
			{"add", "mycelium.toml"}, {"-c", "user.email=t@t", "-c", "user.name=t", "commit", "-q", "-m", "x"}} {
			c := exec.Command("git", a...)
			c.Dir = repo
			if out, err := c.CombinedOutput(); err != nil {
				t.Fatalf("git %v: %v %s", a, err, out)
			}
		}
	}
	mkRepo("good", "name=\"good\"\nsummary=\"fine\"\n")
	mkRepo("broken", "name=\"broken\"\n") // missing the required summary

	manDir := t.TempDir()
	outDir := t.TempDir()
	srv := httptest.NewServer(hub.Handler(manDir, "", outDir, ""))
	defer srv.Close()
	if err := runScan([]string{"--roots", root, "--node", "n", "--out", filepath.Join(t.TempDir(), "m.json"), "--push", srv.URL}); err != nil {
		t.Fatalf("scan --push failed on a node with one broken sidecar: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(outDir, "graph.json"))
	if err != nil {
		t.Fatal(err)
	}
	var g graph.Graph
	if err := json.Unmarshal(data, &g); err != nil {
		t.Fatal(err)
	}
	if len(g.Components) != 1 || g.Components[0].Name != "good" {
		t.Fatalf("hub graph components = %+v, want the good repo pushed", g.Components)
	}

	out := captureStdout(t, func() error {
		if err := runAudit([]string{"--dir", outDir}); err == nil {
			return errors.New("audit reported clean despite a broken sidecar")
		}
		return nil
	})
	if !strings.Contains(out, "invalid-sidecar: github.com/acme/broken") {
		t.Fatalf("audit output lacks the invalid-sidecar finding:\n%s", out)
	}
}

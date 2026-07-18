package scan

import (
	"os"
	"path/filepath"
	"testing"
)

func commitSidecar(t *testing.T, dir, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "mycelium.toml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, dir, "add", "mycelium.toml")
	run(t, dir, "-c", "user.email=t@t", "-c", "user.name=t", "commit", "-q", "-m", "x")
}

func TestScanComponentsOrphansAndDenylist(t *testing.T) {
	root := t.TempDir()

	// own repo with a committed sidecar (working tree)
	widgets := filepath.Join(root, "acme", "widgets")
	mkWorking(t, widgets)
	run(t, widgets, "remote", "add", "origin", "git@github.com:acme/widgets.git")
	commitSidecar(t, widgets, "name=\"widgets\"\nsummary=\"w\"\n")

	// own repo with NO sidecar -> orphan
	gadgets := filepath.Join(root, "acme", "gadgets")
	mkWorking(t, gadgets)
	run(t, gadgets, "remote", "add", "origin", "git@github.com:acme/gadgets.git")
	run(t, gadgets, "-c", "user.email=t@t", "-c", "user.name=t", "commit", "-q", "--allow-empty", "-m", "x")

	// vendor repo (denied) -> neither component nor orphan
	up := filepath.Join(root, "vendor", "upstream")
	mkWorking(t, up)
	run(t, up, "-c", "user.email=t@t", "-c", "user.name=t", "commit", "-q", "--allow-empty", "-m", "x")

	m, err := Scan([]string{root}, Options{
		Node: "test", Source: "local", Now: "2026-07-04T00:00:00Z",
		FallbackHost: "git.example.com", ExcludeOwners: []string{"vendor"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Components) != 1 || m.Components[0].ID != "github.com/acme/widgets" {
		t.Fatalf("components = %+v", m.Components)
	}
	if len(m.Orphans) != 1 || m.Orphans[0].ID != "github.com/acme/gadgets" ||
		filepath.Base(m.Orphans[0].Path) != "gadgets" {
		t.Fatalf("orphans = %+v", m.Orphans)
	}
}

func TestScanAttachesDocGraph(t *testing.T) {
	root := t.TempDir()
	widget := filepath.Join(root, "owner", "widget")
	mkWorking(t, widget)
	run(t, widget, "remote", "add", "origin", "git@github.com:owner/widget.git")
	commitSidecar(t, widget, "name=\"widget\"\nsummary=\"a widget\"\n")

	fake := func(repoPath string) ([]byte, error) {
		return []byte(schemaV1WithDocs), nil
	}
	m, err := Scan([]string{root}, Options{Node: "n", DocGraph: fake})
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Components) != 1 {
		t.Fatalf("want 1 component, got %d", len(m.Components))
	}
	c := m.Components[0]
	if c.DocGraph == nil || c.DocGraph.DocCount != 3 {
		t.Fatalf("digest not attached: %+v", c.DocGraph)
	}
	if _, ok := m.DocGraphs[c.ID]; !ok {
		t.Fatalf("full payload not stashed under id %q; keys=%v", c.ID, m.DocGraphs)
	}
}

func TestScanDocGraphErrorIsNonFatal(t *testing.T) {
	root := t.TempDir()
	widget := filepath.Join(root, "owner", "widget")
	mkWorking(t, widget)
	run(t, widget, "remote", "add", "origin", "git@github.com:owner/widget.git")
	commitSidecar(t, widget, "name=\"widget\"\nsummary=\"a widget\"\n")

	boom := func(repoPath string) ([]byte, error) { return nil, errDocGraphNotInstalled }
	m, err := Scan([]string{root}, Options{Node: "n", DocGraph: boom})
	if err != nil {
		t.Fatalf("docgraph failure must not fail the scan: %v", err)
	}
	if len(m.Components) != 1 || m.Components[0].DocGraph != nil {
		t.Fatalf("failed docgraph → component with no digest; got %+v", m.Components)
	}
	if m.DocGraphs != nil {
		t.Fatalf("no payloads expected, got %v", m.DocGraphs)
	}
}

func TestScanRefPrefersBranchWithHEADFallback(t *testing.T) {
	root := t.TempDir()

	// Repo A: mycelium.toml exists ONLY on dev; the default branch (main) has none.
	a := filepath.Join(root, "acme", "onlydev")
	if err := os.MkdirAll(a, 0o755); err != nil {
		t.Fatal(err)
	}
	run(t, a, "init", "-q", "-b", "main")
	run(t, a, "remote", "add", "origin", "git@github.com:acme/onlydev.git")
	run(t, a, "-c", "user.email=t@t", "-c", "user.name=t", "commit", "-q", "--allow-empty", "-m", "init")
	run(t, a, "checkout", "-q", "-b", "dev")
	commitSidecar(t, a, "name=\"onlydev\"\nsummary=\"d\"\n")
	run(t, a, "checkout", "-q", "main") // leave HEAD on main (no sidecar)

	// Repo B: mycelium.toml on main only, no dev branch — --ref dev must fall back to HEAD.
	b := filepath.Join(root, "acme", "onlymain")
	if err := os.MkdirAll(b, 0o755); err != nil {
		t.Fatal(err)
	}
	run(t, b, "init", "-q", "-b", "main")
	run(t, b, "remote", "add", "origin", "git@github.com:acme/onlymain.git")
	commitSidecar(t, b, "name=\"onlymain\"\nsummary=\"m\"\n")

	// With Ref="dev": A is read from dev (found); B has no dev, falls back to HEAD (found).
	m, err := Scan([]string{root}, Options{
		Node: "test", Source: "local", Now: "t",
		FallbackHost: "git.example.com", Ref: "dev",
	})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, c := range m.Components {
		got[c.Name] = true
	}
	if !got["onlydev"] || !got["onlymain"] {
		t.Fatalf("ref=dev: want onlydev+onlymain, got components=%+v orphans=%v", m.Components, m.Orphans)
	}

	// Without Ref: A is an orphan (main has no sidecar); B still found via HEAD.
	m2, err := Scan([]string{root}, Options{
		Node: "test", Source: "local", Now: "t", FallbackHost: "git.example.com",
	})
	if err != nil {
		t.Fatal(err)
	}
	got2 := map[string]bool{}
	for _, c := range m2.Components {
		got2[c.Name] = true
	}
	if got2["onlydev"] {
		t.Fatalf("no ref: onlydev should be orphan (main has no sidecar), got %+v", m2.Components)
	}
	if !got2["onlymain"] {
		t.Fatalf("no ref: onlymain should be found via HEAD, got %+v", m2.Components)
	}
}

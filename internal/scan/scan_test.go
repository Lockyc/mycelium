package scan

import (
	"os"
	"path/filepath"
	"testing"
)

func commitSidecar(t *testing.T, dir, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "catalog.toml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, dir, "add", "catalog.toml")
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

	m, orphans, err := Scan([]string{root}, Options{
		Node: "test", Source: "local", Now: "2026-07-04T00:00:00Z",
		FallbackHost: "git.example.com", ExcludeOwners: []string{"vendor"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Components) != 1 || m.Components[0].ID != "github.com/acme/widgets" {
		t.Fatalf("components = %+v", m.Components)
	}
	if len(orphans) != 1 || filepath.Base(orphans[0]) != "gadgets" {
		t.Fatalf("orphans = %v", orphans)
	}
}

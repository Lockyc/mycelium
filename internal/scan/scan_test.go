package scan

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func initRepo(t *testing.T, dir, remote string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"init", "-q"},
		{"remote", "add", "origin", remote},
		{"-c", "user.email=t@t", "-c", "user.name=t", "commit", "--allow-empty", "-q", "-m", "x"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
}

func TestScanReadsSidecarAndOrphans(t *testing.T) {
	root := t.TempDir()
	withSidecar := filepath.Join(root, "widgets")
	initRepo(t, withSidecar, "git@github.com:acme/widgets.git")
	if err := os.WriteFile(filepath.Join(withSidecar, "catalog.toml"),
		[]byte("name=\"widgets\"\nsummary=\"x\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	orphanRepo := filepath.Join(root, "gadgets")
	initRepo(t, orphanRepo, "git@github.com:acme/gadgets.git")

	m, orphans, err := Scan([]string{root}, "test", "local-checkout", "2026-07-03T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Components) != 1 {
		t.Fatalf("want 1 component, got %d", len(m.Components))
	}
	if m.Components[0].ID != "github.com/acme/widgets" {
		t.Fatalf("bad id: %q", m.Components[0].ID)
	}
	if len(orphans) != 1 || filepath.Base(orphans[0]) != "gadgets" {
		t.Fatalf("bad orphans: %v", orphans)
	}
}

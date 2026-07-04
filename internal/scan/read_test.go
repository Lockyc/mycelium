package scan

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSidecarAtHEADAndID(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "acme", "widgets")
	mkWorking(t, dir)
	run(t, dir, "remote", "add", "origin", "git@github.com:acme/widgets.git")
	if err := os.WriteFile(filepath.Join(dir, "catalog.toml"), []byte("name=\"widgets\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	r := Repo{Dir: dir, Owner: "acme", Name: "widgets"}

	// Not committed yet -> not in HEAD -> found=false.
	if _, found, err := sidecarAtHEAD(r); err != nil || found {
		t.Fatalf("uncommitted: found=%v err=%v (want found=false)", found, err)
	}
	run(t, dir, "add", "catalog.toml")
	run(t, dir, "-c", "user.email=t@t", "-c", "user.name=t", "commit", "-q", "-m", "x")

	data, found, err := sidecarAtHEAD(r)
	if err != nil || !found {
		t.Fatalf("committed: found=%v err=%v (want found=true)", found, err)
	}
	if string(data) == "" {
		t.Fatal("empty sidecar data")
	}
	if got := repoID(r, "git.example.com"); got != "github.com/acme/widgets" {
		t.Fatalf("id from origin = %q", got)
	}

	// A repo with no origin falls back to <host>/<owner>/<name>.
	bare := filepath.Join(root, "acme", "native.git")
	mkBare(t, bare)
	rb := Repo{Dir: bare, Owner: "acme", Name: "native", Bare: true}
	if got := repoID(rb, "git.example.com"); got != "git.example.com/acme/native" {
		t.Fatalf("fallback id = %q", got)
	}
}

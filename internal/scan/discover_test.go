package scan

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"testing"
)

func mkWorking(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	run(t, dir, "init", "-q")
}

func mkBare(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	run(t, dir, "init", "-q", "--bare")
}

func run(t *testing.T, dir string, args ...string) {
	t.Helper()
	c := exec.Command("git", args...)
	c.Dir = dir
	if out, err := c.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func TestDiscoverReposBareAndWorking(t *testing.T) {
	root := t.TempDir()
	mkWorking(t, filepath.Join(root, "acme", "widgets"))     // working tree
	mkBare(t, filepath.Join(root, "vendor", "upstream.git")) // bare
	// a worktree dir that must be skipped entirely:
	mkWorking(t, filepath.Join(root, "acme", "widgets", ".claude", "worktrees", "feat"))

	repos, err := DiscoverRepos([]string{root})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]Repo{}
	var names []string
	for _, r := range repos {
		got[r.Name] = r
		names = append(names, r.Name)
	}
	sort.Strings(names)
	if len(repos) != 2 {
		t.Fatalf("want 2 repos, got %d: %v", len(repos), names)
	}
	if w := got["widgets"]; w.Owner != "acme" || w.Bare {
		t.Errorf("widgets: %+v", w)
	}
	if u := got["upstream"]; u.Owner != "vendor" || !u.Bare {
		t.Errorf("upstream: %+v (want bare, owner vendor, name upstream)", u)
	}
}

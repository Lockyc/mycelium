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

// One unreadable directory in a shared store must not abort discovery of the
// rest; only an unreadable root is fatal.
func TestDiscoverReposSkipsUnreadableSubdir(t *testing.T) {
	root := t.TempDir()
	mkWorking(t, filepath.Join(root, "acme", "widgets"))
	locked := filepath.Join(root, "locked")
	if err := os.MkdirAll(filepath.Join(locked, "inner"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(locked, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(locked, 0o755) })
	if _, err := os.ReadDir(locked); err == nil {
		t.Skip("running with privileges that ignore directory permissions")
	}

	repos, err := DiscoverRepos([]string{root})
	if err != nil {
		t.Fatalf("an unreadable subdir aborted discovery: %v", err)
	}
	if len(repos) != 1 || repos[0].Name != "widgets" {
		t.Fatalf("want widgets discovered past the unreadable dir, got %+v", repos)
	}
	if _, err := DiscoverRepos([]string{filepath.Join(root, "missing")}); err == nil {
		t.Fatal("a missing root must still fail discovery")
	}
}

package scan

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Repo struct {
	Dir   string // worktree root (working) or bare git dir (bare)
	Owner string // immediate parent directory name
	Name  string // repo basename, ".git" stripped
	Bare  bool
}

func (r Repo) Git(args ...string) *exec.Cmd {
	if r.Bare {
		return exec.Command("git", append([]string{"--git-dir", r.Dir}, args...)...)
	}
	return exec.Command("git", append([]string{"-C", r.Dir}, args...)...)
}

func isBareGitDir(dir string) bool {
	if _, err := os.Stat(filepath.Join(dir, "HEAD")); err != nil {
		return false
	}
	if _, err := os.Stat(filepath.Join(dir, "objects")); err != nil {
		return false
	}
	return true
}

// DiscoverRepos walks each root and returns every git repo found at any depth:
// a working tree (has a .git subdir) or a bare repo (has HEAD + objects). It does
// not descend into a discovered repo, and skips any .claude/worktrees path.
func DiscoverRepos(roots []string) ([]Repo, error) {
	var out []Repo
	for _, root := range roots {
		err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				// An unreadable root is a misconfigured scan; an unreadable dir
				// below it is one corner of the store, so skip it and scan the rest.
				if path == root {
					return err
				}
				fmt.Fprintln(os.Stderr, "warning: skipping unreadable", path+":", err)
				return filepath.SkipDir
			}
			if !d.IsDir() {
				return nil
			}
			// Skip agent worktree checkouts entirely.
			if strings.Contains(path, filepath.Join(".claude", "worktrees")) {
				return filepath.SkipDir
			}
			owner := filepath.Base(filepath.Dir(path))
			name := strings.TrimSuffix(filepath.Base(path), ".git")
			if _, err := os.Stat(filepath.Join(path, ".git")); err == nil {
				out = append(out, Repo{Dir: path, Owner: owner, Name: name, Bare: false})
				return filepath.SkipDir
			}
			if isBareGitDir(path) {
				out = append(out, Repo{Dir: path, Owner: owner, Name: name, Bare: true})
				return filepath.SkipDir
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

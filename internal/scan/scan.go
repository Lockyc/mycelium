package scan

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/lockyc/mycelium/internal/catalog"
)

func gitOutput(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}

// Scan walks each root one level deep. A child dir with a .git is a repo:
// if it has catalog.toml it becomes a component; otherwise its path is an orphan.
func Scan(roots []string, node, source, now string) (catalog.Manifest, []string, error) {
	m := catalog.Manifest{Node: node, Source: source, ScannedAt: now}
	var orphans []string
	for _, root := range roots {
		entries, err := os.ReadDir(root)
		if err != nil {
			return catalog.Manifest{}, nil, err
		}
		for _, e := range entries {
			if !e.IsDir() || e.Name() == ".claude" {
				continue
			}
			dir := filepath.Join(root, e.Name())
			if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
				continue // not a repo
			}
			scPath := filepath.Join(dir, "catalog.toml")
			data, err := os.ReadFile(scPath)
			if err != nil {
				orphans = append(orphans, dir)
				continue
			}
			sc, err := catalog.ParseSidecar(data)
			if err != nil {
				return catalog.Manifest{}, nil, err
			}
			remote, err := gitOutput(dir, "remote", "get-url", "origin")
			if err != nil || strings.TrimSpace(remote) == "" {
				// No resolvable origin → no stable catalog identity. Surface as an
				// orphan rather than emitting a component with an empty ID, which
				// would collapse distinct repos together during merge dedup.
				orphans = append(orphans, dir)
				continue
			}
			commit, _ := gitOutput(dir, "rev-parse", "HEAD")
			m.Components = append(m.Components, catalog.Component{
				ID:      catalog.CanonicalID(remote),
				Name:    sc.Name,
				Path:    dir,
				Commit:  commit,
				Sidecar: sc,
			})
		}
	}
	return m, orphans, nil
}

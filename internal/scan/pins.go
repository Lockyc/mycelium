package scan

import (
	"path"
	"sort"
	"strings"

	"github.com/pelletier/go-toml/v2"

	"github.com/lockyc/mycelium/internal/graph"
)

// DetectPins lists the repos a repo pins as git dependencies in any Cargo.toml
// at ref, as canonical ids. It reads the git object store only, like
// DetectFacts, so the pins describe the scanned ref and not the worktree.
func DetectPins(r Repo, ref string) []string {
	out, err := r.Git("ls-tree", "-r", "--name-only", ref).Output()
	if err != nil {
		return nil
	}
	seen := map[string]bool{}
	for _, p := range strings.Split(string(out), "\n") {
		if p == "" || inVendored(p) || path.Base(p) != "Cargo.toml" {
			continue
		}
		b, err := r.Git("show", ref+":"+p).Output()
		if err != nil {
			continue
		}
		for _, u := range cargoGitURLs(string(b)) {
			seen[graph.CanonicalID(u)] = true
		}
	}
	var pins []string
	for id := range seen {
		pins = append(pins, id)
	}
	sort.Strings(pins)
	return pins
}

// cargoGitURLs returns the `git =` URL of every dependency table in a
// Cargo.toml: dependencies, dev-dependencies, build-dependencies, their
// [workspace] and [target.*] forms.
func cargoGitURLs(src string) []string {
	type deps map[string]any
	type tables struct {
		Dependencies      deps `toml:"dependencies"`
		DevDependencies   deps `toml:"dev-dependencies"`
		BuildDependencies deps `toml:"build-dependencies"`
	}
	var c struct {
		tables
		Workspace struct {
			Dependencies deps `toml:"dependencies"`
		} `toml:"workspace"`
		Target map[string]tables `toml:"target"`
	}
	if toml.Unmarshal([]byte(src), &c) != nil {
		return nil
	}
	all := []deps{c.Dependencies, c.DevDependencies, c.BuildDependencies, c.Workspace.Dependencies}
	for _, t := range c.Target {
		all = append(all, t.Dependencies, t.DevDependencies, t.BuildDependencies)
	}
	var urls []string
	for _, d := range all {
		for _, v := range d {
			if m, ok := v.(map[string]any); ok {
				if g, ok := m["git"].(string); ok && g != "" {
					urls = append(urls, g)
				}
			}
		}
	}
	return urls
}

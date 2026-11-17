package scan

import (
	"encoding/json"
	"path"
	"regexp"
	"sort"
	"strings"

	"github.com/lockyc/mycelium/internal/graph"
)

var cargoVersion = regexp.MustCompile(`(?m)^version\s*=`)

// DetectFacts derives what a repo builds from its committed tree at ref. It
// reads only the git object store (one ls-tree listing plus a few `git show`
// reads), never a worktree, so it behaves the same on a bare repo. Visibility
// is not derivable from the tree and is left empty.
func DetectFacts(r Repo, ref string) graph.Facts {
	out, err := r.Git("ls-tree", "-r", "--name-only", ref).Output()
	if err != nil {
		return graph.Facts{}
	}
	var files []string
	for _, p := range strings.Split(string(out), "\n") {
		if p == "" || inVendored(p) {
			continue
		}
		files = append(files, p)
	}
	show := func(p string) string {
		b, err := r.Git("show", ref+":"+p).Output()
		if err != nil {
			return ""
		}
		return string(b)
	}

	var f graph.Facts
	eco := map[string]bool{}
	var tauri, sparkle, electron bool
	for _, p := range files {
		base := path.Base(p)
		switch {
		case base == "go.mod":
			eco["go"] = true
		case base == "Cargo.toml":
			eco["rust"] = true
		case base == "package.json":
			eco["node"] = true
		case base == "Package.swift", base == "project.yml", strings.Contains(p, ".xcodeproj/"):
			eco["swift"] = true
		}
		switch {
		case base == "tauri.conf.json":
			f.MacApp = true
			if strings.Contains(show(p), `"updater"`) {
				tauri = true
			}
		case base == "project.yml":
			s := show(p)
			if strings.Contains(s, "platform: macOS") && strings.Contains(s, "type: application") {
				f.MacApp = true
			}
			if strings.Contains(s, "SUPublicEDKey") {
				sparkle = true
			}
		case base == "project.pbxproj":
			s := show(p)
			if strings.Contains(s, "SDKROOT = macosx") && strings.Contains(s, "com.apple.product-type.application") {
				f.MacApp = true
			}
		case base == "Info.plist":
			if strings.Contains(show(p), "SUPublicEDKey") {
				sparkle = true
			}
		case strings.HasPrefix(base, "electron-builder."):
			if strings.Contains(show(p), `"publish"`) {
				electron = true
			}
		case p == "justfile":
			if strings.Contains(show(p), "/Applications") {
				f.MacApp = true
			}
		case p == "VERSION":
			f.VersionSource = true
		}
		if p == "Cargo.toml" && cargoVersion.MatchString(show(p)) {
			f.VersionSource = true
		}
		if p == "package.json" {
			var pj struct {
				Version string `json:"version"`
			}
			if json.Unmarshal([]byte(show(p)), &pj) == nil && pj.Version != "" {
				f.VersionSource = true
			}
		}
	}
	switch {
	case tauri:
		f.Updater = "tauri"
	case sparkle:
		f.Updater = "sparkle"
	case electron:
		f.Updater = "electron"
	}
	for e := range eco {
		f.Ecosystems = append(f.Ecosystems, e)
	}
	sort.Strings(f.Ecosystems)
	return f
}

// inVendored reports whether p sits under a dependency directory whose
// manifests describe someone else's project.
func inVendored(p string) bool {
	for _, seg := range strings.Split(p, "/") {
		if seg == "node_modules" || seg == "vendor" {
			return true
		}
	}
	return false
}

package scan

import (
	"encoding/json"
	"path"
	"regexp"
	"sort"
	"strings"

	"github.com/pelletier/go-toml/v2"

	"github.com/lockyc/mycelium/internal/graph"
)

// cargoVersion reports how a Cargo.toml versions its package: a literal string
// in [package] or [workspace.package], or `version.workspace = true`. Parsed as
// TOML so a `version` key under [dependencies.x] is never mistaken for it.
func cargoVersion(src string) (literal, inherits bool, workspace string) {
	var c struct {
		Package struct {
			Version any `toml:"version"`
		} `toml:"package"`
		Workspace struct {
			Package struct {
				Version string `toml:"version"`
			} `toml:"package"`
		} `toml:"workspace"`
	}
	if toml.Unmarshal([]byte(src), &c) != nil {
		return false, false, ""
	}
	workspace = c.Workspace.Package.Version
	switch v := c.Package.Version.(type) {
	case string:
		literal = v != ""
	case map[string]any:
		inherits = v["workspace"] == true
	}
	return literal || workspace != "", inherits, workspace
}

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
	var tauriDirs []string
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
			s := show(p)
			if strings.Contains(s, `"updater"`) {
				tauri = true
			}
			var tc struct {
				Version string `json:"version"`
			}
			if json.Unmarshal([]byte(s), &tc) == nil && tc.Version != "" {
				f.VersionSource = true
			}
			tauriDirs = append(tauriDirs, path.Dir(p))
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
			s := show(p)
			if strings.Contains(s, "SUPublicEDKey") {
				sparkle = true
			}
			if !inTests(p) && strings.Contains(s, "LSMinimumSystemVersion") && plistAPPL.MatchString(s) {
				f.MacApp = true
			}
		case strings.HasPrefix(base, "electron-builder."):
			if strings.Contains(show(p), "publish") {
				electron = true
			}
		case p == "justfile":
			if justfileInstallsApp(show(p)) {
				f.MacApp = true
			}
		case p == "VERSION":
			f.VersionSource = true
		}
		if p == "Cargo.toml" {
			if lit, _, _ := cargoVersion(show(p)); lit {
				f.VersionSource = true
			}
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
	// A Tauri app versions in the Cargo.toml beside its tauri.conf.json,
	// possibly inheriting the root workspace version.
	if !f.VersionSource && len(tauriDirs) > 0 {
		_, _, ws := cargoVersion(show("Cargo.toml"))
		for _, d := range tauriDirs {
			lit, inh, _ := cargoVersion(show(path.Join(d, "Cargo.toml")))
			if (lit && !inh) || (inh && ws != "") {
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

// plistAPPL matches an application bundle's CFBundlePackageType. With the
// macOS-only LSMinimumSystemVersion key it marks a Mac app (iOS plists carry
// MinimumOSVersion instead).
var plistAPPL = regexp.MustCompile(`<key>CFBundlePackageType</key>\s*<string>APPL</string>`)

// inTests reports whether p sits under a Tests directory, whose plists are fixtures.
func inTests(p string) bool {
	for _, seg := range strings.Split(p, "/") {
		if seg == "Tests" || seg == "tests" {
			return true
		}
	}
	return false
}

// justAssign matches a justfile `name := value` assignment.
var justAssign = regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_-]*)\s*:?=\s*(.*)$`)

// installVerb matches a copy/install command as a whole word, so `mvp` or
// `install-app.sh` is not one.
var installVerb = regexp.MustCompile(`(^|[^A-Za-z0-9_.-])(cp|ditto|mv|rsync|install)([^A-Za-z0-9_.-]|$)`)

// justfileInstallsApp reports whether a justfile puts an app into
// /Applications: a non-comment line that runs a copy/install verb and names
// /Applications, directly or through a variable assigned a /Applications path,
// or one that runs the shared install-app.sh installer. Merely naming
// /Applications (to locate another app) is not an install.
func justfileInstallsApp(src string) bool {
	var lines, appVars []string
	for _, line := range strings.Split(src, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		lines = append(lines, line)
		if m := justAssign.FindStringSubmatch(line); m != nil && strings.Contains(m[2], "/Applications") {
			appVars = append(appVars, "{{"+m[1]+"}}")
		}
	}
	for _, line := range lines {
		if strings.Contains(line, "install-app.sh") {
			return true
		}
		if !installVerb.MatchString(line) {
			continue
		}
		if strings.Contains(line, "/Applications") {
			return true
		}
		for _, v := range appVars {
			if strings.Contains(line, v) {
				return true
			}
		}
	}
	return false
}

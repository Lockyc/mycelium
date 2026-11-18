package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/rogpeppe/go-internal/testscript"
)

var updateScripts = flag.Bool("update", false, "rewrite the golden sections of testdata/script/*.txtar")

// TestMain puts myco and a fake docgraph on PATH for the whole test binary, so
// the in-process tests and the scripts alike read a fixed doc-graph rather than
// whatever docgraph this machine has installed.
func TestMain(m *testing.M) {
	os.Exit(testscript.RunMain(m, map[string]func() int{
		"myco":     func() int { main(); return 0 },
		"docgraph": fakeDocgraph,
	}))
}

// fakeDocgraph answers `docgraph graph --json --ref <ref>` with a fixed
// schemaVersion-1 payload: one README, no edges, no islands.
func fakeDocgraph() int {
	fmt.Println(`{"schemaVersion":1,"nodes":[{"path":"README.md"}],"contentEdges":[],"metadataEdges":[],"islands":{"content":[],"metadata":[]}}`)
	return 0
}

// TestScripts runs testdata/script/*.txtar against the built myco. `go test
// ./cmd/myco -run TestScripts -update` rewrites their golden sections.
func TestScripts(t *testing.T) {
	examples, err := filepath.Abs(filepath.Join("..", "..", "examples"))
	if err != nil {
		t.Fatal(err)
	}
	testscript.Run(t, testscript.Params{
		Dir:           filepath.Join("testdata", "script"),
		UpdateScripts: *updateScripts,
		Setup: func(env *testscript.Env) error {
			env.Setenv("HOME", env.WorkDir)
			env.Setenv("GIT_CONFIG_NOSYSTEM", "1")
			env.Setenv("EXAMPLES", examples)
			// Fixed identity and dates make every commit id, and so the goldens, stable.
			for k, v := range map[string]string{
				"GIT_AUTHOR_NAME": "demo", "GIT_AUTHOR_EMAIL": "demo@example.com",
				"GIT_COMMITTER_NAME": "demo", "GIT_COMMITTER_EMAIL": "demo@example.com",
				"GIT_AUTHOR_DATE": "2026-01-01T00:00:00Z", "GIT_COMMITTER_DATE": "2026-01-01T00:00:00Z",
			} {
				env.Setenv(k, v)
			}
			return nil
		},
		Cmds: map[string]func(*testscript.TestScript, bool, []string){
			"materialise": materialise,
			"normalise":   normalise,
		},
	})
}

// materialise turns each examples/repos entry into a git repo under dest with
// an acme origin, as scripts/demo.sh does, and adds two sidecar-less repos:
// needs-sidecar (an orphan) and scratch (ignored by examples/overlay.toml).
func materialise(ts *testscript.TestScript, neg bool, args []string) {
	if neg || len(args) != 2 {
		ts.Fatalf("usage: materialise <examples dir> <dest>")
	}
	src, dest := args[0], ts.MkAbs(args[1])
	git := func(dir string, a ...string) {
		ts.Check(ts.Exec("git", append([]string{"-C", dir}, a...)...))
	}
	repos, err := os.ReadDir(filepath.Join(src, "repos"))
	ts.Check(err)
	for _, r := range repos {
		dir := filepath.Join(dest, r.Name())
		ts.Check(os.MkdirAll(dir, 0o755))
		files, err := os.ReadDir(filepath.Join(src, "repos", r.Name()))
		ts.Check(err)
		for _, f := range files {
			b, err := os.ReadFile(filepath.Join(src, "repos", r.Name(), f.Name()))
			ts.Check(err)
			ts.Check(os.WriteFile(filepath.Join(dir, f.Name()), b, 0o644))
		}
		git(dir, "init", "-q")
		git(dir, "remote", "add", "origin", "git@github.com:acme/"+r.Name()+".git")
		git(dir, "add", "-A")
		git(dir, "commit", "-q", "-m", "example")
	}
	for _, name := range []string{"needs-sidecar", "scratch"} {
		dir := filepath.Join(dest, name)
		ts.Check(os.MkdirAll(dir, 0o755))
		git(dir, "init", "-q")
		git(dir, "remote", "add", "origin", "git@github.com:acme/"+name+".git")
		git(dir, "commit", "-q", "--allow-empty", "-m", "no sidecar")
	}
}

var rfc3339 = regexp.MustCompile(`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?(Z|[+-]\d{2}:\d{2})`)

// normalise readies a file for cmp against a txtar golden: every RFC 3339
// timestamp becomes <time> (scan stamps its manifest with the wall clock), and
// the file ends in a newline, as every txtar section does.
func normalise(ts *testscript.TestScript, neg bool, args []string) {
	if neg || len(args) != 1 {
		ts.Fatalf("usage: normalise <file>")
	}
	path := ts.MkAbs(args[0])
	b, err := os.ReadFile(path)
	ts.Check(err)
	b = rfc3339.ReplaceAll(b, []byte("<time>"))
	if !bytes.HasSuffix(b, []byte("\n")) {
		b = append(b, '\n')
	}
	ts.Check(os.WriteFile(path, b, 0o644))
}

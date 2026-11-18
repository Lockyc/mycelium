package mycelium

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rogpeppe/go-internal/testscript"
)

func TestMain(m *testing.M) {
	os.Exit(testscript.RunMain(m, map[string]func() int{"gh": fakeGH}))
}

// fakeGH stands in for the GitHub CLI in the release scripts. It appends each
// invocation's argv to $GH_LOG, copies a --notes-file to $GH_NOTES (the recipe
// deletes its own on exit), and answers `repo view` with a fixed URL.
func fakeGH() int {
	args := os.Args[1:]
	f, err := os.OpenFile(os.Getenv("GH_LOG"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Fprintln(f, strings.Join(args, " "))
	f.Close()
	if len(args) >= 2 && args[0] == "repo" && args[1] == "view" {
		fmt.Println("https://github.com/acme/demo")
	}
	for i, a := range args {
		if a == "--notes-file" && i+1 < len(args) {
			b, err := os.ReadFile(args[i+1])
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				return 1
			}
			if err := os.WriteFile(os.Getenv("GH_NOTES"), b, 0o644); err != nil {
				fmt.Fprintln(os.Stderr, err)
				return 1
			}
		}
	}
	return 0
}

// TestRelease runs the real justfile's `release` recipe in throwaway repos
// against a bare origin, one script per way a release has gone wrong.
func TestRelease(t *testing.T) {
	for _, tool := range []string{"just", "git", "go"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s not on PATH", tool)
		}
	}
	justfile, err := filepath.Abs("justfile")
	if err != nil {
		t.Fatal(err)
	}
	goEnv := map[string]string{}
	for _, k := range []string{"GOCACHE", "GOMODCACHE", "GOPATH", "GOROOT"} {
		out, err := exec.Command("go", "env", k).Output()
		if err != nil {
			t.Fatal(err)
		}
		goEnv[k] = strings.TrimSpace(string(out))
	}
	testscript.Run(t, testscript.Params{
		Dir: filepath.Join("testdata", "release"),
		Setup: func(env *testscript.Env) error {
			home := filepath.Join(env.WorkDir, "home")
			if err := os.MkdirAll(home, 0o755); err != nil {
				return err
			}
			gitconfig := "[user]\n\tname = t\n\temail = t@example.com\n[init]\n\tdefaultBranch = main\n[commit]\n\tgpgsign = false\n[tag]\n\tgpgsign = false\n"
			if err := os.WriteFile(filepath.Join(home, ".gitconfig"), []byte(gitconfig), 0o644); err != nil {
				return err
			}
			env.Setenv("HOME", home)
			env.Setenv("GIT_CONFIG_NOSYSTEM", "1")
			env.Setenv("JUSTFILE", justfile)
			env.Setenv("GH_LOG", filepath.Join(env.WorkDir, "gh.log"))
			env.Setenv("GH_NOTES", filepath.Join(env.WorkDir, "gh-notes.md"))
			env.Setenv("GOTOOLCHAIN", "local")
			env.Setenv("GOFLAGS", "")
			for k, v := range goEnv {
				env.Setenv(k, v)
			}
			return nil
		},
		Cmds: map[string]func(*testscript.TestScript, bool, []string){
			"mkrelrepo": mkRelRepo,
		},
	})
}

// mkRelRepo builds the state a release starts from: a bare $WORK/origin.git,
// and a $WORK/repo clone on dev holding a trivial Go module, the real justfile,
// a v0.1.0 tag on main, and one commit bumping VERSION to 0.2.0, pushed.
func mkRelRepo(ts *testscript.TestScript, neg bool, args []string) {
	if neg || len(args) != 0 {
		ts.Fatalf("usage: mkrelrepo")
	}
	work := ts.Getenv("WORK")
	repo := filepath.Join(work, "repo")
	git := func(dir string, a ...string) {
		if err := ts.Exec("git", append([]string{"-C", dir}, a...)...); err != nil {
			ts.Fatalf("git %v: %v", a, err)
		}
	}
	write := func(name, body string) {
		ts.Check(os.WriteFile(filepath.Join(repo, name), []byte(body), 0o644))
	}
	git(work, "init", "-q", "--bare", "origin.git")
	git(work, "clone", "-q", "origin.git", "repo")
	just, err := os.ReadFile(ts.Getenv("JUSTFILE"))
	ts.Check(err)
	write("justfile", string(just))
	write("go.mod", "module example.com/demo\n\ngo 1.22\n")
	write("main.go", "package main\n\nfunc main() {}\n")
	write("VERSION", "0.1.0\n")
	git(repo, "add", "-A")
	git(repo, "commit", "-q", "-m", "init")
	git(repo, "tag", "-a", "v0.1.0", "-m", "v0.1.0")
	git(repo, "push", "-q", "origin", "main", "v0.1.0")
	git(repo, "checkout", "-q", "-b", "dev")
	write("VERSION", "0.2.0\n")
	git(repo, "commit", "-q", "-am", "bump to 0.2.0")
	git(repo, "push", "-q", "-u", "origin", "dev")
}

package scan

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/lockyc/mycelium/internal/graph"
)

// resolveRef picks the git ref to read a repo's sidecar from. An empty ref (or
// one this repo doesn't have) resolves to HEAD — so a node can prefer an active
// trunk like "dev" fleet-wide while repos that only have a default branch fall
// back to HEAD instead of being dropped as orphans.
func resolveRef(r Repo, ref string) string {
	if ref == "" {
		return "HEAD"
	}
	if err := r.Git("rev-parse", "--verify", "--quiet", ref+"^{commit}").Run(); err == nil {
		return ref
	}
	return "HEAD"
}

// sidecarAtRef returns the committed mycelium.toml at ref. found=false only when
// git can open the repo and the file is absent at ref, or ref is unborn (an
// orphan). Any other git failure — a repo git refuses (dubious ownership), a
// corrupt one, git not runnable — is an error, which Scan records as an invalid
// sidecar rather than reporting a repo that may well have a sidecar as an orphan.
func sidecarAtRef(r Repo, ref string) ([]byte, bool, error) {
	if _, err := gitOutput(r, "rev-parse", "--git-dir"); err != nil {
		return nil, false, err
	}
	if r.Git("rev-parse", "--verify", "--quiet", ref+"^{commit}").Run() != nil {
		return nil, false, nil
	}
	listed, err := gitOutput(r, "ls-tree", "--name-only", ref, "--", graph.SidecarName)
	if err != nil {
		return nil, false, err
	}
	if len(bytes.TrimSpace(listed)) == 0 {
		return nil, false, nil
	}
	out, err := gitOutput(r, "show", ref+":"+graph.SidecarName)
	if err != nil {
		return nil, false, err
	}
	return out, true, nil
}

// gitOutput runs git in r and returns stdout. Its error carries git's first
// stderr line with the repo's node-local path replaced by "<repo>", since the
// error rides the manifest downstream.
func gitOutput(r Repo, args ...string) ([]byte, error) {
	out, err := r.Git(args...).Output()
	if err == nil {
		return out, nil
	}
	msg := err.Error()
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		if line, _, _ := strings.Cut(strings.TrimSpace(string(exitErr.Stderr)), "\n"); line != "" {
			msg = line
		}
	}
	paths := []string{r.Dir}
	if resolved, rerr := filepath.EvalSymlinks(r.Dir); rerr == nil {
		paths = append(paths, resolved)
	}
	for _, p := range paths {
		msg = strings.ReplaceAll(msg, p, "<repo>")
	}
	return nil, fmt.Errorf("git %s: %s", args[0], msg)
}

func repoID(r Repo, fallbackHost string) string {
	out, err := r.Git("config", "--get", "remote.origin.url").Output()
	if url := strings.TrimSpace(string(out)); err == nil && url != "" {
		return graph.CanonicalID(url)
	}
	return strings.ToLower(fallbackHost + "/" + r.Owner + "/" + r.Name)
}

package scan

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/lockyc/mycelium/internal/catalog"
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

// sidecarAtRef returns the committed mycelium.toml at ref. found=false when the
// file is not present at ref (an orphan); error only on an unexpected failure.
func sidecarAtRef(r Repo, ref string) ([]byte, bool, error) {
	out, err := r.Git("show", ref+":"+catalog.SidecarName).Output()
	if err == nil {
		return out, true, nil
	}
	// git ran but exited non-zero: the path is absent at ref (or ref is unborn)
	// — a genuine "no sidecar". Any non-ExitError (git not runnable, permission
	// denied) is infrastructure breakage: surface it rather than silently
	// dropping every scanned repo as an orphan.
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return nil, false, nil
	}
	return nil, false, fmt.Errorf("read sidecar at %s in %s: %w", ref, r.Dir, err)
}

func repoID(r Repo, fallbackHost string) string {
	out, err := r.Git("config", "--get", "remote.origin.url").Output()
	if url := strings.TrimSpace(string(out)); err == nil && url != "" {
		return catalog.CanonicalID(url)
	}
	return strings.ToLower(fallbackHost + "/" + r.Owner + "/" + r.Name)
}

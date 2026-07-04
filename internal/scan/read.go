package scan

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/lockyc/mycelium/internal/catalog"
)

// sidecarAtHEAD returns the committed catalog.toml at HEAD. found=false when the
// file is not present in HEAD (an orphan); error only on an unexpected failure.
func sidecarAtHEAD(r Repo) ([]byte, bool, error) {
	out, err := r.Git("show", "HEAD:catalog.toml").Output()
	if err == nil {
		return out, true, nil
	}
	// git ran but exited non-zero: the path is absent at HEAD (or HEAD is unborn)
	// — a genuine "no sidecar". Any non-ExitError (git not runnable, permission
	// denied) is infrastructure breakage: surface it rather than silently
	// dropping every scanned repo as an orphan.
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return nil, false, nil
	}
	return nil, false, fmt.Errorf("read sidecar at HEAD in %s: %w", r.Dir, err)
}

func repoID(r Repo, fallbackHost string) string {
	out, err := r.Git("config", "--get", "remote.origin.url").Output()
	if url := strings.TrimSpace(string(out)); err == nil && url != "" {
		return catalog.CanonicalID(url)
	}
	return strings.ToLower(fallbackHost + "/" + r.Owner + "/" + r.Name)
}

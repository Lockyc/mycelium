package scan

import (
	"strings"

	"github.com/lockyc/mycelium/internal/catalog"
)

// sidecarAtHEAD returns the committed catalog.toml at HEAD. found=false when the
// file is not present in HEAD (an orphan); error only on an unexpected failure.
func sidecarAtHEAD(r Repo) ([]byte, bool, error) {
	out, err := r.Git("show", "HEAD:catalog.toml").Output()
	if err != nil {
		// git exits non-zero when the path is absent at HEAD (or HEAD is unborn):
		// treat as "no sidecar", not a hard error.
		return nil, false, nil
	}
	return out, true, nil
}

func repoID(r Repo, fallbackHost string) string {
	out, err := r.Git("config", "--get", "remote.origin.url").Output()
	if url := strings.TrimSpace(string(out)); err == nil && url != "" {
		return catalog.CanonicalID(url)
	}
	return strings.ToLower(fallbackHost + "/" + r.Owner + "/" + r.Name)
}

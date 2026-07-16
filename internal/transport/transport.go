package transport

import (
	"bytes"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/lockyc/mycelium/internal/graph"
)

const ManifestPath = "/manifests"

// maxManifestBytes caps an ingest request body so an oversized (or streamed)
// POST can't exhaust hub memory. Manifests are small; 32 MiB is generous.
const maxManifestBytes = 32 << 20

func Push(hubURL, token string, m graph.Manifest) error {
	body, err := json.Marshal(m)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, strings.TrimRight(hubURL, "/")+ManifestPath, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("push to hub: %s", resp.Status)
	}
	return nil
}

// writeManifestAtomic writes node's manifest to <dir>/<node>.json via a temp file
// + rename, so a concurrent rebuild never observes a partial write. node is already
// validated as a safe single path segment by the caller. The temp name carries a
// non-.json suffix so a leaked temp (rename failure) is ignored by loadManifests.
func writeManifestAtomic(dir, node string, data []byte) error {
	tmp, err := os.CreateTemp(dir, "."+node+".*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Chmod(tmpName, 0o644); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, filepath.Join(dir, node+".json")); err != nil {
		os.Remove(tmpName)
		return err
	}
	return nil
}

func IngestHandler(manifestsDir, token string, onIngest func() error) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if token != "" {
			// constant-time compare so a matching request can't be found by timing.
			if subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte("Bearer "+token)) != 1 {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
		}
		r.Body = http.MaxBytesReader(w, r.Body, maxManifestBytes)
		var m graph.Manifest
		if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
			http.Error(w, "bad manifest json", http.StatusBadRequest)
			return
		}
		if strings.TrimSpace(m.Node) == "" {
			http.Error(w, "manifest missing node id", http.StatusBadRequest)
			return
		}
		if strings.ContainsAny(m.Node, `/\`) || m.Node == "." || m.Node == ".." {
			http.Error(w, "invalid node id", http.StatusBadRequest)
			return
		}
		data, err := json.MarshalIndent(m, "", "  ")
		if err != nil {
			http.Error(w, "encode", http.StatusInternalServerError)
			return
		}
		if err := os.MkdirAll(manifestsDir, 0o755); err != nil {
			http.Error(w, "store", http.StatusInternalServerError)
			return
		}
		// node-keyed: a re-push from the same node replaces its contribution.
		// Write to a temp file and rename so a concurrent rebuild can never read a
		// half-written manifest (loadManifests reads every *.json on each rebuild).
		if err := writeManifestAtomic(manifestsDir, m.Node, data); err != nil {
			http.Error(w, "store", http.StatusInternalServerError)
			return
		}
		if onIngest != nil {
			if err := onIngest(); err != nil {
				http.Error(w, "rebuild failed", http.StatusInternalServerError)
				return
			}
		}
		w.WriteHeader(http.StatusOK)
	})
}

package transport

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/lockyc/mycelium/internal/catalog"
)

const ManifestPath = "/manifests"

func Push(hubURL, token string, m catalog.Manifest) error {
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

func IngestHandler(manifestsDir, token string, onIngest func() error) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if token != "" && r.Header.Get("Authorization") != "Bearer "+token {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		var m catalog.Manifest
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
		if err := os.WriteFile(filepath.Join(manifestsDir, m.Node+".json"), data, 0o644); err != nil {
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

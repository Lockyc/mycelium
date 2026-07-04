package transport

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/lockyc/mycelium/internal/catalog"
)

func TestPushSendsAuthedManifest(t *testing.T) {
	var gotAuth, gotPath, gotNode string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		body, _ := io.ReadAll(r.Body)
		var m catalog.Manifest
		_ = json.Unmarshal(body, &m)
		gotNode = m.Node
		w.WriteHeader(200)
	}))
	defer srv.Close()

	err := Push(srv.URL, "sekret", catalog.Manifest{Node: "forgejo"})
	if err != nil {
		t.Fatal(err)
	}
	if gotAuth != "Bearer sekret" || gotPath != ManifestPath || gotNode != "forgejo" {
		t.Fatalf("auth=%q path=%q node=%q", gotAuth, gotPath, gotNode)
	}
}

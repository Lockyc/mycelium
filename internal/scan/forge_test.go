package scan

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"
)

func forgeServer(t *testing.T, h http.HandlerFunc) (*http.Client, func(string) string) {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return &http.Client{Timeout: 3 * time.Second}, func(string) string { return srv.URL + "/repos" }
}

func TestVisibility(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
		want   string
	}{
		{"public", 200, `{"private":false}`, "public"},
		{"private", 200, `{"private":true}`, "private"},
		{"hidden", 404, ``, "private"},
		{"server error", 500, ``, ""},
		{"rate limited", 403, ``, ""},
		{"bad body", 200, `not json`, ""},
		{"login redirect", 303, ``, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cl, base := forgeServer(t, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/repos/o/r" {
					t.Errorf("path = %s", r.URL.Path)
				}
				if r.Header.Get("Authorization") != "" {
					t.Error("request must be unauthenticated")
				}
				if c.status == 303 {
					w.Header().Set("Location", "/user/login")
				}
				w.WriteHeader(c.status)
				_, _ = w.Write([]byte(c.body))
			})
			if got := Visibility(cl, base, "forge.example/o/r"); got != c.want {
				t.Fatalf("got %q, want %q", got, c.want)
			}
		})
	}
}

func TestVisibilityConnectionRefused(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	url := srv.URL
	srv.Close()
	cl := &http.Client{Timeout: 3 * time.Second}
	if got := Visibility(cl, func(string) string { return url }, "forge.example/o/r"); got != "" {
		t.Fatalf("got %q, want empty", got)
	}
}

func TestVisibilityTimeout(t *testing.T) {
	cl, base := forgeServer(t, func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(300 * time.Millisecond)
		_, _ = w.Write([]byte(`{"private":false}`))
	})
	cl.Timeout = 50 * time.Millisecond
	if got := Visibility(cl, base, "forge.example/o/r"); got != "" {
		t.Fatalf("got %q, want empty on timeout", got)
	}
}

func TestDefaultAPIBase(t *testing.T) {
	cases := map[string]string{
		"github.com":    "https://api.github.com/repos",
		"forge.example": "https://forge.example/api/v1/repos",
	}
	for host, want := range cases {
		if got := DefaultAPIBase(host); got != want {
			t.Errorf("%s: got %q want %q", host, got, want)
		}
	}
}

func TestVisibilityMalformedID(t *testing.T) {
	cl := &http.Client{Timeout: time.Second}
	if got := Visibility(cl, DefaultAPIBase, "bad"); got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestScanFillsVisibilityOnlyWhenAsked(t *testing.T) {
	root := t.TempDir()
	w := filepath.Join(root, "acme", "widgets")
	mkWorking(t, w)
	run(t, w, "remote", "add", "origin", "git@forge.example:acme/widgets.git")
	commitSidecar(t, w, "name=\"widgets\"\nsummary=\"w\"\n")

	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		hits++
		if r.URL.Path != "/repos/acme/widgets" {
			t.Errorf("path = %s", r.URL.Path)
		}
		_, _ = rw.Write([]byte(`{"private":true}`))
	}))
	defer srv.Close()

	opts := Options{Node: "t", FallbackHost: "forge.example", APIBase: func(string) string { return srv.URL + "/repos" }}
	m, err := Scan([]string{root}, opts)
	if err != nil {
		t.Fatal(err)
	}
	if hits != 0 || m.Components[0].Facts.Visibility != "" {
		t.Fatalf("off by default: hits=%d vis=%q", hits, m.Components[0].Facts.Visibility)
	}
	opts.Visibility = true
	m, err = Scan([]string{root}, opts)
	if err != nil {
		t.Fatal(err)
	}
	if got := m.Components[0].Facts.Visibility; got != "private" {
		t.Fatalf("visibility = %q", got)
	}
}

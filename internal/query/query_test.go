package query

import (
	"testing"

	"github.com/lockyc/mycelium/internal/graph"
)

func fixture() graph.Graph {
	return graph.Graph{
		Components: []graph.Component{
			{ID: "github.com/acme/warden", Name: "warden", Sidecar: graph.Sidecar{
				Name: "warden", Summary: "terminals console", Kind: "app", Status: "active",
				Stack: []string{"rust", "tauri"}, Tags: []string{"macos"},
				Provides: []graph.Provides{{Name: "sidebar", Summary: "the chrome sidebar"}},
			}},
			{ID: "github.com/acme/config-core", Name: "config-core", Sidecar: graph.Sidecar{
				Name: "config-core", Summary: "shared config", Kind: "library", Status: "active",
				Stack:    []string{"rust"},
				Provides: []graph.Provides{{Name: "config-shape", Summary: "the config TOML shape", URL: "https://x"}},
			}},
		},
		Capabilities: map[string][]string{
			"sidebar":      {"warden"},
			"config-shape": {"config-core"},
		},
		Edges: []graph.Edge{
			{From: "warden", To: "config-core", Type: "depends-on"},
			{From: "warden", To: "reductable", Type: "related"},
		},
	}
}

func TestCapability(t *testing.T) {
	g := fixture()
	v, ok := Capability(g, "config-shape")
	if !ok {
		t.Fatal("config-shape not found")
	}
	if v.Summary != "the config TOML shape" || v.URL != "https://x" {
		t.Errorf("bad capability view: %+v", v)
	}
	if len(v.Providers) != 1 || v.Providers[0] != "config-core" {
		t.Errorf("bad providers: %+v", v.Providers)
	}
	if _, ok := Capability(g, "nope"); ok {
		t.Error("unknown capability reported found")
	}
}

func TestCapabilities(t *testing.T) {
	got := Capabilities(fixture())
	if len(got) != 2 {
		t.Fatalf("want 2 capabilities, got %d", len(got))
	}
	// sorted by name: config-shape before sidebar
	if got[0].Name != "config-shape" || got[1].Name != "sidebar" {
		t.Errorf("capabilities not name-sorted: %+v", got)
	}
}

func TestComponent(t *testing.T) {
	c, ok := Component(fixture(), "warden")
	if !ok || c.Sidecar.Summary != "terminals console" {
		t.Errorf("component lookup failed: %+v ok=%v", c, ok)
	}
	if _, ok := Component(fixture(), "ghost"); ok {
		t.Error("unknown component reported found")
	}
}

func TestComponentsFilter(t *testing.T) {
	g := fixture()
	if got := Components(g, ComponentFilter{Kind: "app"}); len(got) != 1 || got[0].Name != "warden" {
		t.Errorf("kind=app filter wrong: %+v", got)
	}
	if got := Components(g, ComponentFilter{Stack: "rust"}); len(got) != 2 {
		t.Errorf("stack=rust should match both: %+v", got)
	}
	// AND across filters: rust AND app == warden only
	if got := Components(g, ComponentFilter{Stack: "rust", Kind: "app"}); len(got) != 1 || got[0].Name != "warden" {
		t.Errorf("stack=rust,kind=app wrong: %+v", got)
	}
	if got := Components(g, ComponentFilter{Tag: "macos"}); len(got) != 1 || got[0].Name != "warden" {
		t.Errorf("tag=macos wrong: %+v", got)
	}
	// empty filter = all
	if got := Components(g, ComponentFilter{}); len(got) != 2 {
		t.Errorf("empty filter should return all: %+v", got)
	}
}

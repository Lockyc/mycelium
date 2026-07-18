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

func TestUsedByReversesOnlyUseEdges(t *testing.T) {
	g := fixture()
	// config-core is depended-on by warden (a use edge) → blast radius.
	rels, ok := UsedBy(g, "config-core")
	if !ok {
		t.Fatal("config-core should exist")
	}
	if len(rels) != 1 || rels[0].Name != "warden" || rels[0].Type != "depends-on" {
		t.Errorf("used-by wrong: %+v", rels)
	}
	// warden -> reductable is `related` (thematic), NOT a use edge, so warden's
	// forward uses must exclude it and include only the depends-on.
	uses, ok := Uses(g, "warden")
	if !ok {
		t.Fatal("warden should exist")
	}
	if len(uses) != 1 || uses[0].Name != "config-core" {
		t.Errorf("uses should traverse only use-edges: %+v", uses)
	}
	// unknown name is explicit not-found, not empty success.
	if _, ok := UsedBy(g, "ghost"); ok {
		t.Error("used-by on unknown name reported found")
	}
}

func TestSearch(t *testing.T) {
	g := fixture()
	hits := Search(g, "config")
	// matches component config-core (name) and capability config-shape (name).
	var comp, cap bool
	for _, h := range hits {
		if h.Kind == "component" && h.Name == "config-core" {
			comp = true
		}
		if h.Kind == "capability" && h.Name == "config-shape" {
			cap = true
		}
	}
	if !comp || !cap {
		t.Errorf("search missed matches: %+v", hits)
	}
	if len(Search(g, "")) != 0 {
		t.Error("empty query should return no hits")
	}
	// summary match: "terminals" is in warden's summary.
	if hits := Search(g, "TERMINALS"); len(hits) != 1 || hits[0].Name != "warden" {
		t.Errorf("case-insensitive summary search wrong: %+v", hits)
	}
}

func TestDescriptorsCoverAllQueries(t *testing.T) {
	names := map[string]bool{}
	for _, d := range Descriptors() {
		names[d.Name] = true
		if d.Example == "" {
			t.Errorf("descriptor %q has no example", d.Name)
		}
	}
	for _, want := range []string{"capabilities", "capability", "component", "components", "used-by", "uses", "search"} {
		if !names[want] {
			t.Errorf("Descriptors() missing %q", want)
		}
	}
}

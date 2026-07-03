package catalog

import "testing"

func TestMergeDedupAndCapabilities(t *testing.T) {
	m1 := Manifest{Components: []Component{{
		ID: "github.com/acme/orders-api", Name: "orders-api",
		Sidecar: Sidecar{Name: "orders-api", Provides: []Provides{{Name: "order-events"}}},
	}}}
	m2 := Manifest{Components: []Component{{ // same repo, another node
		ID: "github.com/acme/orders-api", Name: "orders-api", Commit: "abc",
	}}}
	ov := Overlay{
		Nodes: []OverlayNode{{Name: "shared-postgres", Provides: []string{"postgres"}}},
		Edges: []Edge{
			{From: "web-frontend", To: "postgres", Type: "consumes"},
			{From: "x", To: "nonexistent", Type: "consumes"},
		},
	}
	cat := Merge([]Manifest{m1, m2}, ov)

	if len(cat.Components) != 1 {
		t.Fatalf("want 1 deduped component, got %d", len(cat.Components))
	}
	if cat.Components[0].Commit != "abc" {
		t.Fatalf("want commit filled from m2, got %q", cat.Components[0].Commit)
	}
	if got := cat.Capabilities["order-events"]; len(got) != 1 || got[0] != "orders-api" {
		t.Fatalf("bad order-events providers: %v", got)
	}
	if got := cat.Capabilities["postgres"]; len(got) != 1 || got[0] != "shared-postgres" {
		t.Fatalf("bad postgres providers: %v", got)
	}
	if len(cat.Edges) != 1 || len(cat.DanglingEdges) != 1 {
		t.Fatalf("edges=%v dangling=%v", cat.Edges, cat.DanglingEdges)
	}
}

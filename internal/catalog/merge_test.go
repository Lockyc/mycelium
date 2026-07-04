package catalog

import (
	"strings"
	"testing"
)

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
			{From: "orders-api", To: "postgres", Type: "consumes"},    // valid: real source → capability
			{From: "phantom-svc", To: "postgres", Type: "consumes"},   // dangling: source not present
			{From: "orders-api", To: "nonexistent", Type: "consumes"}, // dangling: target not present
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
	if len(cat.Edges) != 1 || len(cat.DanglingEdges) != 2 {
		t.Fatalf("edges=%v dangling=%v", cat.Edges, cat.DanglingEdges)
	}

	// An unresolved source is caught, not just an unresolved target, and each
	// dangling edge carries a reason naming the offending side.
	var srcDangling, tgtDangling bool
	for _, d := range cat.DanglingEdges {
		switch {
		case d.From == "phantom-svc" && strings.Contains(d.Reason, "source"):
			srcDangling = true
		case d.To == "nonexistent" && strings.Contains(d.Reason, "target"):
			tgtDangling = true
		}
	}
	if !srcDangling {
		t.Fatalf("dangling source not detected: %+v", cat.DanglingEdges)
	}
	if !tgtDangling {
		t.Fatalf("dangling target not detected: %+v", cat.DanglingEdges)
	}
}

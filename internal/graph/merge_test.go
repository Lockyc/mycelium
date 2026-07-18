package graph

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
	g := Merge([]Manifest{m1, m2}, ov)

	if len(g.Components) != 1 {
		t.Fatalf("want 1 deduped component, got %d", len(g.Components))
	}
	if g.Components[0].Commit != "abc" {
		t.Fatalf("want commit filled from m2, got %q", g.Components[0].Commit)
	}
	if got := g.Capabilities["order-events"]; len(got) != 1 || got[0] != "orders-api" {
		t.Fatalf("bad order-events providers: %v", got)
	}
	if got := g.Capabilities["postgres"]; len(got) != 1 || got[0] != "shared-postgres" {
		t.Fatalf("bad postgres providers: %v", got)
	}
	if len(g.Edges) != 1 || len(g.DanglingEdges) != 2 {
		t.Fatalf("edges=%v dangling=%v", g.Edges, g.DanglingEdges)
	}

	// An unresolved source is caught, not just an unresolved target, and each
	// dangling edge carries a reason naming the offending side.
	var srcDangling, tgtDangling bool
	for _, d := range g.DanglingEdges {
		switch {
		case d.From == "phantom-svc" && strings.Contains(d.Reason, "source"):
			srcDangling = true
		case d.To == "nonexistent" && strings.Contains(d.Reason, "target"):
			tgtDangling = true
		}
	}
	if !srcDangling {
		t.Fatalf("dangling source not detected: %+v", g.DanglingEdges)
	}
	if !tgtDangling {
		t.Fatalf("dangling target not detected: %+v", g.DanglingEdges)
	}
}

func TestMergeEmptySlicesSerializeAsArrays(t *testing.T) {
	// An empty graph must render every list field as [] (not null) so JSON
	// consumers can index them without a null check.
	out, err := RenderJSON(Merge(nil, Overlay{}))
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	for _, want := range []string{`"components": []`, `"edges": []`, `"dangling_edges": []`, `"orphans": []`} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %s in:\n%s", want, s)
		}
	}
}

func TestMergeOrphansDedupIgnoreAndComponentWins(t *testing.T) {
	// nodeA reports one orphan (gadgets) and a component (widgets).
	// nodeB reports gadgets again (dup), plus "sidecar-later" as an orphan that is
	// a real component on nodeA — so it must NOT be listed as an orphan.
	nodeA := Manifest{
		Components: []Component{
			{ID: "github.com/acme/widgets", Name: "widgets"},
			{ID: "github.com/acme/sidecar-later", Name: "sidecar-later"},
		},
		Orphans: []Orphan{{ID: "github.com/acme/gadgets", Name: "gadgets", Path: "/a/gadgets"}},
	}
	nodeB := Manifest{
		Orphans: []Orphan{
			{ID: "github.com/acme/gadgets", Name: "gadgets", Path: "/b/gadgets"},
			{ID: "github.com/acme/sidecar-later", Name: "sidecar-later", Path: "/b/sidecar-later"},
			{ID: "github.com/acme/scratch", Name: "scratch", Path: "/b/scratch"},
		},
	}
	// ignore scratch — accept a full remote URL, which is canonicalized to match.
	ov := Overlay{Ignore: []string{"git@github.com:acme/scratch.git"}}

	g := Merge([]Manifest{nodeA, nodeB}, ov)

	var ids []string
	for _, o := range g.Orphans {
		ids = append(ids, o.ID)
	}
	if len(ids) != 1 || ids[0] != "github.com/acme/gadgets" {
		t.Fatalf("orphans = %v; want only [github.com/acme/gadgets] "+
			"(gadgets deduped, sidecar-later is a component, scratch ignored)", ids)
	}
}

func TestMergeCarriesDocGraph(t *testing.T) {
	m := Manifest{Node: "a", Components: []Component{{
		ID: "github.com/x/y", Name: "y", DocGraph: &DocGraphDigest{SchemaVersion: 1, DocCount: 7},
	}}}
	g := Merge([]Manifest{m}, Overlay{})
	if len(g.Components) != 1 || g.Components[0].DocGraph == nil || g.Components[0].DocGraph.DocCount != 7 {
		t.Fatalf("first-seen digest not carried: %+v", g.Components)
	}
}

func TestMergeBackfillsDocGraphWhenFirstLacksIt(t *testing.T) {
	first := Manifest{Node: "a", Components: []Component{{ID: "github.com/x/y", Name: "y"}}}
	second := Manifest{Node: "b", Components: []Component{{
		ID: "github.com/x/y", Name: "y", DocGraph: &DocGraphDigest{SchemaVersion: 1, DocCount: 3},
	}}}
	g := Merge([]Manifest{first, second}, Overlay{})
	if len(g.Components) != 1 || g.Components[0].DocGraph == nil || g.Components[0].DocGraph.DocCount != 3 {
		t.Fatalf("digest not backfilled from second node: %+v", g.Components)
	}
}

package audit

import "testing"
import "github.com/lockyc/mycelium/internal/graph"

func TestAuditFindsAllKinds(t *testing.T) {
	g := graph.Graph{
		Components: []graph.Component{{ID: "github.com/acme/widgets"}},
		Orphans:    []graph.Orphan{{ID: "github.com/acme/gadgets", Name: "gadgets", Path: "/repos/gadgets"}},
		DanglingEdges: []graph.DanglingEdge{{
			Edge:   graph.Edge{From: "x", To: "ghost", Type: "consumes"},
			Reason: `target "ghost" is not provided by anything`,
		}},
	}
	findings := Audit(g,
		[]string{"github.com/acme/removed"}, // previously present, now missing
	)
	kinds := map[string]int{}
	for _, f := range findings {
		kinds[f.Kind]++
	}
	if kinds["orphan"] != 1 || kinds["dangling-edge"] != 1 || kinds["staleness"] != 1 {
		t.Fatalf("bad findings: %+v", findings)
	}
}

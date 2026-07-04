package audit

import "testing"
import "github.com/lockyc/mycelium/internal/catalog"

func TestAuditFindsAllKinds(t *testing.T) {
	cat := catalog.Catalog{
		Components:    []catalog.Component{{ID: "github.com/acme/widgets"}},
		DanglingEdges: []catalog.DanglingEdge{{
			Edge:   catalog.Edge{From: "x", To: "ghost", Type: "consumes"},
			Reason: `target "ghost" is not provided by anything`,
		}},
	}
	findings := Audit(cat, nil,
		[]string{"/repos/gadgets"},          // orphan path
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

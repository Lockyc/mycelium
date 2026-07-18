package audit

import (
	"strings"
	"testing"

	"github.com/lockyc/mycelium/internal/graph"
)

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

func TestAuditReportsDocRot(t *testing.T) {
	g := graph.Graph{Components: []graph.Component{
		{ID: "github.com/x/rotty", Name: "rotty", DocGraph: &graph.DocGraphDigest{
			SchemaVersion: 1, DocCount: 5, ContentIslands: []string{"docs/a.md"}, MetadataIslands: []string{"docs/b.md"}}},
		{ID: "github.com/x/clean", Name: "clean", DocGraph: &graph.DocGraphDigest{SchemaVersion: 1, DocCount: 3}},
	}}
	found := Audit(g, nil)
	var rot int
	for _, f := range found {
		if f.Kind == "doc-rot" {
			rot++
			if !strings.Contains(f.Detail, "github.com/x/rotty") || !strings.Contains(f.Detail, "docs/a.md") {
				t.Fatalf("doc-rot detail should name the repo + islands: %q", f.Detail)
			}
		}
	}
	if rot != 1 {
		t.Fatalf("want exactly one doc-rot finding (only rotty), got %d", rot)
	}
}

func TestAuditReportsUnknownDocgraphVersion(t *testing.T) {
	g := graph.Graph{Components: []graph.Component{
		{ID: "github.com/x/newer", Name: "newer", DocGraph: &graph.DocGraphDigest{SchemaVersion: 2}},
	}}
	found := Audit(g, nil)
	var ok bool
	for _, f := range found {
		if f.Kind == "docgraph-version" && strings.Contains(f.Detail, "github.com/x/newer") {
			ok = true
		}
	}
	if !ok {
		t.Fatalf("expected a docgraph-version finding: %+v", found)
	}
}

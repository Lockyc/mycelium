package audit

import (
	"fmt"
	"strings"

	"github.com/lockyc/mycelium/internal/graph"
)

type Finding struct {
	Kind   string
	Detail string
}

func Audit(g graph.Graph, previousIDs []string) []Finding {
	var out []Finding
	for _, o := range g.Orphans {
		out = append(out, Finding{Kind: "orphan",
			Detail: fmt.Sprintf("repo without mycelium.toml: %s", o.ID)})
	}
	for _, e := range g.DanglingEdges {
		out = append(out, Finding{Kind: "dangling-edge",
			Detail: fmt.Sprintf("%s %s %s — %s", e.From, e.Type, e.To, e.Reason)})
	}
	for _, c := range g.Components {
		if c.DocGraph == nil {
			continue
		}
		if c.DocGraph.SchemaVersion != 0 && c.DocGraph.SchemaVersion != 1 {
			out = append(out, Finding{Kind: "docgraph-version",
				Detail: fmt.Sprintf("%s: docgraph schemaVersion %d newer than Mycelium understands (pins 1)", c.ID, c.DocGraph.SchemaVersion)})
			continue
		}
		islands := append(append([]string{}, c.DocGraph.ContentIslands...), c.DocGraph.MetadataIslands...)
		if len(islands) > 0 {
			out = append(out, Finding{Kind: "doc-rot",
				Detail: fmt.Sprintf("%s: %d island doc(s) — %s", c.ID, len(islands), strings.Join(islands, ", "))})
		}
	}
	present := map[string]bool{}
	for _, c := range g.Components {
		present[c.ID] = true
	}
	for _, id := range previousIDs {
		if !present[id] {
			out = append(out, Finding{Kind: "staleness", Detail: fmt.Sprintf("component gone since last run: %s", id)})
		}
	}
	return out
}

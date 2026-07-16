package audit

import (
	"fmt"

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

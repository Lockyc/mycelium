package audit

import (
	"fmt"
	"sort"
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
	// Names key edges, capabilities and every name-based query, so two entries
	// sharing one name are indistinguishable downstream: report each shared name
	// with everything that claims it.
	claims := map[string][]string{}
	var names []string
	claim := func(name, who string) {
		if claims[name] == nil {
			names = append(names, name)
		}
		claims[name] = append(claims[name], who)
	}
	for _, c := range g.Components {
		claim(c.Name, c.ID)
	}
	for _, n := range g.Nodes {
		claim(n.Name, "overlay node")
	}
	sort.Strings(names)
	for _, name := range names {
		if len(claims[name]) > 1 {
			out = append(out, Finding{Kind: "duplicate-name",
				Detail: fmt.Sprintf("%q is claimed by %s", name, strings.Join(claims[name], ", "))})
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

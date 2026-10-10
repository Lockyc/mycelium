package graph

import "sort"

// IsUseEdge reports whether an edge type means "from actually uses to" — the
// dependency edges (consumes / depends-on / deploys-to) that define blast radius.
// It is the exported accessor over useEdgeTypes (defined in render.go), so the
// query layer reuses the one definition of that set instead of re-encoding it.
func IsUseEdge(edgeType string) bool { return useEdgeTypes[edgeType] }

// UseRef is one reverse use edge: From uses the indexed entity via Type.
type UseRef struct {
	From string
	Type string
}

// UsedByIndex reverses g's use edges into entity name → its users: the one
// definition of "Used by" that both MAP.md and `myco query used-by` read.
//
// An edge may target a capability rather than an entity, so a capability target
// is indexed under the capability itself AND under every provider of it — using
// a capability is using whatever provides it, so it belongs in each provider's
// blast radius. An entity is never listed as its own user. Each list is
// deduplicated by (From, Type) and sorted, so the output is deterministic
// whatever the overlay order.
func UsedByIndex(g Graph) map[string][]UseRef {
	seen := map[string]map[UseRef]bool{}
	add := func(target string, r UseRef) {
		if target == r.From {
			return
		}
		if seen[target] == nil {
			seen[target] = map[UseRef]bool{}
		}
		seen[target][r] = true
	}
	for _, e := range g.Edges {
		if !IsUseEdge(e.Type) {
			continue
		}
		r := UseRef{From: e.From, Type: e.Type}
		add(e.To, r)
		for _, provider := range g.Capabilities[e.To] {
			add(provider, r)
		}
	}
	idx := make(map[string][]UseRef, len(seen))
	for target, set := range seen {
		refs := make([]UseRef, 0, len(set))
		for r := range set {
			refs = append(refs, r)
		}
		sort.Slice(refs, func(i, j int) bool {
			if refs[i].From != refs[j].From {
				return refs[i].From < refs[j].From
			}
			return refs[i].Type < refs[j].Type
		})
		idx[target] = refs
	}
	return idx
}

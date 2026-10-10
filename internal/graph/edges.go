package graph

import "sort"

// useEdgeTypes are the edge types that mean "from actually uses to", so reversing
// one yields a true "Used by" — and, together, an entry's blast radius: change
// this thing and these are what must be re-pinned or rebuilt.
//
// thematicEdgeTypes (markets, sells, related) are not consumption, so they are
// deliberately excluded from Used by: "business sells reductable" reversed onto
// reductable as "Used by: business" would be plainly false. They stay in the
// Relationships section, which renders every edge with its type intact.
//
// Together the two sets are every valid edge type (schema/graph.md's `type` enum).
var (
	useEdgeTypes      = map[string]bool{"consumes": true, "depends-on": true, "deploys-to": true}
	thematicEdgeTypes = map[string]bool{"markets": true, "sells": true, "related": true}
)

// IsUseEdge reports whether an edge type means "from actually uses to" — the
// dependency edges (consumes / depends-on / deploys-to) that define blast radius.
// The query layer reuses it instead of re-encoding the set.
func IsUseEdge(edgeType string) bool { return useEdgeTypes[edgeType] }

// IsEdgeType reports whether edgeType is a valid edge type at all. An unknown
// type (a typo like "depend-on") still renders under Relationships but is
// silently absent from Used by, so the audit reports it.
func IsEdgeType(edgeType string) bool { return useEdgeTypes[edgeType] || thematicEdgeTypes[edgeType] }

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

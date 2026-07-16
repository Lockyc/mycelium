package catalog

import (
	"fmt"
	"sort"
)

type Catalog struct {
	Components []Component `json:"components"`
	// Nodes are the overlay's non-repo entries (a managed service, a SaaS
	// dependency) — real actors in the ecosystem that no scan can find because
	// they have no repo. They are carried alongside Components, not merged into
	// them, because they have no id/commit/sidecar; the renderer lists both.
	Nodes         []OverlayNode       `json:"nodes,omitempty"`
	Capabilities  map[string][]string `json:"capabilities"`
	Edges         []Edge              `json:"edges"`
	DanglingEdges []DanglingEdge      `json:"dangling_edges"`
	Orphans       []Orphan            `json:"orphans"`
}

// DanglingEdge is an overlay edge that failed to resolve: its source must be a
// known component or overlay node, and its target a known component, node, or
// capability. Reason names the offending side for the audit report.
type DanglingEdge struct {
	Edge
	Reason string `json:"reason"`
}

func Merge(manifests []Manifest, ov Overlay) Catalog {
	byID := map[string]*Component{}
	var order []string
	for _, m := range manifests {
		for _, c := range m.Components {
			existing, ok := byID[c.ID]
			if !ok {
				cp := c
				byID[c.ID] = &cp
				order = append(order, c.ID)
				continue
			}
			if existing.Commit == "" && c.Commit != "" {
				existing.Commit = c.Commit
			}
			if len(existing.Sidecar.Provides) == 0 && len(c.Sidecar.Provides) > 0 {
				// backfill only the capability list; the authoritative (first-seen)
				// entry keeps its own name/summary/etc.
				existing.Sidecar.Provides = c.Sidecar.Provides
			}
		}
	}

	caps := map[string]map[string]bool{}
	addCap := func(capName, provider string) {
		if caps[capName] == nil {
			caps[capName] = map[string]bool{}
		}
		caps[capName][provider] = true
	}

	comps := []Component{}
	names := map[string]bool{}
	for _, id := range order {
		c := *byID[id]
		comps = append(comps, c)
		names[c.Name] = true
		for _, p := range c.Sidecar.Provides {
			addCap(p.Name, c.Name)
		}
	}
	for _, n := range ov.Nodes {
		names[n.Name] = true
		for _, p := range n.Provides {
			addCap(p, n.Name)
		}
	}

	capIndex := map[string][]string{}
	for capName, provs := range caps {
		var list []string
		for p := range provs {
			list = append(list, p)
		}
		sort.Strings(list)
		capIndex[capName] = list
	}

	edges := []Edge{}
	dangling := []DanglingEdge{}
	for _, e := range ov.Edges {
		_, toIsCap := capIndex[e.To]
		fromOK := names[e.From] // a source is an actor: a component or overlay node
		toOK := toIsCap || names[e.To]
		switch {
		case fromOK && toOK:
			edges = append(edges, e)
		case !fromOK && !toOK:
			dangling = append(dangling, DanglingEdge{e,
				fmt.Sprintf("source %q and target %q both unresolved", e.From, e.To)})
		case !fromOK:
			dangling = append(dangling, DanglingEdge{e,
				fmt.Sprintf("source %q is not a known component or node", e.From)})
		default:
			dangling = append(dangling, DanglingEdge{e,
				fmt.Sprintf("target %q is not provided by anything", e.To)})
		}
	}

	// Orphans: repos scanned without a sidecar. A repo that is a real component
	// on any node is not an orphan; an id in the overlay ignore list is suppressed.
	present := map[string]bool{}
	for _, c := range comps {
		present[c.ID] = true
	}
	ignore := map[string]bool{}
	for _, id := range ov.Ignore {
		ignore[CanonicalID(id)] = true
	}
	orphanByID := map[string]Orphan{}
	var orphanOrder []string
	for _, m := range manifests {
		for _, o := range m.Orphans {
			if present[o.ID] || ignore[o.ID] {
				continue
			}
			if _, seen := orphanByID[o.ID]; !seen {
				orphanByID[o.ID] = o
				orphanOrder = append(orphanOrder, o.ID)
			}
		}
	}
	sort.Strings(orphanOrder)
	orphans := []Orphan{}
	for _, id := range orphanOrder {
		orphans = append(orphans, orphanByID[id])
	}

	nodes := append([]OverlayNode(nil), ov.Nodes...)

	return Catalog{Components: comps, Nodes: nodes, Capabilities: capIndex, Edges: edges, DanglingEdges: dangling, Orphans: orphans}
}

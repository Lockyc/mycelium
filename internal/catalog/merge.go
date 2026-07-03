package catalog

import "sort"

type Catalog struct {
	Components    []Component         `json:"components"`
	Capabilities  map[string][]string `json:"capabilities"`
	Edges         []Edge              `json:"edges"`
	DanglingEdges []Edge              `json:"dangling_edges"`
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

	var comps []Component
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

	var edges, dangling []Edge
	for _, e := range ov.Edges {
		if _, isCap := capIndex[e.To]; isCap || names[e.To] {
			edges = append(edges, e)
		} else {
			dangling = append(dangling, e)
		}
	}

	return Catalog{Components: comps, Capabilities: capIndex, Edges: edges, DanglingEdges: dangling}
}

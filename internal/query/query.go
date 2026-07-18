// Package query provides first-class named queries over a merged graph.Graph.
// It is the single source of query semantics: the myco CLI and the hub's /q/*
// HTTP routes both call these functions, so they can never disagree about what
// a query means. Every function is pure — it takes a graph.Graph and returns
// plain structs, with no I/O and no formatting — which keeps it table-testable
// and lets each surface format however it likes.
package query

import (
	"sort"
	"strings"

	"github.com/lockyc/mycelium/internal/graph"
)

// CapabilityView is one capability: its summary/url (taken from the first
// provider that declares them) and the names of everything that provides it.
type CapabilityView struct {
	Name      string   `json:"name"`
	Summary   string   `json:"summary,omitempty"`
	URL       string   `json:"url,omitempty"`
	Providers []string `json:"providers"`
}

// ComponentFilter narrows Components. An empty field is a wildcard; set fields
// are AND-combined. Stack and Tag match array membership; Kind and Status match
// equality.
type ComponentFilter struct {
	Kind, Stack, Status, Tag string
}

// Capabilities returns every capability, name-sorted, as a view.
func Capabilities(g graph.Graph) []CapabilityView {
	names := make([]string, 0, len(g.Capabilities))
	for n := range g.Capabilities {
		names = append(names, n)
	}
	sort.Strings(names)
	out := make([]CapabilityView, 0, len(names))
	for _, n := range names {
		if v, ok := Capability(g, n); ok {
			out = append(out, v)
		}
	}
	return out
}

// Capability returns one capability's view, or ok=false if no component or
// overlay node provides it. Summary/url are taken from the first component
// provider that declares them (overlay-node providers carry names only).
func Capability(g graph.Graph, name string) (CapabilityView, bool) {
	provs, ok := g.Capabilities[name]
	if !ok {
		return CapabilityView{}, false
	}
	providers := append([]string(nil), provs...)
	sort.Strings(providers)
	v := CapabilityView{Name: name, Providers: providers}
	for _, c := range g.Components {
		for _, p := range c.Sidecar.Provides {
			if p.Name != name {
				continue
			}
			if v.Summary == "" {
				v.Summary = p.Summary
			}
			if v.URL == "" {
				v.URL = p.URL
			}
		}
	}
	return v, true
}

// Component returns one component by name, or ok=false if absent.
func Component(g graph.Graph, name string) (graph.Component, bool) {
	for _, c := range g.Components {
		if c.Name == name {
			return c, true
		}
	}
	return graph.Component{}, false
}

// Components returns components matching f (AND across set fields).
func Components(g graph.Graph, f ComponentFilter) []graph.Component {
	out := []graph.Component{}
	for _, c := range g.Components {
		if f.Kind != "" && c.Sidecar.Kind != f.Kind {
			continue
		}
		if f.Status != "" && c.Sidecar.Status != f.Status {
			continue
		}
		if f.Stack != "" && !contains(c.Sidecar.Stack, f.Stack) {
			continue
		}
		if f.Tag != "" && !contains(c.Sidecar.Tags, f.Tag) {
			continue
		}
		out = append(out, c)
	}
	return out
}

func contains(hay []string, needle string) bool {
	for _, h := range hay {
		if h == needle {
			return true
		}
	}
	return false
}

// Relation is one end of a use-edge for UsedBy/Uses: the related entity and the
// edge type connecting them.
type Relation struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// SearchHit is one search match — what matched and where.
type SearchHit struct {
	Name    string `json:"name"`
	Kind    string `json:"kind"` // "component" or "capability"
	Summary string `json:"summary,omitempty"`
}

// QueryDesc describes one query for the self-documenting index (GET /q and the
// CLI help). It is the single source of the index — both surfaces render it, so
// the index cannot drift from the queries.
type QueryDesc struct {
	Name    string `json:"name"`
	Args    string `json:"args,omitempty"`
	Example string `json:"example"`
}

// UsedBy returns the entities that use name via a use-edge (reverse direction):
// this is name's blast radius. ok=false when name is not a known component or
// overlay node (explicit not-found, distinct from "known but nothing uses it").
func UsedBy(g graph.Graph, name string) ([]Relation, bool) {
	if !entityExists(g, name) {
		return nil, false
	}
	out := []Relation{}
	for _, e := range g.Edges {
		if e.To == name && graph.IsUseEdge(e.Type) {
			out = append(out, Relation{Name: e.From, Type: e.Type})
		}
	}
	return out, true
}

// Uses returns what name uses (forward use-edges). ok=false when name is unknown.
func Uses(g graph.Graph, name string) ([]Relation, bool) {
	if !entityExists(g, name) {
		return nil, false
	}
	out := []Relation{}
	for _, e := range g.Edges {
		if e.From == name && graph.IsUseEdge(e.Type) {
			out = append(out, Relation{Name: e.To, Type: e.Type})
		}
	}
	return out, true
}

// Search is a case-insensitive substring match over component names + summaries
// and capability names. Not a ranked/fuzzy engine (YAGNI). An empty query
// returns no hits.
func Search(g graph.Graph, text string) []SearchHit {
	q := strings.ToLower(strings.TrimSpace(text))
	out := []SearchHit{}
	if q == "" {
		return out
	}
	for _, c := range g.Components {
		if strings.Contains(strings.ToLower(c.Name), q) ||
			strings.Contains(strings.ToLower(c.Sidecar.Summary), q) {
			out = append(out, SearchHit{Name: c.Name, Kind: "component", Summary: c.Sidecar.Summary})
		}
	}
	caps := make([]string, 0, len(g.Capabilities))
	for n := range g.Capabilities {
		caps = append(caps, n)
	}
	sort.Strings(caps)
	for _, n := range caps {
		if strings.Contains(strings.ToLower(n), q) {
			out = append(out, SearchHit{Name: n, Kind: "capability"})
		}
	}
	return out
}

// Descriptors is the self-documenting query index rendered by both /q and the
// CLI help.
func Descriptors() []QueryDesc {
	return []QueryDesc{
		{Name: "capabilities", Example: "myco query capabilities"},
		{Name: "capability", Args: "<name>", Example: "myco query capability monitoring"},
		{Name: "component", Args: "<name>", Example: "myco query component warden"},
		{Name: "components", Args: "[--kind --stack --status --tag]", Example: "myco query components --kind app"},
		{Name: "used-by", Args: "<name>", Example: "myco query used-by config-core"},
		{Name: "uses", Args: "<name>", Example: "myco query uses lector"},
		{Name: "search", Args: "<text>", Example: "myco query search newsletter"},
	}
}

func entityExists(g graph.Graph, name string) bool {
	for _, c := range g.Components {
		if c.Name == name {
			return true
		}
	}
	for _, n := range g.Nodes {
		if n.Name == name {
			return true
		}
	}
	return false
}

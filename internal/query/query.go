// Package query provides first-class named queries over a merged graph.Graph.
// It is the single source of query semantics: the myco CLI and the hub's /q/*
// HTTP routes both call these functions, so they can never disagree about what
// a query means. Every function is pure — it takes a graph.Graph and returns
// plain structs, with no I/O and no formatting — which keeps it table-testable
// and lets each surface format however it likes.
package query

import (
	"sort"

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

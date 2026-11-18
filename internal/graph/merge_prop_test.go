package graph_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/lockyc/mycelium/internal/graph"
	"github.com/lockyc/mycelium/internal/query"
	"pgregory.net/rapid"
)

// Small pools so generated manifests collide: the same id on several nodes, a
// capability with several providers, an edge aimed at a capability.
var (
	propNames    = []string{"alpha", "beta", "gamma", "delta", "omega"}
	propCaps     = []string{"queue", "storage", "auth", "billing"}
	propEdgeType = []string{"consumes", "depends-on", "deploys-to", "markets", "sells", "related"}
)

func propID(name string) string { return "github.com/acme/" + name }

func genProvides() *rapid.Generator[[]graph.Provides] {
	return rapid.Custom(func(t *rapid.T) []graph.Provides {
		var out []graph.Provides
		for _, c := range rapid.SliceOfNDistinct(rapid.SampledFrom(propCaps), 0, 3, rapid.ID[string]).Draw(t, "caps") {
			out = append(out, graph.Provides{Name: c, Summary: rapid.SampledFrom([]string{"", "does " + c}).Draw(t, "capSummary")})
		}
		return out
	})
}

func genComponent() *rapid.Generator[graph.Component] {
	return rapid.Custom(func(t *rapid.T) graph.Component {
		name := rapid.SampledFrom(propNames).Draw(t, "name")
		c := graph.Component{
			ID:     propID(rapid.SampledFrom(propNames).Draw(t, "idName")),
			Name:   name,
			Commit: rapid.SampledFrom([]string{"", "c0ffee", "beef"}).Draw(t, "commit"),
			Sidecar: graph.Sidecar{
				Name:     name,
				Summary:  "summary of " + name + " " + rapid.SampledFrom([]string{"v1", "v2"}).Draw(t, "rev"),
				Status:   rapid.SampledFrom([]string{"", "active", "wip"}).Draw(t, "status"),
				Provides: genProvides().Draw(t, "provides"),
			},
		}
		for _, p := range rapid.SliceOfN(rapid.SampledFrom(propNames), 0, 2).Draw(t, "pins") {
			c.Pins = append(c.Pins, propID(p))
		}
		if rapid.Bool().Draw(t, "hasDocGraph") {
			c.DocGraph = &graph.DocGraphDigest{SchemaVersion: 1, DocCount: rapid.IntRange(0, 3).Draw(t, "docs")}
		}
		return c
	})
}

// genManifests draws 1-4 manifests with distinct scanned_at values. Ties are
// excluded on purpose: FreshestFirst documents that a tie keeps input order.
func genManifests(t *rapid.T) []graph.Manifest {
	n := rapid.IntRange(1, 4).Draw(t, "manifests")
	offsets := rapid.SliceOfNDistinct(rapid.IntRange(0, 1000), n, n, rapid.ID[int]).Draw(t, "scannedAt")
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	ms := make([]graph.Manifest, n)
	for i := range ms {
		ms[i] = graph.Manifest{
			Node:       fmt.Sprintf("node%d", i),
			ScannedAt:  base.Add(time.Duration(offsets[i]) * time.Minute).Format(time.RFC3339),
			Components: rapid.SliceOfN(genComponent(), 0, 4).Draw(t, "components"),
		}
		for _, o := range rapid.SliceOfN(rapid.SampledFrom(propNames), 0, 2).Draw(t, "orphans") {
			ms[i].Orphans = append(ms[i].Orphans, graph.Orphan{ID: propID(o), Name: o})
		}
		for _, o := range rapid.SliceOfN(rapid.SampledFrom(propNames), 0, 1).Draw(t, "invalid") {
			ms[i].InvalidSidecars = append(ms[i].InvalidSidecars, graph.InvalidSidecar{ID: propID(o), Name: o, Error: "bad"})
		}
	}
	return ms
}

func genOverlay(t *rapid.T) graph.Overlay {
	var ov graph.Overlay
	for _, n := range rapid.SliceOfNDistinct(rapid.SampledFrom([]string{"pg", "s3", "omega"}), 0, 2, rapid.ID[string]).Draw(t, "nodes") {
		ov.Nodes = append(ov.Nodes, graph.OverlayNode{
			Name:     n,
			Summary:  "managed " + n,
			Provides: rapid.SliceOfNDistinct(rapid.SampledFrom(propCaps), 0, 2, rapid.ID[string]).Draw(t, "nodeProvides"),
		})
	}
	actors := append(append([]string{}, propNames...), "pg", "s3", "ghost")
	targets := append(append([]string{}, actors...), propCaps...)
	for range rapid.IntRange(0, 6).Draw(t, "edges") {
		ov.Edges = append(ov.Edges, graph.Edge{
			From: rapid.SampledFrom(actors).Draw(t, "from"),
			To:   rapid.SampledFrom(targets).Draw(t, "to"),
			Type: rapid.SampledFrom(propEdgeType).Draw(t, "type"),
		})
	}
	for _, n := range rapid.SliceOfN(rapid.SampledFrom(propNames), 0, 2).Draw(t, "ignore") {
		ov.Ignore = append(ov.Ignore, "git@github.com:acme/"+n+".git")
	}
	return ov
}

// noNulls fails when any value in a decoded JSON document is null: a consumer
// iterating `.edges[]` or `.orphans[]` errors on null where [] is fine.
func noNulls(t *rapid.T, path string, v any) {
	switch x := v.(type) {
	case nil:
		t.Fatalf("graph.json has null at %s", path)
	case map[string]any:
		for k, e := range x {
			noNulls(t, path+"."+k, e)
		}
	case []any:
		for i, e := range x {
			noNulls(t, fmt.Sprintf("%s[%d]", path, i), e)
		}
	}
}

// TestMergeInvariants checks, across generated manifests and overlays, the
// merged-graph invariants that CLAUDE.md states and single-path fixes have
// broken: the result does not depend on manifest order; every MAP.md entry
// (component or overlay node) resolves through query.Component and is found by
// query.Search; a use edge aimed at a capability puts its user in every
// provider's Used by; and graph.json carries no null list.
func TestMergeInvariants(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		ms := genManifests(t)
		ov := genOverlay(t)
		g := graph.Merge(ms, ov)

		want, err := graph.RenderJSON(g)
		if err != nil {
			t.Fatal(err)
		}
		wantMap := graph.RenderMarkdown(g)
		perm := rapid.Permutation(ms).Draw(t, "order")
		pg := graph.Merge(perm, ov)
		got, _ := graph.RenderJSON(pg)
		if !bytes.Equal(got, want) {
			t.Fatalf("Merge depends on manifest order:\n%s\nvs\n%s", want, got)
		}
		if graph.RenderMarkdown(pg) != wantMap {
			t.Fatalf("MAP.md depends on manifest order")
		}

		var entries []string
		for _, c := range g.Components {
			entries = append(entries, c.Name)
		}
		for _, n := range g.Nodes {
			entries = append(entries, n.Name)
		}
		for _, name := range entries {
			if !strings.Contains(wantMap, "### "+name+"\n") {
				t.Fatalf("MAP.md has no entry for %q", name)
			}
			if _, ok := query.Component(g, name); !ok {
				t.Fatalf("MAP.md lists %q but query.Component does not resolve it", name)
			}
			found := false
			for _, h := range query.Search(g, name) {
				found = found || h.Name == name
			}
			if !found {
				t.Fatalf("MAP.md lists %q but query.Search(%q) does not find it", name, name)
			}
		}

		idx := graph.UsedByIndex(g)
		for _, e := range g.Edges {
			if !graph.IsUseEdge(e.Type) {
				continue
			}
			for _, p := range g.Capabilities[e.To] {
				if p == e.From {
					continue
				}
				want := graph.UseRef{From: e.From, Type: e.Type}
				found := false
				for _, r := range idx[p] {
					found = found || r == want
				}
				if !found {
					t.Fatalf("edge %v targets capability %q, but provider %q's Used by lacks %v", e, e.To, p, want)
				}
				rels, ok := query.UsedBy(g, p)
				if !ok {
					t.Fatalf("query.UsedBy(%q): provider not found", p)
				}
				found = false
				for _, r := range rels {
					found = found || (r.Name == e.From && r.Type == e.Type)
				}
				if !found {
					t.Fatalf("query.UsedBy(%q) lacks %s via capability %q", p, e.From, e.To)
				}
			}
		}

		var doc any
		if err := json.Unmarshal(want, &doc); err != nil {
			t.Fatal(err)
		}
		noNulls(t, "", doc)
	})
}

// nilEmpty sets every zero-length slice reachable from v to nil: the flat JSON
// shape tags its lists omitempty, so an empty list and an absent one are the
// same value on the wire.
func nilEmpty(v reflect.Value) {
	switch v.Kind() {
	case reflect.Pointer:
		if !v.IsNil() {
			nilEmpty(v.Elem())
		}
	case reflect.Struct:
		for i := range v.NumField() {
			if v.Field(i).CanSet() {
				nilEmpty(v.Field(i))
			}
		}
	case reflect.Slice:
		if v.Len() == 0 {
			v.Set(reflect.Zero(v.Type()))
			return
		}
		for i := range v.Len() {
			nilEmpty(v.Index(i))
		}
	}
}

// TestComponentJSONRoundTrip checks that every field of a Component survives
// MarshalJSON then UnmarshalJSON. The generator fills fields by reflection, so a
// field added to Component (or Sidecar, Facts, DocGraphDigest) but left out of
// componentJSON fails here without anyone having to name it.
func TestComponentJSONRoundTrip(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		c := rapid.Make[graph.Component]().Draw(t, "component")
		c.Path = ""             // node-local, never serialised
		c.Sidecar.Name = c.Name // the flat shape carries one name; scan keeps them equal
		nilEmpty(reflect.ValueOf(&c))

		b, err := json.Marshal(c)
		if err != nil {
			t.Fatal(err)
		}
		var back graph.Component
		if err := json.Unmarshal(b, &back); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(back, c) {
			t.Fatalf("round-trip changed the component:\nwant %#v\ngot  %#v\nwire %s", c, back, b)
		}
	})
}

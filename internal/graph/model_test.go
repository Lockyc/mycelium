package graph

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestParseSidecar(t *testing.T) {
	data := []byte(`
name = "orders-api"
summary = "Order processing service"
kind = "service"
status = "active"
tags = ["orders", "billing"]

[[provides]]
name = "order-events"
summary = "Publishes order lifecycle events"
`)
	sc, err := ParseSidecar(data)
	if err != nil {
		t.Fatalf("ParseSidecar: %v", err)
	}
	if sc.Name != "orders-api" || sc.Kind != "service" {
		t.Fatalf("bad parse: %+v", sc)
	}
	if len(sc.Provides) != 1 || sc.Provides[0].Name != "order-events" {
		t.Fatalf("bad provides: %+v", sc.Provides)
	}
}

func TestParseSidecarRejectsMissingName(t *testing.T) {
	if _, err := ParseSidecar([]byte(`summary = "x"`)); err == nil {
		t.Fatal("expected error for missing name")
	}
}

// graph.json is FLAT: declared sidecar fields sit at the component top level, with
// no `sidecar` wrapper and no duplicated `name`, so a consumer queries
// `.components[].provides[]` not `.components[].sidecar.provides[]`.
func TestComponentMarshalsFlat(t *testing.T) {
	c := Component{
		ID: "github.com/acme/orders", Name: "orders", Commit: "abc",
		Sidecar: Sidecar{
			Name: "orders", Summary: "order svc", Kind: "service", Status: "active",
			Stack: []string{"go"}, Provides: []Provides{{Name: "order-events", Summary: "events"}},
		},
	}
	b, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if strings.Contains(s, `"sidecar"`) {
		t.Fatalf("graph.json must be flat — no sidecar wrapper: %s", s)
	}
	for _, want := range []string{`"summary":"order svc"`, `"kind":"service"`, `"provides":[`, `"order-events"`} {
		if !strings.Contains(s, want) {
			t.Fatalf("flat output missing %s: %s", want, s)
		}
	}
	// The redundant sidecar name is dropped: exactly one "name":"orders" (the
	// component's), not a second one for the sidecar.
	if n := strings.Count(s, `"name":"orders"`); n != 1 {
		t.Fatalf("expected one component name, got %d occurrences: %s", n, s)
	}
}

// The flat wire shape must round-trip back into the nested struct the code uses —
// the same path graph.json takes on `myco audit` re-read and a manifest on ingest.
func TestComponentFlatRoundTrip(t *testing.T) {
	c := Component{
		ID: "github.com/acme/orders", Name: "orders", Commit: "abc",
		Sidecar: Sidecar{
			Name: "orders", Summary: "order svc", Kind: "service", Status: "active",
			Tags: []string{"billing"}, Stack: []string{"go"},
			Provides: []Provides{{Name: "order-events", Summary: "events", URL: "https://x"}},
		},
	}
	b, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	var round Component
	if err := json.Unmarshal(b, &round); err != nil {
		t.Fatal(err)
	}
	// Sidecar is fully reconstructed, including the name the flat shape drops.
	if round.Sidecar.Name != "orders" || round.Sidecar.Summary != "order svc" ||
		round.Sidecar.Kind != "service" || len(round.Sidecar.Provides) != 1 ||
		round.Sidecar.Provides[0].URL != "https://x" {
		t.Fatalf("round-trip lost sidecar fields: %+v", round.Sidecar)
	}
}

func TestComponentDocGraphOmittedWhenNil(t *testing.T) {
	c := Component{ID: "github.com/x/y", Name: "y"}
	b, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "docGraph") {
		t.Fatalf("nil DocGraph must not serialize a docGraph key: %s", b)
	}
}

func TestComponentDocGraphSerializes(t *testing.T) {
	c := Component{ID: "github.com/x/y", Name: "y", DocGraph: &DocGraphDigest{
		SchemaVersion:    1,
		DocCount:         42,
		ContentEdgeCount: 88,
		ContentIslands:   []string{"docs/stray.md"},
		EntryDocs:        []string{"CLAUDE.md", "README.md"},
	}}
	b, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	var round Component
	if err := json.Unmarshal(b, &round); err != nil {
		t.Fatal(err)
	}
	if round.DocGraph == nil || round.DocGraph.DocCount != 42 ||
		len(round.DocGraph.ContentIslands) != 1 || round.DocGraph.ContentIslands[0] != "docs/stray.md" {
		t.Fatalf("round-trip lost digest: %+v", round.DocGraph)
	}
}

func TestManifestDocGraphsRoundTrip(t *testing.T) {
	m := Manifest{Node: "n", DocGraphs: map[string]json.RawMessage{
		"github.com/x/y": json.RawMessage(`{"schemaVersion":1,"nodes":[]}`),
	}}
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	var round Manifest
	if err := json.Unmarshal(b, &round); err != nil {
		t.Fatal(err)
	}
	if string(round.DocGraphs["github.com/x/y"]) != `{"schemaVersion":1,"nodes":[]}` {
		t.Fatalf("raw payload not preserved: %s", round.DocGraphs["github.com/x/y"])
	}
}

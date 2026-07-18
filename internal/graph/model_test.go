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

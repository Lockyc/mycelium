package catalog

import "testing"

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

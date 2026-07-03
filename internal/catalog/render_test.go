package catalog

import (
	"strings"
	"testing"
)

// TestRenderJSONPathNotLeaked asserts that Component.Path (an absolute filesystem
// path set by the scanner) is never serialised into catalog.json output.
// This is the regression test for the path-leak fix (json:"-" on Component.Path).
func TestRenderJSONPathNotLeaked(t *testing.T) {
	secretPath := "/home/someone/private/widgets"
	c := Catalog{
		Components: []Component{
			{
				ID:     "github.com/acme/widgets",
				Name:   "widgets",
				Path:   secretPath,
				Commit: "abc123",
				Sidecar: Sidecar{
					Name:    "widgets",
					Summary: "widget catalog app",
					Kind:    "app",
					Status:  "active",
				},
			},
		},
	}
	out, err := RenderJSON(c)
	if err != nil {
		t.Fatalf("RenderJSON error: %v", err)
	}
	s := string(out)
	if strings.Contains(s, secretPath) {
		t.Errorf("catalog.json contains the absolute path %q — path leak", secretPath)
	}
	if strings.Contains(s, `"path"`) {
		t.Errorf(`catalog.json contains a "path" key — path leak`)
	}
}

func TestRenderMarkdownGroupsByCapability(t *testing.T) {
	c := Catalog{
		Components: []Component{{Name: "orders-api", Sidecar: Sidecar{Summary: "order service"}}},
		Capabilities: map[string][]string{
			"order-events": {"orders-api"},
			"postgres":     {"shared-postgres"},
		},
	}
	md := RenderMarkdown(c)
	if !strings.Contains(md, "## Capabilities") {
		t.Error("missing capabilities heading")
	}
	// sorted: order-events before postgres
	if strings.Index(md, "order-events") > strings.Index(md, "postgres") {
		t.Error("capabilities not sorted")
	}
	if !strings.Contains(md, "orders-api") || !strings.Contains(md, "order service") {
		t.Error("missing component detail")
	}
}

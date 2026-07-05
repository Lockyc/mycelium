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
	orphanPath := "/home/someone/private/undocumented"
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
		// an orphan also carries a node-local path that must not be serialized.
		Orphans: []Orphan{{ID: "github.com/acme/undocumented", Name: "undocumented", Path: orphanPath}},
	}
	out, err := RenderJSON(c)
	if err != nil {
		t.Fatalf("RenderJSON error: %v", err)
	}
	s := string(out)
	for _, p := range []string{secretPath, orphanPath} {
		if strings.Contains(s, p) {
			t.Errorf("catalog.json contains the absolute path %q — path leak", p)
		}
	}
	if strings.Contains(s, `"path"`) {
		t.Errorf(`catalog.json contains a "path" key — path leak`)
	}
}

func TestRenderMarkdownUndocumentedRepos(t *testing.T) {
	// Present: orphans are listed by name + canonical id, sorted, and the
	// node-local path never appears in the shared markdown.
	withOrphans := RenderMarkdown(Catalog{Orphans: []Orphan{
		{ID: "github.com/acme/zeta", Name: "zeta", Path: "/node/zeta"},
		{ID: "github.com/acme/alpha", Name: "alpha", Path: "/node/alpha"},
	}})
	if !strings.Contains(withOrphans, "## Undocumented repos") {
		t.Fatal("missing undocumented-repos heading")
	}
	if !strings.Contains(withOrphans, "**alpha** — github.com/acme/alpha") {
		t.Errorf("orphan not rendered as name + id:\n%s", withOrphans)
	}
	if strings.Index(withOrphans, "alpha") > strings.Index(withOrphans, "zeta") {
		t.Error("orphans not sorted by name")
	}
	if strings.Contains(withOrphans, "/node/") {
		t.Error("node-local orphan path leaked into markdown")
	}

	// Empty: the section is still rendered with an explicit None line, so an agent
	// can tell "fully documented" apart from "section absent".
	none := RenderMarkdown(Catalog{})
	if !strings.Contains(none, "## Undocumented repos") {
		t.Error("undocumented-repos section should render even when empty")
	}
	if !strings.Contains(none, "_None") {
		t.Errorf("empty state should show a None line:\n%s", none)
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

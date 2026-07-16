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

	// The blurb is the fix, so pin it. Agents read this section routinely; it is
	// the only routine reader an orphan's signal has. It must name the defect and
	// the remedy, or orphans get read past forever (which is what happened while
	// it said "look at them directly if relevant" — a workaround, not a report).
	for _, want := range []string{
		"not a normal state", // it is a defect
		"treat this catalog as incomplete",
		"close the gap",      // the call to action
		"`ignore` list",      // the deliberate-exclusion escape hatch
		"Check the filename", // the misnamed-sidecar trap
	} {
		if !strings.Contains(withOrphans, want) {
			t.Errorf("orphan blurb no longer states %q — it must read as a defect report,\n"+
				"not a workaround that lets a reader proceed without noticing:\n%s", want, withOrphans)
		}
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

// The map is component-first: an entry carries its own capabilities inline, so a
// reader learns what a thing is and what it provides without jumping. There is no
// full capability index — it was a near-bijection (one provider for all but a
// couple of capabilities), so it cost a line per capability to restate a component
// name that the entry below already carried, and told a reader nothing about the
// component it named.
func TestRenderMarkdownComponentFirst(t *testing.T) {
	c := Catalog{
		Components: []Component{{Name: "orders-api", Sidecar: Sidecar{
			Summary:  "order service",
			Provides: []Provides{{Name: "order-events"}, {Name: "order-api"}},
		}}},
		Capabilities: map[string][]string{
			"order-events": {"orders-api"},
			"order-api":    {"orders-api"},
		},
	}
	md := RenderMarkdown(c)
	if !strings.Contains(md, "## Components") {
		t.Fatal("missing components heading")
	}
	if strings.Contains(md, "## Capabilities") {
		t.Errorf("the full capability index is gone — capabilities render inline:\n%s", md)
	}
	if !strings.Contains(md, "Provides: **order-events**, **order-api**") {
		t.Errorf("capabilities not rendered inline in sidecar order:\n%s", md)
	}
	// inline means inside the entry: the capabilities follow the component heading.
	if strings.Index(md, "### orders-api") > strings.Index(md, "order-events") {
		t.Error("capabilities should render under their component, not above it")
	}
	if !strings.Contains(md, "order service") {
		t.Error("missing component detail")
	}
	// a component providing nothing renders no Provides line.
	bare := RenderMarkdown(Catalog{Components: []Component{{Name: "x", Sidecar: Sidecar{Summary: "s"}}}})
	if strings.Contains(bare, "Provides:") {
		t.Errorf("component with no provides should render no Provides line:\n%s", bare)
	}
}

// Only capabilities with more than one provider get a callout. A single-provider
// capability is already stated by its component's own entry, so listing it here
// would rebuild the index this layout deliberately dropped. Overlap is the one
// fact a component-first layout genuinely hides: it is visible only by noticing
// the same capability name on two separate entries.
func TestRenderMarkdownSharedCapabilities(t *testing.T) {
	c := Catalog{
		Components: []Component{
			{Name: "site", Sidecar: Sidecar{Summary: "s"}},
			{Name: "infra", Sidecar: Sidecar{Summary: "i"}},
		},
		Capabilities: map[string][]string{
			"newsletter": {"infra", "site"}, // shared — called out
			"hosting":    {"infra"},         // sole provider — not called out
		},
	}
	md := RenderMarkdown(c)
	if !strings.Contains(md, "## Shared capabilities") {
		t.Fatalf("missing shared-capabilities heading:\n%s", md)
	}
	shared := md[strings.Index(md, "## Shared capabilities"):]
	if !strings.Contains(shared, "**newsletter** — infra, site") {
		t.Errorf("multi-provider capability not called out:\n%s", shared)
	}
	if strings.Contains(shared, "hosting") {
		t.Errorf("single-provider capability must not be listed — that is the index again:\n%s", shared)
	}

	// No overlaps: the section is omitted entirely. Unlike the orphan section, an
	// absent overlap list is not a defect signal — every capability is already
	// stated by its component's entry, so there is nothing a reader could miss.
	solo := RenderMarkdown(Catalog{
		Components:   []Component{{Name: "infra", Sidecar: Sidecar{Summary: "i"}}},
		Capabilities: map[string][]string{"hosting": {"infra"}},
	})
	if strings.Contains(solo, "Shared capabilities") {
		t.Errorf("no overlaps should render no section:\n%s", solo)
	}
}

// Merge feeds overlay nodes into the capability index but Catalog carries them
// separately from Components — so dropping the index would have erased a node
// from the map entirely. Nodes are real entries in the ecosystem, not just names
// hanging off a capability, and this pins that they render as such.
func TestRenderMarkdownRendersOverlayNodes(t *testing.T) {
	c := Catalog{
		Components: []Component{{Name: "zeta", Sidecar: Sidecar{Summary: "a repo"}}},
		Nodes: []OverlayNode{{
			Name:     "shared-postgres",
			Summary:  "managed database, not a repo",
			Provides: []string{"postgres"},
		}},
		Capabilities: map[string][]string{"postgres": {"shared-postgres"}},
	}
	md := RenderMarkdown(c)
	if !strings.Contains(md, "### shared-postgres") {
		t.Fatalf("overlay node missing from the map:\n%s", md)
	}
	if !strings.Contains(md, "managed database, not a repo") {
		t.Errorf("overlay node summary not rendered:\n%s", md)
	}
	if !strings.Contains(md, "Provides: **postgres**") {
		t.Errorf("overlay node capabilities not rendered inline:\n%s", md)
	}
	// nodes and components interleave in one name-sorted list — a reader wants one
	// list of what exists, not two near-identical sections to cross-reference.
	if strings.Index(md, "shared-postgres") > strings.Index(md, "zeta") {
		t.Error("nodes and components should sort together by name")
	}
}

// ParseSidecar requires only name+summary, so kind and status are each optional.
// Joining them unconditionally rendered a dangling separator ("_ · active_").
func TestRenderMarkdownPartialKindStatus(t *testing.T) {
	render := func(kind, status string) string {
		return RenderMarkdown(Catalog{Components: []Component{{
			Name:    "x",
			Sidecar: Sidecar{Summary: "s", Kind: kind, Status: status},
		}}})
	}
	if md := render("tool", "active"); !strings.Contains(md, "_tool · active_") {
		t.Errorf("both present should join with a separator:\n%s", md)
	}
	if md := render("", "active"); !strings.Contains(md, "_active_") || strings.Contains(md, "·") {
		t.Errorf("missing kind should render no separator:\n%s", md)
	}
	if md := render("tool", ""); !strings.Contains(md, "_tool_") || strings.Contains(md, "·") {
		t.Errorf("missing status should render no separator:\n%s", md)
	}
	// scoped to the entry: the orphan section's own "_None …_" line is italic too.
	if md := render("", ""); !strings.Contains(md, "### x\ns\n\n") {
		t.Errorf("neither present should render no meta line at all:\n%s", md)
	}
}

func TestRenderMarkdownRendersTags(t *testing.T) {
	c := Catalog{Components: []Component{{
		Name:    "orders-api",
		Sidecar: Sidecar{Summary: "order service", Kind: "app", Status: "active", Tags: []string{"local-first", "prelaunch"}},
	}}}
	md := RenderMarkdown(c)
	if !strings.Contains(md, "`local-first` `prelaunch`") {
		t.Errorf("tags not rendered as backtick pills:\n%s", md)
	}
	// a component without tags renders no tag line (no stray backticks).
	none := RenderMarkdown(Catalog{Components: []Component{{Name: "x", Sidecar: Sidecar{Summary: "s"}}}})
	if strings.Contains(none, "`") {
		t.Errorf("tagless component should render no backticks:\n%s", none)
	}
}

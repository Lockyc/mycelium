package graph

import (
	"strings"
	"testing"
)

// TestRenderJSONPathNotLeaked asserts that Component.Path (an absolute filesystem
// path set by the scanner) is never serialised into graph.json output.
// This is the regression test for the path-leak fix (json:"-" on Component.Path).
func TestRenderJSONPathNotLeaked(t *testing.T) {
	secretPath := "/home/someone/private/widgets"
	orphanPath := "/home/someone/private/undocumented"
	c := Graph{
		Components: []Component{
			{
				ID:     "github.com/acme/widgets",
				Name:   "widgets",
				Path:   secretPath,
				Commit: "abc123",
				Sidecar: Sidecar{
					Name:    "widgets",
					Summary: "widget tracking app",
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
			t.Errorf("graph.json contains the absolute path %q — path leak", p)
		}
	}
	if strings.Contains(s, `"path"`) {
		t.Errorf(`graph.json contains a "path" key — path leak`)
	}
}

func TestRenderMarkdownUndocumentedRepos(t *testing.T) {
	// Present: orphans are listed by name + canonical id, sorted, and the
	// node-local path never appears in the shared markdown.
	withOrphans := RenderMarkdown(Graph{Orphans: []Orphan{
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
		"treat this map as incomplete",
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
	none := RenderMarkdown(Graph{})
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
	c := Graph{
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
	bare := RenderMarkdown(Graph{Components: []Component{{Name: "x", Sidecar: Sidecar{Summary: "s"}}}})
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
	c := Graph{
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
	solo := RenderMarkdown(Graph{
		Components:   []Component{{Name: "infra", Sidecar: Sidecar{Summary: "i"}}},
		Capabilities: map[string][]string{"hosting": {"infra"}},
	})
	if strings.Contains(solo, "Shared capabilities") {
		t.Errorf("no overlaps should render no section:\n%s", solo)
	}
}

// Merge feeds overlay nodes into the capability index but Graph carries them
// separately from Components — so dropping the index would have erased a node
// from the map entirely. Nodes are real entries in the ecosystem, not just names
// hanging off a capability, and this pins that they render as such.
func TestRenderMarkdownRendersOverlayNodes(t *testing.T) {
	c := Graph{
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

// Stack answers "what is this built with" — non-derivable from the summary and
// disjoint from tags (which answer "what is it about"). It is the one lossy-map
// field worth its bytes: ~20 B an entry to say `go` vs `rust, tauri, typescript`.
func TestRenderMarkdownRendersStack(t *testing.T) {
	md := RenderMarkdown(Graph{Components: []Component{{
		Name:    "curator",
		Sidecar: Sidecar{Summary: "s", Stack: []string{"rust", "tauri", "typescript"}},
	}}})
	if !strings.Contains(md, "Stack: rust, tauri, typescript") {
		t.Errorf("stack not rendered in sidecar order:\n%s", md)
	}
	bare := RenderMarkdown(Graph{Components: []Component{{Name: "x", Sidecar: Sidecar{Summary: "s"}}}})
	if strings.Contains(bare, "Stack:") {
		t.Errorf("component with no stack should render no Stack line:\n%s", bare)
	}
}

// "Used by" is the reverse of the *use* edges only, so the line means one thing:
// change this and these must be re-pinned or rebuilt. Reversing a thematic edge
// into it would be plainly false — "business sells reductable" does not make
// business a user of reductable.
func TestRenderMarkdownUsedBy(t *testing.T) {
	c := Graph{
		Components: []Component{
			{Name: "core", Sidecar: Sidecar{Summary: "a shared core"}},
			{Name: "product", Sidecar: Sidecar{Summary: "the thing sold"}},
		},
		Edges: []Edge{
			// use edges — reversed onto core, sorted despite the input order
			{From: "warden", To: "core", Type: "depends-on"},
			{From: "curator", To: "core", Type: "consumes"},
			{From: "app", To: "core", Type: "deploys-to"},
			// thematic edges — must never surface as "Used by"
			{From: "business", To: "product", Type: "sells"},
			{From: "site", To: "product", Type: "markets"},
			{From: "sibling", To: "product", Type: "related"},
		},
	}
	md := RenderMarkdown(c)
	if !strings.Contains(md, "Used by: app, curator, warden") {
		t.Errorf("use edges not reversed onto the target, sorted:\n%s", md)
	}
	product := md[strings.Index(md, "### product"):]
	product = product[:strings.Index(product, "## ")]
	if strings.Contains(product, "Used by") {
		t.Errorf("thematic edges (sells/markets/related) must not render as Used by:\n%s", product)
	}
	// the Relationships section still carries every edge, with its type intact.
	if !strings.Contains(md, "business sells product") {
		t.Errorf("thematic edges must survive in Relationships:\n%s", md)
	}
}

// ParseSidecar requires only name+summary, so kind and status are each optional.
// Joining them unconditionally rendered a dangling separator ("_ · active_").
func TestRenderMarkdownPartialKindStatus(t *testing.T) {
	render := func(kind, status string) string {
		return RenderMarkdown(Graph{Components: []Component{{
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

// The map lists capability names but drops their summaries (kept in graph.json).
// That omission is only recoverable if the map says so and names where to look —
// otherwise an agent reading the map can't tell a summary exists to query.
func TestRenderMarkdownPointsToCapabilitySummaries(t *testing.T) {
	md := RenderMarkdown(Graph{Components: []Component{{
		Name:    "homelab",
		Sidecar: Sidecar{Summary: "s", Provides: []Provides{{Name: "monitoring", Summary: "uptime checks"}}},
	}}})
	// The map still omits the summary itself...
	if strings.Contains(md, "uptime checks") {
		t.Errorf("capability summary must NOT render into the lossy map:\n%s", md)
	}
	// ...but must point the reader at the first-class query tool — `myco query` (no
	// jq, no need to know the JSON shape) — naming how to recover a capability summary.
	if !strings.Contains(md, "myco query capability") {
		t.Errorf("map must point at `myco query` for capability summaries:\n%s", md)
	}
}

func TestRenderMarkdownRendersTags(t *testing.T) {
	c := Graph{Components: []Component{{
		Name:    "orders-api",
		Sidecar: Sidecar{Summary: "order service", Kind: "app", Status: "active", Tags: []string{"local-first", "prelaunch"}},
	}}}
	md := RenderMarkdown(c)
	if !strings.Contains(md, "`local-first` `prelaunch`") {
		t.Errorf("tags not rendered as backtick pills:\n%s", md)
	}
	// a component without tags renders no tag line (no stray backtick pills). Scope
	// the check to the components section — the map preamble legitimately contains
	// backticks (the graph.json query hint), so whole-doc absence is not the signal.
	none := RenderMarkdown(Graph{Components: []Component{{Name: "x", Sidecar: Sidecar{Summary: "s"}}}})
	_, components, _ := strings.Cut(none, "## Components")
	if strings.Contains(components, "`") {
		t.Errorf("tagless component should render no backticks:\n%s", none)
	}
}

func TestRenderJSONIncludesDocGraphDigest(t *testing.T) {
	g := Graph{Components: []Component{{
		ID: "github.com/x/y", Name: "y",
		DocGraph: &DocGraphDigest{SchemaVersion: 1, DocCount: 4, ContentIslands: []string{"docs/stray.md"}},
	}}}
	b, err := RenderJSON(g)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if !strings.Contains(s, `"docGraph"`) || !strings.Contains(s, `"docs/stray.md"`) {
		t.Fatalf("graph.json must carry the digest: %s", s)
	}
	if strings.Contains(s, `"docGraphs"`) {
		t.Fatalf("graph.json must NOT carry out-of-band full payloads: %s", s)
	}
}

func TestRenderMarkdownFlagsDocIslands(t *testing.T) {
	g := Graph{Components: []Component{
		{ID: "github.com/x/rotty", Name: "rotty", Sidecar: Sidecar{Name: "rotty", Summary: "has rot"},
			DocGraph: &DocGraphDigest{SchemaVersion: 1, DocCount: 5, ContentIslands: []string{"docs/a.md"}, MetadataIslands: []string{"docs/b.md"}}},
		{ID: "github.com/x/clean", Name: "clean", Sidecar: Sidecar{Name: "clean", Summary: "no rot"},
			DocGraph: &DocGraphDigest{SchemaVersion: 1, DocCount: 3}},
	}}
	md := RenderMarkdown(g)
	if !strings.Contains(md, "docs: 2 islands ⚠") {
		t.Fatalf("expected rot flag for rotty:\n%s", md)
	}
	// clean component's block must carry no docs: line — extract just its section
	cleanIdx := strings.Index(md, "### clean")
	if cleanIdx < 0 {
		t.Fatal("clean entry missing")
	}
	// find the end of clean's block: the next ### heading or the next ## section
	cleanEnd := strings.Index(md[cleanIdx+1:], "###")
	if cleanEnd < 0 {
		cleanEnd = strings.Index(md[cleanIdx+1:], "## ")
	}
	if cleanEnd < 0 {
		cleanEnd = len(md)
	} else {
		cleanEnd += cleanIdx + 1
	}
	cleanBlock := md[cleanIdx:cleanEnd]
	if strings.Contains(cleanBlock, "docs:") {
		t.Fatalf("clean doc-graph must add nothing to MAP.md:\n%s", cleanBlock)
	}
}

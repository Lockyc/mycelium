package graph

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

func RenderJSON(g Graph) ([]byte, error) {
	return json.MarshalIndent(g, "", "  ")
}

// entry is the render-level union of the two things that appear in the map: a
// scanned repo (Component) and a non-repo overlay node. They render identically
// bar the fields a node has no notion of (kind, status, tags), which are already
// rendered conditionally.
type entry struct {
	name       string
	summary    string
	kind       string
	status     string
	tags       []string
	stack      []string
	provides   []string
	usedBy     []string
	docIslands int // content+metadata island count; a rot flag, only rendered when > 0
}

// useEdgeTypes are the edge types that mean "from actually uses to", so reversing
// one yields a true "Used by" — and, together, an entry's blast radius: change
// this thing and these are what must be re-pinned or rebuilt.
//
// The other types (markets, sells, related) are thematic, not consumption, so
// they are deliberately excluded: "business sells reductable" reversed onto
// reductable as "Used by: business" would be plainly false. They stay in the
// Relationships section, which renders every edge with its type intact.
var useEdgeTypes = map[string]bool{"consumes": true, "depends-on": true, "deploys-to": true}

func usedBy(edges []Edge) map[string][]string {
	rev := map[string][]string{}
	for _, e := range edges {
		if useEdgeTypes[e.Type] {
			rev[e.To] = append(rev[e.To], e.From)
		}
	}
	for _, users := range rev {
		sort.Strings(users) // edges arrive in overlay order; render deterministically
	}
	return rev
}

func joinNonEmpty(sep string, parts ...string) string {
	kept := parts[:0]
	for _, p := range parts {
		if p != "" {
			kept = append(kept, p)
		}
	}
	return strings.Join(kept, sep)
}

func entries(g Graph) []entry {
	rev := usedBy(g.Edges)
	out := make([]entry, 0, len(g.Components)+len(g.Nodes))
	for _, comp := range g.Components {
		provides := make([]string, 0, len(comp.Sidecar.Provides))
		for _, p := range comp.Sidecar.Provides {
			provides = append(provides, p.Name)
		}
		islands := 0
		if comp.DocGraph != nil {
			islands = len(comp.DocGraph.ContentIslands) + len(comp.DocGraph.MetadataIslands)
		}
		out = append(out, entry{
			name:       comp.Name,
			summary:    comp.Sidecar.Summary,
			kind:       comp.Sidecar.Kind,
			status:     comp.Sidecar.Status,
			tags:       comp.Sidecar.Tags,
			stack:      comp.Sidecar.Stack,
			provides:   provides,
			usedBy:     rev[comp.Name],
			docIslands: islands,
		})
	}
	for _, n := range g.Nodes {
		out = append(out, entry{name: n.Name, summary: n.Summary, provides: n.Provides, usedBy: rev[n.Name]})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].name < out[j].name })
	return out
}

// RenderMarkdown builds the lossy map an agent reads into context to orient.
//
// It is component-first: each entry states what a thing is and what it provides,
// together. The capability-first index this replaced was a near-bijection — all
// but a couple of capabilities had exactly one provider — so it spent a line per
// capability restating a component name, while telling a reader nothing about the
// component it named ("git-mirror — homelab" requires jumping to homelab's entry
// to learn what homelab even is). Sort order was its only real advantage, and
// that is worth nothing to this file's reader: MAP.md is read whole, into
// context. Lookup is graph.json's job — read to orient, query to extract.
func RenderMarkdown(g Graph) string {
	var b strings.Builder
	b.WriteString("# Mycelium map\n\n")

	// Frame the map for its reader: it is the "orient" surface, read whole into
	// context. Lookup is not its job — point at `myco query` as the first-class way
	// to query (it needs no knowledge of graph.json's shape, unlike jq). This also
	// self-describes the one lossy omission an agent can't otherwise tell is there —
	// each entry lists capability *names* but not their summaries/urls. Rendered
	// once, not per entry, so the map stays skimmable. (jq over graph.json is the
	// specific/programmatic path, documented in schema/graph.md, not here.)
	b.WriteString("A lossy overview to read whole and orient — *what exists, and when " +
		"does each apply?* To look anything up, **query the graph with `myco query`** " +
		"(the first-class way — it answers named questions without your needing to know " +
		"`graph.json`'s structure): `myco query` alone lists the queries; e.g. " +
		"`myco query capability <name>`, `myco query used-by <name>`. Capability " +
		"summaries and urls are dropped here to stay skimmable — recover them with " +
		"`myco query capability <name>`.\n\n")

	b.WriteString("## Components\n\n")
	for _, e := range entries(g) {
		fmt.Fprintf(&b, "### %s\n%s\n", e.name, e.summary)
		// Only name+summary are required of a sidecar (see ParseSidecar), so join
		// whichever of kind/status is present rather than assuming both — a missing
		// kind used to render a dangling separator ("_ · active_").
		if meta := joinNonEmpty(" · ", e.kind, e.status); meta != "" {
			fmt.Fprintf(&b, "_%s_\n", meta)
		}
		if len(e.provides) > 0 {
			bolded := make([]string, len(e.provides))
			for i, p := range e.provides {
				bolded[i] = "**" + p + "**"
			}
			fmt.Fprintf(&b, "Provides: %s\n", strings.Join(bolded, ", "))
		}
		if len(e.usedBy) > 0 {
			fmt.Fprintf(&b, "Used by: %s\n", strings.Join(e.usedBy, ", "))
		}
		if len(e.stack) > 0 {
			fmt.Fprintf(&b, "Stack: %s\n", strings.Join(e.stack, ", "))
		}
		if len(e.tags) > 0 {
			quoted := make([]string, len(e.tags))
			for i, t := range e.tags {
				quoted[i] = "`" + t + "`"
			}
			fmt.Fprintf(&b, "%s\n", strings.Join(quoted, " "))
		}
		// Doc-graph rot flag — only the exception surfaces in the lossy map (like
		// Shared capabilities): a clean doc-graph adds nothing; islands (unfindable
		// or unplaced docs) get one terse marker. Full digest lives in graph.json.
		if e.docIslands > 0 {
			fmt.Fprintf(&b, "docs: %d islands ⚠\n", e.docIslands)
		}
		b.WriteString("\n")
	}

	// Overlap is the one fact a component-first layout hides: with capabilities
	// listed per entry, "two components both do newsletter" is visible only by
	// noticing the same name twice, pages apart. So call out the multi-provider
	// capabilities — and only those. Listing sole-provider capabilities here would
	// rebuild the index that was just dropped, since an entry already states them.
	//
	// Omitted entirely when empty, unlike the orphan section below: an absent
	// overlap list is not a defect signal, and nothing is missing from the map
	// when it is absent — every capability still appears on its own entry.
	shared := make([]string, 0, len(g.Capabilities))
	for name, provs := range g.Capabilities {
		if len(provs) > 1 {
			shared = append(shared, name)
		}
	}
	if len(shared) > 0 {
		sort.Strings(shared)
		b.WriteString("## Shared capabilities\n\n")
		b.WriteString("Provided by more than one component — check both before adding a third.\n\n")
		for _, name := range shared {
			fmt.Fprintf(&b, "- **%s** — %s\n", name, strings.Join(g.Capabilities[name], ", "))
		}
		b.WriteString("\n")
	}

	if len(g.Edges) > 0 {
		b.WriteString("## Relationships\n\n")
		for _, e := range g.Edges {
			fmt.Fprintf(&b, "- %s %s %s\n", e.From, e.Type, e.To)
		}
	}

	// Always rendered so a consuming agent learns the section exists — an absent
	// section can't be told apart from "feature not present", but an explicit
	// "None" says the ecosystem is fully documented. Repos that intentionally
	// lack a sidecar are already filtered out via the overlay ignore list.
	//
	// The blurb must read as a DEFECT REPORT, not a workaround. It used to say
	// "look at them directly if relevant", which framed an orphan as a normal
	// alternate path and gave the reader a way to proceed without ever noticing
	// anything was wrong — so orphans sat unfixed while agents read past them
	// every day. This section's reader is the only routine reader the signal
	// has; if the wording doesn't prompt a fix, nothing does.
	b.WriteString("\n## Undocumented repos\n\n")
	if len(g.Orphans) == 0 {
		b.WriteString("_None — every scanned repo has an entry above._\n")
	} else {
		b.WriteString("**Gaps in this map, not a normal state.** Each repo below was scanned " +
			"but carries no `mycelium.toml`, so its purpose and capabilities are missing here — " +
			"treat this map as incomplete while any remain.\n\n" +
			"Read such a repo directly for now. **If you are working in one, close the gap in " +
			"that change:** commit a `mycelium.toml` (identity + `provides`), or add its id to " +
			"the overlay's `ignore` list if it is deliberately undocumented. Already added one " +
			"and still see it here? Check the filename — `mycelium.toml` is the only name read.\n\n")
		orphans := append([]Orphan(nil), g.Orphans...)
		sort.Slice(orphans, func(i, j int) bool { return orphans[i].Name < orphans[j].Name })
		for _, o := range orphans {
			fmt.Fprintf(&b, "- **%s** — %s\n", o.Name, o.ID)
		}
	}
	return b.String()
}

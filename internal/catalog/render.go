package catalog

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

func RenderJSON(c Catalog) ([]byte, error) {
	return json.MarshalIndent(c, "", "  ")
}

// entry is the render-level union of the two things that appear in the map: a
// scanned repo (Component) and a non-repo overlay node. They render identically
// bar the fields a node has no notion of (kind, status, tags), which are already
// rendered conditionally.
type entry struct {
	name     string
	summary  string
	kind     string
	status   string
	tags     []string
	provides []string
}

func entries(c Catalog) []entry {
	out := make([]entry, 0, len(c.Components)+len(c.Nodes))
	for _, comp := range c.Components {
		provides := make([]string, 0, len(comp.Sidecar.Provides))
		for _, p := range comp.Sidecar.Provides {
			provides = append(provides, p.Name)
		}
		out = append(out, entry{
			name:     comp.Name,
			summary:  comp.Sidecar.Summary,
			kind:     comp.Sidecar.Kind,
			status:   comp.Sidecar.Status,
			tags:     comp.Sidecar.Tags,
			provides: provides,
		})
	}
	for _, n := range c.Nodes {
		out = append(out, entry{name: n.Name, summary: n.Summary, provides: n.Provides})
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
// that is worth nothing to this file's reader: CATALOG.md is read whole, into
// context. Lookup is catalog.json's job — read to orient, query to extract.
func RenderMarkdown(c Catalog) string {
	var b strings.Builder
	b.WriteString("# Mycelium catalog\n\n")

	b.WriteString("## Components\n\n")
	for _, e := range entries(c) {
		fmt.Fprintf(&b, "### %s\n%s\n", e.name, e.summary)
		if e.kind != "" || e.status != "" {
			fmt.Fprintf(&b, "_%s · %s_\n", e.kind, e.status)
		}
		if len(e.provides) > 0 {
			bolded := make([]string, len(e.provides))
			for i, p := range e.provides {
				bolded[i] = "**" + p + "**"
			}
			fmt.Fprintf(&b, "Provides: %s\n", strings.Join(bolded, ", "))
		}
		if len(e.tags) > 0 {
			quoted := make([]string, len(e.tags))
			for i, t := range e.tags {
				quoted[i] = "`" + t + "`"
			}
			fmt.Fprintf(&b, "%s\n", strings.Join(quoted, " "))
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
	shared := make([]string, 0, len(c.Capabilities))
	for name, provs := range c.Capabilities {
		if len(provs) > 1 {
			shared = append(shared, name)
		}
	}
	if len(shared) > 0 {
		sort.Strings(shared)
		b.WriteString("## Shared capabilities\n\n")
		b.WriteString("Provided by more than one component — check both before adding a third.\n\n")
		for _, name := range shared {
			fmt.Fprintf(&b, "- **%s** — %s\n", name, strings.Join(c.Capabilities[name], ", "))
		}
		b.WriteString("\n")
	}

	if len(c.Edges) > 0 {
		b.WriteString("## Relationships\n\n")
		for _, e := range c.Edges {
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
	if len(c.Orphans) == 0 {
		b.WriteString("_None — every scanned repo has a catalog entry._\n")
	} else {
		b.WriteString("**Gaps in this map, not a normal state.** Each repo below was scanned " +
			"but carries no `mycelium.toml`, so its purpose and capabilities are missing here — " +
			"treat this catalog as incomplete while any remain.\n\n" +
			"Read such a repo directly for now. **If you are working in one, close the gap in " +
			"that change:** commit a `mycelium.toml` (identity + `provides`), or add its id to " +
			"the overlay's `ignore` list if it is deliberately uncatalogued. Already added one " +
			"and still see it here? Check the filename — `mycelium.toml` is the only name read.\n\n")
		orphans := append([]Orphan(nil), c.Orphans...)
		sort.Slice(orphans, func(i, j int) bool { return orphans[i].Name < orphans[j].Name })
		for _, o := range orphans {
			fmt.Fprintf(&b, "- **%s** — %s\n", o.Name, o.ID)
		}
	}
	return b.String()
}

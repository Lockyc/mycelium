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

func RenderMarkdown(c Catalog) string {
	var b strings.Builder
	b.WriteString("# Mycelium catalog\n\n")

	b.WriteString("## Capabilities\n\n")
	capNames := make([]string, 0, len(c.Capabilities))
	for k := range c.Capabilities {
		capNames = append(capNames, k)
	}
	sort.Strings(capNames)
	for _, name := range capNames {
		fmt.Fprintf(&b, "- **%s** — %s\n", name, strings.Join(c.Capabilities[name], ", "))
	}

	b.WriteString("\n## Components\n\n")
	comps := append([]Component(nil), c.Components...)
	sort.Slice(comps, func(i, j int) bool { return comps[i].Name < comps[j].Name })
	for _, comp := range comps {
		fmt.Fprintf(&b, "### %s\n%s\n", comp.Name, comp.Sidecar.Summary)
		if comp.Sidecar.Kind != "" || comp.Sidecar.Status != "" {
			fmt.Fprintf(&b, "_%s · %s_\n", comp.Sidecar.Kind, comp.Sidecar.Status)
		}
		if len(comp.Sidecar.Tags) > 0 {
			quoted := make([]string, len(comp.Sidecar.Tags))
			for i, t := range comp.Sidecar.Tags {
				quoted[i] = "`" + t + "`"
			}
			fmt.Fprintf(&b, "%s\n", strings.Join(quoted, " "))
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
	b.WriteString("\n## Undocumented repos\n\n")
	if len(c.Orphans) == 0 {
		b.WriteString("_None — every scanned repo has a catalog entry._\n")
	} else {
		b.WriteString("These repos exist in the ecosystem but have no catalog.toml yet — " +
			"look at them directly if relevant.\n\n")
		orphans := append([]Orphan(nil), c.Orphans...)
		sort.Slice(orphans, func(i, j int) bool { return orphans[i].Name < orphans[j].Name })
		for _, o := range orphans {
			fmt.Fprintf(&b, "- **%s** — %s\n", o.Name, o.ID)
		}
	}
	return b.String()
}

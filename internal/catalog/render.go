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
		b.WriteString("\n")
	}

	if len(c.Edges) > 0 {
		b.WriteString("## Relationships\n\n")
		for _, e := range c.Edges {
			fmt.Fprintf(&b, "- %s %s %s\n", e.From, e.Type, e.To)
		}
	}
	return b.String()
}

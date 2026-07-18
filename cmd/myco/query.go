package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/lockyc/mycelium/internal/graph"
	"github.com/lockyc/mycelium/internal/query"
)

func runQuery(args []string) error {
	if len(args) == 0 {
		fmt.Print(queryIndex())
		return nil
	}
	sub := args[0]
	fs := flag.NewFlagSet("query", flag.ContinueOnError)
	dir := fs.String("dir", ".", "artifact dir (with graph.json)")
	url := fs.String("url", "", "hub URL to fetch graph.json from instead of --dir")
	asJSON := fs.Bool("json", false, "emit JSON instead of text")
	kind := fs.String("kind", "", "components: filter by kind")
	stack := fs.String("stack", "", "components: filter by stack member")
	status := fs.String("status", "", "components: filter by status")
	tag := fs.String("tag", "", "components: filter by tag")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	pos := fs.Args()

	g, err := loadQueryGraph(*dir, *url)
	if err != nil {
		return err
	}

	switch sub {
	case "capabilities":
		return emit(*asJSON, query.Capabilities(g), textCapabilities)
	case "capability":
		name, err := arg1(pos, sub)
		if err != nil {
			return err
		}
		v, ok := query.Capability(g, name)
		if !ok {
			return fmt.Errorf("no such capability: %s", name)
		}
		return emit(*asJSON, v, textCapability)
	case "component":
		name, err := arg1(pos, sub)
		if err != nil {
			return err
		}
		c, ok := query.Component(g, name)
		if !ok {
			return fmt.Errorf("no such component: %s", name)
		}
		return emit(*asJSON, c, textComponent)
	case "components":
		cs := query.Components(g, query.ComponentFilter{Kind: *kind, Stack: *stack, Status: *status, Tag: *tag})
		return emit(*asJSON, cs, textComponents)
	case "used-by":
		name, err := arg1(pos, sub)
		if err != nil {
			return err
		}
		rels, ok := query.UsedBy(g, name)
		if !ok {
			return fmt.Errorf("no such component: %s", name)
		}
		return emit(*asJSON, rels, textRelations)
	case "uses":
		name, err := arg1(pos, sub)
		if err != nil {
			return err
		}
		rels, ok := query.Uses(g, name)
		if !ok {
			return fmt.Errorf("no such component: %s", name)
		}
		return emit(*asJSON, rels, textRelations)
	case "search":
		text, err := arg1(pos, sub)
		if err != nil {
			return err
		}
		return emit(*asJSON, query.Search(g, text), textSearch)
	default:
		return fmt.Errorf("unknown query %q\n\n%s", sub, queryIndex())
	}
}

func arg1(pos []string, sub string) (string, error) {
	if len(pos) < 1 {
		return "", fmt.Errorf("query %s needs an argument (e.g. `myco query %s <name>`)", sub, sub)
	}
	return pos[0], nil
}

// emit prints v as JSON when asJSON, else via the text renderer.
func emit[T any](asJSON bool, v T, text func(T) string) error {
	if asJSON {
		b, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(b))
		return nil
	}
	fmt.Print(text(v))
	return nil
}

func loadQueryGraph(dir, url string) (graph.Graph, error) {
	var data []byte
	var err error
	if url != "" {
		data, err = httpGet(strings.TrimSuffix(url, "/") + "/" + graph.GraphJSONName)
	} else {
		data, err = os.ReadFile(filepath.Join(dir, graph.GraphJSONName))
	}
	if err != nil {
		return graph.Graph{}, err
	}
	var g graph.Graph
	if err := json.Unmarshal(data, &g); err != nil {
		return graph.Graph{}, fmt.Errorf("parse %s: %w", graph.GraphJSONName, err)
	}
	return g, nil
}

func httpGet(u string) ([]byte, error) {
	resp, err := http.Get(u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: %s", u, resp.Status)
	}
	return io.ReadAll(resp.Body)
}

func queryIndex() string {
	var b strings.Builder
	b.WriteString("myco query <name> [args] — first-class graph queries (no jq needed)\n\n")
	for _, d := range query.Descriptors() {
		fmt.Fprintf(&b, "  %-13s %-32s %s\n", d.Name, d.Args, d.Example)
	}
	b.WriteString("\nflags: --dir <artifact dir> | --url <hub> | --json\n")
	return b.String()
}

// --- text renderers (one per result type) ---

func textCapabilities(vs []query.CapabilityView) string {
	var b strings.Builder
	for _, v := range vs {
		b.WriteString(textCapability(v))
	}
	return b.String()
}

func textCapability(v query.CapabilityView) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n", v.Name)
	if v.Summary != "" {
		fmt.Fprintf(&b, "  %s\n", v.Summary)
	}
	if v.URL != "" {
		fmt.Fprintf(&b, "  url: %s\n", v.URL)
	}
	fmt.Fprintf(&b, "  providers: %s\n", strings.Join(v.Providers, ", "))
	return b.String()
}

func textComponent(c graph.Component) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s — %s\n", c.Name, c.Sidecar.Summary)
	if meta := strings.TrimSpace(c.Sidecar.Kind + " " + c.Sidecar.Status); meta != "" {
		fmt.Fprintf(&b, "  %s\n", meta)
	}
	if len(c.Sidecar.Stack) > 0 {
		fmt.Fprintf(&b, "  stack: %s\n", strings.Join(c.Sidecar.Stack, ", "))
	}
	for _, p := range c.Sidecar.Provides {
		fmt.Fprintf(&b, "  provides %s: %s\n", p.Name, p.Summary)
	}
	return b.String()
}

func textComponents(cs []graph.Component) string {
	var b strings.Builder
	for _, c := range cs {
		fmt.Fprintf(&b, "%s — %s\n", c.Name, c.Sidecar.Summary)
	}
	return b.String()
}

func textRelations(rs []query.Relation) string {
	var b strings.Builder
	for _, r := range rs {
		fmt.Fprintf(&b, "  %s (%s)\n", r.Name, r.Type)
	}
	if len(rs) == 0 {
		b.WriteString("  (none)\n")
	}
	return b.String()
}

func textSearch(hs []query.SearchHit) string {
	var b strings.Builder
	for _, h := range hs {
		if h.Kind == "capability" {
			fmt.Fprintf(&b, "  (capability) %s\n", h.Name)
		} else {
			fmt.Fprintf(&b, "  %s — %s\n", h.Name, h.Summary)
		}
	}
	if len(hs) == 0 {
		b.WriteString("  (no matches)\n")
	}
	return b.String()
}

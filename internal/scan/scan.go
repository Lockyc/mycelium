package scan

import (
	"encoding/json"

	"github.com/lockyc/mycelium/internal/graph"
)

type Options struct {
	Node          string
	Source        string
	Now           string
	FallbackHost  string
	ExcludeOwners []string
	Ref           string // git ref to read sidecars from; "" or absent → HEAD

	// DocGraph runs docgraph for a repo at a git ref; nil uses the real runDocGraph.
	// Injected so tests need no docgraph binary and CI stays green without it.
	DocGraph DocGraphFunc
}

func Scan(roots []string, opts Options) (graph.Manifest, error) {
	deny := map[string]bool{}
	for _, o := range opts.ExcludeOwners {
		deny[o] = true
	}

	repos, err := DiscoverRepos(roots)
	if err != nil {
		return graph.Manifest{}, err
	}

	run := opts.DocGraph
	if run == nil {
		run = runDocGraph
	}

	m := graph.Manifest{Node: opts.Node, Source: opts.Source, ScannedAt: opts.Now}
	for _, r := range repos {
		if deny[r.Owner] {
			continue
		}
		ref := resolveRef(r, opts.Ref)
		data, found, err := sidecarAtRef(r, ref)
		if err != nil {
			return graph.Manifest{}, err
		}
		if !found {
			m.Orphans = append(m.Orphans, graph.Orphan{
				ID:   repoID(r, opts.FallbackHost),
				Name: r.Name,
				Path: r.Dir,
			})
			continue
		}
		sc, err := graph.ParseSidecar(data)
		if err != nil {
			return graph.Manifest{}, err
		}
		commit, _ := r.Git("rev-parse", ref).Output()
		comp := graph.Component{
			ID:      repoID(r, opts.FallbackHost),
			Name:    sc.Name,
			Commit:  trim(commit),
			Sidecar: sc,
		}
		// Best-effort doc-graph: docgraph reads the committed ref from the object
		// store (docgraph v3.1.0+), so this runs on bare repos too and reflects the
		// exact scanned ref. Any failure is non-fatal — the component simply carries
		// no doc-graph.
		if raw, err := run(r.Dir, ref); err == nil {
			if digest, full, derr := buildDigest(raw); derr == nil && digest != nil {
				comp.DocGraph = digest
				if full != nil {
					if m.DocGraphs == nil {
						m.DocGraphs = map[string]json.RawMessage{}
					}
					m.DocGraphs[comp.ID] = full
				}
			}
		}
		m.Components = append(m.Components, comp)
	}
	return m, nil
}

func trim(b []byte) string {
	s := string(b)
	for len(s) > 0 && (s[len(s)-1] == '\n' || s[len(s)-1] == '\r') {
		s = s[:len(s)-1]
	}
	return s
}

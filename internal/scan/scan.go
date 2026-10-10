package scan

import (
	"encoding/json"
	"strings"

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
		// One repo's broken sidecar never fails the node's scan: it is recorded
		// for the audit and the rest of the node still reaches the hub.
		invalid := func(err error) {
			m.InvalidSidecars = append(m.InvalidSidecars, graph.InvalidSidecar{
				ID:    repoID(r, opts.FallbackHost),
				Name:  r.Name,
				Error: err.Error(),
				Path:  r.Dir,
			})
		}
		data, found, err := sidecarAtRef(r, ref)
		if err != nil {
			invalid(err)
			continue
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
			invalid(err)
			continue
		}
		commit, _ := r.Git("rev-parse", ref).Output()
		comp := graph.Component{
			ID:      repoID(r, opts.FallbackHost),
			Name:    sc.Name,
			Commit:  strings.TrimRight(string(commit), "\r\n"),
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

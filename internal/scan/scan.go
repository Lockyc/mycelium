package scan

import (
	"github.com/lockyc/mycelium/internal/graph"
)

type Options struct {
	Node          string
	Source        string
	Now           string
	FallbackHost  string
	ExcludeOwners []string
	Ref           string // git ref to read sidecars from; "" or absent → HEAD
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
		m.Components = append(m.Components, graph.Component{
			ID:      repoID(r, opts.FallbackHost),
			Name:    sc.Name,
			Commit:  trim(commit),
			Sidecar: sc,
		})
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

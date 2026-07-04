package scan

import (
	"github.com/lockyc/mycelium/internal/catalog"
)

type Options struct {
	Node          string
	Source        string
	Now           string
	FallbackHost  string
	ExcludeOwners []string
	Ref           string // git ref to read sidecars from; "" or absent → HEAD
}

func Scan(roots []string, opts Options) (catalog.Manifest, []string, error) {
	deny := map[string]bool{}
	for _, o := range opts.ExcludeOwners {
		deny[o] = true
	}

	repos, err := DiscoverRepos(roots)
	if err != nil {
		return catalog.Manifest{}, nil, err
	}

	m := catalog.Manifest{Node: opts.Node, Source: opts.Source, ScannedAt: opts.Now}
	var orphans []string
	for _, r := range repos {
		if deny[r.Owner] {
			continue
		}
		ref := resolveRef(r, opts.Ref)
		data, found, err := sidecarAtRef(r, ref)
		if err != nil {
			return catalog.Manifest{}, nil, err
		}
		if !found {
			orphans = append(orphans, r.Dir)
			continue
		}
		sc, err := catalog.ParseSidecar(data)
		if err != nil {
			return catalog.Manifest{}, nil, err
		}
		commit, _ := r.Git("rev-parse", ref).Output()
		m.Components = append(m.Components, catalog.Component{
			ID:      repoID(r, opts.FallbackHost),
			Name:    sc.Name,
			Commit:  trim(commit),
			Sidecar: sc,
		})
	}
	return m, orphans, nil
}

func trim(b []byte) string {
	s := string(b)
	for len(s) > 0 && (s[len(s)-1] == '\n' || s[len(s)-1] == '\r') {
		s = s[:len(s)-1]
	}
	return s
}

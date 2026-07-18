package graph

import "testing"

func TestRepoDocGraphRoute(t *testing.T) {
	got := RepoDocGraphRoute("github.com/lockyc/mycelium")
	want := "/repos/github.com/lockyc/mycelium/docgraph.json"
	if got != want {
		t.Fatalf("RepoDocGraphRoute = %q, want %q", got, want)
	}
	// The route is built from the same prefix/suffix the serve handler parses.
	if RepoDocGraphPrefix != "/repos/" || RepoDocGraphSuffix != "/docgraph.json" {
		t.Fatalf("route constants drifted: prefix=%q suffix=%q", RepoDocGraphPrefix, RepoDocGraphSuffix)
	}
}

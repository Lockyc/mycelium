package graph

import "testing"

func TestCanonicalID(t *testing.T) {
	cases := map[string]string{
		"https://github.com/acme/widgets.git":        "github.com/acme/widgets",
		"git@github.com:acme/widgets.git":            "github.com/acme/widgets",
		"https://GitHub.com/acme/Widgets":            "github.com/acme/widgets",
		"ssh://git@git.example.com/acme/billing.git": "git.example.com/acme/billing",
		// one repo over different transports is one id: the port is not identity
		"ssh://git@git.example.com:2222/acme/billing.git": "git.example.com/acme/billing",
		"https://git.example.com:8443/acme/billing":       "git.example.com/acme/billing",
		"git@git.example.com:acme/billing":                "git.example.com/acme/billing",
		// scp-style with no user (an ssh Host alias) keeps its owner
		"github.com:acme/widgets.git": "github.com/acme/widgets",
		// a trailing slash must not shield the .git suffix
		"https://github.com/acme/widgets.git/": "github.com/acme/widgets",
	}
	for in, want := range cases {
		if got := CanonicalID(in); got != want {
			t.Errorf("CanonicalID(%q) = %q, want %q", in, got, want)
		}
	}
}

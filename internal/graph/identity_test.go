package graph

import "testing"

func TestCanonicalID(t *testing.T) {
	cases := map[string]string{
		"https://github.com/acme/widgets.git":        "github.com/acme/widgets",
		"git@github.com:acme/widgets.git":            "github.com/acme/widgets",
		"https://GitHub.com/acme/Widgets":            "github.com/acme/widgets",
		"ssh://git@git.example.com/acme/billing.git": "git.example.com/acme/billing",
	}
	for in, want := range cases {
		if got := CanonicalID(in); got != want {
			t.Errorf("CanonicalID(%q) = %q, want %q", in, got, want)
		}
	}
}

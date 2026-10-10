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
		// a trailing slash must not shield the .git suffix
		"https://github.com/acme/widgets.git/": "github.com/acme/widgets",
	}
	for in, want := range cases {
		if got := CanonicalID(in); got != want {
			t.Errorf("CanonicalID(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestSafeRelID exercises the path-safety predicate shared by the hub's
// write-time guard (writeDocGraphs) and the serve route's read-time guard
// (/repos/<id>/docgraph.json) — both trust boundaries gate on this one function.
func TestSafeRelID(t *testing.T) {
	cases := []struct {
		id   string
		want bool
	}{
		{"", false},
		{"..", false},
		{"../x", false},
		{"a/../../b", false},
		{".hidden", false},
		{"a/../b", false},
		{"github.com/x/y", true},
		{"git.example.com/owner/repo", true},
	}
	for _, c := range cases {
		if got := SafeRelID(c.id); got != c.want {
			t.Errorf("SafeRelID(%q) = %v, want %v", c.id, got, c.want)
		}
	}
}

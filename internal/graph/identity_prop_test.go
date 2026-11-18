package graph

import (
	"regexp"
	"strings"
	"testing"

	"pgregory.net/rapid"
)

var (
	propLabel = rapid.StringMatching(`[a-zA-Z0-9][a-zA-Z0-9-]{0,6}`)
	propPath  = rapid.StringMatching(`[a-zA-Z0-9][a-zA-Z0-9_-]{0,6}(\.[a-zA-Z0-9]{1,4})?`)
)

// TestCanonicalIDTransportEquivalence spells one repo every way a git remote
// can — https and ssh URLs with or without user and port, scp with or without
// user, each with or without .git and a trailing slash — and checks that all of
// them canonicalise to the same lower-cased host/owner/repo. Components dedupe
// on this id, so a spelling that misses merges two repos or splits one.
func TestCanonicalIDTransportEquivalence(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		host := strings.Join(rapid.SliceOfN(propLabel, 1, 3).Draw(t, "host"), ".")
		owner := strings.Join(rapid.SliceOfN(propPath, 1, 2).Draw(t, "owner"), "/")
		repo := propPath.Draw(t, "repo")
		user := rapid.SampledFrom([]string{"", "git", "deploy", "me:tok3n"}).Draw(t, "user")
		port := rapid.SampledFrom([]string{"", "22", "2222", "8443"}).Draw(t, "port")
		suffix := rapid.SampledFrom([]string{"", ".git", "/", ".git/"}).Draw(t, "suffix")
		if strings.HasSuffix(strings.ToLower(repo), ".git") {
			t.Skip("a repo name ending .git is indistinguishable from the suffix")
		}

		at := ""
		if user != "" {
			at = user + "@"
		}
		hostPort := host
		if port != "" {
			hostPort += ":" + port
		}
		path := owner + "/" + repo + suffix
		spellings := []string{
			"https://" + at + hostPort + "/" + path,
			"ssh://" + at + hostPort + "/" + path,
			"git://" + hostPort + "/" + path,
		}
		if !strings.Contains(user, ":") { // scp form has no password field
			spellings = append(spellings, at+host+":"+path)
		}
		want := strings.ToLower(host + "/" + owner + "/" + repo)
		for _, s := range spellings {
			if got := CanonicalID(s); got != want {
				t.Fatalf("CanonicalID(%q) = %q, want %q", s, got, want)
			}
		}
	})
}

// plausibleID is the shape of an id a real remote yields: a host, then path
// segments of the characters forges allow in owner and repo names.
var plausibleID = regexp.MustCompile(`^[a-z0-9.-]+(/[a-z0-9._-]+)+$`)

// FuzzCanonicalID checks that CanonicalID never panics, and that an id it
// produces canonicalises to itself: the overlay's ignore list canonicalises
// entries that are usually ids already, so an id that maps elsewhere never
// matches its orphan. Ids no forge could produce (a repo still ending .git
// after one strip) are left out. The seeds run in the gate; `just fuzz`
// explores.
func FuzzCanonicalID(f *testing.F) {
	for _, s := range []string{
		"https://github.com/acme/widgets.git",
		"git@github.com:acme/widgets.git",
		"https://GitHub.com/acme/Widgets",
		"ssh://git@git.example.com/acme/billing.git",
		"ssh://git@git.example.com:2222/acme/billing.git",
		"https://git.example.com:8443/acme/billing",
		"git@git.example.com:acme/billing",
		"github.com:acme/widgets.git",
		"https://github.com/acme/widgets.git/",
		"github.com/acme/widgets",
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, remote string) {
		id := CanonicalID(remote)
		if !plausibleID.MatchString(id) || strings.HasSuffix(id, ".git") {
			return
		}
		if again := CanonicalID(id); again != id {
			t.Fatalf("CanonicalID not idempotent: %q -> %q -> %q", remote, id, again)
		}
	})
}

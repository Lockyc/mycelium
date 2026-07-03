package main

import "testing"

func TestUsageMentionsSubcommands(t *testing.T) {
	got := usage()
	for _, sub := range []string{"scan", "build", "serve", "audit", "validate"} {
		if !contains(got, sub) {
			t.Errorf("usage() missing subcommand %q", sub)
		}
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || indexOf(s, sub) >= 0)
}
func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

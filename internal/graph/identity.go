package graph

import "strings"

// CanonicalID normalizes a git remote URL to a stable host/owner/repo identity.
// Handles scp-style (git@host:owner/repo.git) and URL-style (scheme://[user@]host/owner/repo.git).
func CanonicalID(remote string) string {
	s := strings.TrimSpace(remote)
	// scp-style: git@host:owner/repo(.git)
	if !strings.Contains(s, "://") && strings.Contains(s, "@") && strings.Contains(s, ":") {
		at := strings.Index(s, "@")
		s = s[at+1:]                        // host:owner/repo.git
		s = strings.Replace(s, ":", "/", 1) // host/owner/repo.git
	} else {
		if i := strings.Index(s, "://"); i >= 0 {
			s = s[i+3:] // [user@]host/owner/repo.git
		}
		if at := strings.Index(s, "@"); at >= 0 {
			s = s[at+1:]
		}
	}
	s = strings.TrimSuffix(s, ".git")
	s = strings.Trim(s, "/")
	parts := strings.SplitN(s, "/", 2)
	if len(parts) == 2 {
		return strings.ToLower(parts[0]) + "/" + strings.ToLower(parts[1])
	}
	return strings.ToLower(s)
}

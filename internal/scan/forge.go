package scan

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/lockyc/mycelium/internal/graph"
)

const (
	forgeTimeout     = 3 * time.Second
	forgeConcurrency = 8
)

// DefaultAPIBase maps a forge host to the repos endpoint prefix of its API:
// GitHub's own API host, or the Forgejo/Gitea /api/v1 path on any other host.
func DefaultAPIBase(host string) string {
	if host == "github.com" {
		return "https://api.github.com/repos"
	}
	return "https://" + host + "/api/v1/repos"
}

// githubAPIHost is the only host that receives GITHUB_TOKEN.
const githubAPIHost = "api.github.com"

// Visibility asks the forge whether the repo behind a
// component id (host/owner/repo) is "public" or "private". Forges hide private
// repos from anonymous callers, so a 404 means private. Anything else that is
// not a clear answer (network failure, 5xx, rate limit, a redirect to a login
// page, an unreadable body) returns "" rather than guessing. Requests are
// anonymous, except that GITHUB_TOKEN, when set, is sent as a bearer token to
// api.github.com alone (lifting GitHub's anonymous rate limit); a 404 still
// reads as private, since the token may lack access to the repo.
func Visibility(c *http.Client, apiBase func(host string) string, id string) string {
	parts := strings.Split(id, "/")
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return ""
	}
	// A redirect is a login wall or a moved repo, never an answer about this one.
	noRedirect := *c
	noRedirect.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	target := apiBase(parts[0]) + "/" + parts[1] + "/" + parts[2]
	req, err := http.NewRequest(http.MethodGet, target, nil)
	if err != nil {
		return ""
	}
	if tok := os.Getenv("GITHUB_TOKEN"); tok != "" && req.URL.Host == githubAPIHost {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	resp, err := noRedirect.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusNotFound:
		return "private"
	case http.StatusOK:
		var body struct {
			Private *bool `json:"private"`
		}
		if json.NewDecoder(resp.Body).Decode(&body) != nil || body.Private == nil {
			return ""
		}
		if *body.Private {
			return "private"
		}
		return "public"
	}
	return ""
}

// fillVisibility sets Facts.Visibility on every component, at most
// forgeConcurrency lookups at a time.
func fillVisibility(comps []graph.Component, apiBase func(string) string) {
	if apiBase == nil {
		apiBase = DefaultAPIBase
	}
	client := &http.Client{Timeout: forgeTimeout}
	sem := make(chan struct{}, forgeConcurrency)
	var wg sync.WaitGroup
	for i := range comps {
		wg.Add(1)
		sem <- struct{}{}
		go func(c *graph.Component) {
			defer wg.Done()
			defer func() { <-sem }()
			c.Facts.Visibility = Visibility(client, apiBase, c.ID)
		}(&comps[i])
	}
	wg.Wait()
}

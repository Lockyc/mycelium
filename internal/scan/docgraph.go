package scan

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"

	"github.com/lockyc/mycelium/internal/graph"
)

// docgraphSchemaVersion is the docgraph graph payload version Mycelium understands.
// A payload with any other version is recorded-but-not-interpreted (see buildDigest).
const docgraphSchemaVersion = 1

// conventionalEntryDocs is Mycelium's set of orientation-doc paths. docgraph's
// frozen contract exposes no root/entry flag, so entryDocs is the subset of THIS
// set present in the repo — single-sourced here, referenced nowhere else.
var conventionalEntryDocs = []string{"CLAUDE.md", "README.md", "docs/index.md"}

// stderr is used by the not-installed warning, capturable in tests.
var stderr io.Writer = os.Stderr

// DocGraphFunc runs docgraph for a repo checkout and returns its raw JSON output.
// Injected via scan.Options.DocGraph so tests need no docgraph binary; nil uses
// the real runDocGraph.
type DocGraphFunc func(repoPath string) ([]byte, error)

// errDocGraphNotInstalled is the sentinel for "docgraph is not on PATH" — the
// benign degradation case (the component simply carries no doc-graph).
var errDocGraphNotInstalled = errors.New("docgraph not on PATH")

var notInstalledOnce sync.Once

// runDocGraph shells out to `docgraph graph --json` in repoPath. It is read-only
// (docgraph reads via `git ls-files`), preserving the nodes-are-read-only-on-repos
// invariant. A missing binary is reported as errDocGraphNotInstalled, logged once.
func runDocGraph(repoPath string) ([]byte, error) {
	cmd := exec.Command("docgraph", "graph", "--json")
	cmd.Dir = repoPath
	out, err := cmd.Output()
	if errors.Is(err, exec.ErrNotFound) {
		notInstalledOnce.Do(func() {
			fmt.Fprintln(stderr, "warning: docgraph not on PATH — components will carry no doc-graph")
		})
		return nil, errDocGraphNotInstalled
	}
	if err != nil {
		return nil, fmt.Errorf("docgraph graph --json in %s: %w", repoPath, err)
	}
	return out, nil
}

// docgraphPayload is the subset of `docgraph graph --json` Mycelium reads. Node
// objects carry more (type/title/description/verified/review/hasFrontmatter);
// Mycelium reads only path, per "signal not size".
type docgraphPayload struct {
	SchemaVersion int `json:"schemaVersion"`
	Nodes         []struct {
		Path string `json:"path"`
	} `json:"nodes"`
	ContentEdges  []json.RawMessage `json:"contentEdges"`
	MetadataEdges []json.RawMessage `json:"metadataEdges"`
	Islands       struct {
		Content  []string `json:"content"`
		Metadata []string `json:"metadata"`
	} `json:"islands"`
}

// buildDigest parses raw docgraph output into a digest and (for a v1 repo with
// docs) the raw payload to serve out-of-band. Returns:
//   - (nil, nil, nil)                  when the repo has no docs (nodes: [])
//   - (&{SchemaVersion:N}, nil, nil)   when schemaVersion != 1 (recorded, not interpreted)
//   - (&digest, raw, nil)              otherwise
func buildDigest(raw []byte) (*graph.DocGraphDigest, json.RawMessage, error) {
	var p docgraphPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, nil, fmt.Errorf("parse docgraph payload: %w", err)
	}
	if p.SchemaVersion != docgraphSchemaVersion {
		return &graph.DocGraphDigest{SchemaVersion: p.SchemaVersion}, nil, nil
	}
	if len(p.Nodes) == 0 {
		return nil, nil, nil
	}
	present := map[string]bool{}
	for _, n := range p.Nodes {
		present[n.Path] = true
	}
	var entries []string
	for _, e := range conventionalEntryDocs {
		if present[e] {
			entries = append(entries, e)
		}
	}
	d := &graph.DocGraphDigest{
		SchemaVersion:     p.SchemaVersion,
		DocCount:          len(p.Nodes),
		ContentEdgeCount:  len(p.ContentEdges),
		MetadataEdgeCount: len(p.MetadataEdges),
		ContentIslands:    p.Islands.Content,
		MetadataIslands:   p.Islands.Metadata,
		EntryDocs:         entries,
	}
	return d, json.RawMessage(raw), nil
}

package graph

import (
	"encoding/json"
	"fmt"

	toml "github.com/pelletier/go-toml/v2"
)

// SidecarName is the committed per-repo sidecar file Mycelium scans for. Named
// after the tool so an agent or dev seeing it in a repo root has a thread to
// pull — grep the name, find Mycelium.
const SidecarName = "mycelium.toml"

// MapName and GraphJSONName are the two rendered artifact filenames. They are
// declared once here and referenced by every writer, reader, and route — a
// writer and a reader disagreeing about a name is then unrepresentable.
const (
	MapName       = "MAP.md"
	GraphJSONName = "graph.json"
)

// RepoDocGraphPrefix/Suffix and RepoDocGraphRoute are the single source for the
// per-repo full doc-graph route. The hub stamps RepoDocGraphRoute(id) onto each
// digest's `url` (so a graph.json consumer follows a link instead of rebuilding
// the route), the hub writes the payload at the mirrored on-disk path, and the
// serve handler parses the same prefix/suffix — one definition so the link, the
// writer, and the reader can't drift. The route is relative (resolve it against
// the hub the graph was fetched from); the id spans path segments (canonical ids
// contain slashes).
const (
	RepoDocGraphPrefix = "/repos/"
	RepoDocGraphSuffix = "/docgraph.json"
)

func RepoDocGraphRoute(id string) string { return RepoDocGraphPrefix + id + RepoDocGraphSuffix }

type Provides struct {
	Name    string `toml:"name" json:"name"`
	Summary string `toml:"summary" json:"summary"`
	URL     string `toml:"url,omitempty" json:"url,omitempty"`
}

type Sidecar struct {
	Name     string     `toml:"name" json:"name"`
	Summary  string     `toml:"summary" json:"summary"`
	Kind     string     `toml:"kind" json:"kind"`
	Status   string     `toml:"status" json:"status"`
	Tags     []string   `toml:"tags" json:"tags,omitempty"`
	Stack    []string   `toml:"stack" json:"stack,omitempty"`
	Provides []Provides `toml:"provides" json:"provides,omitempty"`
}

type OverlayNode struct {
	Name     string   `toml:"name" json:"name"`
	Summary  string   `toml:"summary" json:"summary"`
	Provides []string `toml:"provides" json:"provides,omitempty"`
}

type Edge struct {
	From string `toml:"from" json:"from"`
	To   string `toml:"to" json:"to"`
	Type string `toml:"type" json:"type"`
}

type Overlay struct {
	Nodes []OverlayNode `toml:"node" json:"node,omitempty"`
	Edges []Edge        `toml:"edge" json:"edge,omitempty"`
	// Ignore lists canonical repo ids (as printed by `myco audit`) that are known
	// to intentionally lack a mycelium.toml; matching orphans are suppressed from
	// the graph. Full remote URLs are accepted too — they are canonicalized.
	Ignore []string `toml:"ignore" json:"ignore,omitempty"`
}

// DocGraphDigest is the compact per-component summary of a repo's docgraph
// doc-graph — the queryable "orient" signal, not the whole doc-graph. Island
// PATHS (not just counts) are kept because they are the actionable rot signal
// and are few by construction; the full node/edge lists are not (they live in
// the out-of-band full payload, Manifest.DocGraphs). Built by the node from
// `docgraph graph --json`; carried into graph.json unchanged.
type DocGraphDigest struct {
	SchemaVersion     int      `json:"schemaVersion"`
	DocCount          int      `json:"docCount"`
	ContentEdgeCount  int      `json:"contentEdgeCount"`
	MetadataEdgeCount int      `json:"metadataEdgeCount"`
	ContentIslands    []string `json:"contentIslands,omitempty"`  // unfindable docs (the rot signal)
	MetadataIslands   []string `json:"metadataIslands,omitempty"` // docs with no declared placement
	EntryDocs         []string `json:"entryDocs,omitempty"`       // conventional roots present
	// URL is a self-navigating link to this repo's full doc-graph payload —
	// RepoDocGraphRoute(component id), stamped by the hub at render so a consumer
	// follows a link rather than reconstructing the route from the id. omitempty:
	// only a hub-rendered graph carries it (a node's raw manifest does not).
	URL string `json:"url,omitempty"`
}

type Component struct {
	ID      string  `json:"id"` // canonical git URL
	Name    string  `json:"name"`
	Path    string  `json:"-"`
	Commit  string  `json:"commit"`
	Sidecar Sidecar `json:"sidecar"`
	// DocGraph is the repo's docgraph digest, derived per-node like Path — but
	// unlike Path it IS serialized downstream (meaningful in the merged graph).
	// omitempty: a component with no docs / no docgraph carries nothing.
	DocGraph *DocGraphDigest `json:"docGraph,omitempty"`
}

// componentJSON is the FLAT on-the-wire and in-graph shape of a Component: the
// declared sidecar fields sit alongside the derived id/commit/docGraph, with no
// `sidecar` wrapper. This is deliberate — the struct nests Sidecar for internal
// clarity (declared vs derived), but a consumer querying graph.json should not
// have to know that: it queries `.components[].provides[]`, not
// `.components[].sidecar.provides[]`. The sidecar's own `name` is dropped from the
// output because scan sets Component.Name = Sidecar.Name, so serialising both only
// duplicated one string. Overlay nodes (OverlayNode) are already flat, so this
// makes components and nodes consistent in the graph.
//
// Footgun: MarshalJSON and UnmarshalJSON below MUST stay symmetric — the same
// componentJSON is both the write shape (node → manifest, hub → graph.json) and
// the read shape (hub ingest, `myco audit` re-reading graph.json). Add a field to
// one path only and a round-trip silently drops it.
type componentJSON struct {
	ID       string          `json:"id"`
	Name     string          `json:"name"`
	Commit   string          `json:"commit"`
	Summary  string          `json:"summary"`
	Kind     string          `json:"kind"`
	Status   string          `json:"status"`
	Tags     []string        `json:"tags,omitempty"`
	Stack    []string        `json:"stack,omitempty"`
	Provides []Provides      `json:"provides,omitempty"`
	DocGraph *DocGraphDigest `json:"docGraph,omitempty"`
}

func (c Component) MarshalJSON() ([]byte, error) {
	return json.Marshal(componentJSON{
		ID: c.ID, Name: c.Name, Commit: c.Commit,
		Summary: c.Sidecar.Summary, Kind: c.Sidecar.Kind, Status: c.Sidecar.Status,
		Tags: c.Sidecar.Tags, Stack: c.Sidecar.Stack, Provides: c.Sidecar.Provides,
		DocGraph: c.DocGraph,
	})
}

func (c *Component) UnmarshalJSON(b []byte) error {
	var f componentJSON
	if err := json.Unmarshal(b, &f); err != nil {
		return err
	}
	c.ID, c.Name, c.Commit, c.DocGraph = f.ID, f.Name, f.Commit, f.DocGraph
	c.Sidecar = Sidecar{
		Name:    f.Name, // reconstruct the field the flat shape drops (always == Name)
		Summary: f.Summary, Kind: f.Kind, Status: f.Status,
		Tags: f.Tags, Stack: f.Stack, Provides: f.Provides,
	}
	return nil
}

type Manifest struct {
	Node       string      `json:"node"`
	Source     string      `json:"source"`
	ScannedAt  string      `json:"scanned_at"`
	Components []Component `json:"components"`
	Orphans    []Orphan    `json:"orphans,omitempty"`
	// DocGraphs carries each component's FULL `docgraph graph --json` payload,
	// keyed by canonical id, out-of-band from the digest on Component. It rides
	// the manifest to the hub but is never part of Graph, so it never bloats
	// graph.json; the hub writes each to <outDir>/repos/<id>/docgraph.json.
	DocGraphs map[string]json.RawMessage `json:"docGraphs,omitempty"`
}

// Orphan is a scanned repo with no committed mycelium.toml. It rides in the
// manifest (and, after merge, the graph) so the audit can surface a missing
// sidecar as persistent ecosystem rot rather than a transient scan-time warning.
type Orphan struct {
	ID   string `json:"id"`   // canonical git-remote id (or fallback host/owner/name)
	Name string `json:"name"` // repo basename
	// Path is the node-local repo path, kept in-memory for the scan-time warning
	// only. Never serialized — like Component.Path, graph.json and the pushed
	// manifest carry no filesystem paths (a node's path is meaningless downstream).
	Path string `json:"-"`
}

func ParseSidecar(data []byte) (Sidecar, error) {
	var sc Sidecar
	if err := toml.Unmarshal(data, &sc); err != nil {
		return Sidecar{}, fmt.Errorf("parse sidecar: %w", err)
	}
	if sc.Name == "" {
		return Sidecar{}, fmt.Errorf("%s: missing required field 'name'", SidecarName)
	}
	if sc.Summary == "" {
		return Sidecar{}, fmt.Errorf("%s %q: missing required field 'summary'", SidecarName, sc.Name)
	}
	return sc, nil
}

func ParseOverlay(data []byte) (Overlay, error) {
	var ov Overlay
	if err := toml.Unmarshal(data, &ov); err != nil {
		return Overlay{}, fmt.Errorf("parse overlay: %w", err)
	}
	return ov, nil
}

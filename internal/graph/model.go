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

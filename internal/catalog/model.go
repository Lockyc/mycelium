package catalog

import (
	"fmt"

	toml "github.com/pelletier/go-toml/v2"
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
}

type Component struct {
	ID      string  `json:"id"` // canonical git URL
	Name    string  `json:"name"`
	Path    string  `json:"-"`
	Commit  string  `json:"commit"`
	Sidecar Sidecar `json:"sidecar"`
}

type Manifest struct {
	Node       string      `json:"node"`
	Source     string      `json:"source"`
	ScannedAt  string      `json:"scanned_at"`
	Components []Component `json:"components"`
}

func ParseSidecar(data []byte) (Sidecar, error) {
	var sc Sidecar
	if err := toml.Unmarshal(data, &sc); err != nil {
		return Sidecar{}, fmt.Errorf("parse sidecar: %w", err)
	}
	if sc.Name == "" {
		return Sidecar{}, fmt.Errorf("catalog.toml: missing required field 'name'")
	}
	if sc.Summary == "" {
		return Sidecar{}, fmt.Errorf("catalog.toml %q: missing required field 'summary'", sc.Name)
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

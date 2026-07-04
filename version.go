// Package mycelium exposes build-level metadata shared across the myco binary.
package mycelium

import (
	_ "embed"
	"strings"
)

//go:embed VERSION
var rawVersion string

// Version is the semver of this build. The tracked VERSION file at the repo
// root is the single source of truth; it is embedded at compile time so the
// binary self-reports (`myco version`) without a build-time -ldflags step.
var Version = strings.TrimSpace(rawVersion)

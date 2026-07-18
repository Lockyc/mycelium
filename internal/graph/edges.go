package graph

// IsUseEdge reports whether an edge type means "from actually uses to" — the
// dependency edges (consumes / depends-on / deploys-to) that define blast radius.
// It is the exported accessor over useEdgeTypes (defined in render.go), so the
// query layer reuses the one definition of that set instead of re-encoding it.
func IsUseEdge(edgeType string) bool { return useEdgeTypes[edgeType] }

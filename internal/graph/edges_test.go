package graph

import "testing"

func TestIsUseEdge(t *testing.T) {
	for _, ty := range []string{"consumes", "depends-on", "deploys-to"} {
		if !IsUseEdge(ty) {
			t.Errorf("IsUseEdge(%q) = false, want true", ty)
		}
	}
	for _, ty := range []string{"markets", "sells", "related", "", "unknown"} {
		if IsUseEdge(ty) {
			t.Errorf("IsUseEdge(%q) = true, want false", ty)
		}
	}
}

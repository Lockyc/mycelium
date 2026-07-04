package mycelium

import "testing"

func TestVersionEmbedded(t *testing.T) {
	if Version == "" {
		t.Fatal("Version is empty; the VERSION file was not embedded")
	}
}

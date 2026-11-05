package atomicfile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteFileReplacesAndLeavesNoTemp(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "graph.json")
	for _, content := range []string{"old", "new"} {
		if err := WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if got, _ := os.ReadFile(p); string(got) != "new" {
		t.Fatalf("content = %q, want new", got)
	}
	if fi, _ := os.Stat(p); fi.Mode().Perm() != 0o644 {
		t.Fatalf("perm = %v, want 0644", fi.Mode().Perm())
	}
	if ents, _ := os.ReadDir(dir); len(ents) != 1 {
		t.Fatalf("temp file left behind: %v", ents)
	}
}

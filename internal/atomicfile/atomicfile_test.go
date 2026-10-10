package atomicfile

import (
	"errors"
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

func TestReplaceDirSwapsWholeTree(t *testing.T) {
	parent := t.TempDir()
	dir := filepath.Join(parent, "repos")
	write := func(name string) func(string) error {
		return func(tmp string) error { return os.WriteFile(filepath.Join(tmp, name), []byte("x"), 0o644) }
	}
	if err := ReplaceDir(dir, write("a")); err != nil {
		t.Fatal(err)
	}
	if err := ReplaceDir(dir, write("b")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "a")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("old tree's file survived the swap: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "b")); err != nil {
		t.Fatalf("new tree missing: %v", err)
	}
	if ents, _ := os.ReadDir(parent); len(ents) != 1 {
		t.Fatalf("temp/trash dirs left behind: %v", ents)
	}
}

func TestReplaceDirFillFailureKeepsOld(t *testing.T) {
	parent := t.TempDir()
	dir := filepath.Join(parent, "repos")
	if err := ReplaceDir(dir, func(tmp string) error { return os.WriteFile(filepath.Join(tmp, "a"), nil, 0o644) }); err != nil {
		t.Fatal(err)
	}
	boom := errors.New("boom")
	if err := ReplaceDir(dir, func(string) error { return boom }); !errors.Is(err, boom) {
		t.Fatalf("err = %v, want boom", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "a")); err != nil {
		t.Fatalf("a failed fill must leave the old tree: %v", err)
	}
	if ents, _ := os.ReadDir(parent); len(ents) != 1 {
		t.Fatalf("temp dir left behind: %v", ents)
	}
}

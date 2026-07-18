package scan

import (
	"encoding/json"
	"testing"
)

const schemaV1WithDocs = `{
  "schemaVersion": 1,
  "repoRoot": "/x",
  "nodes": [
    {"path": "CLAUDE.md", "type": "architecture", "hasFrontmatter": true},
    {"path": "README.md", "hasFrontmatter": false},
    {"path": "docs/stray.md", "hasFrontmatter": false}
  ],
  "contentEdges": [{"from":"a","to":"b","kind":"link"},{"from":"a","to":"c","kind":"mention"}],
  "metadataEdges": [{"from":"a","to":"b","rel":"part-of","note":""}],
  "islands": {"content": ["docs/stray.md"], "metadata": ["docs/floating.md"]}
}`

func TestBuildDigestSchemaV1(t *testing.T) {
	d, full, err := buildDigest([]byte(schemaV1WithDocs))
	if err != nil {
		t.Fatal(err)
	}
	if d == nil {
		t.Fatal("expected a digest")
	}
	if d.SchemaVersion != 1 || d.DocCount != 3 || d.ContentEdgeCount != 2 || d.MetadataEdgeCount != 1 {
		t.Fatalf("bad counts: %+v", d)
	}
	if len(d.ContentIslands) != 1 || d.ContentIslands[0] != "docs/stray.md" {
		t.Fatalf("bad content islands: %+v", d.ContentIslands)
	}
	if len(d.MetadataIslands) != 1 || d.MetadataIslands[0] != "docs/floating.md" {
		t.Fatalf("bad metadata islands: %+v", d.MetadataIslands)
	}
	// entryDocs: CLAUDE.md + README.md present, docs/index.md absent
	if len(d.EntryDocs) != 2 || d.EntryDocs[0] != "CLAUDE.md" || d.EntryDocs[1] != "README.md" {
		t.Fatalf("bad entryDocs: %+v", d.EntryDocs)
	}
	if string(full) == "" {
		t.Fatal("expected raw payload to be returned for a v1 repo with docs")
	}
	// full payload must be valid JSON echoing the input
	var check map[string]any
	if err := json.Unmarshal(full, &check); err != nil {
		t.Fatalf("full payload not valid json: %v", err)
	}
}

func TestBuildDigestNoDocsOmitted(t *testing.T) {
	empty := `{"schemaVersion":1,"repoRoot":"/x","nodes":[],"contentEdges":[],"metadataEdges":[],"islands":{"content":[],"metadata":[]}}`
	d, full, err := buildDigest([]byte(empty))
	if err != nil {
		t.Fatal(err)
	}
	if d != nil || full != nil {
		t.Fatalf("no docs → digest and payload must both be nil, got d=%v full=%v", d, full)
	}
}

func TestBuildDigestVersionMismatchRecordedNotInterpreted(t *testing.T) {
	newer := `{"schemaVersion":2,"nodes":[{"path":"CLAUDE.md"}],"contentEdges":[],"metadataEdges":[],"islands":{"content":[],"metadata":[]}}`
	d, full, err := buildDigest([]byte(newer))
	if err != nil {
		t.Fatal(err)
	}
	if d == nil || d.SchemaVersion != 2 {
		t.Fatalf("version must be recorded, got %+v", d)
	}
	if d.DocCount != 0 || d.EntryDocs != nil || full != nil {
		t.Fatalf("unknown version must not be interpreted: %+v full=%v", d, full)
	}
}

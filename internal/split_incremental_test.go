package internal

import (
	"os"
	"path/filepath"
	"testing"
)

// writeTree writes files (relative path → content) under root, creating the folders.
func writeTree(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, text := range files {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// A shard of files with no declaration has no shard file and no entry in Shards, but its checksum is in EmptyShards.
// The next incremental run finds nothing changed: the declaration-free shard is not taken for a changed one.
func TestSplitIncremental_DeclarationFreeShardIsUnchanged(t *testing.T) {
	root, out := t.TempDir(), t.TempDir()
	writeTree(t, root, map[string]string{
		"a/x.go":    "package a\n\nfunc X() {}\n",
		"data/t.go": "package data\n\nvar Table = 1\n",
	})
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}

	results, err := NewDirProcessor(cfg, 1, true, false, "all").ProcessDirectory(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := WriteSplitOutput(results, out, root, "dir"); err != nil {
		t.Fatal(err)
	}
	m, err := loadManifest(out)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Shards) != 1 || m.Shards[0].Path != "a.json" {
		t.Fatalf("Shards = %+v, want only a.json", m.Shards)
	}
	if m.EmptyShards["data.json"] == "" {
		t.Fatalf("EmptyShards = %v, want a checksum for data.json", m.EmptyShards)
	}

	changed, err := NewDirProcessor(cfg, 1, true, false, "all").ProcessDirectoryIncremental(root, out, "dir")
	if err != nil {
		t.Fatal(err)
	}
	if len(changed) != 0 {
		t.Errorf("nothing changed, but %d results were re-processed", len(changed))
	}
}

// A declaration-free shard whose file changes is re-processed; its checksum follows the change.
func TestSplitIncremental_DeclarationFreeShardChangeIsReprocessed(t *testing.T) {
	root, out := t.TempDir(), t.TempDir()
	writeTree(t, root, map[string]string{
		"a/x.go":    "package a\n\nfunc X() {}\n",
		"data/t.go": "package data\n\nvar Table = 1\n",
	})
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	results, err := NewDirProcessor(cfg, 1, true, false, "all").ProcessDirectory(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := WriteSplitOutput(results, out, root, "dir"); err != nil {
		t.Fatal(err)
	}
	before, err := loadManifest(out)
	if err != nil {
		t.Fatal(err)
	}

	writeTree(t, root, map[string]string{"data/t.go": "package data\n\nvar Table = 2\n"})
	changed, err := NewDirProcessor(cfg, 1, true, false, "all").ProcessDirectoryIncremental(root, out, "dir")
	if err != nil {
		t.Fatal(err)
	}
	if len(changed) != 1 {
		t.Fatalf("%d results re-processed, want the one changed file", len(changed))
	}
	if _, err := WriteSplitOutputIncremental(changed, out, root, "dir"); err != nil {
		t.Fatal(err)
	}
	after, err := loadManifest(out)
	if err != nil {
		t.Fatal(err)
	}
	if after.EmptyShards["data.json"] == "" || after.EmptyShards["data.json"] == before.EmptyShards["data.json"] {
		t.Errorf("EmptyShards[data.json] = %q, want a new checksum (was %q)", after.EmptyShards["data.json"], before.EmptyShards["data.json"])
	}
	if len(after.Shards) != 1 || after.Shards[0].Path != "a.json" {
		t.Errorf("Shards = %+v, want a.json kept", after.Shards)
	}
}

// A shard whose files lose every declaration is retired: its shard file goes, and its checksum stays in EmptyShards.
func TestSplitIncremental_ShardLosingDeclarationsIsRetired(t *testing.T) {
	root, out := t.TempDir(), t.TempDir()
	writeTree(t, root, map[string]string{"a/x.go": "package a\n\nfunc X() {}\n"})
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	results, err := NewDirProcessor(cfg, 1, true, false, "all").ProcessDirectory(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := WriteSplitOutput(results, out, root, "dir"); err != nil {
		t.Fatal(err)
	}

	writeTree(t, root, map[string]string{"a/x.go": "package a\n\nvar V = 1\n"})
	changed, err := NewDirProcessor(cfg, 1, true, false, "all").ProcessDirectoryIncremental(root, out, "dir")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := WriteSplitOutputIncremental(changed, out, root, "dir"); err != nil {
		t.Fatal(err)
	}

	m, err := loadManifest(out)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Shards) != 0 {
		t.Errorf("Shards = %+v, want none: a.json lost its declarations", m.Shards)
	}
	if m.EmptyShards["a.json"] == "" {
		t.Errorf("EmptyShards = %v, want a checksum for a.json", m.EmptyShards)
	}
	if _, err := os.Stat(filepath.Join(out, "a.json")); !os.IsNotExist(err) {
		t.Errorf("a.json still exists after its declarations went: %v", err)
	}
}

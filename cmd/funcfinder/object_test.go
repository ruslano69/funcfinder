package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestObjectWriteRejectsConcurrentChange(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "source.go")
	if err := os.WriteFile(path, []byte("current"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := writeCodeObject(path, []byte("stale"), []byte("replacement"), 0600); err == nil {
		t.Fatal("concurrent change overwritten")
	}
	got, _ := os.ReadFile(path)
	if string(got) != "current" {
		t.Fatal("original damaged")
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatal("temporary file leaked")
	}
	if err := writeCodeObject(path, got, []byte("replacement"), 0600); err != nil {
		t.Fatal(err)
	}
	got, _ = os.ReadFile(path)
	if string(got) != "replacement" {
		t.Fatal("replacement not written")
	}
}

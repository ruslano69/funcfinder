package internal

import (
	"os"
	"path/filepath"
	"testing"
)

// writeDetectTree creates the files under a new temporary directory and returns its path.
func writeDetectTree(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for rel, content := range files {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// A project whose root holds only configuration (go.mod, Makefile) is detected by the sources in its subfolders.
func TestDetectLanguage_SourcesInSubfolders(t *testing.T) {
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	root := writeDetectTree(t, map[string]string{
		"go.mod":           "module x\n",
		"Makefile":         "all:\n",
		"cmd/tool/main.go": "package main\n",
		"internal/a.go":    "package internal\n",
		"internal/b.go":    "package internal\n",
		"scripts/setup.py": "print(1)\n",
	})

	lc, counts, err := DetectLanguage(cfg, root)
	if err != nil {
		t.Fatal(err)
	}
	if lc == nil || lc.Name != "Go" {
		t.Fatalf("language = %v, want Go", lc)
	}
	if counts["go"] != 3 || counts["py"] != 1 {
		t.Errorf("counts = %v, want go 3 and py 1", counts)
	}
}

// A tie goes to the language key that sorts first, on every run: the answer does not depend on map order.
func TestDetectLanguage_TieIsDeterministic(t *testing.T) {
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	root := writeDetectTree(t, map[string]string{"a.go": "package a\n", "b.py": "x = 1\n"})
	for i := 0; i < 20; i++ {
		lc, _, err := DetectLanguage(cfg, root)
		if err != nil {
			t.Fatal(err)
		}
		if lc == nil || lc.Name != "Go" {
			t.Fatalf("run %d: language = %v, want Go (go sorts before py)", i, lc)
		}
	}
}

// A single file is detected by its extension, so it needs no -l.
func TestDetectLanguage_SingleFile(t *testing.T) {
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	root := writeDetectTree(t, map[string]string{"src/lib.rs": "fn main() {}\n"})
	lc, _, err := DetectLanguage(cfg, filepath.Join(root, "src", "lib.rs"))
	if err != nil {
		t.Fatal(err)
	}
	if lc == nil || lc.Name != "Rust" {
		t.Errorf("language = %v, want Rust", lc)
	}
}

// A directory with no supported source file has no language, and that is not an error.
func TestDetectLanguage_NoSources(t *testing.T) {
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	root := writeDetectTree(t, map[string]string{"README.md": "# x\n", "go.mod": "module x\n"})
	lc, counts, err := DetectLanguage(cfg, root)
	if err != nil {
		t.Fatal(err)
	}
	if lc != nil || len(counts) != 0 {
		t.Errorf("language = %v, counts = %v, want none", lc, counts)
	}
}

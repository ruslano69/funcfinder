package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ruslano69/funcfinder/internal"
)

// A minified bundle has a line longer than 64 KB. stat must count every line of such a file, not stop at that line.
func TestAnalyzeFile_CountsLinesAfterLongLine(t *testing.T) {
	cfg, err := internal.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	lc, err := cfg.GetLanguageConfig("js")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "bundle.js")
	content := "var a = 1;\nvar blob = \"" + strings.Repeat("y", 90_000) + "\";\nvar b = 2;\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	_, metrics := analyzeFile(path, lc)
	if metrics.TotalLines != 3 {
		t.Fatalf("TotalLines = %d, want 3 (the lines after the long one must be counted)", metrics.TotalLines)
	}
}

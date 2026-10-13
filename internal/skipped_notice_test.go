package internal

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// The notice names the skipped extensions, most files first: the Shell, PowerShell and HTML files of a repository.
func TestSkippedNotice_NamesExtensionsMostFirst(t *testing.T) {
	got := skippedNotice(map[string]int{".ps1": 4, ".sh": 9, ".html": 1})
	want := "skipped 14 files with no language for their extension: .sh 9, .ps1 4, .html 1"
	if got != want {
		t.Errorf("skippedNotice = %q, want %q", got, want)
	}
}

// Nothing skipped means no notice at all.
func TestSkippedNotice_EmptyIsSilent(t *testing.T) {
	if got := skippedNotice(nil); got != "" {
		t.Errorf("skippedNotice(nil) = %q, want none", got)
	}
}

// A file without an extension is named as such; more than six kinds are cut off with "...".
func TestSkippedNotice_NoExtensionAndCutOff(t *testing.T) {
	if got := skippedNotice(map[string]int{"": 2}); got != "skipped 2 files with no language for their extension: (no extension) 2" {
		t.Errorf("no-extension notice = %q", got)
	}

	many := map[string]int{}
	for i := 1; i <= 8; i++ {
		many[fmt.Sprintf(".e%d", i)] = i
	}
	got := skippedNotice(many)
	if !strings.HasSuffix(got, ".e8 8, .e7 7, .e6 6, .e5 5, .e4 4, .e3 3, ...") {
		t.Errorf("cut-off notice = %q, want the six most frequent kinds then \"...\"", got)
	}
	if !strings.HasPrefix(got, "skipped 36 files ") {
		t.Errorf("cut-off notice = %q, want the total of all kinds (36)", got)
	}
}

// writeSkipFiles writes files (relative path → content) under root, creating the folders.
func writeSkipFiles(t *testing.T, root string, files map[string]string) {
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

// A walk counts the files whose extension has no language, and leaves the supported ones out of the count.
func TestCollectFiles_CountsSkippedExtensions(t *testing.T) {
	root := t.TempDir()
	writeSkipFiles(t, root, map[string]string{
		"a/x.go":       "package a\n",
		"scripts/b.sh": "echo one\n",
		"scripts/c.sh": "echo two\n",
		"LICENSE":      "text\n",
	})
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	dp := NewDirProcessor(cfg, 1, true, false, "all")
	jobs, err := dp.collectFiles(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 1 || filepath.Base(jobs[0].Path) != "x.go" {
		t.Errorf("jobs = %d, want only x.go", len(jobs))
	}
	if want := map[string]int{".sh": 2, "": 1}; !reflect.DeepEqual(dp.skipped, want) {
		t.Errorf("skipped = %v, want %v", dp.skipped, want)
	}
}

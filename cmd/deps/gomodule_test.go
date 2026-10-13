package main

import (
	"os"
	"path/filepath"
	"testing"
)

// An import under the module of go.mod is internal; another module, or a name that only shares a prefix with it, is not.
func TestClassifyDep_GoModule(t *testing.T) {
	local := map[string]bool{"example.com/x": true}
	cases := map[string]string{
		"example.com/x":        "int",
		"example.com/x/pkg/a":  "int",
		"example.com/xy/pkg":   "ext",
		"github.com/other/lib": "ext",
		"fmt":                  "std",
	}
	for dep, want := range cases {
		if got := classifyDep(dep, "go", local); got != want {
			t.Errorf("classifyDep(%q, go) = %q, want %q", dep, got, want)
		}
	}
}

// The module is read from the nearest go.mod at or above the analyzed directory, so a run on a subdirectory still
// knows it. Without a go.mod there is no module.
func TestFindGoModulePath(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/x\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(root, "pkg", "a")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := findGoModulePath(sub); got != "example.com/x" {
		t.Errorf("findGoModulePath(subdir) = %q, want example.com/x", got)
	}
	if got := findGoModulePath(t.TempDir()); got != "" {
		t.Errorf("findGoModulePath(no go.mod) = %q, want none", got)
	}
}

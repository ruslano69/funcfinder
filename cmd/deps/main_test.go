package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestIsStdlib_Python guards against the empty-string-prefix bug where
// stdlibPrefixes["py"] used to start with "", and since
// strings.HasPrefix(x, "") is always true in Go, every single Python
// import — including first-party and third-party ones — was classified
// as stdlib.
func TestIsStdlib_Python(t *testing.T) {
	tests := []struct {
		module string
		want   bool
	}{
		{"os", true},
		{"os.path", true},
		{"sys", true},
		{"collections.abc", true},
		{"django", false},
		{"django.db.models", false},
		{"asgiref.sync", false},
		{"requests", false},
		// A name that merely starts with a stdlib module's letters must not
		// false-positive via a naive prefix match (e.g. "os" vs "osprey").
		{"osprey", false},
	}

	for _, tt := range tests {
		t.Run(tt.module, func(t *testing.T) {
			if got := isStdlib(tt.module, "py"); got != tt.want {
				t.Errorf("isStdlib(%q, \"py\") = %v, want %v", tt.module, got, tt.want)
			}
		})
	}
}

func TestDetectPythonLocalPackages(t *testing.T) {
	dir := t.TempDir()

	// Package: django/__init__.py
	if err := os.MkdirAll(filepath.Join(dir, "django"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "django", "__init__.py"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	// Top-level module: helpers.py
	if err := os.WriteFile(filepath.Join(dir, "helpers.py"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	// A directory without __init__.py is not a package (e.g. a docs/ folder).
	if err := os.MkdirAll(filepath.Join(dir, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}

	got := detectPythonLocalPackages(dir)

	if !got["django"] {
		t.Errorf("expected \"django\" to be detected as a local package, got %+v", got)
	}
	if !got["helpers"] {
		t.Errorf("expected \"helpers\" to be detected as a local module, got %+v", got)
	}
	if got["docs"] {
		t.Errorf("\"docs\" has no __init__.py and must not be treated as a package, got %+v", got)
	}
}

// TestClassifyDep_Python verifies that django.* imports scanned from within
// the django project itself are classified as internal (first-party), a
// genuine third-party package as external, and stdlib modules as std.
func TestClassifyDep_Python(t *testing.T) {
	localPkgs := map[string]bool{"django": true}

	tests := []struct {
		dep  string
		want string
	}{
		{"os", "std"},
		{"django.db.models", "int"},
		{"django.conf", "int"},
		{"asgiref.sync", "ext"},
		{"requests", "ext"}, // bare third-party import (e.g. "import requests"), not a local package
	}

	for _, tt := range tests {
		t.Run(tt.dep, func(t *testing.T) {
			if got := classifyDep(tt.dep, "py", localPkgs); got != tt.want {
				t.Errorf("classifyDep(%q, \"py\", localPkgs) = %q, want %q", tt.dep, got, tt.want)
			}
		})
	}
}

package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Rust paths use "::" and no "/" or ".", so the generic rule in classifyDep
// cannot tell them apart. The first segment decides where an item comes from.

// rustModDecl matches `mod name;`, the declaration of a module in a file of the
// crate (an inline `mod name { ... }` has no file of its own and is not one).
var rustModDecl = regexp.MustCompile(`^\s*(?:pub(?:\([^)]*\))?\s+)?mod\s+([A-Za-z_][A-Za-z0-9_]*)\s*;`)

// classifyRustDep sorts a Rust path: std, core and alloc are the standard
// library; crate, self and super are this crate; a module declared in the crate
// and the crate's own name are internal; anything else is an external crate.
func classifyRustDep(dep string, localPkgs map[string]bool) string {
	root := rustRoot(dep)
	switch root {
	case "crate", "self", "super":
		return "int"
	}
	if localPkgs[root] {
		return "int"
	}
	return "ext"
}

// rustRoot returns the first segment of a Rust path: "store" for
// "store::{Named, Store}", "serde" for "serde::Deserialize".
func rustRoot(dep string) string {
	root := strings.TrimSpace(dep)
	if i := strings.IndexAny(root, ":{ "); i >= 0 {
		root = root[:i]
	}
	return root
}

// detectRustLocalPackages names what is local to the scanned tree: every module
// declared with `mod x;` in a .rs file, and the crate name from a Cargo.toml,
// with dashes read as underscores, which is how Rust names the crate. The
// target and hidden directories are skipped.
func detectRustLocalPackages(dir string) map[string]bool {
	names := make(map[string]bool)
	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			name := d.Name()
			if path != dir && (name == "target" || strings.HasPrefix(name, ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		switch {
		case d.Name() == "Cargo.toml":
			if crate := cargoPackageName(path); crate != "" {
				names[strings.ReplaceAll(crate, "-", "_")] = true
			}
		case strings.HasSuffix(d.Name(), ".rs"):
			data, err := os.ReadFile(path)
			if err != nil {
				return nil
			}
			for _, line := range strings.Split(string(data), "\n") {
				if m := rustModDecl.FindStringSubmatch(line); m != nil {
					names[m[1]] = true
				}
			}
		}
		return nil
	})
	return names
}

// cargoPackageName reads the name from the [package] table of a Cargo.toml.
func cargoPackageName(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	inPackage := false
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") {
			inPackage = trimmed == "[package]"
			continue
		}
		if !inPackage {
			continue
		}
		if key, val, ok := strings.Cut(trimmed, "="); ok && strings.TrimSpace(key) == "name" {
			return strings.Trim(strings.TrimSpace(val), `"`)
		}
	}
	return ""
}

// rustUseStart matches the first line of a use item, with or without pub.
var rustUseStart = regexp.MustCompile(`^\s*(?:pub(?:\([^)]*\))?\s+)?use\s`)

// normalizeRustUse joins a use item read over several lines into the form a
// single-line use has: single spaces, no padding inside the braces, no trailing
// comma before the closing brace. "use kanzi::{ a, b, };" becomes
// "use kanzi::{a, b};".
func normalizeRustUse(stmt string) string {
	s := strings.Join(strings.Fields(stmt), " ")
	s = strings.ReplaceAll(s, "{ ", "{")
	s = strings.ReplaceAll(s, " }", "}")
	s = strings.ReplaceAll(s, " ,", ",")
	s = strings.ReplaceAll(s, ",}", "}")
	return s
}

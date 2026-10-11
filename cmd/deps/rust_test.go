package main

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/ruslano69/funcfinder/internal"
)

// Standard library paths are recognised under the same language key deps gets
// from -l rust (LangKey "rust"). The table had the key "rs", so none matched.
func TestIsStdlib_Rust(t *testing.T) {
	for _, mod := range []string{"std::collections::HashMap", "std::{io, fmt}", "core::fmt", "alloc::vec::Vec"} {
		if !isStdlib(mod, "rust") {
			t.Errorf("isStdlib(%q, rust) = false, want true", mod)
		}
	}
	if isStdlib("serde::Deserialize", "rust") {
		t.Error("isStdlib(serde::Deserialize, rust) = true, want false: serde is an external crate")
	}
}

// Rust paths use "::" and no "/" or ".", so the generic rule called every one of
// them internal. The first segment decides: std/core/alloc are stdlib, crate,
// self and super are this crate, a module declared in the crate or the crate's
// own name is internal, anything else is external.
func TestClassifyDep_Rust(t *testing.T) {
	local := map[string]bool{"store": true, "mycrate": true}
	cases := map[string]string{
		"std::collections::HashMap": "std",
		"core::fmt":                 "std",
		"alloc::vec::Vec":           "std",
		"crate::store::Store":       "int",
		"self::util":                "int",
		"super::parent":             "int",
		"store::{Named, Store}":     "int",
		"mycrate::{helper, Named}":  "int",
		"serde::Deserialize":        "ext",
		"serde_json::Value":         "ext",
	}
	for dep, want := range cases {
		if got := classifyDep(dep, "rust", local); got != want {
			t.Errorf("classifyDep(%q, rust) = %q, want %q", dep, got, want)
		}
	}
}

// `pub use` and `pub(crate) use` are imports too. A `mod x;` declaration is not
// an import and is not reported as one.
func TestCollectFileImports_RustPubUse(t *testing.T) {
	src := `use std::collections::HashMap;
pub use store::{Named, Store};
pub(crate) use crate::a::B;
mod store;

fn main() {}
`
	path := filepath.Join(t.TempDir(), "lib.rs")
	if err := os.WriteFile(path, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	config, err := internal.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	lc := config["rust"]
	if lc == nil {
		t.Fatal("rust config not found")
	}

	got := collectFileImports(path, lc, nil)
	want := []string{"std::collections::HashMap", "store::{Named, Store}", "crate::a::B"}
	sort.Strings(got)
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("collectFileImports = %v, want %v", got, want)
	}
}

// The crate's own name (from Cargo.toml, with dashes read as underscores, which
// is how Rust names it) and every `mod x;` declared in its files are internal.
func TestDetectRustLocalPackages(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Cargo.toml"),
		[]byte("[package]\nname = \"my-crate\"\nversion = \"0.1.0\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "src", "lib.rs"),
		[]byte("mod store;\npub mod util;\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	got := detectRustLocalPackages(dir)
	want := map[string]bool{"my_crate": true, "store": true, "util": true}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("detectRustLocalPackages = %v, want %v", got, want)
	}
}

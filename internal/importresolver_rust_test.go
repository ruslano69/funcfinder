package internal

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

// writeCargo writes a Cargo.toml with the given text under root/rel, creating the folders.
func writeCargo(t *testing.T, root, rel, text string) {
	t.Helper()
	dir := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Cargo.toml"), []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

// A crate is imported by its package name with "-" read as "_". A path into it reaches the shard of the module
// directory it names, and std or external crates are not intra-project: they are not counted at all.
func TestRustShardGraph_CrateImportsAndModulePaths(t *testing.T) {
	root := t.TempDir()
	writeCargo(t, root, "crates/core-lib", "[package]\nname = \"core-lib\"\n")
	writeCargo(t, root, "crates/app", "[package]\nname = \"app\"\n")
	coreLib := filepath.Join(root, "crates", "core-lib", "src", "lib.rs")
	coreParse := filepath.Join(root, "crates", "core-lib", "src", "parse", "mod.rs")
	appMain := filepath.Join(root, "crates", "app", "src", "main.rs")
	fileImports := map[string][]string{
		coreLib:   {"crate::parse::run", "serde_json::json"},
		coreParse: {"std::fmt"},
		appMain:   {"core_lib::f", "core_lib::parse::run", "std::sync::Arc"},
	}

	graph, stats := BuildShardGraphFor(root, "dir", fileImports, ResolveContext{RustCrates: DetectRustCrates(root)})

	shardOf := func(path string) string { return PathToShardName(ShardKeyForPath(path, root, "dir")) }
	want := map[string][]string{
		shardOf(appMain): {shardOf(coreLib), shardOf(coreParse)},
		shardOf(coreLib): {shardOf(coreParse)},
	}
	for src, deps := range want {
		var got []string
		for dst := range graph[src] {
			got = append(got, dst)
		}
		sort.Strings(got)
		sort.Strings(deps)
		if !reflect.DeepEqual(got, deps) {
			t.Errorf("edges from %s = %v, want %v", src, got, deps)
		}
	}

	// core_lib::f, core_lib::parse::run and crate::parse::run are intra-project; serde_json and std are not counted.
	if stats.Resolvable != 3 || stats.Resolved != 3 {
		t.Errorf("stats = %+v, want 3 resolvable and 3 resolved", stats)
	}
	if w := stats.Warning(); w != "" {
		t.Errorf("unexpected warning: %s", w)
	}
}

// A crate whose [lib] sets a name is imported by that name, even when the package name differs.
func TestRustShardGraph_LibNameOverridesPackageName(t *testing.T) {
	root := t.TempDir()
	writeCargo(t, root, "crates/engine", "[package]\nname = \"gridcraft-engine\"\n\n[lib]\nname = \"gridcraft_engine\"\n")
	writeCargo(t, root, "crates/app", "[package]\nname = \"app\"\n")
	engine := filepath.Join(root, "crates", "engine", "src", "lib.rs")
	appMain := filepath.Join(root, "crates", "app", "src", "main.rs")
	fileImports := map[string][]string{
		engine:  nil, // the crate's own file is in the tree, so its shard exists
		appMain: {"gridcraft_engine::Session"},
	}

	graph, stats := BuildShardGraphFor(root, "dir", fileImports, ResolveContext{RustCrates: DetectRustCrates(root)})

	shardOf := func(path string) string { return PathToShardName(ShardKeyForPath(path, root, "dir")) }
	if _, ok := graph[shardOf(appMain)][shardOf(engine)]; !ok {
		t.Errorf("no edge from app to the engine crate: %v", graph[shardOf(appMain)])
	}
	if stats.Resolved != 1 {
		t.Errorf("stats = %+v, want 1 resolved", stats)
	}
}

// A crate that the tree imports but does not contain is counted as resolvable and unresolved, so the warning fires:
// the empty graph is no longer silent.
func TestRustShardGraph_UnresolvedCrateWarns(t *testing.T) {
	root := t.TempDir()
	writeCargo(t, root, "crates/core-lib", "[package]\nname = \"core-lib\"\n")
	appMain := filepath.Join(root, "app", "src", "main.rs")
	fileImports := map[string][]string{appMain: {"core_lib::f", "core_lib::g", "core_lib::h"}}

	_, stats := BuildShardGraphFor(root, "dir", fileImports, ResolveContext{RustCrates: DetectRustCrates(root)})

	if stats.Resolvable != 3 || stats.Resolved != 0 {
		t.Errorf("stats = %+v, want 3 resolvable and 0 resolved", stats)
	}
	if stats.Warning() == "" {
		t.Error("no warning for a crate import that resolved to no shard")
	}
}

// cargoCrateName reads the name from [package] and [lib] only: workspace and dependency tables name no crate.
func TestCargoCrateName(t *testing.T) {
	cases := map[string]string{
		"[package]\nname = \"a-b\"\n":                           "a_b",
		"[package]\nname = \"p\"\n\n[lib]\nname = \"l\"\n":      "l",
		"[workspace]\nmembers = [\"x\"]\n":                      "",
		"[dependencies]\nname = \"not-me\"\n":                   "",
		"[package]\nversion = \"1.0.0\"\n[lib]\npath = \"x\"\n": "",
	}
	for text, want := range cases {
		if got := cargoCrateName(text); got != want {
			t.Errorf("cargoCrateName(%q) = %q, want %q", text, got, want)
		}
	}
}

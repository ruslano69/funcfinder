package internal

import (
	"os"
	"path/filepath"
	"testing"
)

// TestBuildShardGraph_MisrootedShardsWarns pins the TODO.md "deps trust bug"
// fix: a --shards run whose rootDir doesn't line up with where the aliased
// files actually live must not silently return a confident, mostly-empty
// graph — the resolution-rate stat must flag it.
func TestBuildShardGraph_MisrootedShardsWarns(t *testing.T) {
	aliases := map[string]string{"@/": "lib/"}

	t.Run("correctly rooted: aliases and relative imports resolve, no warning", func(t *testing.T) {
		fileImports := map[string][]string{
			"/proj/src/a.ts": {"./b", "@/x"},
			"/proj/src/b.ts": {},
			"/proj/lib/x.ts": {},
		}
		_, stats := BuildShardGraph("/proj", "dir", "", fileImports, aliases)
		if stats.Resolvable != 2 || stats.Resolved != 2 {
			t.Fatalf("want Resolvable=2 Resolved=2, got %+v", stats)
		}
		if w := stats.Warning(); w != "" {
			t.Fatalf("want no warning for a fully-resolved graph, got: %s", w)
		}
	})

	t.Run("misrooted: alias target directory absent from this root, warns", func(t *testing.T) {
		// Simulates `deps frontend/src --shards` when tsconfig (and the "lib/"
		// alias target) actually live one level up, at "frontend/" — from
		// this (wrong) root, none of the alias-prefixed imports can resolve.
		fileImports := map[string][]string{
			"/proj/src/a.ts": {"@/x", "@/y", "@/z", "@/w"},
		}
		graph, stats := BuildShardGraph("/proj", "dir", "", fileImports, aliases)
		if stats.Resolvable != 4 || stats.Resolved != 0 {
			t.Fatalf("want Resolvable=4 Resolved=0, got %+v", stats)
		}
		w := stats.Warning()
		if w == "" {
			t.Fatal("want a warning for a 0% resolution rate, got none")
		}
		// The graph itself is exactly the silent failure mode described in
		// TODO.md: present, but empty/leaf-only, with nothing to indicate why.
		if len(graph["src"]) != 0 {
			t.Fatalf("expected the misrooted graph to have no resolved edges, got %+v", graph)
		}
	})

	t.Run("small sample: 0% resolved but below minResolvableSample, no warning", func(t *testing.T) {
		fileImports := map[string][]string{
			"/proj/src/a.ts": {"@/x", "@/y"},
		}
		_, stats := BuildShardGraph("/proj", "dir", "", fileImports, aliases)
		if stats.Resolvable != 2 || stats.Resolved != 0 {
			t.Fatalf("want Resolvable=2 Resolved=0, got %+v", stats)
		}
		if w := stats.Warning(); w != "" {
			t.Fatalf("want no warning below the minimum sample size, got: %s", w)
		}
	})

	t.Run("no intra-project-looking imports at all: no warning", func(t *testing.T) {
		fileImports := map[string][]string{
			"/proj/src/a.ts": {"react", "lodash", "some-external-pkg"},
		}
		_, stats := BuildShardGraph("/proj", "dir", "", fileImports, aliases)
		if stats.Resolvable != 0 {
			t.Fatalf("want Resolvable=0 for purely external imports, got %+v", stats)
		}
		if w := stats.Warning(); w != "" {
			t.Fatalf("want no warning for a project with no internal deps, got: %s", w)
		}
	})
}

// TestDetectTSConfigAbove pins the other half of the deps trust-bug fix: the
// direct signal for "rooted one level too deep, below the tsconfig", which
// DetectTSAliases can't itself report (it just quietly finds nothing).
func TestDetectTSConfigAbove(t *testing.T) {
	root := t.TempDir()
	frontend := filepath.Join(root, "frontend")
	src := filepath.Join(frontend, "src")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	tscPath := filepath.Join(frontend, "tsconfig.json")
	if err := os.WriteFile(tscPath, []byte(`{}`), 0o644); err != nil {
		t.Fatalf("write tsconfig: %v", err)
	}

	t.Run("finds tsconfig one level above", func(t *testing.T) {
		got := DetectTSConfigAbove(src)
		if got != tscPath {
			t.Fatalf("want %q, got %q", tscPath, got)
		}
	})

	t.Run("correctly rooted: tsconfig is at rootDir itself, not above it", func(t *testing.T) {
		// DetectTSConfigAbove only looks at ancestors, not rootDir itself —
		// finding it locally is DetectTSAliases's job, not this one's.
		got := DetectTSConfigAbove(frontend)
		if got != "" {
			t.Fatalf("want no result when tsconfig is at rootDir (not an ancestor), got %q", got)
		}
	})

	t.Run("no tsconfig anywhere above: empty result", func(t *testing.T) {
		bare := t.TempDir()
		nested := filepath.Join(bare, "a", "b")
		if err := os.MkdirAll(nested, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if got := DetectTSConfigAbove(nested); got != "" {
			t.Fatalf("want empty result, got %q", got)
		}
	})
}

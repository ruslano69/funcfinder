package internal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- IgnoreMatcher ---

func TestIgnoreMatcher_BasicPatterns(t *testing.T) {
	tmpDir := t.TempDir()
	gitignore := "*.log\nbuild/\n# comment\n\nvendor\n"
	if err := os.WriteFile(filepath.Join(tmpDir, ".gitignore"), []byte(gitignore), 0644); err != nil {
		t.Fatalf("write .gitignore: %v", err)
	}

	m := NewIgnoreMatcher(tmpDir)

	cases := []struct {
		path  string
		isDir bool
		want  bool
	}{
		{"app.log", false, true},
		{"src/app.log", false, true},
		{"main.go", false, false},
		{"build", true, true},
		{"build/output.bin", false, false}, // directory pattern only matches the dir itself
		{"vendor", true, true},
		// "vendor" has no trailing slash, so it is NOT marked as a
		// directory-only pattern (isDir is only set for patterns ending
		// in "/") — it matches any path with "vendor" as a path segment,
		// files included.
		{"vendor/pkg.go", false, true},
	}
	for _, c := range cases {
		got := m.Matches(c.path, c.isDir)
		if got != c.want {
			t.Errorf("Matches(%q, isDir=%v) = %v, want %v", c.path, c.isDir, got, c.want)
		}
	}
}

func TestIgnoreMatcher_NoGitignoreFile(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewIgnoreMatcher(tmpDir)
	if m.Matches("anything.go", false) {
		t.Error("Matches() = true with no .gitignore present, want false")
	}
}

func TestIgnoreMatcher_NegationLinesAreDropped(t *testing.T) {
	// parsePatterns simply discards "!"-prefixed lines instead of
	// re-including a path ignored by an earlier pattern — so a negated
	// path stays ignored as long as another pattern still covers it.
	tmpDir := t.TempDir()
	gitignore := "*.log\n!important.log\n"
	if err := os.WriteFile(filepath.Join(tmpDir, ".gitignore"), []byte(gitignore), 0644); err != nil {
		t.Fatalf("write .gitignore: %v", err)
	}
	m := NewIgnoreMatcher(tmpDir)
	if !m.Matches("important.log", false) {
		t.Error("negation line is dropped, not subtracted — *.log should still cover important.log")
	}
	if !m.Matches("other.log", false) {
		t.Error("non-negated *.log pattern should still match")
	}
}

// --- CollectSourceFiles ---

func TestCollectSourceFiles(t *testing.T) {
	tmpDir := t.TempDir()
	mustWrite(t, filepath.Join(tmpDir, "main.go"), "package main\n")
	mustWrite(t, filepath.Join(tmpDir, "README.md"), "# readme\n")
	mustMkdir(t, filepath.Join(tmpDir, "sub"))
	mustWrite(t, filepath.Join(tmpDir, "sub", "helper.go"), "package main\n")
	mustMkdir(t, filepath.Join(tmpDir, ".hidden"))
	mustWrite(t, filepath.Join(tmpDir, ".hidden", "skip.go"), "package main\n")

	config, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	goConfig, err := config.GetLanguageConfig("go")
	if err != nil {
		t.Fatalf("GetLanguageConfig(go) error = %v", err)
	}

	t.Run("recursive with lang filter", func(t *testing.T) {
		files, err := CollectSourceFiles(tmpDir, goConfig, true, false)
		if err != nil {
			t.Fatalf("CollectSourceFiles() error = %v", err)
		}
		if len(files) != 2 {
			t.Fatalf("got %d files, want 2 (main.go + sub/helper.go): %v", len(files), files)
		}
		for _, f := range files {
			if strings.Contains(f, ".hidden") {
				t.Errorf("hidden directory should be skipped, got %s", f)
			}
		}
	})

	t.Run("non-recursive", func(t *testing.T) {
		files, err := CollectSourceFiles(tmpDir, goConfig, false, false)
		if err != nil {
			t.Fatalf("CollectSourceFiles() error = %v", err)
		}
		if len(files) != 1 {
			t.Fatalf("got %d files, want 1 (main.go only): %v", len(files), files)
		}
	})

	t.Run("nil langConfig accepts every file", func(t *testing.T) {
		files, err := CollectSourceFiles(tmpDir, nil, true, false)
		if err != nil {
			t.Fatalf("CollectSourceFiles() error = %v", err)
		}
		// main.go, README.md, sub/helper.go — .hidden still skipped
		if len(files) != 3 {
			t.Fatalf("got %d files, want 3: %v", len(files), files)
		}
	})
}

// --- ProcessDirectory (collectFiles + processFilesParallel + worker) ---

func TestProcessDirectory_EndToEnd(t *testing.T) {
	tmpDir := t.TempDir()
	mustWrite(t, filepath.Join(tmpDir, "a.go"), "package main\n\nfunc Foo() {}\n")
	mustWrite(t, filepath.Join(tmpDir, "b.go"), "package main\n\nfunc Bar() {}\n")
	mustWrite(t, filepath.Join(tmpDir, "notes.txt"), "not source\n")

	config, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	dp := NewDirProcessor(config, 2, true, false, "functions")

	results, err := dp.ProcessDirectory(tmpDir)
	if err != nil {
		t.Fatalf("ProcessDirectory() error = %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2 (a.go, b.go)", len(results))
	}

	names := map[string]bool{}
	for _, r := range results {
		for _, fn := range r.Functions {
			names[fn.Name] = true
		}
	}
	if !names["Foo"] || !names["Bar"] {
		t.Errorf("expected functions Foo and Bar, got %v", names)
	}
}

func TestProcessDirectory_EmptyDir(t *testing.T) {
	tmpDir := t.TempDir()
	config, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	dp := NewDirProcessor(config, 1, true, false, "functions")

	results, err := dp.ProcessDirectory(tmpDir)
	if err != nil {
		t.Fatalf("ProcessDirectory() error = %v", err)
	}
	if len(results) != 0 {
		t.Errorf("got %d results, want 0 for empty dir", len(results))
	}
}

func TestNewDirProcessor_DefaultsWorkersWhenNonPositive(t *testing.T) {
	config, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	dp := NewDirProcessor(config, 0, true, false, "functions")
	if dp.workers <= 0 {
		t.Errorf("workers = %d, want > 0 (should default to NumCPU)", dp.workers)
	}
}

// --- AggregateDirResults / formatters ---

func sampleDirResults() []DirResult {
	return []DirResult{
		{
			Path:      filepath.Join("pkg", "a.go"),
			Functions: []FunctionBounds{{Name: "Foo", Start: 3, End: 5}},
			Classes:   []ClassBounds{{Name: "Thing", Start: 1, End: 10}},
		},
		{
			Path:      filepath.Join("pkg", "sub", "b.go"),
			Functions: []FunctionBounds{{Name: "Bar", Start: 7, End: 9}},
		},
	}
}

func TestAggregateDirResults_JSON(t *testing.T) {
	out := AggregateDirResults(sampleDirResults(), true, false, false)
	for _, want := range []string{`"path"`, "Foo", "Bar", "Thing", `"total_files": 2`, `"total_functions": 2`, `"total_classes": 1`} {
		if !strings.Contains(out, want) {
			t.Errorf("JSON output missing %q\noutput:\n%s", want, out)
		}
	}
}

func TestAggregateDirResults_Grep(t *testing.T) {
	out := AggregateDirResults(sampleDirResults(), false, false, false)
	if !strings.Contains(out, "Foo") || !strings.Contains(out, "Bar") || !strings.Contains(out, "Thing") {
		t.Errorf("grep output missing expected entries:\n%s", out)
	}
	if !strings.Contains(out, "3:") {
		t.Errorf("grep output should include line numbers:\n%s", out)
	}
}

func TestAggregateDirResults_Tree(t *testing.T) {
	out := AggregateDirResults(sampleDirResults(), false, true, false)
	for _, want := range []string{"a.go", "b.go", "Foo", "Bar", "Thing"} {
		if !strings.Contains(out, want) {
			t.Errorf("tree output missing %q\noutput:\n%s", want, out)
		}
	}
}

func TestAggregateDirResults_TreeFull(t *testing.T) {
	out := AggregateDirResults(sampleDirResults(), false, false, true)
	if !strings.Contains(out, "Foo") {
		t.Errorf("tree-full output missing function name:\n%s", out)
	}
}

func TestFormatDirResultsTree_Empty(t *testing.T) {
	out := formatDirResultsTree(nil, false)
	if out != "No functions found" {
		t.Errorf("formatDirResultsTree(nil) = %q, want %q", out, "No functions found")
	}
}

// --- escapeJSON / itoa ---

func TestEscapeJSON(t *testing.T) {
	in := "line1\nline2\ttab\\back\"quote\rcr"
	out := escapeJSON(in)
	if strings.ContainsRune(out, '\n') || strings.ContainsRune(out, '\t') || strings.ContainsRune(out, '\r') {
		t.Errorf("escapeJSON left raw control characters: %q", out)
	}
	if !strings.Contains(out, `\n`) || !strings.Contains(out, `\t`) || !strings.Contains(out, `\"`) || !strings.Contains(out, `\\`) {
		t.Errorf("escapeJSON missing expected escape sequences: %q", out)
	}
}

func TestItoa(t *testing.T) {
	if itoa(42) != "42" {
		t.Errorf("itoa(42) = %q, want %q", itoa(42), "42")
	}
}

func TestPathToShardNameAlias(t *testing.T) {
	if pathToShardName("internal/sub") != PathToShardName("internal/sub") {
		t.Error("pathToShardName alias diverges from PathToShardName")
	}
}

// --- WriteSplitOutput / formatManifestJSON / loadManifest ---

func TestWriteSplitOutput_AndLoadManifest(t *testing.T) {
	tmpDir := t.TempDir()
	mustMkdir(t, filepath.Join(tmpDir, "pkg"))
	mustWrite(t, filepath.Join(tmpDir, "pkg", "a.go"), "package pkg\n\nfunc Foo() {}\n")
	mustWrite(t, filepath.Join(tmpDir, "pkg", "b.go"), "package pkg\n\nfunc Bar() {}\n")

	config, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	dp := NewDirProcessor(config, 1, true, false, "functions")
	results, err := dp.ProcessDirectory(tmpDir)
	if err != nil {
		t.Fatalf("ProcessDirectory() error = %v", err)
	}

	outDir := filepath.Join(tmpDir, ".codemap")
	summary, err := WriteSplitOutput(results, outDir, tmpDir, "dir")
	if err != nil {
		t.Fatalf("WriteSplitOutput() error = %v", err)
	}
	if !strings.Contains(summary, "manifest.json") {
		t.Errorf("summary missing manifest mention: %s", summary)
	}

	manifestPath := filepath.Join(outDir, "manifest.json")
	if _, err := os.Stat(manifestPath); err != nil {
		t.Fatalf("manifest.json not written: %v", err)
	}

	m, err := loadManifest(outDir)
	if err != nil {
		t.Fatalf("loadManifest() error = %v", err)
	}
	if m.TotalFunctions != 2 {
		t.Errorf("TotalFunctions = %d, want 2", m.TotalFunctions)
	}
	if len(m.Shards) != 1 {
		t.Fatalf("got %d shards, want 1 (both files share the pkg/ dir)", len(m.Shards))
	}
	if m.Shards[0].Checksum == "" {
		t.Error("shard checksum should not be empty")
	}
}

func TestLoadManifest_MissingFile(t *testing.T) {
	tmpDir := t.TempDir()
	if _, err := loadManifest(tmpDir); err == nil {
		t.Error("loadManifest() on directory with no manifest.json should error")
	}
}

// --- Incremental split ---

func TestProcessDirectoryIncremental_SkipsUnchangedShards(t *testing.T) {
	tmpDir := t.TempDir()
	mustMkdir(t, filepath.Join(tmpDir, "pkg"))
	mustWrite(t, filepath.Join(tmpDir, "pkg", "a.go"), "package pkg\n\nfunc Foo() {}\n")

	config, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	dp := NewDirProcessor(config, 1, true, false, "functions")
	outDir := filepath.Join(tmpDir, ".codemap")

	// First (full) pass.
	results, err := dp.ProcessDirectory(tmpDir)
	if err != nil {
		t.Fatalf("ProcessDirectory() error = %v", err)
	}
	if _, err := WriteSplitOutput(results, outDir, tmpDir, "dir"); err != nil {
		t.Fatalf("WriteSplitOutput() error = %v", err)
	}

	// Second pass, nothing changed: incremental should report 0 changed jobs.
	incResults, err := dp.ProcessDirectoryIncremental(tmpDir, outDir, "dir")
	if err != nil {
		t.Fatalf("ProcessDirectoryIncremental() error = %v", err)
	}
	if len(incResults) != 0 {
		t.Errorf("got %d incremental results, want 0 (nothing changed)", len(incResults))
	}

	summary, err := WriteSplitOutputIncremental(incResults, outDir, tmpDir, "dir")
	if err != nil {
		t.Fatalf("WriteSplitOutputIncremental() error = %v", err)
	}
	if !strings.Contains(summary, "0 updated") {
		t.Errorf("summary should report 0 updated shards: %s", summary)
	}

	m, err := loadManifest(outDir)
	if err != nil {
		t.Fatalf("loadManifest() error = %v", err)
	}
	if m.TotalFunctions != 1 {
		t.Errorf("TotalFunctions after no-op incremental = %d, want 1 (preserved from old manifest)", m.TotalFunctions)
	}
}

func TestProcessDirectoryIncremental_DetectsChangedFile(t *testing.T) {
	tmpDir := t.TempDir()
	mustMkdir(t, filepath.Join(tmpDir, "pkg"))
	filePath := filepath.Join(tmpDir, "pkg", "a.go")
	mustWrite(t, filePath, "package pkg\n\nfunc Foo() {}\n")

	config, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	dp := NewDirProcessor(config, 1, true, false, "functions")
	outDir := filepath.Join(tmpDir, ".codemap")

	results, err := dp.ProcessDirectory(tmpDir)
	if err != nil {
		t.Fatalf("ProcessDirectory() error = %v", err)
	}
	if _, err := WriteSplitOutput(results, outDir, tmpDir, "dir"); err != nil {
		t.Fatalf("WriteSplitOutput() error = %v", err)
	}

	// Modify the file's content so its checksum changes.
	mustWrite(t, filePath, "package pkg\n\nfunc Foo() {}\n\nfunc Baz() {}\n")

	incResults, err := dp.ProcessDirectoryIncremental(tmpDir, outDir, "dir")
	if err != nil {
		t.Fatalf("ProcessDirectoryIncremental() error = %v", err)
	}
	if len(incResults) != 1 {
		t.Fatalf("got %d incremental results, want 1 (changed file reprocessed)", len(incResults))
	}

	summary, err := WriteSplitOutputIncremental(incResults, outDir, tmpDir, "dir")
	if err != nil {
		t.Fatalf("WriteSplitOutputIncremental() error = %v", err)
	}
	if !strings.Contains(summary, "1 updated") {
		t.Errorf("summary should report 1 updated shard: %s", summary)
	}

	m, err := loadManifest(outDir)
	if err != nil {
		t.Fatalf("loadManifest() error = %v", err)
	}
	if m.TotalFunctions != 2 {
		t.Errorf("TotalFunctions after incremental update = %d, want 2 (Foo + Baz)", m.TotalFunctions)
	}
}

// --- test helpers ---

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}

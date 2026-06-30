package internal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- BuildFileCallGraph: alias resolution & filtering not covered elsewhere ---

func TestBuildFileCallGraph_AliasResolution(t *testing.T) {
	goConfig := getGoConfig(t)
	content := "package main\n\nfunc Run() {\n\tfoo.Helper()\n}\n"
	tmpfile := createTempFile(t, content, "test_callgraph_alias_*.go")
	defer os.Remove(tmpfile)

	fcg, err := BuildFileCallGraph(tmpfile, goConfig, nil, map[string]string{"foo": "pkg/foo"})
	if err != nil {
		t.Fatalf("BuildFileCallGraph() error = %v", err)
	}
	if !hasEdge(fcg, "Run", "pkg/foo.Helper") {
		t.Errorf("expected aliased edge Run -> pkg/foo.Helper, got %+v", fcg.Calls)
	}
}

func TestBuildFileCallGraph_UnaliasedReceiverKeepsReceiverName(t *testing.T) {
	goConfig := getGoConfig(t)
	content := "package main\n\nfunc Run() {\n\tobj.Method()\n}\n"
	tmpfile := createTempFile(t, content, "test_callgraph_receiver_*.go")
	defer os.Remove(tmpfile)

	fcg, err := BuildFileCallGraph(tmpfile, goConfig, nil, nil)
	if err != nil {
		t.Fatalf("BuildFileCallGraph() error = %v", err)
	}
	if !hasEdge(fcg, "Run", "obj.Method") {
		t.Errorf("expected edge Run -> obj.Method, got %+v", fcg.Calls)
	}
}

func TestBuildFileCallGraph_BareCallToUnknownFunctionSkipped(t *testing.T) {
	goConfig := getGoConfig(t)
	// strconv.Itoa is package-qualified so it's tracked; a bare call to an
	// undeclared local-looking function should be dropped since it isn't in
	// knownFuncs/localFuncs.
	content := "package main\n\nfunc Run() {\n\tunknownHelper()\n}\n"
	tmpfile := createTempFile(t, content, "test_callgraph_unknown_*.go")
	defer os.Remove(tmpfile)

	fcg, err := BuildFileCallGraph(tmpfile, goConfig, nil, nil)
	if err != nil {
		t.Fatalf("BuildFileCallGraph() error = %v", err)
	}
	if hasEdge(fcg, "Run", "unknownHelper") {
		t.Error("bare call to a function not in knownFuncs should be skipped")
	}
}

func TestBuildFileCallGraph_SelfRecursionNotRecorded(t *testing.T) {
	// Documents current behavior: callee == caller is explicitly filtered out,
	// so direct recursive calls do not appear as edges.
	goConfig := getGoConfig(t)
	content := "package main\n\nfunc Recurse() {\n\tRecurse()\n}\n"
	tmpfile := createTempFile(t, content, "test_callgraph_recurse_*.go")
	defer os.Remove(tmpfile)

	fcg, err := BuildFileCallGraph(tmpfile, goConfig, map[string]bool{"Recurse": true}, nil)
	if err != nil {
		t.Fatalf("BuildFileCallGraph() error = %v", err)
	}
	if hasEdge(fcg, "Recurse", "Recurse") {
		t.Error("self-recursive call should not be recorded as an edge")
	}
}

func TestBuildFileCallGraph_KnownFuncsFromOtherFiles(t *testing.T) {
	goConfig := getGoConfig(t)
	content := "package main\n\nfunc Run() {\n\tSibling()\n}\n"
	tmpfile := createTempFile(t, content, "test_callgraph_known_*.go")
	defer os.Remove(tmpfile)

	fcg, err := BuildFileCallGraph(tmpfile, goConfig, map[string]bool{"Sibling": true}, nil)
	if err != nil {
		t.Fatalf("BuildFileCallGraph() error = %v", err)
	}
	if !hasEdge(fcg, "Run", "Sibling") {
		t.Error("call to a function known via knownFuncs (e.g. another file in the dir) should be recorded")
	}
}

// --- BuildDirCallGraph ---

func TestBuildDirCallGraph_AcrossFiles(t *testing.T) {
	tmpDir := t.TempDir()
	aPath := filepath.Join(tmpDir, "a.go")
	bPath := filepath.Join(tmpDir, "b.go")
	mustWrite(t, aPath, "package main\n\nfunc Caller() {\n\tCallee()\n}\n")
	mustWrite(t, bPath, "package main\n\nfunc Callee() {}\n")

	config, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	dp := NewDirProcessor(config, 1, true, false, "functions")
	results, err := dp.ProcessDirectory(tmpDir)
	if err != nil {
		t.Fatalf("ProcessDirectory() error = %v", err)
	}

	cg := BuildDirCallGraph(results, config, nil)
	if cg.TotalFunctions != 2 {
		t.Errorf("TotalFunctions = %d, want 2 (Caller, Callee)", cg.TotalFunctions)
	}
	if cg.TotalCalls != 1 {
		t.Errorf("TotalCalls = %d, want 1", cg.TotalCalls)
	}
	if len(cg.Files) != 1 {
		t.Fatalf("got %d files with calls, want 1 (only a.go has a call)", len(cg.Files))
	}
	if !hasEdge(&cg.Files[0], "Caller", "Callee") {
		t.Errorf("expected cross-file edge Caller -> Callee, got %+v", cg.Files[0].Calls)
	}
}

func TestBuildDirCallGraph_SkipsFilesWithNoFunctions(t *testing.T) {
	cg := BuildDirCallGraph([]DirResult{{Path: "empty.go"}}, Config{}, nil)
	if len(cg.Files) != 0 {
		t.Errorf("expected no files processed when a DirResult has zero functions, got %d", len(cg.Files))
	}
	if cg.TotalFunctions != 0 {
		t.Errorf("TotalFunctions = %d, want 0", cg.TotalFunctions)
	}
}

func TestBuildDirCallGraph_SkipsUnsupportedExtension(t *testing.T) {
	config, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	results := []DirResult{
		{
			Path:      "notes.unsupportedext",
			Functions: []FunctionBounds{{Name: "Foo", Start: 1, End: 2}},
		},
	}
	cg := BuildDirCallGraph(results, config, nil)
	if len(cg.Files) != 0 {
		t.Errorf("expected file with unsupported extension to be skipped, got %d files", len(cg.Files))
	}
	// TotalFunctions still counts it — it's a global name tally, independent
	// of whether the file's calls could be resolved.
	if cg.TotalFunctions != 1 {
		t.Errorf("TotalFunctions = %d, want 1", cg.TotalFunctions)
	}
}

// --- ReverseCallGraph ---

func TestReverseCallGraph(t *testing.T) {
	cg := &CallGraphResult{
		Files: []FileCallGraph{
			{
				Path: "a.go",
				Calls: []CallEdge{
					{Caller: "Handler", Callee: "Validate", Line: 5},
					{Caller: "Handler", Callee: "Save", Line: 6},
				},
			},
			{
				Path: "b.go",
				Calls: []CallEdge{
					{Caller: "Middleware", Callee: "Validate", Line: 10},
					// Duplicate caller->callee pair across files should be deduped.
					{Caller: "Handler", Callee: "Validate", Line: 20},
				},
			},
		},
	}

	rev := ReverseCallGraph(cg)

	callers := rev["Validate"]
	if len(callers) != 2 {
		t.Fatalf("Validate callers = %v, want 2 unique entries (Handler, Middleware)", callers)
	}
	want := map[string]bool{"Handler": true, "Middleware": true}
	for _, c := range callers {
		if !want[c] {
			t.Errorf("unexpected caller %q for Validate", c)
		}
	}

	if got := rev["Save"]; len(got) != 1 || got[0] != "Handler" {
		t.Errorf("Save callers = %v, want [Handler]", got)
	}

	if _, ok := rev["Nobody"]; ok {
		t.Error("ReverseCallGraph should not produce entries for callees that don't exist")
	}
}

func TestReverseCallGraph_Empty(t *testing.T) {
	rev := ReverseCallGraph(&CallGraphResult{})
	if len(rev) != 0 {
		t.Errorf("got %d entries for empty call graph, want 0", len(rev))
	}
}

// --- FormatCallGraphText ---

func TestFormatCallGraphText(t *testing.T) {
	cg := &CallGraphResult{
		Files: []FileCallGraph{
			{
				Path: "internal/dirprocessor.go",
				Calls: []CallEdge{
					{Caller: "ProcessDirectory", Callee: "collectFiles", Line: 66},
					{Caller: "ProcessDirectory", Callee: "processFilesParallel", Line: 76},
				},
			},
		},
	}

	out := FormatCallGraphText(cg)

	if !strings.HasPrefix(out, "internal/dirprocessor.go\n") {
		t.Errorf("output should start with the file path, got:\n%s", out)
	}
	if !strings.Contains(out, "ProcessDirectory → collectFiles") {
		t.Errorf("missing forward-arrow edge line, got:\n%s", out)
	}
	if strings.HasSuffix(out, "\n") {
		t.Error("FormatCallGraphText should trim the trailing newline")
	}
}

func TestFormatCallGraphText_Empty(t *testing.T) {
	out := FormatCallGraphText(&CallGraphResult{})
	if out != "" {
		t.Errorf("FormatCallGraphText(empty) = %q, want empty string", out)
	}
}

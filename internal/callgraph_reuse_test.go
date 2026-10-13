package internal

import (
	"reflect"
	"testing"
)

// TestBuildDirCallGraph_ReusedBoundsMatchRefind runs the directory job on the fixtures of every language and builds
// the call graph twice: with the bounds the job found (the fast path), and with LangKey cleared so that each file's
// bounds are found again (the path BuildDirCallGraph took before). The graphs must be the same.
func TestBuildDirCallGraph_ReusedBoundsMatchRefind(t *testing.T) {
	config, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{"../test_examples", "../test_files"} {
		results, err := NewDirProcessor(config, 0, true, false, "functions").ProcessDirectory(dir)
		if err != nil {
			t.Fatal(err)
		}
		withFuncs := 0
		refind := make([]DirResult, len(results))
		for i, r := range results {
			if len(r.Functions) > 0 {
				withFuncs++
			}
			r.LangKey = ""
			refind[i] = r
		}
		if withFuncs < 3 {
			t.Fatalf("%s: only %d files with functions; the fixtures are missing", dir, withFuncs)
		}
		got := BuildDirCallGraph(results, config, nil)
		want := BuildDirCallGraph(refind, config, nil)
		if got.TotalCalls == 0 {
			t.Fatalf("%s: no calls found", dir)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s: graph with reused bounds differs from the graph with bounds found again: %d vs %d calls",
				dir, got.TotalCalls, want.TotalCalls)
		}
	}
}

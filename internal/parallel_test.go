package internal

import (
	"reflect"
	"sync/atomic"
	"testing"
)

// TestForEachParallel_EachIndexOnce checks that every index is visited exactly once, for empty, small and large n.
func TestForEachParallel_EachIndexOnce(t *testing.T) {
	for _, n := range []int{0, 1, 3, 1000} {
		counts := make([]int32, n)
		ForEachParallel(n, func(i int) { atomic.AddInt32(&counts[i], 1) })
		for i, c := range counts {
			if c != 1 {
				t.Fatalf("n=%d: index %d visited %d times", n, i, c)
			}
		}
	}
}

// TestFindFunctionsInFile_MatchesFindFunctions checks that running a finder on lines read by the caller gives what
// FindFunctions gives when it reads the file itself, for every fixture of a language with a finder.
func TestFindFunctionsInFile_MatchesFindFunctions(t *testing.T) {
	config, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	files, err := CollectSourceFiles("../test_examples", nil, true)
	if err != nil || len(files) < 10 {
		t.Fatalf("collecting fixtures: %d files, %v", len(files), err)
	}
	for _, path := range files {
		lc := config.GetLanguageByExtension(path)
		if lc == nil {
			continue
		}
		lines, err := readFileLines(path)
		if err != nil {
			t.Fatal(err)
		}
		want, werr := CreateFinder(lc, "", "map", false, false).FindFunctions(path)
		got, gerr := FindFunctionsInFile(CreateFinder(lc, "", "map", false, false), path, lines)
		if (werr == nil) != (gerr == nil) || !reflect.DeepEqual(got, want) {
			t.Errorf("%s: FindFunctionsInFile differs from FindFunctions", path)
		}
	}
}

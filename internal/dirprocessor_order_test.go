package internal

import (
	"math/rand"
	"testing"
)

// The output of a directory run must not depend on the order in which workers finished the files:
// the tree's children come from a map, whose iteration order is random in Go.
func TestAggregateDirResults_IndependentOfResultOrder(t *testing.T) {
	results := []DirResult{
		{Path: "a/x.go", Functions: []FunctionBounds{{Name: "f", Start: 1, End: 2}}},
		{Path: "a/y.go", Functions: []FunctionBounds{{Name: "g", Start: 3, End: 4}}},
		{Path: "b/z.go", Functions: []FunctionBounds{{Name: "h", Start: 5, End: 6}}},
		{Path: "c.go", Functions: []FunctionBounds{{Name: "k", Start: 7, End: 8}}},
		{Path: "d/e/w.go", Functions: []FunctionBounds{{Name: "m", Start: 9, End: 10}}},
	}

	modes := []struct {
		name                    string
		jsonOut, treeMode, full bool
	}{
		{"grep", false, false, false},
		{"tree", false, true, false},
		{"tree-full", false, false, true},
		{"json", true, false, false},
	}

	rng := rand.New(rand.NewSource(1))
	for _, mode := range modes {
		want := AggregateDirResults(results, mode.jsonOut, mode.treeMode, mode.full)
		for trial := 0; trial < 50; trial++ {
			shuffled := append([]DirResult(nil), results...)
			rng.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
			if got := AggregateDirResults(shuffled, mode.jsonOut, mode.treeMode, mode.full); got != want {
				t.Fatalf("%s: output changed with input order (trial %d):\n%s\n--- want ---\n%s", mode.name, trial, got, want)
			}
		}
	}
}

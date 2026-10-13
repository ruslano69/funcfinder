package main

import (
	"reflect"
	"testing"
)

// TestSortedCalls_TiesInNameOrder pins the order: most calls first, names in order among equal counts. The map is
// read in random order, so the test sorts the same map many times and expects one result.
func TestSortedCalls_TiesInNameOrder(t *testing.T) {
	counts := map[string]int{"delta": 2, "alpha": 2, "zeta": 5, "beta": 1, "gamma": 2, "eps": 1}
	want := []string{"zeta", "alpha", "delta", "gamma", "beta", "eps"}
	for run := 0; run < 50; run++ {
		var got []string
		for _, c := range sortedCalls(counts) {
			got = append(got, c.name)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("run %d: sortedCalls order = %v, want %v", run, got, want)
		}
	}
	// With -n 2 the cut falls among the tied names: the first by name is kept, every time.
	if got := toCallEntries(sortedCalls(counts), 2); got[1].Name != "alpha" {
		t.Errorf("toCallEntries(-n 2)[1] = %q, want alpha", got[1].Name)
	}
}

package main

import (
	"reflect"
	"testing"
)

// countMaxBoolOps counts the operators of the busiest line: && and || in one line, and and / or in Python. The
// example from the review, a && (b || c) && !d, has three.
func TestCountMaxBoolOps(t *testing.T) {
	lines := []string{
		"if a && (b || c) && !d {",
		"x := a || b",
		"y := 1",
	}
	if got := countMaxBoolOps(lines); got != 3 {
		t.Errorf("countMaxBoolOps = %d, want 3", got)
	}
	if got := countMaxBoolOps([]string{"if a and b or c:", "color := 1"}); got != 2 {
		t.Errorf("countMaxBoolOps (python) = %d, want 2; color must not count as or", got)
	}
	if got := countMaxBoolOps(nil); got != 0 {
		t.Errorf("countMaxBoolOps(nil) = %d, want 0", got)
	}
}

// The -bool limit flags a function whose busiest line has more operators than the limit, and it is named bool in the
// JSON keys. The default is 2: four conditions on one line, the range the review called hard to read.
func TestBoolLimit(t *testing.T) {
	m := ComplexityMetrics{LinesOfCode: 10, DecisionCount: 2, MaxNestingDepth: 1, MaxBoolOps: 3}
	if got := triggeredFlags(m, defaultThresholds); !reflect.DeepEqual(got, []string{"bool"}) {
		t.Errorf("triggeredFlags with 3 operators = %v, want bool", got)
	}
	if got := breakdownFlags(m, defaultThresholds); !reflect.DeepEqual(got, []string{"long boolean chain (>2 operators on a line)"}) {
		t.Errorf("breakdownFlags with 3 operators = %v", got)
	}

	// Three operators pass a limit of 3; with the limit off the flag never fires.
	raised := breakdownThresholds{Lines: 100, Decisions: 10, Bool: 3, Depth: 4, Loops: 3}
	if got := triggeredFlags(m, raised); len(got) != 0 {
		t.Errorf("triggeredFlags with -bool 3 = %v, want none", got)
	}
	off := defaultThresholds
	off.Bool = 0
	if got := triggeredFlags(m, off); len(got) != 0 {
		t.Errorf("triggeredFlags with -bool 0 = %v, want none", got)
	}
}

// A repeated -exclude collects every pattern, and -bool and -exclude take their values next to them when arguments are
// reordered.
func TestExcludeAndBoolKeepValues(t *testing.T) {
	var list stringList
	for _, v := range []string{"*_test.go", "gen/*.go"} {
		if err := list.Set(v); err != nil {
			t.Fatal(err)
		}
	}
	if want := (stringList{"*_test.go", "gen/*.go"}); !reflect.DeepEqual(list, want) {
		t.Errorf("stringList = %v, want %v", list, want)
	}

	in := []string{"internal/finder.go", "-exclude", "*_test.go", "-bool", "2", "-exclude", "gen/*.go"}
	want := []string{"-exclude", "*_test.go", "-bool", "2", "-exclude", "gen/*.go", "internal/finder.go"}
	if got := reorderArgs(in); !reflect.DeepEqual(got, want) {
		t.Errorf("reorderArgs = %v\nwant %v", got, want)
	}
}

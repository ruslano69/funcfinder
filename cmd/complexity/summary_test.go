package main

import "testing"

// Three functions: a simple one, one that passes four limits, and a loop nested three deep. The counts follow from
// the defaults: 2 flagged, and the score in them is 50+12.
func TestSummarizeCounts(t *testing.T) {
	files := []FileComplexity{{Functions: []ComplexityMetrics{
		{Complexity: 3, LinesOfCode: 10, DecisionCount: 2, MaxNestingDepth: 1},
		{Complexity: 50, LinesOfCode: 150, DecisionCount: 12, MaxNestingDepth: 4, MaxBoolOps: 3},
		{Complexity: 12, LinesOfCode: 20, DecisionCount: 3, MaxNestingDepth: 0, MaxLoopNestingDepth: 3},
	}}}

	s := summarize(files, defaultThresholds)
	if s.Files != 1 || s.Functions != 3 || s.TotalScore != 65 {
		t.Errorf("files=%d functions=%d total=%d, want 1, 3, 65", s.Files, s.Functions, s.TotalScore)
	}
	if s.Levels["SIMPLE"] != 1 || s.Levels["VERY_HIGH"] != 1 || s.Levels["CRITICAL"] != 1 {
		t.Errorf("levels = %v, want SIMPLE 1, VERY_HIGH 1, CRITICAL 1", s.Levels)
	}
	if s.Flagged != 2 || s.FlaggedScore != 62 {
		t.Errorf("flagged=%d score=%d, want 2 and 62", s.Flagged, s.FlaggedScore)
	}
	want := map[string]int{"len": 1, "if": 1, "bool": 1, "deep": 1, "loop": 1}
	for key, n := range want {
		if s.ByLimit[key] != n {
			t.Errorf("ByLimit[%s] = %d, want %d (all = %v)", key, s.ByLimit[key], n, s.ByLimit)
		}
	}
	if len(s.LimitOrder) != 5 || s.LimitOrder[0] != "len" || s.LimitOrder[4] != "loop" {
		t.Errorf("LimitOrder = %v, want len ... loop", s.LimitOrder)
	}
}

// The limits line spells every limit, with off for a limit of 0, so a saved summary says what it was measured with.
func TestLimitsLine(t *testing.T) {
	if got := limitsLine(defaultThresholds); got != "-len 100  -if 10  -bool 2  -deep 4  -loop 3" {
		t.Errorf("limitsLine(defaults) = %q", got)
	}
	off := breakdownThresholds{Lines: 0, Decisions: 30, Bool: 0, Depth: 7, Loops: 0}
	if got := limitsLine(off); got != "-len off  -if 30  -bool off  -deep 7  -loop off" {
		t.Errorf("limitsLine(off) = %q", got)
	}
}

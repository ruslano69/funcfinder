package main

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// triggered_flags uses the same limits as the text flags: its keys name the limits a function passes, in the same
// order, and a function that passes none has an empty list, not null.
func TestTriggeredFlagsKeys(t *testing.T) {
	big := ComplexityMetrics{LinesOfCode: 943, DecisionCount: 185, MaxNestingDepth: 4, MaxLoopNestingDepth: 3}
	if got := triggeredFlags(big, defaultThresholds); !reflect.DeepEqual(got, []string{"len", "if", "deep", "loop"}) {
		t.Errorf("triggeredFlags(big) = %v, want len, if, deep, loop", got)
	}

	// A limit raised past the function drops its key and leaves the others.
	raised := breakdownThresholds{Lines: 1000, Decisions: 10, Depth: 4, Loops: 3}
	if got := triggeredFlags(big, raised); !reflect.DeepEqual(got, []string{"if", "deep", "loop"}) {
		t.Errorf("triggeredFlags with -len 1000 = %v, want if, deep, loop", got)
	}
}

// A function that passes no limit, or a run with every limit off, gives an empty list, and the JSON shows [].
func TestTriggeredFlagsEmptyIsJSONArray(t *testing.T) {
	small := ComplexityMetrics{LinesOfCode: 10, DecisionCount: 2, MaxNestingDepth: 1}
	got := triggeredFlags(small, defaultThresholds)
	if got == nil || len(got) != 0 {
		t.Fatalf("triggeredFlags(small) = %#v, want an empty non-nil list", got)
	}

	big := ComplexityMetrics{LinesOfCode: 943, DecisionCount: 185, MaxNestingDepth: 4, MaxLoopNestingDepth: 3}
	if off := triggeredFlags(big, breakdownThresholds{}); off == nil || len(off) != 0 {
		t.Fatalf("triggeredFlags with every limit 0 = %#v, want an empty non-nil list", off)
	}

	small.TriggeredFlags = got
	data, err := json.Marshal(small)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"triggered_flags":[]`) {
		t.Errorf("JSON = %s, want triggered_flags as []", data)
	}
}

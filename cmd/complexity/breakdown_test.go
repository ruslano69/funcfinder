package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/ruslano69/funcfinder/internal"
)

// rustConfig loads the Rust language settings the complexity command uses.
func rustConfig(t *testing.T) *internal.LanguageConfig {
	t.Helper()
	cfg, err := internal.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	lc := cfg["rust"]
	if lc == nil {
		t.Fatal("no rust language config")
	}
	return lc
}

// A decision is an if, match or loop keyword wherever it sits on the line, including inside an expression. An else is a
// sibling and is not one, a name such as loop_count is not a keyword, and a line that combines conditions with && or ||
// is one boolean line however many operators it has.
func TestCountDecisionsAndBooleanLines_Rust(t *testing.T) {
	lines := []string{
		"if a {",
		"} else {",
		"for x in xs {",
		"    if b && c {",
		"    }",
		"}",
		"let y = a || b;",
		"let v = if q { 1 } else { 2 };",
		"let loop_count = 1;",
	}
	if got := countDecisions(lines); got != 4 {
		t.Errorf("countDecisions = %d, want 4 (if, for, if, the if inside the let; else and loop_count are not)", got)
	}
	if got := countBooleanLines(lines); got != 2 {
		t.Errorf("countBooleanLines = %d, want 2 (the && line and the || line)", got)
	}
}

// The calibrated nested function: four decisions, one inside another, so the score is 1+2+3+4 = 10 and the branch depth
// is 4. The figures on the metrics must agree with that.
func TestBreakdownOfNestedFunction(t *testing.T) {
	src := "fn nested4(a: bool, b: bool, c: bool, d: bool) {\n" +
		"    if a {\n" +
		"        if b {\n" +
		"            if c {\n" +
		"                if d {\n" +
		"                }\n" +
		"            }\n" +
		"        }\n" +
		"    }\n" +
		"}\n"
	path := filepath.Join(t.TempDir(), "nested.rs")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	res := analyzeFileComplexity(path, rustConfig(t))
	if len(res.Functions) != 1 {
		t.Fatalf("functions = %d, want 1", len(res.Functions))
	}
	fn := res.Functions[0]
	if fn.DecisionCount != 4 || fn.MaxNestingDepth != 4 || fn.Complexity != 10 || fn.BooleanLines != 0 {
		t.Errorf("decisions=%d depth=%d score=%d boolean=%d, want 4, 4, 10, 0",
			fn.DecisionCount, fn.MaxNestingDepth, fn.Complexity, fn.BooleanLines)
	}
}

// Each threshold is named once, in reading order, and a small function passes none.
func TestBreakdownFlags(t *testing.T) {
	big := ComplexityMetrics{LinesOfCode: 943, DecisionCount: 185, MaxNestingDepth: 4, MaxLoopNestingDepth: 2}
	want := []string{"long (>300 lines)", "many decisions (>50)", "nested loops"}
	if got := breakdownFlags(big); !reflect.DeepEqual(got, want) {
		t.Errorf("breakdownFlags(big) = %v, want %v", got, want)
	}

	deep := ComplexityMetrics{LinesOfCode: 10, DecisionCount: 2, MaxNestingDepth: 5}
	if got := breakdownFlags(deep); !reflect.DeepEqual(got, []string{"deep branches (depth >=5)"}) {
		t.Errorf("breakdownFlags(deep) = %v", got)
	}

	small := ComplexityMetrics{LinesOfCode: 10, DecisionCount: 2, MaxNestingDepth: 1}
	if got := breakdownFlags(small); len(got) != 0 {
		t.Errorf("breakdownFlags(small) = %v, want none", got)
	}
}

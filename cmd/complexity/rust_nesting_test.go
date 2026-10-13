package main

import (
	"os"
	"path/filepath"
	"testing"
)

// scoreOfRust returns the cognitive score of the named Rust function in src.
func scoreOfRust(t *testing.T, src, name string) int {
	t.Helper()
	path := filepath.Join(t.TempDir(), "probe.rs")
	if err := os.WriteFile(path, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, fn := range analyzeFileComplexity(path, rustConfig(t)).Functions {
		if fn.Name == name {
			return fn.Complexity
		}
	}
	t.Fatalf("function %q not found", name)
	return 0
}

// A decision keyword inside an expression is a decision: a match that opens a block after `let v =` nests like a
// statement-level one. The score used to read the keyword only at the start of a line, so this match cost nothing.
func TestRustMatchInExpressionScores(t *testing.T) {
	src := "fn f(a: bool, b: Option<u8>) -> u8 {\n" +
		"    if a {\n" +
		"        let v = match b {\n" +
		"            Some(x) => x,\n" +
		"            None => 0,\n" +
		"        };\n" +
		"        v\n" +
		"    } else {\n" +
		"        0\n" +
		"    }\n" +
		"}\n"
	// The if costs 1; the match inside it costs 1+1; the else is a flat sibling and costs 1.
	if got := scoreOfRust(t, src, "f"); got != 4 {
		t.Errorf("score = %d, want 4 (if 1, match inside it 2, else 1)", got)
	}
}

// A labelled loop ('outer: loop) is a loop: the label in front of the keyword does not hide it.
func TestRustLabelledLoopScores(t *testing.T) {
	src := "fn f(xs: &[u8]) {\n" +
		"    'outer: loop {\n" +
		"        for x in xs {\n" +
		"            if *x == 0 {\n" +
		"                break 'outer;\n" +
		"            }\n" +
		"        }\n" +
		"    }\n" +
		"}\n"
	// The loop costs 1; the for inside it 1+1; the if inside the for 1+2.
	if got := scoreOfRust(t, src, "f"); got != 6 {
		t.Errorf("score = %d, want 6 (loop 1, for 2, if 3)", got)
	}
}

// The Rust nesting pattern finds a keyword anywhere on a cleaned line. A `for` counts only as a loop (`for pat in
// expr`): an `impl Trait for Type` header, a `for<'a>` bound and a name that starts with a keyword are not decisions.
func TestRustNestingPatternKeywordAnywhere(t *testing.T) {
	re := rustConfig(t).NestingRegex()
	if re == nil {
		t.Fatal("rust has no nesting pattern")
	}

	decisions := []string{
		"let v = match b {",
		"let y = if c { 1 } else { 2 };",
		"'outer: loop {",
		"for x in xs {",
		"for Path { name, frames } in paths {",
		"    while let Some(x) = it.next() {",
		"    if a {",
		"return match x {",
	}
	for _, line := range decisions {
		if !re.MatchString(line) {
			t.Errorf("nesting pattern should match %q", line)
		}
	}

	notDecisions := []string{
		"impl Shape for Circle {",
		"impl<T: Clone> From<T> for Wrapper<T> {",
		"where F: for<'a> Fn(&'a u8) {",
		"fn for_each(xs: &[u8]) {",
		"let formatted = 3;",
		"let loop_count = 1;",
		"let matches = 2;",
	}
	for _, line := range notDecisions {
		if re.MatchString(line) {
			t.Errorf("nesting pattern should not match %q", line)
		}
	}
}

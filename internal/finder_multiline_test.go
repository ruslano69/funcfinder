package internal

import (
	"strings"
	"testing"
)

func getRustConfig(t *testing.T) *LanguageConfig {
	t.Helper()
	config, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	rs := config["rust"]
	if rs == nil {
		t.Fatal("rust config not found")
	}
	return rs
}

func getCConfig(t *testing.T) *LanguageConfig {
	t.Helper()
	config, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	c := config["c"]
	if c == nil {
		t.Fatal("c config not found")
	}
	return c
}

// findFunctionsSimple is the non-nested path (Rust, C, C++, C#, Java, D,
// Kotlin, PHP). This pins multiline-signature handling for that path — the
// case the dead `else if currentFunc != nil` branch at finder.go:223 appeared
// to handle but never actually reached (the multiline continuation is in fact
// handled by the `if currentFunc != nil` branch on the following lines).
func TestFindFunctionsSimple_MultilineSignature(t *testing.T) {
	rustCfg := getRustConfig(t)
	if rustCfg.SupportsNested {
		t.Skip("rust unexpectedly uses the nested path")
	}

	code := `fn simple() {
    let x = 1;
}

fn with_where<T>(x: T) -> Result<T>
where
    T: Deserialize,
{
    Ok(x)
}

fn after() {
    done();
}`
	lines := strings.Split(code, "\n")
	finder := NewFinder(rustCfg, nil, true, false, false)

	result, err := finder.FindFunctionsInLines(lines, 1, "test.rs")
	if err != nil {
		t.Fatalf("FindFunctionsInLines() error = %v", err)
	}

	got := map[string][2]int{}
	for _, fn := range result.Functions {
		got[fn.Name] = [2]int{fn.Start, fn.End}
	}

	// All three functions must be found, including the one with a multiline
	// where-clause signature.
	for _, name := range []string{"simple", "with_where", "after"} {
		if _, ok := got[name]; !ok {
			t.Errorf("function %q not found; got %v", name, got)
		}
	}

	// with_where must span from its `fn` line (5) through its closing brace
	// (10) — the where-clause lines must not prematurely end it.
	if span, ok := got["with_where"]; ok {
		if span[0] != 5 {
			t.Errorf("with_where.Start = %d, want 5", span[0])
		}
		if span[1] != 10 {
			t.Errorf("with_where.End = %d, want 10 (where-clause must not end the fn early)", span[1])
		}
	}
}

// TestFindFunctionsSimple_SingleLineBodyOnSignatureLine regression-tests a bug
// found while investigating TODO.md's "callgraph -l is a hint" item: a
// single-line function body on the *signature* line itself — e.g. Rust's
// idiomatic `fn helper() -> i32 { 42 }` — has a net brace delta of 0 (one
// open, one close), which findFunctionsSimple's "braceCount > 0" check
// treated identically to "no brace at all yet, wait for a multiline
// signature's opening brace". The function was left open indefinitely and
// silently absorbed every following line (including an entire next
// function) until an unrelated brace-balance event happened to close it.
func TestFindFunctionsSimple_SingleLineBodyOnSignatureLine(t *testing.T) {
	rustCfg := getRustConfig(t)
	if rustCfg.SupportsNested {
		t.Skip("rust unexpectedly uses the nested path")
	}

	code := `fn helper() -> i32 { 42 }
fn main() {
    let x = helper();
    println!("{}", x);
}`
	lines := strings.Split(code, "\n")
	finder := NewFinder(rustCfg, nil, true, false, false)

	result, err := finder.FindFunctionsInLines(lines, 1, "test.rs")
	if err != nil {
		t.Fatalf("FindFunctionsInLines() error = %v", err)
	}

	got := map[string][2]int{}
	for _, fn := range result.Functions {
		got[fn.Name] = [2]int{fn.Start, fn.End}
	}
	if len(result.Functions) != 2 {
		t.Fatalf("got %d functions %+v, want exactly [helper, main]", len(result.Functions), got)
	}
	if span, ok := got["helper"]; !ok || span != [2]int{1, 1} {
		t.Errorf("helper span = %v (ok=%v), want [1,1] (single-line, not swallowing main)", span, ok)
	}
	if span, ok := got["main"]; !ok || span != [2]int{2, 5} {
		t.Errorf("main span = %v (ok=%v), want [2,5]", span, ok)
	}
}

// TestFindFunctionsSimple_SingleLineBodyOnBraceLine regression-tests the
// other manifestation of the same bug shape, for K&R-style C: the signature
// line ends bare (no brace, per C's func_pattern requiring end-of-line after
// the parameter list), and the *next* line supplies both the opening AND
// closing brace for a one-line body:
//
//	int helper(void)
//	{ return 42; }
//
// Here prevDepth is 0 (nothing open yet) and depth nets to 0 (one open, one
// close) on that line — indistinguishable, without also checking whether the
// line actually contained a brace, from a pure multiline-signature
// continuation line that hasn't reached its opening brace yet.
func TestFindFunctionsSimple_SingleLineBodyOnBraceLine(t *testing.T) {
	cCfg := getCConfig(t)
	if cCfg.SupportsNested {
		t.Skip("c unexpectedly uses the nested path")
	}

	code := `int helper(void)
{ return 42; }
int main(void)
{
    return helper();
}`
	lines := strings.Split(code, "\n")
	finder := NewFinder(cCfg, nil, true, false, false)

	result, err := finder.FindFunctionsInLines(lines, 1, "test.c")
	if err != nil {
		t.Fatalf("FindFunctionsInLines() error = %v", err)
	}

	got := map[string][2]int{}
	for _, fn := range result.Functions {
		got[fn.Name] = [2]int{fn.Start, fn.End}
	}
	if len(result.Functions) != 2 {
		t.Fatalf("got %d functions %+v, want exactly [helper, main]", len(result.Functions), got)
	}
	if span, ok := got["helper"]; !ok || span != [2]int{1, 2} {
		t.Errorf("helper span = %v (ok=%v), want [1,2] (not swallowing main)", span, ok)
	}
	if span, ok := got["main"]; !ok || span != [2]int{3, 6} {
		t.Errorf("main span = %v (ok=%v), want [3,6]", span, ok)
	}
}

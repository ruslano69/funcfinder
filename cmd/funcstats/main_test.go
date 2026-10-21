package main

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/ruslano69/funcfinder/internal"
)

func TestCallCounts_GoParenthesizedControlExpressions(t *testing.T) {
	src := `package sample
func predicate() bool { return true }
func value() int { return 1 }
func recur(n int) int {
    if (predicate()) { println(len(make([]byte, 1))) }
    for (predicate()) { break }
    switch (value()) {
    case (value()): println("case")
    }
    if (n > 0) { return (recur(n - 1)) }
    return (int(value()))
}`
	// Check actual Go syntax, including optional parentheses around conditions
	// and return expressions, independently of the function finder.
	if _, err := parser.ParseFile(token.NewFileSet(), "control.go", src, 0); err != nil {
		t.Fatalf("invalid Go fixture: %v", err)
	}
	want := map[string]int{
		"predicate": 2,
		"value":     3,
		"recur":     1,
		"println":   2,
		"len":       1,
		"make":      1,
		"int":       1,
	}
	if got := callCountsOf(t, "go", "control.go", src); !reflect.DeepEqual(got, want) {
		t.Errorf("calls = %v, want %v (keywords excluded, real calls retained)", got, want)
	}
}

func TestCallCounts_JavaSplitAndInlineSignatures(t *testing.T) {
	counts := callCountsOf(t, "java", "sample.java", `class C {
    @Deprecated
    public <T> T
    unused(T value) { return value; }
    int
    recurse(int n) { return n > 0 ? recurse(n - 1) : 0; }
    void inline() { inline(); }
    @interface Tag { String[] names() default {"a", "b"}; }
}`)
	for name, want := range map[string]int{"unused": 0, "recurse": 1, "inline": 1, "names": 0} {
		if counts[name] != want {
			t.Errorf("%s count = %d, want %d; all counts: %v", name, counts[name], want, counts)
		}
	}
}

func TestCallCounts_CSharpSignaturesAndRealRecursion(t *testing.T) {
	counts := callCountsOf(t, "cs", "sample.cs", `class C {
    C() { }
    int Recurse(int n) {
        return n > 0 ? Recurse(n - 1) : 0;
    }
    void Inline() => Inline();
    ~C() { }
    public static implicit operator C(int n) => new C();
}`)
	for name, want := range map[string]int{"Recurse": 1, "Inline": 1, "C": 1} {
		if counts[name] != want {
			t.Errorf("%s count = %d, want %d; all counts: %v", name, counts[name], want, counts)
		}
	}
}

func TestCallCounts_JavaScriptExpressionsAndDollarRecursion(t *testing.T) {
	counts := callCountsOf(t, "js", "sample.cjs", `function $cash(n) { return n ? $cash(n-1) : 0; }
const arrow = n => arrow(n);
const alias = function inner(n) { return n ? inner(n-1) : 0; };
function
split() { return 1; }
`)
	for name, want := range map[string]int{"$cash": 1, "arrow": 1, "inner": 1, "split": 0} {
		if counts[name] != want {
			t.Errorf("%s count = %d, want %d; all counts: %v", name, counts[name], want, counts)
		}
	}
}

// callCountsOf writes src to a temp file and runs analyzeFile against it,
// mirroring cmd/complexity/nesting_test.go's depthOf helper style.
func callCountsOf(t *testing.T, lang, filename, src string) map[string]int {
	t.Helper()

	cfg, err := internal.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	langConfig, err := cfg.GetLanguageConfig(lang)
	if err != nil {
		t.Fatalf("%s language config: %v", lang, err)
	}

	path := filepath.Join(t.TempDir(), filename)
	if err := os.WriteFile(path, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}

	calls, _ := analyzeFile(path, langConfig)
	return calls
}

// TestAnalyzeFile_NeverCalledFunctionCountsZero pins the core bug: funcstats used
// to count every function's own "name(" signature token as a call to
// itself, so a function that is never called anywhere still showed count=1
// — making it indistinguishable from a function genuinely called once.
func TestAnalyzeFile_NeverCalledFunctionCountsZero(t *testing.T) {
	src := `package main

func helper(x int) int {
	return x + 1
}

func neverCalled(y int) int {
	return y * 2
}

func main() {
	println(helper(5))
	println(helper(10))
}
`
	calls := callCountsOf(t, "go", "probe.go", src)

	if got := calls["neverCalled"]; got != 0 {
		t.Errorf("neverCalled count = %d, want 0 (it is never called, only defined)", got)
	}
	if got := calls["helper"]; got != 2 {
		t.Errorf("helper count = %d, want 2 (called twice from main, definition line must not add a phantom +1)", got)
	}
	if got := calls["main"]; got != 0 {
		t.Errorf("main count = %d, want 0 (never called from within this file)", got)
	}
}

// TestAnalyzeFile_RecursiveCallStillCounted documents the intended
// difference from callgraph's broader (whole-body) self-match skip: funcstats
// must only exclude the exact signature-line occurrence of a function's
// name, not every occurrence of that name inside its own body — a real
// recursive call still counts.
func TestAnalyzeFile_RecursiveCallStillCounted(t *testing.T) {
	src := `package main

func factorial(n int) int {
	if n <= 1 {
		return 1
	}
	return n * factorial(n-1)
}

func main() {
	println(factorial(5))
}
`
	calls := callCountsOf(t, "go", "recur.go", src)

	// 1 recursive call inside factorial's own body + 1 call from main = 2.
	// (Not 3, which is what the old bug would have produced by also
	// counting the "func factorial(" signature line itself.)
	if got := calls["factorial"]; got != 2 {
		t.Errorf("factorial count = %d, want 2 (1 recursive + 1 from main, signature line excluded)", got)
	}
}

// TestAnalyzeFile_DecoratedDefinitionNotCounted covers a case the naive
// "skip the finder's FunctionBounds.Start line" approach would miss:
// decorators push a Python function's Start line to the decorator, not to
// the "def name(" line itself. The self-definition check must find the
// actual signature line regardless of how many decorators precede it.
func TestAnalyzeFile_DecoratedDefinitionNotCounted(t *testing.T) {
	src := `def plain_helper(x):
    return x + 1


@staticmethod
@some.decorator(arg=True)
def decorated_target(y):
    return y * 2


def caller():
    plain_helper(1)
    decorated_target(2)
    decorated_target(3)
`
	calls := callCountsOf(t, "py", "decor.py", src)

	if got := calls["decorated_target"]; got != 2 {
		t.Errorf("decorated_target count = %d, want 2 (called twice from caller)", got)
	}
	if got := calls["plain_helper"]; got != 1 {
		t.Errorf("plain_helper count = %d, want 1", got)
	}
	if got := calls["caller"]; got != 0 {
		t.Errorf("caller count = %d, want 0 (never called)", got)
	}
}

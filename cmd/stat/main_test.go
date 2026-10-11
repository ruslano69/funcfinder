package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ruslano69/funcfinder/internal"
)

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

// TestAnalyzeFile_NeverCalledFunctionCountsZero pins the core bug: stat used
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
// difference from callgraph's broader (whole-body) self-match skip: stat
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

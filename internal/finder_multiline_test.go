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

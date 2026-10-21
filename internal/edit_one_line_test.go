package internal

import (
	"strings"
	"testing"
)

// A one-line function that stands alone on its line may have ';' inside its body: that is the function's own code.
func TestEdit_StandaloneOneLineFunctionWithSemicolon(t *testing.T) {
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	lc, err := cfg.GetLanguageConfig("c")
	if err != nil {
		t.Fatalf("config: %v", err)
	}
	src := "int one(void) { return 1; }\nint two(void) { return 2; }\n"
	res, err := RemoveFunction(lc, []byte(src), FuncTarget{Name: "one"})
	if err != nil {
		t.Fatalf("standalone one-line function refused: %v", err)
	}
	if want := "int two(void) { return 2; }\n"; string(res.Output) != want {
		t.Fatalf("output mismatch: %q", res.Output)
	}

	// Two bodies on one line are other code, even when the finder reports only the first one.
	if _, err := RemoveFunction(lc, []byte("int one(void) { return 1; } int two(void) { return 2; }\n"), FuncTarget{Name: "one"}); err == nil || !strings.Contains(err.Error(), "other code") {
		t.Fatalf("two bodies on one line accepted: %v", err)
	}
}

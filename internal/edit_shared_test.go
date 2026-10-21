package internal

import (
	"strings"
	"testing"
)

// A function that shares its first or last line with other code is refused: spans are whole lines, so the edit
// would delete that code too.
func TestEdit_RefusesSharedLines(t *testing.T) {
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	cases := []struct {
		lang, src, name string
	}{
		{"c", "int add(int x) {\n    return x;\n} int b(void) { return 2; }\n", "add"},
		{"c", "int add(int x) { return x; }\nint b(void) { return 2; }\n", "add"},
		{"py", "x = 1\ndef add(a):\n    return a; y = 2\n", "add"},
		{"js", "const a = 1; function add(x) {\n  return x;\n}\n", "add"},
	}
	for _, tc := range cases {
		lc, err := cfg.GetLanguageConfig(tc.lang)
		if err != nil {
			t.Fatalf("config: %v", err)
		}
		if _, err := RemoveFunction(lc, []byte(tc.src), FuncTarget{Name: tc.name}); err == nil || !strings.Contains(err.Error(), "other code") {
			t.Errorf("%s: shared line not refused: %q (%v)", tc.lang, tc.src, err)
		}
		if _, err := ReplaceFunction(lc, []byte(tc.src), FuncTarget{Name: tc.name}, nil, []byte("def add(a):\n    return a\n")); err == nil || !strings.Contains(err.Error(), "other code") {
			t.Errorf("%s: replace on shared line not refused: %v", tc.lang, err)
		}
	}
}

// A comment between a Python header and its only statement does not give the block a body.
func TestRemoveFunction_CommentDoesNotCountAsBody(t *testing.T) {
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	lc, err := cfg.GetLanguageConfig("py")
	if err != nil {
		t.Fatalf("config: %v", err)
	}
	src := "class Demo:\n    # note\n    def add(self):\n        return 1\n"
	if _, err := RemoveFunction(lc, []byte(src), FuncTarget{Name: "add"}); err == nil || !strings.Contains(err.Error(), "without a body") {
		t.Fatalf("comment counted as a body: %v", err)
	}
}

func TestReplaceFunction_CommentDoesNotCountAsBody(t *testing.T) {
	lc := pyConfig(t)
	src := "class Demo:\n    # note\n    def add(self):\n        return 1\n"
	if _, err := ReplaceFunction(lc, []byte(src), FuncTarget{Name: "add"}, nil, []byte("    def add(self):\n")); err == nil {
		t.Fatalf("header without body accepted")
	}
}

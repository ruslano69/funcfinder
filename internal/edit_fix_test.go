package internal

import (
	"strings"
	"testing"
)

// A Python signature that continues over lines: its closing line sits at the def's indentation and is a continuation,
// not a statement of the body.
func TestReplaceFunction_PythonMultiLineSignature(t *testing.T) {
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	lc, err := cfg.GetLanguageConfig("py")
	if err != nil {
		t.Fatalf("config: %v", err)
	}
	src := "class Demo:\n    def add(\n        self, a, b\n    ) -> int:\n        return a + b\n\n    def keep(self):\n        return 0\n"
	repl := "    def add(\n        self, a, b\n    ) -> int:\n        return a * b\n"
	res, err := ReplaceFunction(lc, []byte(src), FuncTarget{Name: "add"}, nil, []byte(repl))
	if err != nil {
		t.Fatalf("multi-line signature refused: %v", err)
	}
	if want := strings.Replace(src, "    def add(\n        self, a, b\n    ) -> int:\n        return a + b\n", repl, 1); string(res.Output) != want {
		t.Fatalf("output mismatch:\n%s", res.Output)
	}
	// A header that never gets its body is still refused.
	if _, err := ReplaceFunction(lc, []byte(src), FuncTarget{Name: "add"}, nil, []byte("    def add(\n        self, a, b\n    ) -> int:\n")); err == nil || !strings.Contains(err.Error(), "no indented body") {
		t.Fatalf("header without body accepted: %v", err)
	}
}

// A bodyless declaration ends with ';'. That is the declaration's end, not other code on its line.
func TestRemoveFunction_JavaBodylessDeclaration(t *testing.T) {
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	lc, err := cfg.GetLanguageConfig("java")
	if err != nil {
		t.Fatalf("config: %v", err)
	}
	src := "public interface Creator {\n  Object create(Type type);\n  void reset();\n}\n"
	res, err := RemoveFunction(lc, []byte(src), FuncTarget{Name: "create"})
	if err != nil {
		t.Fatalf("bodyless declaration refused: %v", err)
	}
	if want := "public interface Creator {\n  void reset();\n}\n"; string(res.Output) != want {
		t.Fatalf("output mismatch:\n%s", res.Output)
	}
}

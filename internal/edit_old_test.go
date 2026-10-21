package internal

import (
	"strings"
	"testing"
)

// --old usually comes from a file written by --extract, which ends with a newline; that must still match.
func TestReplaceFunction_OldTextMayEndWithNewline(t *testing.T) {
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	lc, err := cfg.GetLanguageConfig("go")
	if err != nil {
		t.Fatalf("config: %v", err)
	}
	src := []byte("package demo\n\nfunc Add(a, b int) int {\n\treturn a + b\n}\n")
	old := []byte("func Add(a, b int) int {\n\treturn a + b\n}\n")
	repl := []byte("func Add(a, b int) int {\n\treturn b + a\n}\n")
	if _, err := ReplaceFunction(lc, src, FuncTarget{Name: "Add"}, old, repl); err != nil {
		t.Fatalf("old text with trailing newline refused: %v", err)
	}
	stale := []byte("func Add(a, b int) int {\n\treturn a - b\n}\n")
	if _, err := ReplaceFunction(lc, src, FuncTarget{Name: "Add"}, stale, repl); err == nil || !strings.Contains(err.Error(), "not the expected text") {
		t.Fatalf("different old text accepted: %v", err)
	}
}

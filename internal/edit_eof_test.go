package internal

import (
	"strings"
	"testing"
)

// The Python finder reports the last function of a file one line past its end. Edits of that function must work.
func TestEdit_LastFunctionInFile(t *testing.T) {
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	lc, err := cfg.GetLanguageConfig("py")
	if err != nil {
		t.Fatalf("config: %v", err)
	}
	src := []byte("def add(a, b):\n    return a + b\n")

	res, err := ReplaceFunction(lc, src, FuncTarget{Name: "add"}, nil, []byte("def add(a, b):\n    return a * b\n"))
	if err != nil {
		t.Fatalf("replace the last function: %v", err)
	}
	if want := "def add(a, b):\n    return a * b\n"; string(res.Output) != want {
		t.Fatalf("replace output %q, want %q", res.Output, want)
	}

	res, err = InsertAfterFunction(lc, src, FuncTarget{Name: "add"}, []byte("def sub(a, b):\n    return a - b\n"))
	if err != nil {
		t.Fatalf("insert after the last function: %v", err)
	}
	want := strings.Replace(string(src), "    return a + b\n", "    return a + b\n\ndef sub(a, b):\n    return a - b\n", 1)
	if string(res.Output) != want {
		t.Fatalf("insert output %q, want %q", res.Output, want)
	}
}

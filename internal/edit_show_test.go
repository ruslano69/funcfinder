package internal

import (
	"strings"
	"testing"
)

// The lines NewStart to NewEnd of the output are the lines the edit added, so --show prints the right place.
func TestEdit_NewLinesLocateTheAddedText(t *testing.T) {
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	lc, err := cfg.GetLanguageConfig("go")
	if err != nil {
		t.Fatalf("config: %v", err)
	}
	src := "package demo\n\nfunc Add(a, b int) int {\n\treturn a + b\n}\n\nfunc keep() {}\n"
	outLines := func(out []byte) []string {
		return strings.Split(string(out), "\n")
	}

	repl := "func Add(a, b int) int {\n\treturn b + a\n}"
	res, err := ReplaceFunction(lc, []byte(src), FuncTarget{Name: "Add"}, nil, []byte(repl))
	if err != nil {
		t.Fatalf("replace: %v", err)
	}
	if got := strings.Join(outLines(res.Output)[res.NewStart-1:res.NewEnd], "\n"); got != repl {
		t.Errorf("replace: lines %d-%d hold %q, want %q", res.NewStart, res.NewEnd, got, repl)
	}

	add := "func Sub(a, b int) int {\n\treturn a - b\n}"
	res, err = InsertAfterFunction(lc, []byte(src), FuncTarget{Name: "Add"}, []byte(add))
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	if got := strings.Join(outLines(res.Output)[res.NewStart-1:res.NewEnd], "\n"); got != add {
		t.Errorf("insert: lines %d-%d hold %q, want %q", res.NewStart, res.NewEnd, got, add)
	}
}

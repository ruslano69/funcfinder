package internal

import (
	"strings"
	"testing"
)

func goEditConfig(t *testing.T) *LanguageConfig {
	t.Helper()
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	goCfg, err := cfg.GetLanguageConfig("go")
	if err != nil {
		t.Fatalf("go config: %v", err)
	}
	return goCfg
}

const goEditSource = `package demo

// Add returns the sum.
func Add(a, b int) int {
	return a + b
}

type Store struct{}

func (s *Store) Save() error {
	return nil
}

func (s *Other) Save() error {
	return errOther
}

var errOther = fmt.Errorf("other")
`

func TestReplaceGoFunction_ReplacesOnlyTheSpan(t *testing.T) {
	cfg := goEditConfig(t)
	repl := []byte("func Add(a, b int) int {\n\treturn a + b + 0\n}\n")
	res, err := ReplaceGoFunction(cfg, []byte(goEditSource), GoFuncTarget{Name: "Add"}, nil, repl)
	if err != nil {
		t.Fatalf("ReplaceGoFunction: %v", err)
	}
	if res.Start != 4 || res.End != 6 {
		t.Fatalf("span = %d-%d, want 4-6", res.Start, res.End)
	}
	want := strings.Replace(goEditSource, "\treturn a + b\n}\n", "\treturn a + b + 0\n}\n", 1)
	if string(res.Output) != want {
		t.Fatalf("output mismatch:\n%s", res.Output)
	}
}

func TestReplaceGoFunction_KeepsCRLF(t *testing.T) {
	cfg := goEditConfig(t)
	src := strings.ReplaceAll(goEditSource, "\n", "\r\n")
	res, err := ReplaceGoFunction(cfg, []byte(src), GoFuncTarget{Name: "Add"}, nil, []byte("func Add(a, b int) int {\r\n\treturn b + a\r\n}\r\n"))
	if err != nil {
		t.Fatalf("ReplaceGoFunction: %v", err)
	}
	if strings.Count(string(res.Output), "\r\n") != strings.Count(string(res.Output), "\n") {
		t.Fatalf("output has mixed line endings")
	}
	if !strings.Contains(string(res.Output), "return b + a\r\n") {
		t.Fatalf("replacement not applied")
	}
}

func TestReplaceGoFunction_DuplicateNameNeedsLine(t *testing.T) {
	cfg := goEditConfig(t)
	repl := []byte("func (s *Other) Save() error {\n\treturn nil\n}\n")
	if _, err := ReplaceGoFunction(cfg, []byte(goEditSource), GoFuncTarget{Name: "Save"}, nil, repl); err == nil || !strings.Contains(err.Error(), "start at lines") {
		t.Fatalf("expected ambiguity error, got %v", err)
	}
	res, err := ReplaceGoFunction(cfg, []byte(goEditSource), GoFuncTarget{Name: "Save", Line: 14}, nil, repl)
	if err != nil {
		t.Fatalf("ReplaceGoFunction with line: %v", err)
	}
	if !strings.Contains(string(res.Output), "func (s *Store) Save() error {\n\treturn nil\n}\n") {
		t.Fatalf("the Store method changed; Other was targeted:\n%s", res.Output)
	}
	if !strings.Contains(string(res.Output), "func (s *Other) Save() error {\n\treturn nil\n}\n") {
		t.Fatalf("Other method was not replaced:\n%s", res.Output)
	}
}

func TestReplaceGoFunction_RefusesStaleText(t *testing.T) {
	cfg := goEditConfig(t)
	repl := []byte(`func Add(a, b int) int {
	return 0
}
`)
	stale := []byte(`func Add(a, b int) int {
	return a - b
}`)
	if _, err := ReplaceGoFunction(cfg, []byte(goEditSource), GoFuncTarget{Name: "Add"}, stale, repl); err == nil || !strings.Contains(err.Error(), "not the expected text") {
		t.Fatalf("expected stale-text refusal, got %v", err)
	}
	current := []byte(`func Add(a, b int) int {
	return a + b
}`)
	if _, err := ReplaceGoFunction(cfg, []byte(goEditSource), GoFuncTarget{Name: "Add"}, current, repl); err != nil {
		t.Fatalf("matching old text refused: %v", err)
	}
}

func TestReplaceGoFunction_RefusesInvalidReplacements(t *testing.T) {
	cfg := goEditConfig(t)
	cases := map[string]string{
		"two functions": "func Add() {}\n\nfunc Extra() {}\n",
		"rename":        "func Plus(a, b int) int {\n\treturn a + b\n}\n",
		"not parsing":   "func Add( {\n",
		"adds import":   "import \"os\"\n\nfunc Add() {}\n",
		"var instead":   "var Add = 1\n",
		"empty":         "\n \n",
	}
	for name, repl := range cases {
		if _, err := ReplaceGoFunction(cfg, []byte(goEditSource), GoFuncTarget{Name: "Add"}, nil, []byte(repl)); err == nil {
			t.Errorf("%s: replacement accepted", name)
		}
	}
}

func TestReplaceGoFunction_UnknownTargetAndMixedEndings(t *testing.T) {
	cfg := goEditConfig(t)
	repl := []byte("func Nope() {}\n")
	if _, err := ReplaceGoFunction(cfg, []byte(goEditSource), GoFuncTarget{Name: "Nope"}, nil, repl); err == nil {
		t.Errorf("missing function accepted")
	}
	mixed := strings.Replace(goEditSource, "\n", "\r\n", 3)
	if _, err := ReplaceGoFunction(cfg, []byte(mixed), GoFuncTarget{Name: "Add"}, nil, []byte("func Add() {}\n")); err == nil || !strings.Contains(err.Error(), "mixed") {
		t.Errorf("mixed endings not refused: %v", err)
	}
}

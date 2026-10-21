package internal

import (
	"strings"
	"testing"
)

// removeWant is the source with the function's lines and the blank lines after them removed.
func removeWant(src, old string) string {
	idx := strings.Index(src, old)
	return src[:idx] + strings.TrimLeft(src[idx+len(old)+1:], "\n")
}

func TestRemoveFunction_EveryLanguage(t *testing.T) {
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	for _, fx := range editFixtures {
		t.Run(fx.lang, func(t *testing.T) {
			lc, err := cfg.GetLanguageConfig(fx.lang)
			if err != nil {
				t.Fatalf("config: %v", err)
			}
			res, err := RemoveFunction(lc, []byte(fx.src), FuncTarget{Name: fx.name})
			if err != nil {
				t.Fatalf("RemoveFunction: %v", err)
			}
			if want := removeWant(fx.src, fx.old); string(res.Output) != want {
				t.Fatalf("output mismatch:\n%s\nwant:\n%s", res.Output, want)
			}
		})
	}
}

func TestRemoveFunction_GoAndDecorators(t *testing.T) {
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	lc, err := cfg.GetLanguageConfig("go")
	if err != nil {
		t.Fatalf("config: %v", err)
	}
	src := "package demo\n\nfunc Add(a, b int) int {\n\treturn a + b\n}\n\nfunc keep() {}\n"
	res, err := RemoveFunction(lc, []byte(src), FuncTarget{Name: "Add"})
	if err != nil {
		t.Fatalf("RemoveFunction: %v", err)
	}
	if want := "package demo\n\nfunc keep() {}\n"; string(res.Output) != want {
		t.Fatalf("output mismatch:\n%s", res.Output)
	}

	py, err := cfg.GetLanguageConfig("py")
	if err != nil {
		t.Fatalf("config: %v", err)
	}
	dec := "@staticmethod\ndef add(a, b):\n    return a + b\n\n\ndef keep():\n    return 0\n"
	res, err = RemoveFunction(py, []byte(dec), FuncTarget{Name: "add"})
	if err != nil {
		t.Fatalf("RemoveFunction with decorator: %v", err)
	}
	if want := "def keep():\n    return 0\n"; string(res.Output) != want {
		t.Fatalf("decorator left behind:\n%s", res.Output)
	}
}

// Functions nested in the removed function go with it; the finder must still report the others unchanged.
func TestRemoveFunction_NestedFunctionsGoWithIt(t *testing.T) {
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	lc, err := cfg.GetLanguageConfig("py")
	if err != nil {
		t.Fatalf("config: %v", err)
	}
	src := "def outer():\n    def inner():\n        return 1\n    return inner()\n\n\ndef keep():\n    return 0\n"
	res, err := RemoveFunction(lc, []byte(src), FuncTarget{Name: "outer"})
	if err != nil {
		t.Fatalf("RemoveFunction: %v", err)
	}
	if want := "def keep():\n    return 0\n"; string(res.Output) != want {
		t.Fatalf("output mismatch:\n%s", res.Output)
	}
}

// Removing the last member of a Python class would leave the class without a body.
func TestRemoveFunction_RefusesEmptyPythonBlock(t *testing.T) {
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	lc, err := cfg.GetLanguageConfig("py")
	if err != nil {
		t.Fatalf("config: %v", err)
	}
	src := "class Demo:\n    def add(self):\n        return 1\n"
	if _, err := RemoveFunction(lc, []byte(src), FuncTarget{Name: "add"}); err == nil || !strings.Contains(err.Error(), "without a body") {
		t.Fatalf("empty class accepted: %v", err)
	}
}

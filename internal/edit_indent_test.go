package internal

import (
	"strings"
	"testing"
)

const pyClassSource = "class Demo:\n    def add(self, a, b):\n        return a + b\n\n    def keep(self):\n        return 0\n"

func pyConfig(t *testing.T) *LanguageConfig {
	t.Helper()
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	lc, err := cfg.GetLanguageConfig("py")
	if err != nil {
		t.Fatalf("config: %v", err)
	}
	return lc
}

// A replacement must stay at the function's indentation; otherwise the method leaves its class or its body breaks.
func TestReplaceFunction_PythonIndentation(t *testing.T) {
	lc := pyConfig(t)
	refused := map[string]string{
		"dedented to top level": "def add(self, a, b):\n    return a * b\n",
		"body under the def":    "    def add(self, a, b):\n    return a * b\n",
		"tab instead of spaces": "\tdef add(self, a, b):\n\t\treturn a * b\n",
		"mixed tab and spaces":  "    \tdef add(self, a, b):\n        return a * b\n",
		"body dedented":         "    def add(self, a, b):\n        return a * b\n    return 1\n",
	}
	for name, repl := range refused {
		if _, err := ReplaceFunction(lc, []byte(pyClassSource), FuncTarget{Name: "add"}, nil, []byte(repl)); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}

	ok := "    def add(self, a, b):\n        return a * b\n"
	res, err := ReplaceFunction(lc, []byte(pyClassSource), FuncTarget{Name: "add"}, nil, []byte(ok))
	if err != nil {
		t.Fatalf("correct indentation refused: %v", err)
	}
	if want := strings.Replace(pyClassSource, "    def add(self, a, b):\n        return a + b\n", ok, 1); string(res.Output) != want {
		t.Fatalf("output mismatch:\n%s", res.Output)
	}
}

// A triple-quoted string may hold lines at any column; they are content, not indentation.
func TestReplaceFunction_PythonTripleQuotedContent(t *testing.T) {
	lc := pyConfig(t)
	repl := "    def add(self, a, b):\n        s = \"\"\"\nno indent here\n\"\"\"\n        return s\n"
	if _, err := ReplaceFunction(lc, []byte(pyClassSource), FuncTarget{Name: "add"}, nil, []byte(repl)); err != nil {
		t.Fatalf("string content refused: %v", err)
	}
}

// New methods inserted into a class must sit at the class member indentation.
func TestInsertAfterFunction_PythonIndentation(t *testing.T) {
	lc := pyConfig(t)
	if _, err := InsertAfterFunction(lc, []byte(pyClassSource), FuncTarget{Name: "add"}, []byte("def sub(self, a, b):\n    return a - b\n")); err == nil {
		t.Errorf("column-0 method inserted into a class")
	}
	good := "    def sub(self, a, b):\n        return a - b\n"
	res, err := InsertAfterFunction(lc, []byte(pyClassSource), FuncTarget{Name: "add"}, []byte(good))
	if err != nil {
		t.Fatalf("correct indentation refused: %v", err)
	}
	if want := strings.Replace(pyClassSource, "        return a + b\n", "        return a + b\n\n"+good, 1); string(res.Output) != want {
		t.Fatalf("output mismatch:\n%s", res.Output)
	}
}

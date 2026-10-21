package internal

import (
	"strings"
	"testing"
)

// editFixture is a source with two functions. "name" is the one replaced: old is its exact text and repl the
// replacement with the same name and a changed body.
type editFixture struct {
	lang   string
	src    string
	name   string
	old    string
	repl   string
	braces bool
}

var editFixtures = []editFixture{
	{
		lang:   "c",
		src:    "int add(int a, int b) {\n    return a + b;\n}\n\nint keep(void) {\n    return 0;\n}\n",
		name:   "add",
		old:    "int add(int a, int b) {\n    return a + b;\n}",
		repl:   "int add(int a, int b) {\n    return a + b + 0;\n}",
		braces: true,
	},
	{
		lang:   "cpp",
		src:    "int add(int a, int b) {\n    return a + b;\n}\n\nint keep() {\n    return 0;\n}\n",
		name:   "add",
		old:    "int add(int a, int b) {\n    return a + b;\n}",
		repl:   "int add(int a, int b) {\n    return a + b + 0;\n}",
		braces: true,
	},
	{
		lang:   "d",
		src:    "int add(int a, int b) {\n    return a + b;\n}\n\nint keep() {\n    return 0;\n}\n",
		name:   "add",
		old:    "int add(int a, int b) {\n    return a + b;\n}",
		repl:   "int add(int a, int b) {\n    return a + b + 0;\n}",
		braces: true,
	},
	{
		lang:   "cs",
		src:    "class Demo {\n    int Add(int a, int b) {\n        return a + b;\n    }\n\n    int Keep() {\n        return 0;\n    }\n}\n",
		name:   "Add",
		old:    "    int Add(int a, int b) {\n        return a + b;\n    }",
		repl:   "    int Add(int a, int b) {\n        return a + b + 0;\n    }",
		braces: true,
	},
	{
		lang:   "java",
		src:    "public class Demo {\n    public int add(int a, int b) {\n        return a + b;\n    }\n\n    public int keep() {\n        return 0;\n    }\n}\n",
		name:   "add",
		old:    "    public int add(int a, int b) {\n        return a + b;\n    }",
		repl:   "    public int add(int a, int b) {\n        return a + b + 0;\n    }",
		braces: true,
	},
	{
		lang:   "js",
		src:    "function add(a, b) {\n  return a + b;\n}\n\nfunction keep() {\n  return 0;\n}\n",
		name:   "add",
		old:    "function add(a, b) {\n  return a + b;\n}",
		repl:   "function add(a, b) {\n  return a + b + 0;\n}",
		braces: true,
	},
	{
		lang:   "ts",
		src:    "function add(a: number, b: number): number {\n  return a + b;\n}\n\nfunction keep(): number {\n  return 0;\n}\n",
		name:   "add",
		old:    "function add(a: number, b: number): number {\n  return a + b;\n}",
		repl:   "function add(a: number, b: number): number {\n  return a + b + 0;\n}",
		braces: true,
	},
	{
		lang: "py",
		src:  "def add(a, b):\n    return a + b\n\n\ndef keep():\n    return 0\n",
		name: "add",
		old:  "def add(a, b):\n    return a + b",
		repl: "def add(a, b):\n    return a + b + 0",
	},
	{
		lang:   "rust",
		src:    "fn add(a: i32, b: i32) -> i32 {\n    a + b\n}\n\nfn keep() -> i32 {\n    0\n}\n",
		name:   "add",
		old:    "fn add(a: i32, b: i32) -> i32 {\n    a + b\n}",
		repl:   "fn add(a: i32, b: i32) -> i32 {\n    a + b + 0\n}",
		braces: true,
	},
	{
		lang: "ruby",
		src:  "def add(a, b)\n  a + b\nend\n\ndef keep\n  0\nend\n",
		name: "add",
		old:  "def add(a, b)\n  a + b\nend",
		repl: "def add(a, b)\n  a + b + 0\nend",
	},
	{
		lang:   "php",
		src:    "<?php\n\nfunction add($a, $b) {\n    return $a + $b;\n}\n\nfunction keep() {\n    return 0;\n}\n",
		name:   "add",
		old:    "function add($a, $b) {\n    return $a + $b;\n}",
		repl:   "function add($a, $b) {\n    return $a + $b + 0;\n}",
		braces: true,
	},
	{
		lang:   "kotlin",
		src:    "fun add(a: Int, b: Int): Int {\n    return a + b\n}\n\nfun keep(): Int {\n    return 0\n}\n",
		name:   "add",
		old:    "fun add(a: Int, b: Int): Int {\n    return a + b\n}",
		repl:   "fun add(a: Int, b: Int): Int {\n    return a + b + 0\n}",
		braces: true,
	},
	{
		lang:   "swift",
		src:    "func add(a: Int, b: Int) -> Int {\n    return a + b\n}\n\nfunc keep() -> Int {\n    return 0\n}\n",
		name:   "add",
		old:    "func add(a: Int, b: Int) -> Int {\n    return a + b\n}",
		repl:   "func add(a: Int, b: Int) -> Int {\n    return a + b + 0\n}",
		braces: true,
	},
	{
		lang:   "scala",
		src:    "object Demo {\n  def add(a: Int, b: Int): Int = {\n    a + b\n  }\n\n  def keep(): Int = {\n    0\n  }\n}\n",
		name:   "add",
		old:    "  def add(a: Int, b: Int): Int = {\n    a + b\n  }",
		repl:   "  def add(a: Int, b: Int): Int = {\n    a + b + 0\n  }",
		braces: true,
	},
}

func TestReplaceFunction_EveryLanguage(t *testing.T) {
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
			res, err := ReplaceFunction(lc, []byte(fx.src), FuncTarget{Name: fx.name}, []byte(fx.old), []byte(fx.repl))
			if err != nil {
				t.Fatalf("ReplaceFunction: %v", err)
			}
			want := strings.Replace(fx.src, fx.old, fx.repl, 1)
			if string(res.Output) != want {
				t.Fatalf("output mismatch:\n%s\nwant:\n%s", res.Output, want)
			}
		})
	}
}

func TestReplaceFunction_RefusesRenameAndExtraFunction(t *testing.T) {
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
			renamed := strings.Replace(fx.repl, fx.name, fx.name+"Renamed", 1)
			if _, err := ReplaceFunction(lc, []byte(fx.src), FuncTarget{Name: fx.name}, nil, []byte(renamed)); err == nil {
				t.Errorf("rename accepted")
			}
			twice := fx.repl + "\n\n" + fx.repl
			if _, err := ReplaceFunction(lc, []byte(fx.src), FuncTarget{Name: fx.name}, nil, []byte(twice)); err == nil {
				t.Errorf("second copy of the function accepted")
			}
		})
	}
}

func TestReplaceFunction_RefusesUnbalancedBrackets(t *testing.T) {
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	for _, fx := range editFixtures {
		if !fx.braces {
			continue
		}
		t.Run(fx.lang, func(t *testing.T) {
			lc, err := cfg.GetLanguageConfig(fx.lang)
			if err != nil {
				t.Fatalf("config: %v", err)
			}
			open := strings.Replace(fx.repl, "}", "", 1)
			if _, err := ReplaceFunction(lc, []byte(fx.src), FuncTarget{Name: fx.name}, nil, []byte(open)); err == nil {
				t.Errorf("missing closing brace accepted")
			}
		})
	}
}

func TestReplaceFunction_StaleTextEveryLanguage(t *testing.T) {
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
			stale := strings.Replace(fx.old, fx.name, fx.name+"Old", 1)
			if _, err := ReplaceFunction(lc, []byte(fx.src), FuncTarget{Name: fx.name}, []byte(stale), []byte(fx.repl)); err == nil || !strings.Contains(err.Error(), "not the expected text") {
				t.Errorf("stale text accepted: %v", err)
			}
		})
	}
}

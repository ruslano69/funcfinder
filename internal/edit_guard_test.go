package internal

import (
	"strings"
	"testing"
)

// The header --extract prints, and a markdown fence, are removed before the replacement is checked.
func TestEdit_RemovesPastedHeaderAndFence(t *testing.T) {
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	lc, err := cfg.GetLanguageConfig("go")
	if err != nil {
		t.Fatalf("config: %v", err)
	}
	src := "package demo\n\nfunc Add(a, b int) int {\n\treturn a + b\n}\n"
	body := "func Add(a, b int) int {\n\treturn b + a\n}\n"
	want := "package demo\n\nfunc Add(a, b int) int {\n\treturn b + a\n}\n"
	for name, repl := range map[string]string{
		"header":      "// Add: 3-5\n" + body,
		"fence":       "```go\n" + body + "```\n",
		"both":        "// Add: 3-5\n```go\n" + body + "```\n",
		"plain":       body,
		"header only": "// Add: 3-5\n" + body,
	} {
		res, err := ReplaceFunction(lc, []byte(src), FuncTarget{Name: "Add"}, nil, []byte(repl))
		if err != nil {
			t.Errorf("%s: refused: %v", name, err)
			continue
		}
		if string(res.Output) != want {
			t.Errorf("%s: output %q", name, res.Output)
		}
	}
	// A header naming another function is an ordinary comment: it is kept, not removed.
	res, err := ReplaceFunction(lc, []byte(src), FuncTarget{Name: "Add"}, nil, []byte("// Other: 3-5\n"+body))
	if err != nil || !strings.Contains(string(res.Output), "// Other: 3-5\nfunc Add") {
		t.Errorf("a comment with another name was not kept: %v", err)
	}
	// --old may come straight from --extract, with its header.
	if _, err := ReplaceFunction(lc, []byte(src), FuncTarget{Name: "Add"}, []byte("// Add: 3-5\n"+strings.TrimSuffix(strings.SplitN(src, "\n\n", 2)[1], "\n")), []byte(body)); err != nil {
		t.Errorf("--old with header refused: %v", err)
	}
}

// A file that starts with a byte order mark keeps it, even when the function on line 1 is replaced.
func TestEdit_KeepsByteOrderMark(t *testing.T) {
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	lc, err := cfg.GetLanguageConfig("py")
	if err != nil {
		t.Fatalf("config: %v", err)
	}
	src := "\xef\xbb\xbfdef add(a):\n    return a\n"
	res, err := ReplaceFunction(lc, []byte(src), FuncTarget{Name: "add"}, nil, []byte("def add(a):\n    return a * 2\n"))
	if err != nil {
		t.Fatalf("replace: %v", err)
	}
	if want := "\xef\xbb\xbfdef add(a):\n    return a * 2\n"; string(res.Output) != want {
		t.Fatalf("BOM lost or output changed: %q", res.Output)
	}
}

// The Removed and Added lines drive the preview.
func TestEdit_PreviewLines(t *testing.T) {
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	lc, err := cfg.GetLanguageConfig("go")
	if err != nil {
		t.Fatalf("config: %v", err)
	}
	src := "package demo\n\nfunc Add(a, b int) int {\n\treturn a + b\n}\n"
	res, err := ReplaceFunction(lc, []byte(src), FuncTarget{Name: "Add"}, nil, []byte("func Add(a, b int) int {\n\treturn b + a\n}\n"))
	if err != nil {
		t.Fatalf("replace: %v", err)
	}
	if strings.Join(res.Removed, "|") != "func Add(a, b int) int {|\treturn a + b|}" {
		t.Errorf("removed lines %q", res.Removed)
	}
	if strings.Join(res.Added, "|") != "func Add(a, b int) int {|\treturn b + a|}" {
		t.Errorf("added lines %q", res.Added)
	}
}

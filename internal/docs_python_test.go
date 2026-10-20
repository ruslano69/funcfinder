package internal

import "testing"

// A def or class prints its docstring when it has one: a multi-line docstring is dedented and printed without its quotes,
// a one-line one is printed as its text, and a def without a docstring prints nothing.
func TestFormatDocs_PythonDocstring(t *testing.T) {
	src := "def add(a, b):\n" +
		"    \"\"\"Adds two numbers.\n" +
		"\n" +
		"    Both must be ints.\n" +
		"    \"\"\"\n" +
		"    return a + b\n" +
		"\n" +
		"\n" +
		"class Box:\n" +
		"    '''A box.'''\n" +
		"    pass\n" +
		"\n" +
		"\n" +
		"def nodoc():\n" +
		"    return 1\n"
	path := createTempFile(t, src, "docs_*.py")
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}

	names := map[string]bool{"add": true, "Box": true, "nodoc": true}
	got, err := FormatDocs(cfg, []DirResult{ProcessSingleFile(cfg, path, "py", "all")}, names)
	if err != nil {
		t.Fatal(err)
	}
	want := path + ":1: function add\n" +
		"Adds two numbers.\n" +
		"\n" +
		"Both must be ints.\n" +
		"def add(a, b):\n" +
		"\n" +
		path + ":9: class Box\n" +
		"A box.\n" +
		"class Box:\n" +
		"\n" +
		path + ":14: function nodoc\n" +
		"def nodoc():\n"
	if got != want {
		t.Errorf("FormatDocs =\n%s\nwant\n%s", got, want)
	}
}

// A declaration with no docstring falls back to the comment lines above it, as before.
func TestDeclarationDoc_CommentFallback(t *testing.T) {
	lines := []string{"# note", "def f():", "    x = 1"}
	if got := declarationDoc(lines, 2, langConfigOf(t, "py")); len(got) != 1 || got[0] != "# note" {
		t.Errorf("declarationDoc = %q, want the comment above", got)
	}

	// A body that starts with code has no docstring.
	if got := pythonDocstring(lines, 2); got != nil {
		t.Errorf("pythonDocstring without a docstring = %q, want nil", got)
	}
}

// A multi-line signature is read to its colon, and an f-string opening the body is not a docstring.
func TestPythonDocstring_MultiLineSignatureAndFString(t *testing.T) {
	lines := []string{
		"def run(",
		"    a,",
		"    b):",
		"    \"\"\"Runs it.\"\"\"",
		"    return a",
	}
	if got := pythonDocstring(lines, 1); len(got) != 1 || got[0] != "Runs it." {
		t.Errorf("multi-line signature: pythonDocstring = %q, want [Runs it.]", got)
	}

	fstr := []string{"def g():", "    f\"\"\"not a doc {x}\"\"\""}
	if got := pythonDocstring(fstr, 1); got != nil {
		t.Errorf("f-string: pythonDocstring = %q, want nil", got)
	}
}

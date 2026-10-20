package internal

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// The Windows drive letter stays in the path, a line without text is a hit, and a line that is not a location is skipped.
func TestParseHits(t *testing.T) {
	input := `H:\x\a.go:12:func f() {
b.go:7
not a hit
c.go:3:x:9:y
`
	want := []Hit{
		{Path: `H:\x\a.go`, Line: 12, Text: "func f() {"},
		{Path: "b.go", Line: 7},
		{Path: "c.go", Line: 3, Text: "x:9:y"},
	}
	if got := ParseHits(strings.NewReader(input)); !reflect.DeepEqual(got, want) {
		t.Errorf("ParseHits = %+v\nwant %+v", got, want)
	}
}

// A comment directly above a function documents it: the hit names the function and the comment's lines are extracted.
// A comment separated from the next function by a blank line documents nothing.
func TestEncloseHits_DocComment(t *testing.T) {
	src := "package p\n" + // 1
		"\n" + // 2
		"// Adds two numbers.\n" + // 3
		"// It is pure.\n" + // 4
		"func add() int {\n" + // 5
		"\treturn 1\n" + // 6
		"}\n" + // 7
		"\n" + // 8
		"// Orphan note.\n" + // 9
		"\n" + // 10
		"func other() {}\n" // 11
	root := writeDetectTree(t, map[string]string{"a.go": src})
	file := filepath.Join(root, "a.go")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	got := EncloseHits(cfg, []Hit{{Path: file, Line: 3}, {Path: file, Line: 9}})

	want := []Enclosure{
		{Path: file, Line: 3, Kind: "comment or string", Documents: "add", DocStart: 5, DocEnd: 7,
			Doc: []string{"// Adds two numbers.", "// It is pure."}},
		{Path: file, Line: 9, Kind: "comment or string"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("EncloseHits =\n%+v\nwant\n%+v", got, want)
	}
}

// A Git Bash path converts to the Windows form; any other path is left alone.
func TestMsysPath(t *testing.T) {
	cases := []struct {
		in, out string
		ok      bool
	}{
		{"/h/Ruslan/Code/a.go", "H:/Ruslan/Code/a.go", true},
		{"/C/x", "C:/x", true},
		{"/x", "", false},
		{`H:\Ruslan\a.go`, "", false},
		{"relative/a.go", "", false},
	}
	for _, c := range cases {
		out, ok := msysPath(c.in)
		if out != c.out || ok != c.ok {
			t.Errorf("msysPath(%q) = %q, %v; want %q, %v", c.in, out, ok, c.out, c.ok)
		}
	}
}

// A hit is attributed to the function that holds its line, and the line is classified: code, a comment only, or code
// with a string that contains comment markers. A line above every function is outside any function, and a line past the
// end of the file is unknown.
func TestEncloseHits_GoFunctions(t *testing.T) {
	src := "package p\n" + // 1
		"\n" + // 2
		"func f() int {\n" + // 3
		"\tx := 1\n" + // 4
		"\t// note: y\n" + // 5
		"\treturn x\n" + // 6
		"}\n" + // 7
		"\n" + // 8
		"func g() string {\n" + // 9
		"\treturn \"a // not a comment\"\n" + // 10
		"}\n" // 11
	root := writeDetectTree(t, map[string]string{"a.go": src})
	file := filepath.Join(root, "a.go")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	hits := []Hit{{Path: file, Line: 4}, {Path: file, Line: 5}, {Path: file, Line: 10}, {Path: file, Line: 1}, {Path: file, Line: 999}}
	got := EncloseHits(cfg, hits)

	want := []Enclosure{
		{Path: file, Line: 4, Function: "f", Start: 3, End: 7, Kind: "code"},
		{Path: file, Line: 5, Function: "f", Start: 3, End: 7, Kind: "comment or string"},
		{Path: file, Line: 10, Function: "g", Start: 9, End: 11, Kind: "code with comment or string"},
		{Path: file, Line: 1, Kind: "code"},
		{Path: file, Line: 999, Kind: "unknown"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("EncloseHits =\n%+v\nwant\n%+v", got, want)
	}
}

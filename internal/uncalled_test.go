package internal

import (
	"path/filepath"
	"reflect"
	"testing"
)

// uncalledMain is the main file of the tree. The expectations below use its line numbers: Used is called on line 4,
// register and handler on line 5, Commented is named only in a comment on line 13, Commented is declared on line 14,
// Unused on line 16, and Unused2 is called only from Unused (line 17).
const uncalledMain = "package main\n" +
	"\n" +
	"func main() {\n" +
	"\tUsed()\n" +
	"\tregister(handler)\n" +
	"}\n" +
	"\n" +
	"func Used() {}\n" +
	"\n" +
	"func handler() {}\n" +
	"\n" +
	"func register(f func()) {}\n" +
	"// Commented is only named in this comment.\n" +
	"func Commented() {}\n" +
	"\n" +
	"func Unused() {\n" +
	"\tUnused2()\n" +
	"}\n" +
	"\n" +
	"func Unused2() {}\n"

// goLanguage returns the Go language settings, failing the test when they do not load.
func goLanguage(t *testing.T) *LanguageConfig {
	t.Helper()
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	lc, err := cfg.GetLanguageConfig("go")
	if err != nil {
		t.Fatal(err)
	}
	return lc
}

// A function is a candidate when no code line names it. Used as a value (handler), named in a call (Used, register),
// called only from another uncalled function (Unused2) or named only in a comment (Commented) are the cases that differ.
func TestFindUncalled_GoTree(t *testing.T) {
	root := writeDetectTree(t, map[string]string{
		"main.go": uncalledMain,
		// A test file: its functions are not candidates, and a call from it counts as a use.
		"main_test.go": "package main\n\nimport \"testing\"\n\nfunc TestUnused(t *testing.T) {\n\tUnused2()\n}\n\nfunc helperOnlyForTests() {}\n",
		// An interface method is called by the runtime, so no line names it.
		"t.go": "package main\n\ntype T struct{}\n\nfunc (T) String() string { return \"\" }\n",
	})
	files := []string{
		filepath.Join(root, "main.go"),
		filepath.Join(root, "main_test.go"),
		filepath.Join(root, "t.go"),
	}

	report, err := FindUncalled(goLanguage(t), root, files)
	if err != nil {
		t.Fatal(err)
	}
	// Considered: Used, handler, register, Commented, Unused and Unused2. main, the test functions and String are not.
	if report.Considered != 6 {
		t.Errorf("Considered = %d, want 6", report.Considered)
	}

	want := []UncalledFunction{
		{File: files[0], Line: 14, Name: "Commented", Exported: true},
		{File: files[0], Line: 16, Name: "Unused", Exported: true},
	}
	if !reflect.DeepEqual(report.Candidates, want) {
		t.Errorf("candidates = %+v\nwant %+v", report.Candidates, want)
	}
}

// A call on a declaration's own line counts: `func a() { b() }` uses b, so only a is a candidate.
func TestFindUncalled_CallOnDeclarationLine(t *testing.T) {
	root := writeDetectTree(t, map[string]string{"a.go": "package p\n\nfunc a() { b() }\n\nfunc b() {}\n"})
	file := filepath.Join(root, "a.go")

	report, err := FindUncalled(goLanguage(t), root, []string{file})
	if err != nil {
		t.Fatal(err)
	}
	if report.Considered != 2 {
		t.Errorf("Considered = %d, want 2", report.Considered)
	}
	want := []UncalledFunction{{File: file, Line: 3, Name: "a", Exported: false}}
	if !reflect.DeepEqual(report.Candidates, want) {
		t.Errorf("candidates = %+v, want %+v", report.Candidates, want)
	}
}

// An exported name under an internal directory cannot be imported from outside the module, so it is not marked.
func TestFindUncalled_ExportedUnderInternal(t *testing.T) {
	root := writeDetectTree(t, map[string]string{"internal/x.go": "package internal\n\nfunc Lonely() {}\n"})
	file := filepath.Join(root, "internal", "x.go")

	report, err := FindUncalled(goLanguage(t), root, []string{file})
	if err != nil {
		t.Fatal(err)
	}
	want := []UncalledFunction{{File: file, Line: 3, Name: "Lonely", Exported: false}}
	if !reflect.DeepEqual(report.Candidates, want) {
		t.Errorf("candidates = %+v, want %+v", report.Candidates, want)
	}
}

// Test files are recognised by their names, per language.
func TestIsTestFile(t *testing.T) {
	cases := []struct {
		path, lang string
		want       bool
	}{
		{filepath.Join("pkg", "x_test.go"), "go", true},
		{filepath.Join("pkg", "x.go"), "go", false},
		{filepath.Join("tests", "test_x.py"), "py", true},
		{filepath.Join("pkg", "x_test.py"), "py", true},
		{filepath.Join("pkg", "x.py"), "py", false},
		{filepath.Join("src", "app.test.ts"), "ts", true},
		{filepath.Join("src", "app.spec.js"), "js", true},
		{filepath.Join("src", "app.ts"), "ts", false},
	}
	for _, c := range cases {
		if got := isTestFile(c.path, c.lang); got != c.want {
			t.Errorf("isTestFile(%s, %s) = %v, want %v", c.path, c.lang, got, c.want)
		}
	}
}

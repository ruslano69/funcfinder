package internal

import (
	"reflect"
	"strings"
	"testing"
)

// The comment lines directly above a Go declaration are its doc comment, in file order.
func TestDocLines_GoLineComments(t *testing.T) {
	lines := []string{"package p", "", "// Run starts the server.", "// It blocks.", "func Run() {}"}
	got := DocLines(lines, 5, langConfigOf(t, "go"))
	want := []string{"// Run starts the server.", "// It blocks."}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("DocLines = %q, want %q", got, want)
	}
}

// A blank line ends the comment: a comment separated from the declaration is not its doc.
func TestDocLines_BlankLineEndsComment(t *testing.T) {
	lines := []string{"// Old note.", "", "func X() {}"}
	if got := DocLines(lines, 3, langConfigOf(t, "go")); got != nil {
		t.Errorf("DocLines = %q, want none", got)
	}
}

// Decorators directly above a declaration are skipped: the comment above them is the doc.
func TestDocLines_SkipsDecorator(t *testing.T) {
	lines := []string{"# Adds numbers.", "@staticmethod", "def add(a, b):"}
	got := DocLines(lines, 3, langConfigOf(t, "py"))
	want := []string{"# Adds numbers."}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("DocLines = %q, want %q", got, want)
	}
}

// A block comment that ends on the line above is the doc; only its lines starting with "*" are taken.
func TestDocLines_BlockComment(t *testing.T) {
	lines := []string{"/**", " * Adds.", " * @return sum", " */", "int add(int a)"}
	got := DocLines(lines, 5, langConfigOf(t, "cpp"))
	want := []string{"/**", "* Adds.", "* @return sum", "*/"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("DocLines = %q, want %q", got, want)
	}
}

// A line of code above a block end stops the walk: it is not a block comment.
func TestDocLines_CodeBeforeBlockEndIsNotDoc(t *testing.T) {
	lines := []string{"x := 1", " */", "int add(int a)"}
	if got := DocLines(lines, 3, langConfigOf(t, "cpp")); got != nil {
		t.Errorf("DocLines = %q, want none", got)
	}
}

// --doc prints each named declaration: a header, its doc comment, and its first line; a declaration with no comment
// has no doc lines; a name that matches nothing is not printed.
func TestFormatDocs_GoDeclarations(t *testing.T) {
	src := "package sample\n\n// Run starts the server.\n// It blocks until stop.\nfunc Run() {}\n\ntype Server struct{}\n\nfunc Stop() {}\n"
	path := createTempFile(t, src, "docs_*.go")
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}

	names := map[string]bool{"Run": true, "Server": true, "Stop": true, "Missing": true}
	got, err := FormatDocs(cfg, []DirResult{ProcessSingleFile(cfg, path, "go", "all")}, names)
	if err != nil {
		t.Fatal(err)
	}
	want := path + ":5: function Run\n" +
		"// Run starts the server.\n" +
		"// It blocks until stop.\n" +
		"func Run() {}\n" +
		"\n" +
		path + ":7: struct Server\n" +
		"type Server struct{}\n" +
		"\n" +
		path + ":9: function Stop\n" +
		"func Stop() {}\n"
	if got != want {
		t.Errorf("FormatDocs =\n%s\nwant\n%s", got, want)
	}
}

// No match gives an empty output, which the CLI reports as an error.
func TestFormatDocs_NoMatch(t *testing.T) {
	src := "package sample\n\nfunc Run() {}\n"
	path := createTempFile(t, src, "docs_*.go")
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	got, err := FormatDocs(cfg, []DirResult{ProcessSingleFile(cfg, path, "go", "all")}, map[string]bool{"Missing": true})
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(got) != "" {
		t.Errorf("FormatDocs = %q, want empty", got)
	}
}

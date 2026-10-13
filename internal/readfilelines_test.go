package internal

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// writeTempFile writes content to a file in a test directory and returns its path.
func writeTempFile(t *testing.T, name string, content []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// A minified bundle has lines far longer than bufio.Scanner's default 64 KB token limit. Reading it must not fail.
func TestReadFileLines_LineLongerThan64KB(t *testing.T) {
	long := strings.Repeat("x", 100_000)
	content := []byte("first\n" + long + "\nlast\n")
	path := writeTempFile(t, "bundle.js", content)

	lines, err := readFileLines(path)
	if err != nil {
		t.Fatalf("readFileLines: %v", err)
	}
	want := []string{"first", long, "last"}
	if !reflect.DeepEqual(lines, want) {
		t.Fatalf("got %d lines (first %.20q), want %d lines", len(lines), lines[0], len(want))
	}
}

// The reader keeps the line rules that the scanner had: CR before LF is dropped, an empty line is kept, and a last
// line without a newline is still a line; a file that ends with a newline has no extra empty line.
func TestReadFileLines_LineRules(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    []string
	}{
		{"crlf", "a\r\nb\r\n", []string{"a", "b"}},
		{"empty lines kept", "a\n\nb\n", []string{"a", "", "b"}},
		{"no final newline", "a\nb", []string{"a", "b"}},
		{"trailing blank line", "a\n\n", []string{"a", ""}},
		{"lone CR at end", "a\r", []string{"a"}},
		{"empty file", "", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			path := writeTempFile(t, "f.txt", []byte(c.content))
			got, err := readFileLines(path)
			if err != nil {
				t.Fatal(err)
			}
			if len(got) == 0 && len(c.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, c.want) {
				t.Fatalf("got %q, want %q", got, c.want)
			}
		})
	}
}

// A file with a long line is read by the directory run too, not skipped: its functions are found.
func TestProcessDirectory_LongLineFileIsRead(t *testing.T) {
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	var src bytes.Buffer
	src.WriteString("function before() { return 1; }\n")
	src.WriteString("var blob = \"" + strings.Repeat("y", 90_000) + "\";\n")
	src.WriteString("function after() { return 2; }\n")
	if err := os.WriteFile(filepath.Join(dir, "bundle.js"), src.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}

	dp := NewDirProcessor(cfg, 1, true, false, "functions")
	results, err := dp.ProcessDirectory(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, r := range results {
		if r.Error != nil {
			t.Fatalf("%s: %v", r.Path, r.Error)
		}
		for _, fn := range r.Functions {
			names = append(names, fn.Name)
		}
	}
	want := []string{"before", "after"}
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("functions = %v, want %v", names, want)
	}
}

// A file that cannot be processed is reported on stderr with its path, not dropped without a word.
func TestReportFileErrors_NamesTheFile(t *testing.T) {
	var out bytes.Buffer
	results := []DirResult{
		{Path: "ok.go"},
		{Path: "broken.js", Error: os.ErrPermission},
	}
	reportFileErrors(&out, results)
	got := out.String()
	if !strings.Contains(got, "broken.js") || strings.Contains(got, "ok.go") {
		t.Fatalf("warnings = %q, want one line naming broken.js only", got)
	}
}

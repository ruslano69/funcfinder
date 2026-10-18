package internal

import (
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
	"unicode"
)

// TestDStatementLinesNeverMatchFuncHeader guards the skip in prepareDLines: a statement line has its first character
// replaced with '?', and the function pattern is then not run on it. That is exact only while no function header can
// start with '?'. The check covers every line of the D fixtures and a line per statement keyword, with the
// replacement prepareDLines makes.
func TestDStatementLinesNeverMatchFuncHeader(t *testing.T) {
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	lc, err := cfg.GetLanguageConfig("d")
	if err != nil {
		t.Fatal(err)
	}
	var lines []string
	for _, kw := range []string{"return", "throw", "new", "delete", "if", "else", "for", "foreach", "while", "switch",
		"try", "catch", "with", "assert", "version", "debug", "scope", "mixin", "pragma", "cast"} {
		lines = append(lines,
			kw+" (a) int f(int x) {", "    "+kw+" foo(bar(baz));", "static "+kw+" void g()", kw+"(Windows) void h() {",
			"final "+kw+" T k(T)(T v)")
	}
	err = filepath.WalkDir("..", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if name := d.Name(); name == "build" || name == ".git" || name == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, ".d") {
			fileLines, err := readFileLines(path)
			if err != nil {
				return err
			}
			prepared, _ := prepareDLines(fileLines, lc)
			lines = append(lines, fileLines...)
			lines = append(lines, prepared...)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, line := range lines {
		text := strings.TrimSpace(line)
		if dTypeHeader.MatchString(text) || !dStatementHeader.MatchString(text) {
			continue
		}
		code := []rune(line)
		for j, ch := range code {
			if !unicode.IsSpace(ch) {
				code[j] = '?'
				break
			}
		}
		checked++
		if dFuncHeader(string(code), lc) {
			t.Errorf("statement line matches the function pattern after the '?' replacement: %q", string(code))
		}
	}
	if checked < 100 {
		t.Fatalf("only %d statement lines checked: the fixtures are missing", checked)
	}
}

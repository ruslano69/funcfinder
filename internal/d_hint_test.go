package internal

import (
	"io/fs"
	"math/rand"
	"path/filepath"
	"strings"
	"testing"
)

// dFixtureLines returns the lines of every D file in the repository (build/, .git and node_modules skipped).
func dFixtureLines(t *testing.T) [][]string {
	t.Helper()
	var files [][]string
	err := filepath.WalkDir("..", func(path string, d fs.DirEntry, err error) error {
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
			lines, err := readFileLines(path)
			if err != nil {
				return err
			}
			files = append(files, lines)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no D fixtures found")
	}
	return files
}

// TestDFuncHintMatchesPattern checks prepareDLinesHinted: wherever a line's hint is known, it is what the function
// pattern gives on that prepared line, the text the nested scan reads. A wrong "no match" would hide a function.
func TestDFuncHintMatchesPattern(t *testing.T) {
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	lc, err := cfg.GetLanguageConfig("d")
	if err != nil {
		t.Fatal(err)
	}
	known, matched := 0, 0
	for _, lines := range dFixtureLines(t) {
		prepared, ends, hints := prepareDLinesHinted(lines, lc)
		plain, plainEnds := prepareDLines(lines, lc)
		if strings.Join(prepared, "\n") != strings.Join(plain, "\n") || len(ends) != len(plainEnds) {
			t.Fatal("prepareDLinesHinted and prepareDLines prepare different text")
		}
		for i, h := range hints {
			if h == dHintUnknown {
				continue
			}
			known++
			got := lc.FuncRegex().MatchString(prepared[i])
			if got {
				matched++
			}
			if got != (h == dHintMatch) {
				t.Errorf("hint %d, pattern match %v on prepared line %q", h, got, prepared[i])
			}
		}
	}
	if known < 300 || matched == 0 {
		t.Fatalf("%d known hints, %d matches: the fixtures do not exercise the hint", known, matched)
	}
}

// TestDAttributeGateNeverHidesAMatch checks dAttributeLineMatch's first-character gate: a line it rejects never
// matches dAttributeLine. It covers every D fixture line, raw and prepared, each word of the line, and random lines.
func TestDAttributeGateNeverHidesAMatch(t *testing.T) {
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	lc, err := cfg.GetLanguageConfig("d")
	if err != nil {
		t.Fatal(err)
	}
	var lines []string
	for _, file := range dFixtureLines(t) {
		prepared, _ := prepareDLines(file, lc)
		lines = append(lines, file...)
		lines = append(lines, prepared...)
		for _, l := range file {
			lines = append(lines, strings.Fields(l)...)
		}
	}
	words := []string{"@safe", "@nogc", "public", "private", "protected", "export", "pure", "nothrow", "const",
		"immutable", "shared", "inout", "ref", "scope", " ", "\t", "\f", "\r", "x", "static", "auto", "@", "_"}
	rng := rand.New(rand.NewSource(1))
	for k := 0; k < 20000; k++ {
		var b strings.Builder
		for n := rng.Intn(5); n >= 0; n-- {
			b.WriteString(words[rng.Intn(len(words))])
			if rng.Intn(2) == 0 {
				b.WriteString(" ")
			}
		}
		lines = append(lines, b.String())
	}
	matched := 0
	for _, l := range lines {
		want := dAttributeLine.MatchString(l)
		if want {
			matched++
		}
		if got := dAttributeLineMatch(l); got != want {
			t.Errorf("dAttributeLineMatch(%q) = %v, pattern gives %v", l, got, want)
		}
	}
	if matched == 0 {
		t.Fatal("no attribute line matched: the test checks nothing")
	}
}

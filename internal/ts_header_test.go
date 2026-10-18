package internal

import (
	"io/fs"
	"math/rand"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// TestTSHeaderScannerPatternIsCurrent fails when languages.json changes the TS function pattern: the scanner stands
// for tsFuncPatternSource only, and must be updated with it.
func TestTSHeaderScannerPatternIsCurrent(t *testing.T) {
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	lc, err := cfg.GetLanguageConfig("ts")
	if err != nil {
		t.Fatal(err)
	}
	if lc.FuncPattern != tsFuncPatternSource || !tsUsesHeaderScanner(lc) {
		t.Fatal("the TS func_pattern in languages.json differs from tsFuncPatternSource: update tsHeaderSubmatch and the constant together")
	}
}

var tsHeaderTokens = []string{
	" ", "  ", "\t", "\f", "\r", "\v", "\n",
	"export", "default", "async", "function", "function*", "*", "const", "let", "var", "public", "private",
	"protected", "static", "override", "readonly", "abstract", "get", "set", "constructor",
	"foo", "bar", "x1", "_", "9", "é", "ж", "\x80", "$", "#",
	"(", ")", "()", "(a: T)", "((a))", "(((a)))", "<", ">", "<T>", "<T, U>", "=", "=>", ":", ": T", ": (a) => T",
	"{", "}", ";", ",", "\"", "'", "?", ".", "[", "]",
}

func randomTSHeaderLine(rng *rand.Rand) string {
	var b strings.Builder
	for n := 1 + rng.Intn(10); n > 0; n-- {
		b.WriteString(tsHeaderTokens[rng.Intn(len(tsHeaderTokens))])
		if rng.Intn(3) == 0 {
			b.WriteByte(' ')
		}
	}
	return b.String()
}

// structuredTSHeader builds a line shaped like one of the three TS header forms, with near misses at every part.
func structuredTSHeader(r *rand.Rand) string {
	var b strings.Builder
	b.WriteString(dPick(r, "", " ", "\t", "  "))
	switch r.Intn(3) {
	case 0:
		for _, w := range []string{"export", "default", "async"} {
			if r.Intn(2) == 0 {
				b.WriteString(w + dPick(r, " ", "  ", "\t", ""))
			}
		}
		b.WriteString(dPick(r, "function", "function ", "function*", "function *", "function* ", "func"))
		b.WriteString(dPick(r, " ", "", "  ", "\t"))
		b.WriteString(dPick(r, "foo", "bar9", "é", "", "$x", "get"))
		b.WriteString(dPick(r, "(", " (", "<T>(", "<", " ", "", "x"))
	case 1:
		if r.Intn(2) == 0 {
			b.WriteString("export" + dPick(r, " ", ""))
		}
		b.WriteString(dPick(r, "const", "let", "var", "conster"))
		b.WriteString(dPick(r, " ", "", "  "))
		b.WriteString(dPick(r, "f", "handler", "é", ""))
		b.WriteString(dPick(r, "", " "))
		if r.Intn(2) == 0 {
			b.WriteString(dPick(r, ":", ": ", ":  "))
			for n := r.Intn(4); n > 0; n-- {
				b.WriteString(dPick(r, "T", "(a: A) => B", "=>", " ", "==", "<T>", "{ a: string }", "=", "é"))
			}
		}
		b.WriteString(dPick(r, "=", " = ", "= ", "==", ""))
		b.WriteString(dPick(r, "", "async", "async ", " async"))
		b.WriteString(dPick(r, "(", "<", " (", "x", "", "()"))
	default:
		for n := r.Intn(4); n > 0; n-- {
			b.WriteString(dPick(r, "public", "static", "async", "get", "set", "readonly", "abstract", "publicx"))
			b.WriteString(dPick(r, " ", "", "  "))
		}
		b.WriteString(dPick(r, "", "*", "* ", " *"))
		b.WriteString(dPick(r, "foo", "get", "set", "é", "", "static"))
		b.WriteString(dPick(r, "", " ", "<T>", "<T", " <T, U> "))
		b.WriteString(dPick(r, "()", "(a: T)", "((a))", "(((a)))", "(", " ()", "(a, (b))"))
		b.WriteString(dPick(r, "", " ", ": T", ": ", ":", ": Promise<void>", ": a;b", ": {", ":{"))
		b.WriteString(dPick(r, " {", "{", "", ";", " { return 1; }", "  {"))
	}
	return b.String()
}

// TestTSHeaderScannerMatchesPattern compares tsHeaderSubmatch with FindStringSubmatch of the TS function pattern, all
// eight results, on TS fixture lines (raw, cleaned, and every suffix of a cleaned line, as the inline-header search
// reads them) and on generated lines.
func TestTSHeaderScannerMatchesPattern(t *testing.T) {
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	lc, err := cfg.GetLanguageConfig("ts")
	if err != nil {
		t.Fatal(err)
	}
	var lines []string
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
		if cfg.GetLanguageByExtension(path) != lc {
			return nil
		}
		raw, err := readFileLines(path)
		if err != nil {
			return err
		}
		lines = append(lines, raw...)
		for _, cleaned := range NewSanitizer(lc, false).CleanLines(raw) {
			for i := 0; i < len(cleaned); i++ {
				lines = append(lines, cleaned[i:])
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) < 1000 {
		t.Fatalf("only %d TS fixture lines", len(lines))
	}
	rng := rand.New(rand.NewSource(1))
	for k := 0; k < 300000; k++ {
		lines = append(lines, randomTSHeaderLine(rng))
	}
	for k := 0; k < 300000; k++ {
		lines = append(lines, structuredTSHeader(rng))
	}
	re := lc.FuncRegex()
	matched := map[int]int{}
	for _, line := range lines {
		want := re.FindStringSubmatch(line)
		got := tsHeaderSubmatch(line)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("tsHeaderSubmatch(%q) = %q, pattern gives %q", line, got, want)
		}
		for g := 1; g < len(want); g++ {
			if want[g] != "" {
				matched[g]++
			}
		}
	}
	for g := 1; g <= 7; g++ {
		if matched[g] == 0 {
			t.Errorf("no line set group %d: the corpus does not exercise it", g)
		}
	}
}

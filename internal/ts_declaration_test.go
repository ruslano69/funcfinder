package internal

import (
	"io/fs"
	"math/rand"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// TestTSDeclarationScannerPatternsAreCurrent fails when languages.json changes the TS declaration or declarator pattern:
// the scanners stand for the constants only, and must be updated with them.
func TestTSDeclarationScannerPatternsAreCurrent(t *testing.T) {
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	lc, err := cfg.GetLanguageConfig("ts")
	if err != nil {
		t.Fatal(err)
	}
	if !tsUsesDeclarationScanner(lc) {
		t.Fatal("the TS declaration_pattern in languages.json differs from tsDeclarationPatternSource: update tsDeclarationSubmatch and the constant together")
	}
	if !tsUsesDeclaratorScanner(lc) {
		t.Fatal("the TS declarator_pattern in languages.json differs from tsDeclaratorPatternSource: update tsDeclaratorSubmatch and the constant together")
	}
	if js, _ := cfg.GetLanguageConfig("js"); tsUsesDeclarationScanner(js) || tsUsesDeclaratorScanner(js) {
		t.Fatal("the TS scanners must not stand for another language's patterns")
	}
}

var tsDeclarationTokens = []string{
	" ", "  ", "\t", "\f", "\r", "\n", "\v",
	"export", "default", "declare", "public", "protected", "private", "static", "readonly", "override", "async",
	"function", "abstract", "get", "set", "new",
	"foo", "bar", "x1", "_", "9", "é", "ж", "\x80", "$", "#",
	"?", "(", ")", "()", "(a: T)", "((a))", "(((a)))", "(a, (b), c)", "<", ">", "<T>", "<T, U>", "<T extends (A)>", "=>", "=",
	":", ": T", ": void", ";", "; ", ";;", " ;", "{", "}", ",", ".", "[", "]", "\"", "'",
}

func randomTSDeclarationLine(rng *rand.Rand) string {
	var b strings.Builder
	for n := 1 + rng.Intn(12); n > 0; n-- {
		b.WriteString(tsDeclarationTokens[rng.Intn(len(tsDeclarationTokens))])
		if rng.Intn(3) == 0 {
			b.WriteByte(' ')
		}
	}
	return b.String()
}

// structuredTSDeclaration builds a line shaped like a bodyless TS signature, with near misses at every part.
func structuredTSDeclaration(r *rand.Rand) string {
	var b strings.Builder
	b.WriteString(dPick(r, "", " ", "\t", "  ", "\n"))
	for n := r.Intn(4); n > 0; n-- {
		b.WriteString(dPick(r, "export", "default", "declare", "public", "protected", "private", "static", "readonly",
			"override", "async", "function", "publicx", "abstract"))
		b.WriteString(dPick(r, " ", "  ", "\t", "", "\n"))
	}
	if r.Intn(3) == 0 {
		b.WriteString("abstract" + dPick(r, " ", "", "  ", "\t"))
		if r.Intn(2) == 0 {
			b.WriteString("async" + dPick(r, " ", "", "  "))
		}
	}
	b.WriteString(dPick(r, "foo", "get", "async", "abstract", "static", "é", "$x", "bar9", "function", ""))
	b.WriteString(dPick(r, "", "?", "??", " ", " ?"))
	b.WriteString(dPick(r, "", " ", "<T>", " <T>", "<T, U>", "<T extends (A)>", "<T", "<(T)>", "<T> ", "<T>>", "<<T>", "<T>x>"))
	b.WriteString(dPick(r, "(", " (", "(", "", "x", "\t("))
	b.WriteString(dPick(r, ")", "a)", "a: T)", "(a))", "(a)", "(a, (b))", "((a))", "(((a)))", "a, b)", "a: (b, c))", "(a", "a) b)"))
	b.WriteString(dPick(r, "", " ", ": void", " : T", ":", ": ", ": T;", ":;", ": a; b", ": {", ": T ", " \n: T", ": a\nb"))
	b.WriteString(dPick(r, ";", " ;", "; ", ";  ", ";\n", "", ";;", "}", "; // c", ";\t\n"))
	return b.String()
}

var tsDeclaratorTokens = []string{
	" ", "  ", "\t", "\f", "\r", "\n",
	"async", "foo", "x1", "_", "é", "\x80", "$",
	"=", "==", "=>", ":", ": T", ": (a) => T", "<", ">", "<T>", "(", ")", "()", ",", ";", "{", "}", "[", "]", "?", ".", "\"",
}

func randomTSDeclaratorLine(rng *rand.Rand) string {
	var b strings.Builder
	for n := 1 + rng.Intn(10); n > 0; n-- {
		b.WriteString(tsDeclaratorTokens[rng.Intn(len(tsDeclaratorTokens))])
		if rng.Intn(3) == 0 {
			b.WriteByte(' ')
		}
	}
	return b.String()
}

// structuredTSDeclarator builds a line shaped like a later declarator of a const list, with near misses at every part.
func structuredTSDeclarator(r *rand.Rand) string {
	var b strings.Builder
	b.WriteString(dPick(r, "", "", " ", "\t", "  "))
	b.WriteString(dPick(r, "f", "handler", "é", "async", "x$", "", "9"))
	b.WriteString(dPick(r, "", " ", "  "))
	if r.Intn(2) == 0 {
		b.WriteString(dPick(r, ":", ": ", ":  "))
		for n := r.Intn(4); n > 0; n-- {
			b.WriteString(dPick(r, "T", "(a: A) => B", "=>", " ", "==", "<T>", "{ a: string }", "=", "é", "\n"))
		}
	}
	b.WriteString(dPick(r, "=", " = ", "= ", "==", "", "=>"))
	b.WriteString(dPick(r, "", "async", "async ", " async", "asyncx", "async\t"))
	b.WriteString(dPick(r, "(", "<", " (", "x", "", "()", "[", "{"))
	b.WriteString(dPick(r, "", " ,", ") => 1", ")", " b = 2"))
	return b.String()
}

// TestTSDeclarationScannersMatchPatterns compares tsDeclarationSubmatch and tsDeclaratorSubmatch with the regular
// expressions on TS and JS fixture lines (raw, cleaned, and every suffix of a cleaned line, as the inline-header search
// reads them) and on generated lines: every group, and nil against nil.
func TestTSDeclarationScannersMatchPatterns(t *testing.T) {
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
		fileLang := cfg.GetLanguageByExtension(path)
		if fileLang == nil || (fileLang.LangKey != "ts" && fileLang.LangKey != "js") {
			return nil
		}
		raw, err := readFileLines(path)
		if err != nil {
			return err
		}
		lines = append(lines, raw...)
		for _, cleaned := range NewSanitizer(fileLang, false).CleanLines(raw) {
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
		t.Fatalf("only %d TS/JS fixture lines", len(lines))
	}
	rng := rand.New(rand.NewSource(1))
	for k := 0; k < 300000; k++ {
		lines = append(lines, randomTSDeclarationLine(rng), structuredTSDeclaration(rng), randomTSDeclaratorLine(rng), structuredTSDeclarator(rng))
	}
	declRe, declaratorRe := lc.DeclarationRegex(), lc.DeclaratorRegex()
	groups := map[int]int{}
	declarators := 0
	for _, line := range lines {
		want := declRe.FindStringSubmatch(line)
		got := tsDeclarationSubmatch(line)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("tsDeclarationSubmatch(%q) = %q, pattern gives %q", line, got, want)
		}
		for g := 1; g < len(want); g++ {
			if want[g] != "" {
				groups[g]++
			}
		}
		want = declaratorRe.FindStringSubmatch(line)
		got = tsDeclaratorSubmatch(line)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("tsDeclaratorSubmatch(%q) = %q, pattern gives %q", line, got, want)
		}
		if want != nil {
			declarators++
		}
	}
	for g := 1; g <= 2; g++ {
		if groups[g] == 0 {
			t.Errorf("no line set declaration group %d: the corpus does not exercise it", g)
		}
	}
	if declarators == 0 {
		t.Error("no line matched the declarator pattern: the corpus does not exercise it")
	}
	t.Logf("%d lines; declaration groups %v; %d declarator matches", len(lines), groups, declarators)
}

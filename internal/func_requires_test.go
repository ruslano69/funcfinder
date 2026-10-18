package internal

import (
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

// func_requires must never hide a header. Over every fixture and source file, each cleaned line that the gate skips
// must match neither the function pattern nor the declarator pattern (the inline search runs both).
func TestFuncRequiresNeverHidesAHeader(t *testing.T) {
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	skipped := 0
	err = filepath.WalkDir("..", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if name == "build" || name == "node_modules" || name == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		lc := cfg.GetLanguageByExtension(path)
		if lc == nil || len(lc.FuncRequires) == 0 {
			return nil
		}
		lines, err := readFileLines(path)
		if err != nil {
			return err
		}
		for i, cleaned := range NewSanitizer(lc, false).CleanLines(lines) {
			if lc.MayStartFunction(cleaned) {
				continue
			}
			skipped++
			if lc.FuncRegex().MatchString(cleaned) {
				t.Errorf("%s:%d: the gate skipped a line that the function pattern matches: %q", path, i+1, cleaned)
			}
			if re := lc.DeclaratorRegex(); re != nil && re.MatchString(cleaned) {
				t.Errorf("%s:%d: the gate skipped a line that the declarator pattern matches: %q", path, i+1, cleaned)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if skipped == 0 {
		t.Fatal("no line was skipped: the test checks nothing")
	}
}

// TestMayDeclareNeverHidesADeclaration checks the declaration gate on every fixture line of the languages with a
// declaration_pattern: a line that MayDeclare skips must not match the pattern.
func TestMayDeclareNeverHidesADeclaration(t *testing.T) {
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	skipped, checked := 0, 0
	err = filepath.WalkDir("..", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if name == "build" || name == "node_modules" || name == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		lc := cfg.GetLanguageByExtension(path)
		if lc == nil || lc.DeclarationRegex() == nil {
			return nil
		}
		lines, err := readFileLines(path)
		if err != nil {
			return err
		}
		for i, cleaned := range NewSanitizer(lc, false).CleanLines(lines) {
			checked++
			if lc.MayDeclare(cleaned) {
				continue
			}
			skipped++
			if lc.DeclarationRegex().MatchString(cleaned) {
				t.Errorf("%s:%d: the gate skipped a line that the declaration pattern matches: %q", path, i+1, cleaned)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if skipped == 0 || checked == skipped {
		t.Fatalf("checked %d lines, skipped %d: the test checks nothing", checked, skipped)
	}
}

// TestMayQuoteMethodNeverHidesAMatch checks the quoted-method gate: every quoted_method_pattern starts with `^\s*` and
// a quote, and on every fixture line of those languages (and generated lines) a line the gate rejects never matches.
func TestMayQuoteMethodNeverHidesAMatch(t *testing.T) {
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	var langs []*LanguageConfig
	for _, lc := range cfg {
		if lc.QuotedMethodRegex() == nil {
			continue
		}
		if !strings.HasPrefix(lc.QuotedMethodPattern, `^\s*((?:"`) {
			t.Fatalf("%s: quoted_method_pattern no longer starts with whitespace and a quote; update MayQuoteMethod", lc.LangKey)
		}
		langs = append(langs, lc)
	}
	if len(langs) == 0 {
		t.Fatal("no language has a quoted_method_pattern")
	}
	var lines []string
	err = filepath.WalkDir("..", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if name := d.Name(); name == "build" || name == "node_modules" || name == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		for _, lc := range langs {
			if cfg.GetLanguageByExtension(path) == lc {
				file, err := readFileLines(path)
				if err != nil {
					return err
				}
				lines = append(lines, file...)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{`"`, `'`, ` "`, "\t'", "\v\"", `x"`, ""} {
		for _, rest := range []string{`a"() {`, `a'(x): T {`, `"() {`, `() {`, `b"(c) : {`} {
			lines = append(lines, q+rest)
		}
	}
	matched := 0
	for _, lc := range langs {
		for _, l := range lines {
			want := lc.QuotedMethodRegex().MatchString(l)
			if want {
				matched++
			}
			if want && !lc.MayQuoteMethod(l) {
				t.Errorf("%s: the gate rejects %q, which the quoted-method pattern matches", lc.LangKey, l)
			}
		}
	}
	if matched == 0 {
		t.Fatal("no line matched a quoted-method pattern: the test checks nothing")
	}
}

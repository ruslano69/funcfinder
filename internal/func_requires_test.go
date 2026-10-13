package internal

import (
	"io/fs"
	"path/filepath"
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

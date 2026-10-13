package internal

import (
	"io/fs"
	"path/filepath"
	"testing"
)

// type_keywords must never hide a type. Over every fixture and source file, each cleaned line that the gate skips
// must match neither the class pattern nor any struct pattern.
func TestTypeKeywordsNeverHideAType(t *testing.T) {
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
		if lc == nil || len(lc.TypeKeywords) == 0 {
			return nil
		}
		lines, err := readFileLines(path)
		if err != nil {
			return err
		}
		for i, cleaned := range NewSanitizer(lc, false).CleanLines(lines) {
			if lc.MayStartType(cleaned) {
				continue
			}
			skipped++
			if re := lc.ClassRegex(); re != nil && re.MatchString(cleaned) {
				t.Errorf("%s:%d: the gate skipped a line that the class pattern matches: %q", path, i+1, cleaned)
			}
			for kind, re := range lc.GetStructPatterns() {
				if re.MatchString(cleaned) {
					t.Errorf("%s:%d: the gate skipped a line that the %s pattern matches: %q", path, i+1, kind, cleaned)
				}
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

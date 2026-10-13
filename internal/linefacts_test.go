package internal

import (
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

// LineFacts replace the inputs the decisions computed before: the cleaned line from CleanLine in sequence, the state
// a line starts in, GetIndentLevel and hasCodeText of the raw line. Over every fixture and source file, each field
// must equal the old computation, line by line.
func TestLineFactsMatchPreviousInputs(t *testing.T) {
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
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
		if lc == nil {
			return nil
		}
		lines, err := readFileLines(path)
		if err != nil {
			return err
		}

		facts := NewSanitizer(lc, false).CleanLinesFacts(lines)
		if len(facts) != len(lines) {
			t.Fatalf("%s: %d facts for %d lines", path, len(facts), len(lines))
		}

		// the previous computation, in its own sanitizer so that the two passes do not share state
		old := NewSanitizer(lc, false)
		state := StateNormal
		for i, raw := range lines {
			start := state
			var cleaned string
			cleaned, state = old.CleanLine(raw, start)

			f := facts[i]
			if f.StartState != start {
				t.Fatalf("%s:%d: StartState = %v, want %v", path, i+1, f.StartState, start)
			}
			if f.Cleaned != cleaned {
				t.Fatalf("%s:%d: Cleaned = %q, want %q", path, i+1, f.Cleaned, cleaned)
			}
			if f.Indent != GetIndentLevel(raw) {
				t.Fatalf("%s:%d: Indent = %d, want %d (the raw line's)", path, i+1, f.Indent, GetIndentLevel(raw))
			}
			// the previous rule, and the comment rule: a line that starts outside a literal with the line-comment marker
			wantCode := hasCodeText(raw)
			if start == StateNormal && lc.LineComment != "" && strings.HasPrefix(strings.TrimSpace(raw), lc.LineComment) {
				wantCode = false
			}
			if f.CodeText != wantCode {
				t.Fatalf("%s:%d: CodeText = %v, want %v", path, i+1, f.CodeText, wantCode)
			}
			checked++
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if checked == 0 {
		t.Fatal("no line was checked")
	}
}

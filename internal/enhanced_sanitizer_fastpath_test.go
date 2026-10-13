package internal

import (
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

// CleanLine returns early for lines no literal or comment can start on. Over every fixture and source file, each
// line must clean exactly as the rune-by-rune cleaner does, in the state the file is really in.
func TestCleanLineFastPathMatchesSlowPath(t *testing.T) {
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
		// Two sanitizers run the same file in step: each keeps its own delimiter state, so a call on one does not
		// change what the other sees.
		fastS, slowS := NewSanitizer(lc, false), NewSanitizer(lc, false)
		fastState, slowState := StateNormal, StateNormal
		for i, line := range lines {
			var fast, slow string
			fast, fastState = fastS.CleanLine(line, fastState)
			slow, slowState = slowS.cleanLineSlow(line, slowState)
			if fast != slow || fastState != slowState {
				t.Fatalf("%s:%d: fast %q/%v, slow %q/%v (line %q)", path, i+1, fast, fastState, slow, slowState, line)
			}
			checked++
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if checked == 0 {
		t.Fatal("no lines were checked")
	}

	// Invalid UTF-8 is not a valid line: the fast path must not return the bytes as they are.
	lc := cfg["go"]
	s := NewSanitizer(lc, false)
	bad := "x := \"a\" \xff // c"
	fast, _ := s.CleanLine(bad, StateNormal)
	slow, _ := s.cleanLineSlow(bad, StateNormal)
	if fast != slow {
		t.Errorf("invalid UTF-8 line: fast %q, slow %q", fast, slow)
	}
	if strings.Contains(fast, "\xff") {
		t.Errorf("invalid UTF-8 byte survived cleaning: %q", fast)
	}
}

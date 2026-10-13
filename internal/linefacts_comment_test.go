package internal

import "testing"

// A line that is a comment (`# note` in Python and Ruby) holds no code text. A `#` inside a string that starts on an
// earlier line is text of the literal: that line keeps its code text.
func TestLineFactsHashCommentHasNoCodeText(t *testing.T) {
	cfg := langConfigOf(t, "py")
	lines := []string{
		"# note",
		"x = '''",
		"# inside the string",
		"'''",
		"y = 1",
	}
	facts := NewSanitizer(cfg, false).CleanLinesFacts(lines)

	if facts[0].CodeText {
		t.Errorf("line 1 %q: CodeText = true, want false (a comment line)", lines[0])
	}
	if facts[2].StartState == StateNormal {
		t.Fatalf("line 3 starts in Normal state: the test does not reach the string case")
	}
	if !facts[2].CodeText {
		t.Errorf("line 3 %q inside a string: CodeText = false, want true (the text of the literal)", lines[2])
	}
	if !facts[4].CodeText {
		t.Errorf("line 5 %q: CodeText = false, want true", lines[4])
	}
}

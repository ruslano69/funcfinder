package internal

import (
	"strings"
	"testing"
)

// A raw string opened as r" (Python, D), r#" (Rust) or @" (C# verbatim) closes at its own delimiter, and a triple-quoted
// string with a prefix (r""") is a docstring: its quotes inside are text. Before, the raw string's closing was looked for
// as its opener, so the string never closed, or closed at the second quote of a triple, and the lines after it were
// misread.
func TestSanitizerRawStringsCloseAtTheirDelimiter(t *testing.T) {
	cases := []struct {
		lang, src, after string
	}{
		{"py", "x = r\"a\tb\" + \"c\"\nz = 2", "z = 2"},
		{"py", "h = r'^## (\\S+)$'\nz = 2", "z = 2"},
		{"py", "t = r\"\"\"a \"quoted\" b\"\"\"\nz = 2", "z = 2"},
		{"py", "t = r\"\"\"\nx \"y\"\n\"\"\"\nz = 2", "z = 2"},
		{"cs", "var s = @\"a\"\"b\";\nint y = 1;", "int y = 1;"},
		{"rust", "let s = r#\"a\"b\"#;\nlet y = 1;", "let y = 1;"},
	}
	for _, c := range cases {
		cfg := langConfigOf(t, c.lang)
		facts := NewSanitizer(cfg, false).CleanLinesFacts(strings.Split(c.src, "\n"))
		last := facts[len(facts)-1]
		if last.StartState != StateNormal {
			t.Errorf("%s %q: the last line starts in state %v, want Normal: the raw string did not close", c.lang, c.src, last.StartState)
			continue
		}
		if got := strings.TrimSpace(last.Cleaned); got != c.after {
			t.Errorf("%s %q: the last line cleaned = %q, want %q", c.lang, c.src, got, c.after)
		}
	}
}

// A triple-quoted string with a raw prefix (r""") is a docstring: the prefix is code and the quotes open a multi-line
// string that closes only with three quotes. The lines inside start in that state, and their text is blank. Before, the
// prefix was taken for a raw opener: the next line started in a single-quote string, and its word leaked into the code.
func TestSanitizerRawTripleQuotedIsDocstring(t *testing.T) {
	cfg := langConfigOf(t, "py")
	lines := []string{"t = r\"\"\"", "x \"y\"", "\"\"\"", "z = 2"}
	facts := NewSanitizer(cfg, false).CleanLinesFacts(lines)

	if facts[1].StartState != StateMultiLineString {
		t.Fatalf("line 2 starts in state %v, want MultiLineString", facts[1].StartState)
	}
	if got := strings.TrimSpace(facts[1].Cleaned); got != "" {
		t.Errorf("line 2 cleaned = %q, want blank: it is inside the docstring", got)
	}
	if facts[3].StartState != StateNormal {
		t.Fatalf("line 4 starts in state %v, want Normal", facts[3].StartState)
	}
	if got := strings.TrimSpace(facts[3].Cleaned); got != "z = 2" {
		t.Errorf("line 4 cleaned = %q, want %q", got, "z = 2")
	}
}

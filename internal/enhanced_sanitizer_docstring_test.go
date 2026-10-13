package internal

import (
	"strings"
	"testing"
)

// A Python docstring closes only with its own three quotes: a single quote inside it is text. Before, the sanitizer
// closed a multi-line string at the first unescaped `"` (the rule for C# verbatim strings, applied to every string),
// so the rest of the docstring was read as code and the line after it was blanked as text.
func TestSanitizerDocstringWithQuotes(t *testing.T) {
	cfg := langConfigOf(t, "py")
	src := "\"\"\"Doc\n\"quoted\" text\n\"\"\"\nimport re\n"
	facts := NewSanitizer(cfg, false).CleanLinesFacts(strings.Split(src, "\n"))

	// line 2 is inside the docstring; line 3 closes it; line 4 is code
	if facts[1].StartState == StateNormal {
		t.Fatalf("line 2 starts in Normal state: the docstring did not open")
	}
	if facts[3].StartState != StateNormal {
		t.Fatalf("line 4 starts in state %v, want Normal: the docstring closed early", facts[3].StartState)
	}
	if got := strings.TrimSpace(facts[3].Cleaned); got != "import re" {
		t.Fatalf("line 4 cleaned = %q, want %q", got, "import re")
	}
}

// Code after the closing of a multi-line string on the same line is code: `""", {"key": 1})` closes the string and the
// rest of the line stays. Before, the rest of the line was blanked as if it were still text, and the brackets it held
// were lost.
func TestSanitizerCodeAfterMultiLineStringCloses(t *testing.T) {
	cfg := langConfigOf(t, "py")
	lines := []string{"    execute(\"\"\"", "SQL (\"quoted\")", "\"\"\", {\"key\": 1})", "    return 1"}
	facts := NewSanitizer(cfg, false).CleanLinesFacts(lines)

	last := strings.TrimSpace(facts[2].Cleaned)
	if !strings.HasSuffix(last, "{     : 1})") {
		t.Fatalf("line 3 cleaned = %q, want the code after the closing quotes kept (ending in %q)", last, "{     : 1})")
	}
	if facts[3].StartState != StateNormal {
		t.Errorf("line 4 starts in state %v, want Normal", facts[3].StartState)
	}
}

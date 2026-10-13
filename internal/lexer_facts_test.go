package internal

import (
	"strings"
	"testing"
)

// A lexer that cleans its language (D, Scala) hands its text to the finders as it is: the sanitizer does not clean it a
// second time. A Scala symbol literal ('a) is code in the lexer's text, and the facts keep that text exactly.
func TestLexerFactsKeepTheLexerText(t *testing.T) {
	src := "object O {\n  val m = Map('a -> 1, 'b -> 2)\n  def f(x: Int): Int = x + 1\n}\n"
	lines := strings.Split(src, "\n")
	cleaned := cleanScalaLines(lines)
	facts := lexerFacts(cleaned, lines)

	for i := range cleaned {
		if facts[i].Cleaned != cleaned[i] {
			t.Fatalf("line %d: Cleaned = %q, want the lexer's text %q", i+1, facts[i].Cleaned, cleaned[i])
		}
		if facts[i].StartState != StateNormal {
			t.Fatalf("line %d starts in state %v: the lexer's text starts in the normal state", i+1, facts[i].StartState)
		}
	}
	if facts[1].Indent != GetIndentLevel(lines[1]) {
		t.Errorf("Indent = %d, want the raw line's %d", facts[1].Indent, GetIndentLevel(lines[1]))
	}
}

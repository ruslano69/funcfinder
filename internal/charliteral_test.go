package internal

import (
	"strings"
	"testing"
)

// A rune/char literal like '{' or '}' must not leak into brace counting. Before
// char_delimiters was set for the C-family/Kotlin languages, a lexer line such
// as `if c == '{' {` added a phantom +1 to the depth counter, causing the
// enclosing function to stay "open" and swallow every declaration after it.
func TestCharLiteralBraces_NoSwallow(t *testing.T) {
	cfg := getGoConfig(t)

	code := `package p

func (l *Lexer) Next() rune {
	if l.ch == '{' {
		return '{'
	}
	return 0
}

func SanitizeFieldName(s string) string {
	return s
}
`
	f := NewFinder(cfg, nil, true, false, false)
	res, err := f.FindFunctionsInLines(strings.Split(code, "\n"), 1, "t.go")
	if err != nil {
		t.Fatalf("FindFunctionsInLines() error = %v", err)
	}

	got := map[string]bool{}
	for _, fn := range res.Functions {
		got[fn.Name] = true
	}

	// Both must be present. The bug manifested as Next being swallowed so that
	// only SanitizeFieldName survived.
	for _, name := range []string{"Next", "SanitizeFieldName"} {
		if !got[name] {
			t.Errorf("function %q not found (funcs: %v) — char literal likely leaked into brace count", name, got)
		}
	}
}

// The sanitizer itself must blank out the contents of a char literal (including
// braces) for every language that declares a char delimiter.
func TestCharLiteralSanitized(t *testing.T) {
	c, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	for _, lang := range []string{"go", "cs", "java", "d", "kotlin"} {
		cfg, err := c.GetLanguageConfig(lang)
		if err != nil {
			t.Fatalf("GetLanguageConfig(%q) error = %v", lang, err)
		}
		s := NewSanitizer(cfg, false)
		cleaned, _ := s.CleanLine(`if c == '{' {`, StateNormal)
		if CountBraces(cleaned) != 1 {
			t.Errorf("%s: CountBraces(%q) = %d, want 1 (char-literal brace not blanked)", lang, cleaned, CountBraces(cleaned))
		}
	}
}

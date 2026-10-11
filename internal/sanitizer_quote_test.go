package internal

import (
	"strings"
	"testing"
)

// A string closes only on the delimiter that opened it: an apostrophe inside
// "..." (or a double quote inside '...') is string content. Before, any string
// delimiter closed the string, so "it's" reopened a string at the trailing
// quote and the rest of the file was treated as a string — braces uncounted,
// every following function lost.
func TestSanitizerStringClosesOnOwnDelimiter(t *testing.T) {
	config, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	for _, lang := range []string{"ts", "js", "go", "py"} {
		cfg := config[lang]
		if cfg == nil {
			t.Fatalf("%s config not found", lang)
		}
		s := NewSanitizer(cfg, false)
		line := `x = "it's {not code}"; y = { a }`
		got, state := s.CleanLine(line, StateNormal)
		if state != StateNormal {
			t.Errorf("%s: state after line = %v, want Normal", lang, state)
		}
		if strings.Contains(got, "not code") {
			t.Errorf("%s: string content leaked into code: %q", lang, got)
		}
		if strings.Count(got, "{") != 1 || strings.Count(got, "}") != 1 {
			t.Errorf("%s: want exactly the code braces after the string, got %q", lang, got)
		}
	}
}

// In JS/TS a regular quoted string cannot span lines, so an unmatched quote —
// typically an apostrophe in JSX text, <p>Зв'язок</p> — ends with its line.
func TestSanitizerSingleLineStringsJSX(t *testing.T) {
	config, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	s := NewSanitizer(config["ts"], false)
	_, state := s.CleanLine(`  return <p>Зв'язок відновлено</p>;`, StateNormal)
	if state != StateNormal {
		t.Errorf("state after JSX apostrophe line = %v, want Normal", state)
	}
}

// End to end: functions after a line with an apostrophe inside a string and
// after JSX text with an apostrophe are still found with correct bounds.
func TestTypeScriptFunctionsAfterApostrophes(t *testing.T) {
	config, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	code := `export const A = () => {
  const s = "зв'язок";
  return s;
};

export const B: React.FC<P> = () => {
  return <p>Зв'язок відновлено</p>;
};

export const C = () => {
  if (ok) {
    return 1;
  }
  return 2;
};`
	finder := NewFinder(config["ts"], nil, true, false, false)
	result, err := finder.FindFunctionsInLines(strings.Split(code, "\n"), 1, "test.tsx")
	if err != nil {
		t.Fatalf("FindFunctionsInLines() error = %v", err)
	}
	got := map[string][2]int{}
	for _, fn := range result.Functions {
		got[fn.Name] = [2]int{fn.Start, fn.End}
	}
	want := map[string][2]int{"A": {1, 4}, "B": {6, 8}, "C": {10, 15}}
	for name, span := range want {
		if g, ok := got[name]; !ok || g != span {
			t.Errorf("%s = %v (found %v), want %v; all: %v", name, g, ok, span, got)
		}
	}
}

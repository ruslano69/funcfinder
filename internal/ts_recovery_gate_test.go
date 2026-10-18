package internal

import (
	"strings"
	"testing"
)

// The recovery gate sends a file to the whole-file token pass only for a header shape the pass can read. Lines that
// look like a header's start but are a call continued over lines (quoted arguments, a parenthesized value, generic
// arguments) must not pass it; the header shapes must.
func TestTSRecoveryGateShapes(t *testing.T) {
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	lc, err := cfg.GetLanguageConfig("ts")
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		src  string
		want bool
	}{
		{"call arguments starting a line with a string", "foo(\n  (\"a\"), b,\n);", false},
		{"quoted method", "class A {\n  \"quoted\"(x: number) {\n    return x;\n  }\n}", true},
		{"async quoted method", "const o = {\n  async 'name'(x) {\n  },\n};", true},
		{"value in parentheses after a split declaration", "const total =\n  (a + b) * 2;", false},
		{"ternary in parentheses", "const pick =\n  (cond)\n    ? a\n    : b;", false},
		{"arrow after a split declaration", "const handler =\n  (req: Request): Response => {\n    return res;\n  };", true},
		{"arrow with parameters over lines", "const handler =\n  (\n    a: number,\n    b: number,\n  ) => a + b;", true},
		{"generic call over lines", "useThing<Config>(\n  a,\n  b,\n);", false},
		{"generic method over lines", "class A {\n  run<T>(\n    a: T,\n  ): T {\n    return a;\n  }\n}", true},
	}
	for _, c := range cases {
		lines := strings.Split(c.src, "\n")
		facts := NewSanitizer(lc, false).CleanLinesFacts(lines)
		if got := tsCandidateFile(lines, facts, map[tsCandidateKey]int{}, 0); got != c.want {
			t.Errorf("%s: gate %v, want %v", c.name, got, c.want)
		}
	}
	if !tsParenValue("(a, (b)") {
		t.Error("a parenthesis that does not close in the text must stay a candidate")
	}
}

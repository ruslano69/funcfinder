package internal

import (
	"strings"
	"testing"
)

func TestCppRawLiteralMasking(t *testing.T) {
	for _, prefix := range []string{"R", "u8R", "uR", "UR", "LR"} {
		for _, delim := range []string{"", "GSRT", "abcdefghijklmnop", `a"b`} {
			t.Run(prefix+delim, func(t *testing.T) {
				lines := []string{
					`auto text = ` + prefix + `"` + delim + `(`,
					`" // /* } ; int Hidden() { return 0; }`,
					`)wrong" int AlsoHidden() { return 0; }`,
					`)` + delim + `"; int Visible() { return 1; }`,
					`int After() { return 2; }`,
				}
				cfg := langConfigOf(t, "cpp")
				facts := NewSanitizer(cfg, false).CleanLinesFacts(lines)
				for _, i := range []int{1, 2} {
					if strings.TrimSpace(facts[i].Cleaned) != "" || facts[i].StartState != StateRawString {
						t.Fatalf("payload leaked at %d: %+v", i, facts[i])
					}
				}
				if !strings.Contains(facts[3].Cleaned, "int Visible()") || facts[4].StartState != StateNormal {
					t.Fatalf("closing delimiter lost following code: %+v", facts)
				}
				r, err := NewFinder(cfg, nil, true, true, false).FindFunctionsInLines(lines, 0, "raw.cpp")
				if err != nil || len(r.Functions) != 2 || r.Functions[0].Name != "Visible" || r.Functions[1].Name != "After" {
					t.Fatalf("functions=%+v, error=%v", r, err)
				}
			})
		}
	}
}

func TestCppRawLiteralOpenerValidation(t *testing.T) {
	for _, source := range []string{`R"abcdefghijklmnopq(`, `R"bad delim(`, "R\"bad\t(", `R"bad\(`, `R"bad)(`, `R"unterminated`, `R"é(`} {
		if consumed, _ := cppRawLiteralOpener([]rune(source), 0); consumed != 0 {
			t.Errorf("accepted invalid opener %q", source)
		}
	}
	for _, source := range []string{`nameR"(`, `nameu8R"(`, `éR"(`} {
		for i := range []rune(source) {
			if consumed, _ := cppRawLiteralOpener([]rune(source), i); consumed != 0 {
				t.Errorf("recognized raw token inside identifier %q at %d", source, i)
			}
		}
	}
}

func TestCppRawLiteralAdjacentAndUnclosed(t *testing.T) {
	cfg := langConfigOf(t, "cpp")
	s := NewSanitizer(cfg, false)
	line := `auto s = R"one(" } )two")one" R"two(/* " )one")two"; int Real() { return 1; }`
	cleaned, state := s.CleanLine(line, StateNormal)
	if state != StateNormal || strings.Contains(cleaned, "/*") || strings.Count(cleaned, "}") != 1 || !strings.Contains(cleaned, "int Real()") {
		t.Fatalf("adjacent literals: %q, %v", cleaned, state)
	}
	lines := []string{`auto s = R"GSRT(`, `int Hidden() { return 0; }`, `)other";`}
	facts := s.CleanLinesFacts(lines)
	if strings.TrimSpace(facts[2].Cleaned) != "" || facts[2].StartState != StateRawString {
		t.Fatalf("unterminated payload leaked: %+v", facts)
	}
	facts = s.CleanLinesFacts([]string{`int NewFile() { return 1; }`})
	if facts[0].Cleaned != `int NewFile() { return 1; }` {
		t.Fatalf("literal state leaked across files: %+v", facts)
	}
}

package internal

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// These tests pin the fix for the byte/rune indexing mismatch in CleanLine.
// Before the fix, the result buffer was sized by byte count, leaving
// (bytes-runes) trailing spaces on any line containing multibyte runes, and
// the block-comment / docstring handlers sliced the line by byte at a rune
// index — misaligned whenever a multibyte rune preceded the delimiter.

func TestCleanLine_NonASCII_NoTrailingSpaceArtifact(t *testing.T) {
	goCfg := getGoConfig(t)
	cases := []string{
		`x := "ü" + y`,
		`s := "привет" + z`,
		`a := "🚀🚀🚀"`,
		`func Тест() { return }`,
		`日本語 := 1`,
	}
	for _, line := range cases {
		san := NewSanitizer(goCfg, false)
		clean, _ := san.CleanLine(line, StateNormal)
		// The cleaned line must have exactly the same rune length as the input:
		// sanitizing replaces characters with spaces 1:1, never adding or
		// dropping any. The pre-fix byte-sized buffer violated this by padding
		// with (bytes-runes) extra trailing spaces.
		if utf8.RuneCountInString(clean) != utf8.RuneCountInString(line) {
			t.Errorf("rune-length changed for %q:\n  clean=%q (%d runes, want %d)",
				line, clean, utf8.RuneCountInString(clean), utf8.RuneCountInString(line))
		}
	}
}

func TestCleanLine_NonASCII_CodePreserved(t *testing.T) {
	goCfg := getGoConfig(t)
	// A multibyte identifier outside any literal/comment must survive cleaning
	// untouched.
	line := `func Тест() { return }`
	san := NewSanitizer(goCfg, false)
	clean, state := san.CleanLine(line, StateNormal)
	if clean != line {
		t.Errorf("non-literal multibyte code was altered:\n  in=   %q\n  clean=%q", line, clean)
	}
	if state != StateNormal {
		t.Errorf("state = %v, want StateNormal", state)
	}
}

func TestCleanLine_NonASCII_BlockCommentAfterMultibyteClosesCorrectly(t *testing.T) {
	goCfg := getGoConfig(t)
	// A multibyte rune inside a string literal shifts the byte offset of the
	// following block comment. The comment must still be detected, blanked, and
	// the parser must return to StateNormal on the same line.
	line := `x := "ü" /* comment */ + y`
	san := NewSanitizer(goCfg, false)
	clean, state := san.CleanLine(line, StateNormal)

	if state != StateNormal {
		t.Fatalf("block comment after multibyte rune left parser in %v, want StateNormal", state)
	}
	if strings.Contains(clean, "comment") {
		t.Errorf("block comment content not blanked: %q", clean)
	}
	// Code before and after the literal/comment must survive.
	if !strings.HasPrefix(clean, "x :=") {
		t.Errorf("leading code lost: %q", clean)
	}
	if !strings.Contains(clean, "+ y") {
		t.Errorf("trailing code after comment lost: %q", clean)
	}
}

func TestCleanLine_NonASCII_LineCommentAfterMultibyte(t *testing.T) {
	goCfg := getGoConfig(t)
	line := `a := "🚀" // tail comment`
	san := NewSanitizer(goCfg, false)
	clean, state := san.CleanLine(line, StateNormal)
	if state != StateNormal {
		t.Errorf("state = %v, want StateNormal", state)
	}
	if strings.Contains(clean, "tail") {
		t.Errorf("line comment after multibyte rune not stripped: %q", clean)
	}
	if !strings.HasPrefix(clean, "a :=") {
		t.Errorf("leading code lost: %q", clean)
	}
}

func TestCleanLine_NonASCII_UnterminatedBlockCommentLeaksState(t *testing.T) {
	goCfg := getGoConfig(t)
	// An unterminated block comment opened after a multibyte rune must carry
	// StateBlockComment to the next line.
	line := `q := "тест" /* open`
	san := NewSanitizer(goCfg, false)
	_, state := san.CleanLine(line, StateNormal)
	if state != StateBlockComment {
		t.Errorf("unterminated block comment state = %v, want StateBlockComment", state)
	}
}

func TestCleanLine_NonASCII_MultilineDocstringClosesAcrossMultibyte(t *testing.T) {
	pyCfg := getPyConfig(t)
	// Open a docstring on line 1, content with cyrillic on line 2, close on
	// line 3, then real code on line 4 — the call must survive.
	lines := []string{
		`def f():`,
		`    """`,
		`    Документация функции.`,
		`    """`,
		`    return target()`,
	}
	san := NewSanitizer(pyCfg, false)
	state := StateNormal
	var lastClean string
	for _, l := range lines {
		lastClean, state = san.CleanLine(l, state)
	}
	if state != StateNormal {
		t.Errorf("after closing docstring, state = %v, want StateNormal", state)
	}
	if !strings.Contains(lastClean, "target") {
		t.Errorf("code after multibyte docstring was blanked: %q", lastClean)
	}
}

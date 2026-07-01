package knowledge

import (
	"strings"
	"unicode"
)

// NormalizeForIndex strips unambiguous garbage that pollutes the FTS index and
// the term vocabulary before a document is stored.
//
// Always removed (safe for any corpus): the Unicode replacement char U+FFFD
// (decode failures), and C0/C1 control + format characters — except newline and
// tab, which are kept for readable display. Additionally, any runes listed in
// stripRunes are removed; this is the escape hatch for a corpus-specific OCR
// artifact (e.g. a PDF that misextracts a separator glyph as an omega), which
// must NOT be hardcoded because the same glyph is legitimate elsewhere (Ω = ohms).
//
// Removed characters become a space, and horizontal whitespace runs are then
// collapsed, so words that the artifact glued together ("theΩstatements") split
// back into tokens ("the statements").
func NormalizeForIndex(s, stripRunes string) string {
	strip := make(map[rune]bool, len(stripRunes))
	for _, r := range stripRunes {
		strip[r] = true
	}
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r == '\n' || r == '\t':
			b.WriteRune(r)
		case r == '�', unicode.Is(unicode.Cc, r), unicode.Is(unicode.Cf, r), strip[r]:
			b.WriteByte(' ')
		default:
			b.WriteRune(r)
		}
	}
	return collapseSpaces(b.String())
}

// collapseSpaces collapses runs of ASCII spaces to one, preserving newlines/tabs.
func collapseSpaces(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	pendingSpace := false
	for _, r := range s {
		if r == ' ' {
			pendingSpace = true
			continue
		}
		if pendingSpace {
			b.WriteByte(' ')
			pendingSpace = false
		}
		b.WriteRune(r)
	}
	if pendingSpace {
		b.WriteByte(' ')
	}
	return b.String()
}

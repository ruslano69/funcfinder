package internal

import (
	"math/rand"
	"strings"
	"testing"
)

// TestDLexerFastPathsMatchSlow compares the ASCII fast paths with the full ones on every D fixture line and on random
// lines: cleanLine against cleanLineSlow (two lexers over the same file, so their states must stay equal too) and
// normalizeDPrefix against normalizeDPrefixSlow.
func TestDLexerFastPathsMatchSlow(t *testing.T) {
	files := dFixtureLines(t)
	pieces := []string{"int", " ", "\t", "foo", "(", ")", "{", "}", ";", "/", "//", "/*", "*/", "/+", "+/", "\"", "'",
		"`", "q{", "q\"", "r\"", "@safe", "static", "if", "version", "debug(", "extern(C)", "public", "auto", "final",
		"é", "x.y", "_", "1", "=", "[", "]", "\\", "\x80"}
	rng := rand.New(rand.NewSource(1))
	for k := 0; k < 200; k++ {
		var file []string
		for n := 0; n < 50; n++ {
			var b strings.Builder
			for m := rng.Intn(8); m >= 0; m-- {
				b.WriteString(pieces[rng.Intn(len(pieces))])
			}
			file = append(file, b.String())
		}
		files = append(files, file)
	}
	lines, fast := 0, 0
	for _, file := range files {
		a, b := dLineLexer{}, dLineLexer{}
		for _, line := range file {
			lines++
			if a.commentDepth == 0 && a.qEnd == "" && a.qDepth == 0 && a.quote == 0 && a.tokens == 0 && dPlainCode(line) {
				fast++
			}
			got, want := a.cleanLine(line), b.cleanLineSlow(line)
			if got != want || a != b {
				t.Fatalf("cleanLine(%q) = %q, state %+v; cleanLineSlow gives %q, state %+v", line, got, a, want, b)
			}
			for _, s := range []string{line, got} {
				if g, w := normalizeDPrefix(s), normalizeDPrefixSlow(s); g != w {
					t.Fatalf("normalizeDPrefix(%q) = %q, slow path gives %q", s, g, w)
				}
			}
		}
	}
	if fast == 0 || fast == lines {
		t.Fatalf("fast path taken on %d of %d lines: the test exercises only one path", fast, lines)
	}
}

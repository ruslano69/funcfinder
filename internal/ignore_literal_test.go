package internal

import (
	"math/rand"
	"strings"
	"testing"
)

// TestIgnoreMatcher_LiteralGateMatchesRegex checks that the literal gate never changes a match: on assorted patterns
// and random paths, Matches gives what running every pattern's regex gives.
func TestIgnoreMatcher_LiteralGateMatchesRegex(t *testing.T) {
	m := &IgnoreMatcher{}
	m.parsePatterns(strings.Join([]string{
		"*.log", "**/build", "/_*", "a?c", "node_modules/", "foo/**/bar", ".env", "[x]", "a+b", `x\y`, "*", "**",
		"dir/sub/", "/root.txt", "*.min.*", "?", "!keep.log", "# comment", "b.c/", "/a/b/**", "**/*.map",
		// patternToRegex leaves * as a quantifier on the character before it: `ab*c` matches "ac".
		"ab*c", "xy*", "q.*z",
	}, "\n"))
	if got := globLiteral("ab*c"); got != "a" && got != "c" {
		t.Errorf(`globLiteral("ab*c") = %q: the b is optional in the regex and must not be in the literal`, got)
	}
	// Only *, ? and /_* (regex ^_*$, where the _ is optional) have no literal. The globs with ** do not compile
	// (`**` is not a valid regex) and parsePatterns drops them.
	noLiteral := 0
	for _, p := range m.patterns {
		if p.literal == "" {
			noLiteral++
		}
	}
	if noLiteral != 3 {
		t.Errorf("%d patterns without a literal, want 3 (*, ?, /_*)", noLiteral)
	}
	slow := func(path string, isDir bool) bool {
		for _, p := range m.patterns {
			if p.directory && !isDir {
				continue
			}
			if p.regex.MatchString(path) {
				return true
			}
		}
		return false
	}
	pieces := []string{"a", "b", "c", "x", "y", "_", ".", "/", "log", "build", "node_modules", "foo", "bar", "env", "[x]",
		"a+b", `\`, "dir", "sub", "root.txt", "min", "map", "keep", "ac", "abbc", "q", "z", "xyyy"}
	rng := rand.New(rand.NewSource(1))
	matched := 0
	for k := 0; k < 50000; k++ {
		var b strings.Builder
		for n := 1 + rng.Intn(6); n > 0; n-- {
			b.WriteString(pieces[rng.Intn(len(pieces))])
		}
		path, isDir := b.String(), rng.Intn(2) == 0
		got, want := m.Matches(path, isDir), slow(path, isDir)
		if got != want {
			t.Fatalf("Matches(%q, %v) = %v, regexes give %v", path, isDir, got, want)
		}
		if got {
			matched++
		}
	}
	if matched == 0 {
		t.Fatal("no path matched: the test checks nothing")
	}
}

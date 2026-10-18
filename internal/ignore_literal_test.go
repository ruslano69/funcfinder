package internal

import (
	"math/rand"
	"strings"
	"testing"
)

// gitSemanticsIgnore is the .gitignore of TestIgnoreMatcher_GitSemantics.
var gitSemanticsIgnore = strings.Join([]string{
	"*.log", "build/", "/root.txt", "docs/tmp/", ".agents/**", "**/cache", "foo/**/bar", "a?c", "/_*", "lib/*.js",
	"# comment", "x#y",
}, "\n")

// TestIgnoreMatcher_GitSemantics pins the matcher to git. Every expectation below is what `git check-ignore
// --no-index` answered for the same .gitignore and path (git 2.x, 2026-10-18); isDir marks a directory.
func TestIgnoreMatcher_GitSemantics(t *testing.T) {
	m := &IgnoreMatcher{}
	m.parsePatterns(gitSemanticsIgnore)
	cases := []struct {
		path  string
		isDir bool
		want  bool
	}{
		{"app.log", false, true}, {"src/app.log", false, true}, {"app.logx", false, false},
		{"build", true, true}, {"src/build", true, true}, {"buildfile/build", false, false},
		{"root.txt", false, true}, {"sub/root.txt", false, false},
		{"docs/tmp", true, true}, {"src/docs/tmp", true, false},
		{".agents/x.md", false, true}, {".agents/sub/y", false, true},
		{"cache", true, true}, {"a/b/cache", true, true}, {"c2/cache", false, true},
		{"foo/bar", false, true}, {"foo/x/y/bar", false, true}, {"foox/bar", false, false}, {"z/foo/bar", false, false},
		{"abc", false, true}, {"abbc", false, false},
		{"_private", true, true}, {"x/_private", true, false},
		{"lib/a.js", false, true}, {"lib/sub/a.js", false, false}, {"src/lib/a.js", false, false},
		{"x#y", false, true},
		// Windows paths from filepath.Rel use "\".
		{`src\app.log`, false, true}, {`docs\tmp`, true, true}, {`foo\x\bar`, false, true}, {`lib\sub\a.js`, false, false},
	}
	for _, c := range cases {
		if got := m.Matches(c.path, c.isDir); got != c.want {
			t.Errorf("Matches(%q, isDir=%v) = %v, git says %v", c.path, c.isDir, got, c.want)
		}
	}
}

// TestIgnoreMatcher_LiteralGateMatchesRegex checks that the literal gate never changes a match: on assorted patterns
// and random paths, Matches gives what running every pattern's regex gives.
func TestIgnoreMatcher_LiteralGateMatchesRegex(t *testing.T) {
	m := &IgnoreMatcher{}
	m.parsePatterns(strings.Join([]string{
		"*.log", "**/build", "/_*", "a?c", "node_modules/", "foo/**/bar", ".env", "[x]", "a+b", `x\y`, "*", "**",
		"dir/sub/", "/root.txt", "*.min.*", "?", "!keep.log", "# comment", "b.c/", "/a/b/**", "**/*.map",
		"ab*c", "xy*", "q.*z", "a/**", "**/z/**",
	}, "\n"))
	// Only the all-wildcard globs (*, **, ?) have no literal, so the gate is exercised by every other pattern.
	noLiteral := 0
	for _, p := range m.patterns {
		if p.literal == "" {
			noLiteral++
		}
	}
	if noLiteral != 3 {
		t.Errorf("%d patterns without a literal, want 3 (*, **, ?)", noLiteral)
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
	pieces := []string{"a", "b", "c", "x", "y", "z", "_", ".", "/", "log", "build", "node_modules", "foo", "bar", "env",
		"[x]", "a+b", `\`, "dir", "sub", "root.txt", "min", "map", "keep", "ac", "abbc", "q", "xyyy"}
	rng := rand.New(rand.NewSource(1))
	matched := 0
	for k := 0; k < 50000; k++ {
		var b strings.Builder
		for n := 1 + rng.Intn(6); n > 0; n-- {
			b.WriteString(pieces[rng.Intn(len(pieces))])
		}
		path, isDir := b.String(), rng.Intn(2) == 0
		got, want := m.Matches(path, isDir), slow(strings.ReplaceAll(path, `\`, "/"), isDir)
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

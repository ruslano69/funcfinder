package internal

import (
	"strings"
	"sync/atomic"
	"unicode/utf8"
)

// TypeScript function headers are recognised by tsHeaderSubmatch, a scanner written for the TS func_pattern of
// languages.json, as dHeaderSubmatch is for D (d_header.go). Running the pattern was a fifth of a TypeScript run. The
// scanner returns exactly the pattern's FindStringSubmatch: nil, or the match and all seven groups, in Go's
// leftmost-first order. The finder reads group 4 (const/let/var) as well as the names, so every group is kept.
//
// The scanner is used only while the configured pattern is tsFuncPatternSource, byte for byte; otherwise the regexp
// runs. TestTSHeaderScannerMatchesPattern compares the two, and TestTSHeaderScannerPatternIsCurrent fails when
// languages.json changes the pattern: update both together.
const tsFuncPatternSource = `^\s*(?:(export\s+)?(?:default\s+)?(async\s+)?function\s*\*?\s+({IDENT}+)\s*[<(]|(?:export\s+)?(const` +
	`|let|var)\s+({IDENT}+)\s*(?::\s*(?:[^=]|=>)+?)?=\s*(async\s*)?[<(]|(?:(?:public|private|protected|st` +
	`atic|async|override|readonly|abstract|get|set)\s+)*\*?\s*({IDENT}+)\s*(?:<[^>]*>)?\s*\((?:[^()]|\([^` +
	`()]*\))*\)\s*(?::\s*[^{;]+?)?\s*\{)`

var tsScannerConfig atomic.Pointer[LanguageConfig]

// tsUsesHeaderScanner reports whether the TS header scanner stands for config's function pattern.
func tsUsesHeaderScanner(config *LanguageConfig) bool {
	if tsScannerConfig.Load() == config {
		return true
	}
	if config.LangKey == "ts" && config.FuncPattern == tsFuncPatternSource {
		tsScannerConfig.Store(config)
		return true
	}
	return false
}

// tsMethodModifiers are the words of the method alternative's modifier list, in the pattern's order.
var tsMethodModifiers = []string{"public", "private", "protected", "static", "async", "override", "readonly",
	"abstract", "get", "set"}

var tsVariableKeywords = []string{"const", "let", "var"}

// tsWordSpace matches `word\s+` at i and returns its end, or -1.
func tsWordSpace(s string, i int, word string) int {
	if !strings.HasPrefix(s[i:], word) {
		return -1
	}
	e := i + len(word)
	if w := dSkipSpace(s, e); w > e {
		return w
	}
	return -1
}

// tsHeaderSubmatch returns what FindStringSubmatch with the TS function pattern returns for s. Its alternatives are
// tried in order: a function declaration, a const/let/var whose value starts with "(" or "<", a class method.
func tsHeaderSubmatch(s string) []string {
	p := dSkipSpace(s, 0) // whitespace given back by ^\s* never helps: no alternative needs to start with it
	if m := tsFunctionDeclaration(s, p); m != nil {
		return m
	}
	if m := tsVariableHeader(s, p); m != nil {
		return m
	}
	return tsMethodHeader(s, p)
}

// tsFunctionDeclaration: `(export\s+)?(?:default\s+)?(async\s+)?function\s*\*?\s+({IDENT}+)\s*[<(]`. Each optional word
// is taken when present: leaving it out would put that word where the next one or "function" is needed.
func tsFunctionDeclaration(s string, i int) []string {
	exported, async := "", ""
	if e := tsWordSpace(s, i, "export"); e >= 0 {
		exported, i = s[i:e], e
	}
	if e := tsWordSpace(s, i, "default"); e >= 0 {
		i = e
	}
	if e := tsWordSpace(s, i, "async"); e >= 0 {
		async, i = s[i:e], e
	}
	if !strings.HasPrefix(s[i:], "function") {
		return nil
	}
	i += len("function")
	// `\s*\*?\s+`: with a "*", whitespace must follow it; without one, at least one whitespace must precede the name.
	w := dSkipSpace(s, i)
	name := w
	if w < len(s) && s[w] == '*' {
		name = dSkipSpace(s, w+1)
		if name == w+1 {
			return nil
		}
	} else if w == i {
		return nil
	}
	e := dIdentEnd(s, name)
	if e == name {
		return nil
	}
	x := dSkipSpace(s, e)
	if x >= len(s) || (s[x] != '<' && s[x] != '(') {
		return nil
	}
	return []string{s[:x+1], exported, async, s[name:e], "", "", "", ""}
}

// tsVariableHeader: `(?:export\s+)?(const|let|var)\s+({IDENT}+)\s*(?::\s*(?:[^=]|=>)+?)?=\s*(async\s*)?[<(]`.
func tsVariableHeader(s string, i int) []string {
	if e := tsWordSpace(s, i, "export"); e >= 0 {
		i = e
	}
	keyword := ""
	for _, kw := range tsVariableKeywords {
		if strings.HasPrefix(s[i:], kw) {
			keyword = kw
			break
		}
	}
	if keyword == "" {
		return nil
	}
	e := i + len(keyword)
	w := dSkipSpace(s, e)
	if w == e {
		return nil
	}
	n := dIdentEnd(s, w)
	if n == w {
		return nil
	}
	end, async, ok := tsAssignedValue(s, dSkipSpace(s, n))
	if !ok {
		return nil
	}
	return []string{s[:end], "", "", "", keyword, s[w:n], async, ""}
}

// tsAssignedValue: `(?::\s*(?:[^=]|=>)+?)?=\s*(async\s*)?[<(]` from q, where the name and the whitespace after it
// end. It returns the offset just past the "<" or "(" (the end of the match), the text of the async group, and whether
// the value matched. It is the tail shared by a const|let|var header and a declarator.
func tsAssignedValue(s string, q int) (end int, async string, ok bool) {
	if q >= len(s) || s[q] != ':' {
		return tsValueEnd(s, q)
	}
	// The type annotation `:\s*(?:[^=]|=>)+?` is lazy: its possible ends are tried shortest first. After the greedy
	// \s*, the items form one chain (a rune other than "=", or "=>") until an "=" not followed by ">". Giving back
	// whitespace adds one more end, where that whitespace stops; earlier ends are whitespace, not "=".
	start := q + 1
	ws := dSkipSpace(s, start)
	for cur := ws; cur < len(s); {
		if s[cur] == '=' {
			if cur+1 >= len(s) || s[cur+1] != '>' {
				break
			}
			cur += 2
		} else {
			_, size := utf8.DecodeRuneInString(s[cur:])
			cur += size
		}
		if end, async, ok = tsValueEnd(s, cur); ok {
			return end, async, true
		}
	}
	if ws > start {
		return tsValueEnd(s, ws)
	}
	return 0, "", false // without the annotation the "=" would be needed where the ":" is
}

// tsValueEnd: `=\s*(async\s*)?[<(]` at i.
func tsValueEnd(s string, i int) (end int, async string, ok bool) {
	if i >= len(s) || s[i] != '=' {
		return 0, "", false
	}
	j := dSkipSpace(s, i+1)
	if strings.HasPrefix(s[j:], "async") {
		if k := dSkipSpace(s, j+len("async")); k < len(s) && (s[k] == '<' || s[k] == '(') {
			return k + 1, s[j:k], true
		}
	}
	if j < len(s) && (s[j] == '<' || s[j] == '(') {
		return j + 1, "", true
	}
	return 0, "", false
}

// tsMethodHeader: `(?:(?:public|...|set)\s+)*\*?\s*({IDENT}+)\s*(?:<[^>]*>)?\s*\(...\)\s*(?::\s*[^{;]+?)?\s*\{`. The
// modifier list is greedy, so the most modifiers are tried first: a modifier word can also be the method's name.
func tsMethodHeader(s string, p int) []string {
	var buf [8]int
	positions := append(buf[:0], p)
	for {
		e := -1
		for _, kw := range tsMethodModifiers {
			if e = tsWordSpace(s, positions[len(positions)-1], kw); e >= 0 {
				break
			}
		}
		if e < 0 {
			break
		}
		positions = append(positions, e)
	}
	for k := len(positions) - 1; k >= 0; k-- {
		if m := tsMethodAt(s, positions[k]); m != nil {
			return m
		}
	}
	return nil
}

func tsMethodAt(s string, i int) []string {
	j := i
	if j < len(s) && s[j] == '*' {
		j++ // without the "*", the name would have to start at it
	}
	j = dSkipSpace(s, j)
	n := dIdentEnd(s, j)
	if n == j {
		return nil
	}
	x := dSkipSpace(s, n)
	if x < len(s) && s[x] == '<' {
		k := strings.IndexByte(s[x+1:], '>')
		if k < 0 {
			return nil // without the type parameters "(" would be needed at the "<"
		}
		x = dSkipSpace(s, x+k+2)
	}
	pe := dParenEnd(s, x)
	if pe < 0 {
		return nil
	}
	q := dSkipSpace(s, pe)
	if q < len(s) && s[q] == ':' {
		// The lazy `[^{;]+?` then `\s*\{` succeeds exactly when the first "{" or ";" after the ":" is a "{" with at
		// least one character before it; the match ends at that "{".
		k := strings.IndexAny(s[q+1:], "{;")
		if k <= 0 || s[q+1+k] != '{' {
			return nil
		}
		return []string{s[:q+k+2], "", "", "", "", "", "", s[j:n]}
	}
	if q < len(s) && s[q] == '{' {
		return []string{s[:q+1], "", "", "", "", "", "", s[j:n]}
	}
	return nil
}

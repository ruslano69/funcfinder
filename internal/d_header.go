package internal

import (
	"strings"
	"sync/atomic"
	"unicode"
	"unicode/utf8"
)

// D function headers are recognised by dHeaderSubmatch, a scanner written for the D func_pattern of languages.json.
// The pattern is a large alternation with backtracking, and running it took about a quarter of a D run; the scanner
// walks the same structure directly. It gives the pattern's result exactly: whether a line matches, and the groups,
// chosen as Go's leftmost-first regexp chooses them (the same order of alternatives, greedy before shorter).
//
// The scanner is used only while the configured pattern is dFuncPatternSource, byte for byte; otherwise the regexp
// runs. TestDHeaderScannerMatchesPattern compares the two on every D fixture line and on generated lines, and
// TestDHeaderScannerPatternIsCurrent fails when languages.json changes the pattern: update both together.
const dFuncPatternSource = `^\s*(?:(?:@{IDENT}+(?:\((?:[^()]|\([^()]*\))*\))?|public|private|protected|package|export|static|fin` +
	`al|override|abstract|synchronized|pure|nothrow|scope|ref|const|immutable|shared|inout|auto)\s+)*(?:(` +
	`~?this|new)|(?:(?:const|immutable|shared|inout|typeof)\s*\((?:[^()]|\([^()]*\))*\)|\.?{IDENT}+(?:\.{` +
	`IDENT}+)*(?:!(?:\((?:[^()]|\([^()]*\))*\)|{IDENT}+))?)(?:\s*\*|\[[^\]]*\])*(?:(?:\s*\*)+\s*|\s+)(?:(` +
	`?:delegate|function)\((?:[^()]|\([^()]*\))*\)(?:\s+(?:@{IDENT}+|pure|nothrow))*\s+)?({IDENT}+))\s*\(` +
	`|^\s*(?:(?:@{IDENT}+(?:\((?:[^()]|\([^()]*\))*\))?|public|private|protected|package|export|static|fi` +
	`nal|override|abstract|synchronized|pure|nothrow|scope|ref|const|immutable|shared|inout|auto)\s+)*@pr` +
	`operty\s+({IDENT}+)\s*\(`

// dScannerConfig remembers the last configuration found to carry dFuncPatternSource, so the check is a pointer
// comparison after the first call.
var dScannerConfig atomic.Pointer[LanguageConfig]

// dUsesHeaderScanner reports whether the D header scanner stands for config's function pattern.
func dUsesHeaderScanner(config *LanguageConfig) bool {
	if dScannerConfig.Load() == config {
		return true
	}
	if config.LangKey == "d" && config.FuncPattern == dFuncPatternSource {
		dScannerConfig.Store(config)
		return true
	}
	return false
}

// dModifiers are the words of the pattern's modifier alternation, in its order.
var dModifiers = []string{"public", "private", "protected", "package", "export", "static", "final", "override",
	"abstract", "synchronized", "pure", "nothrow", "scope", "ref", "const", "immutable", "shared", "inout", "auto"}

// dTypeConstructors are the words of `(?:const|immutable|shared|inout|typeof)\s*\(...\)`.
var dTypeConstructors = []string{"const", "immutable", "shared", "inout", "typeof"}

// dIsSpace is the regexp class \s: [\t\n\f\r ].
func dIsSpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\f' || b == '\r'
}

// dSkipSpace returns the end of \s* from i.
func dSkipSpace(s string, i int) int {
	for i < len(s) && dIsSpace(s[i]) {
		i++
	}
	return i
}

// dIdentEnd returns the end of [\p{L}\p{Nd}_]* from i, reading runes as the regexp does (an invalid byte is one
// U+FFFD, which is not a letter).
func dIdentEnd(s string, i int) int {
	for i < len(s) {
		if b := s[i]; b < utf8.RuneSelf {
			if b == '_' || 'a' <= b && b <= 'z' || 'A' <= b && b <= 'Z' || '0' <= b && b <= '9' {
				i++
				continue
			}
			return i
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		if !unicode.IsLetter(r) && !unicode.Is(unicode.Nd, r) {
			return i
		}
		i += size
	}
	return i
}

// dParenEnd matches `\((?:[^()]|\([^()]*\))*\)` at i: parentheses nested at most two deep. It returns the end, or
// -1. The match is unique: the alternatives start with different characters, and the star stops only at ")".
func dParenEnd(s string, i int) int {
	if i >= len(s) || s[i] != '(' {
		return -1
	}
	for j := i + 1; j < len(s); {
		switch s[j] {
		case ')':
			return j + 1
		case '(':
			k := j + 1
			for k < len(s) && s[k] != '(' && s[k] != ')' {
				k++
			}
			if k >= len(s) || s[k] != ')' {
				return -1 // a third level or no close: no shorter star helps, since none ends before a ")"
			}
			j = k + 1
		default:
			j++ // every other byte belongs to a rune that is not a parenthesis
		}
	}
	return -1
}

// dModifierEnd matches one `(?:@{IDENT}+(?:\(...\))?|public|...|auto)\s+` at i and returns its end, or -1. Only one
// way can match: an identifier is read in full (a shorter one leaves an identifier character, which neither the
// parentheses nor \s+ accept), and dropping the parentheses leaves a "(" where \s+ is needed.
func dModifierEnd(s string, i int) int {
	if i < len(s) && s[i] == '@' {
		j := dIdentEnd(s, i+1)
		if j == i+1 {
			return -1
		}
		if p := dParenEnd(s, j); p >= 0 {
			if w := dSkipSpace(s, p); w > p {
				return w
			}
		}
		if w := dSkipSpace(s, j); w > j {
			return w
		}
		return -1
	}
	for _, kw := range dModifiers {
		if strings.HasPrefix(s[i:], kw) {
			e := i + len(kw)
			if w := dSkipSpace(s, e); w > e {
				return w
			}
		}
	}
	return -1
}

// dHeaderSubmatch returns what FindStringSubmatch with the D function pattern returns for s: nil, or the match and
// its three groups (`~?this|new`, the typed name, the @property name). Whitespace given back by a greedy \s never
// helps here, since nothing that follows one starts with whitespace; every other choice point is tried in order.
func dHeaderSubmatch(s string) []string {
	// The modifier list: positions after 0..K modifiers. The greedy star tries the most first.
	var buf [8]int
	positions := append(buf[:0], dSkipSpace(s, 0))
	for {
		e := dModifierEnd(s, positions[len(positions)-1])
		if e < 0 {
			break
		}
		positions = append(positions, e)
	}
	for k := len(positions) - 1; k >= 0; k-- {
		if m := dHeaderAt(s, positions[k]); m != nil {
			return m
		}
	}
	// The @property alternative, tried only when the first one fails everywhere.
	for k := len(positions) - 1; k >= 0; k-- {
		pos := positions[k]
		if !strings.HasPrefix(s[pos:], "@property") {
			continue
		}
		w := dSkipSpace(s, pos+len("@property"))
		if w == pos+len("@property") {
			continue
		}
		if m := dNameAt(s, w, 3); m != nil {
			return m
		}
	}
	return nil
}

// dHeaderAt tries `(~?this|new)|TYPE...NAME` followed by `\s*\(` at pos.
func dHeaderAt(s string, pos int) []string {
	for _, word := range []string{"~this", "this", "new"} {
		if !strings.HasPrefix(s[pos:], word) {
			continue
		}
		e := pos + len(word)
		if w := dSkipSpace(s, e); w < len(s) && s[w] == '(' {
			return []string{s[:w+1], s[pos:e], "", ""}
		}
	}
	// TYPE: `(?:const|...|typeof)\s*\(...\)` first, then `\.?{IDENT}+(?:\.{IDENT}+)*(?:!(?:\(...\)|{IDENT}+))?`.
	for _, kw := range dTypeConstructors {
		if strings.HasPrefix(s[pos:], kw) {
			if p := dParenEnd(s, dSkipSpace(s, pos+len(kw))); p >= 0 {
				if m := dTypeTail(s, p); m != nil {
					return m
				}
			}
			break // the words share no prefix that also matches here
		}
	}
	j := pos
	if j < len(s) && s[j] == '.' {
		j++
	}
	e := dIdentEnd(s, j)
	if e == j {
		return nil
	}
	for e < len(s) && s[e] == '.' {
		n := dIdentEnd(s, e+1)
		if n == e+1 {
			break
		}
		e = n
	}
	if e < len(s) && s[e] == '!' {
		// Without the template argument the tail would start at "!", which it cannot.
		if p := dParenEnd(s, e+1); p >= 0 {
			e = p
		} else if n := dIdentEnd(s, e+1); n > e+1 {
			e = n
		} else {
			return nil
		}
	}
	return dTypeTail(s, e)
}

// dTypeTail tries `(?:\s*\*|\[[^\]]*\])*(?:(?:\s*\*)+\s*|\s+)DELEGATE?NAME\s*\(` after a type ending at t.
func dTypeTail(s string, t int) []string {
	var endBuf, starBuf [8]int
	ends := append(endBuf[:0], t) // positions after 0, 1, ... pointer/array suffixes
	for cur := t; ; {
		if w := dSkipSpace(s, cur); w < len(s) && s[w] == '*' {
			cur = w + 1
		} else if cur < len(s) && s[cur] == '[' {
			k := strings.IndexByte(s[cur+1:], ']')
			if k < 0 {
				break
			}
			cur += k + 2
		} else {
			break
		}
		ends = append(ends, cur)
	}
	for i := len(ends) - 1; i >= 0; i-- {
		e := ends[i]
		// Separator, first alternative: `(?:\s*\*)+\s*`, most stars first.
		stars := starBuf[:0]
		for cur := e; ; {
			w := dSkipSpace(s, cur)
			if w >= len(s) || s[w] != '*' {
				break
			}
			cur = w + 1
			stars = append(stars, cur)
		}
		for j := len(stars) - 1; j >= 0; j-- {
			if m := dDelegateName(s, dSkipSpace(s, stars[j])); m != nil {
				return m
			}
		}
		// Second alternative: `\s+`.
		if g := dSkipSpace(s, e); g > e {
			if m := dDelegateName(s, g); m != nil {
				return m
			}
		}
	}
	return nil
}

// dDelegateName tries `(?:(?:delegate|function)\(...\)(?:\s+(?:@{IDENT}+|pure|nothrow))*\s+)?` then the name at g.
func dDelegateName(s string, g int) []string {
	for _, kw := range []string{"delegate", "function"} {
		if !strings.HasPrefix(s[g:], kw) {
			continue
		}
		p := dParenEnd(s, g+len(kw))
		if p < 0 {
			break
		}
		var attrBuf [8]int
		attrs := append(attrBuf[:0], p)
		for cur := p; ; {
			w := dSkipSpace(s, cur)
			if w == cur || w >= len(s) {
				break
			}
			if s[w] == '@' {
				n := dIdentEnd(s, w+1)
				if n == w+1 {
					break
				}
				cur = n
			} else if strings.HasPrefix(s[w:], "pure") {
				cur = w + len("pure")
			} else if strings.HasPrefix(s[w:], "nothrow") {
				cur = w + len("nothrow")
			} else {
				break
			}
			attrs = append(attrs, cur)
		}
		for i := len(attrs) - 1; i >= 0; i-- {
			if h := dSkipSpace(s, attrs[i]); h > attrs[i] {
				if m := dNameAt(s, h, 2); m != nil {
					return m
				}
			}
		}
		break
	}
	return dNameAt(s, g, 2)
}

// dNameAt matches `({IDENT}+)\s*\(` at h, putting the name in group 2 or 3. The name is read in full: a shorter one
// leaves an identifier character where \s or "(" is needed.
func dNameAt(s string, h, group int) []string {
	e := dIdentEnd(s, h)
	if e == h {
		return nil
	}
	w := dSkipSpace(s, e)
	if w >= len(s) || s[w] != '(' {
		return nil
	}
	m := []string{s[:w+1], "", "", ""}
	m[group] = s[h:e]
	return m
}

// FuncSubmatch returns what FindStringSubmatch of the function pattern returns for s. For D it runs the header
// scanner, which gives the same result; callers that test signature lines (callgraph, funcstats) use it so D does
// not run the costly pattern there either.
func (lc *LanguageConfig) FuncSubmatch(s string) []string {
	if dUsesHeaderScanner(lc) {
		return dHeaderSubmatch(s)
	}
	if re := lc.FuncRegex(); re != nil {
		return re.FindStringSubmatch(s)
	}
	return nil
}

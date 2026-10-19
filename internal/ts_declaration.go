package internal

import (
	"strings"
	"sync/atomic"
)

// The TypeScript declaration_pattern (a bodyless signature: an overload, an interface member, an abstract method) and
// declarator_pattern (a later declarator of a const list) are read by scanners written for them, as tsHeaderSubmatch is
// for func_pattern (ts_header.go). The declaration pattern ran for every line that ends in ';' and was a fifth of a
// TypeScript run; the declarator pattern runs after every ',' of a line. The scanners return exactly what
// FindStringSubmatch returns, in Go's leftmost-first order, and are used only while languages.json holds the patterns
// below byte for byte; TestTSDeclarationScannersMatchPatterns compares them with the regular expressions, and
// TestTSDeclarationScannerPatternsAreCurrent fails when languages.json changes a pattern: update both together.
const tsDeclarationPatternSource = `^(?:\s*(?:(?:export|default|declare|public|protected|private|static|readonly|override|async|function` +
	`)\s+)*({IDENT}+)\??\s*(?:<[^()]*>)?\s*\((?:[^()]|\([^()]*\))*\)\s*:.+;\s*$|\s*(?:(?:export|default|d` +
	`eclare|public|protected|private|static|readonly|override|async)\s+)*abstract\s+(?:async\s+)?({IDENT}` +
	`+)\??\s*(?:<[^()]*>)?\s*\((?:[^()]|\([^()]*\))*\)(?:\s*:.+)?;\s*$)`

const tsDeclaratorPatternSource = `^\s*({IDENT}+)\s*(?::\s*(?:[^=]|=>)+?)?=\s*(?:async\s*)?[<(]`

var tsDeclarationScannerConfig, tsDeclaratorScannerConfig atomic.Pointer[LanguageConfig]

// tsUsesDeclarationScanner reports whether tsDeclarationSubmatch stands for config's declaration pattern.
func tsUsesDeclarationScanner(config *LanguageConfig) bool {
	if tsDeclarationScannerConfig.Load() == config {
		return true
	}
	if config.LangKey == "ts" && config.DeclarationPattern == tsDeclarationPatternSource {
		tsDeclarationScannerConfig.Store(config)
		return true
	}
	return false
}

// tsUsesDeclaratorScanner reports whether tsDeclaratorSubmatch stands for config's declarator pattern.
func tsUsesDeclaratorScanner(config *LanguageConfig) bool {
	if tsDeclaratorScannerConfig.Load() == config {
		return true
	}
	if config.LangKey == "ts" && config.DeclaratorPattern == tsDeclaratorPatternSource {
		tsDeclaratorScannerConfig.Store(config)
		return true
	}
	return false
}

// tsDeclarationModifiers are the words of the first alternative's modifier list (`function` included), and
// tsAbstractModifiers those of the second, which has `abstract` instead.
var tsDeclarationModifiers = []string{"export", "default", "declare", "public", "protected", "private", "static",
	"readonly", "override", "async", "function"}

var tsAbstractModifiers = tsDeclarationModifiers[:len(tsDeclarationModifiers)-1]

// isRE2Space is RE2's \s: [\t\n\f\r ].
func isRE2Space(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\f' || c == '\r'
}

// tsDeclarationSubmatch returns what FindStringSubmatch with the TS declaration pattern returns for s: nil, or the
// whole line and two groups, the name of the first alternative (a signature `name(...): type;`) or of the second (an
// `abstract` member, whose type is optional).
func tsDeclarationSubmatch(s string) []string {
	p := dSkipSpace(s, 0) // whitespace given back by ^\s* never helps: every alternative goes on with a word
	if name, ok := tsSignatureDeclaration(s, p); ok {
		return []string{s, name, ""}
	}
	if name, ok := tsAbstractDeclaration(s, p); ok {
		return []string{s, "", name}
	}
	return nil
}

// tsModifierChain returns the offsets after each prefix of the greedy `(?:(?:word)\s+)*`, the first being p.
func tsModifierChain(s string, p int, words []string, buf []int) []int {
	positions := append(buf[:0], p)
	for {
		e := -1
		for _, kw := range words {
			if e = tsWordSpace(s, positions[len(positions)-1], kw); e >= 0 {
				break
			}
		}
		if e < 0 {
			return positions
		}
		positions = append(positions, e)
	}
}

// tsSignatureDeclaration: `\s*(?:(?:export|...|function)\s+)*({IDENT}+)\??\s*(?:<[^()]*>)?\s*\((?:[^()]|\([^()]*\))*\)\s*:.+;\s*$`.
// The modifier list is greedy, so the most modifiers are tried first: a modifier word can also be the name.
func tsSignatureDeclaration(s string, p int) (string, bool) {
	var buf [8]int
	positions := tsModifierChain(s, p, tsDeclarationModifiers, buf[:])
	for k := len(positions) - 1; k >= 0; k-- {
		name, close, ok := tsNameAndParameters(s, positions[k])
		if ok && tsAnnotatedEnd(s, close) {
			return name, true
		}
	}
	return "", false
}

// tsAbstractDeclaration: `\s*(?:(?:export|...|async)\s+)*abstract\s+(?:async\s+)?({IDENT}+)\??\s*(?:<[^()]*>)?\s*\((?:[^()]|\([^()]*\))*\)(?:\s*:.+)?;\s*$`.
func tsAbstractDeclaration(s string, p int) (string, bool) {
	var buf [8]int
	positions := tsModifierChain(s, p, tsAbstractModifiers, buf[:])
	// Fewer modifiers would leave a modifier word where `abstract` is needed: only the longest chain can match.
	i := tsWordSpace(s, positions[len(positions)-1], "abstract")
	if i < 0 {
		return "", false
	}
	// `(?:async\s+)?` is greedy: with the word first, then without it (the name may be "async").
	if e := tsWordSpace(s, i, "async"); e >= 0 {
		if name, close, ok := tsNameAndParameters(s, e); ok && tsAbstractEnd(s, close) {
			return name, true
		}
	}
	if name, close, ok := tsNameAndParameters(s, i); ok && tsAbstractEnd(s, close) {
		return name, true
	}
	return "", false
}

// tsNameAndParameters: `({IDENT}+)\??\s*(?:<[^()]*>)?\s*\((?:[^()]|\([^()]*\))*\)` at i. It returns the name and the
// offset of the ")" that closes the parameter list.
func tsNameAndParameters(s string, i int) (name string, close int, ok bool) {
	e := dIdentEnd(s, i)
	if e == i {
		return "", 0, false
	}
	name = s[i:e]
	if e < len(s) && s[e] == '?' {
		e++
	}
	x := dSkipSpace(s, e)
	if x < len(s) && s[x] == '<' {
		// `<[^()]*>` then `\s*\(`: the run of characters that are not parentheses ends at the first "(" or ")"; the
		// greedy `[^()]*` leaves the last ">" of the run for the closing one, and only that ">" can be followed by
		// whitespace and the "(".
		r := x + 1
		for r < len(s) && s[r] != '(' && s[r] != ')' {
			r++
		}
		if r >= len(s) || s[r] != '(' {
			return "", 0, false
		}
		gt := strings.LastIndexByte(s[x+1:r], '>')
		if gt < 0 || dSkipSpace(s, x+1+gt+1) != r {
			return "", 0, false
		}
		x = r
	}
	if x >= len(s) || s[x] != '(' {
		return "", 0, false
	}
	close = tsParenthesesEnd(s, x+1)
	return name, close, close >= 0
}

// tsParenthesesEnd: `(?:[^()]|\([^()]*\))*\)` from i, just after a "(": the offset of the ")" that ends the list, or -1.
// The list holds characters that are not parentheses and groups with no parenthesis inside; nothing else can match,
// and backtracking cannot move the end.
func tsParenthesesEnd(s string, i int) int {
	for i < len(s) {
		switch s[i] {
		case ')':
			return i
		case '(':
			j := i + 1
			for j < len(s) && s[j] != '(' && s[j] != ')' {
				j++
			}
			if j >= len(s) || s[j] != ')' {
				return -1
			}
			i = j + 1
		default:
			i++
		}
	}
	return -1
}

// tsAnnotatedEnd: `\s*:.+;\s*$` after the ")" at close. The greedy `.+` gives back until a ";" is followed by
// whitespace only, so the ";" is the last character that is not whitespace, with something other than a newline (".")
// between it and the ":".
func tsAnnotatedEnd(s string, close int) bool {
	j := dSkipSpace(s, close+1)
	if j >= len(s) || s[j] != ':' {
		return false
	}
	e := len(s)
	for e > 0 && isRE2Space(s[e-1]) {
		e--
	}
	if e-1 < j+2 || s[e-1] != ';' { // `.+` needs at least one character between ":" and ";"
		return false
	}
	return strings.IndexByte(s[j+1:e-1], '\n') < 0
}

// tsAbstractEnd: `(?:\s*:.+)?;\s*$` after the ")" at close: the annotated form, or a ";" right after the ")".
func tsAbstractEnd(s string, close int) bool {
	if tsAnnotatedEnd(s, close) {
		return true
	}
	i := close + 1
	if i >= len(s) || s[i] != ';' {
		return false
	}
	return dSkipSpace(s, i+1) == len(s)
}

// tsDeclaratorSubmatch returns what FindStringSubmatch with the TS declarator pattern returns for s: nil, or the
// match (up to the "<" or "(" that starts the value) and the name.
func tsDeclaratorSubmatch(s string) []string {
	p := dSkipSpace(s, 0)
	n := dIdentEnd(s, p)
	if n == p {
		return nil
	}
	end, _, ok := tsAssignedValue(s, dSkipSpace(s, n))
	if !ok {
		return nil
	}
	return []string{s[:end], s[p:n]}
}

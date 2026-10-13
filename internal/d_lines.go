package internal

import (
	"regexp"
	"strings"
	"unicode"
)

var dAttributeLine = regexp.MustCompile(`^\s*(?:(?:@[\p{L}\p{Nd}_]+|public|private|protected|export|pure|nothrow|const|immutable|shared|inout|ref|scope)\s*)+$`)
var dTypeHeader = regexp.MustCompile(`^\s*(?:(?:@[\p{L}\p{Nd}_]+|public|private|protected|package|static|final|abstract)\s+)*(?:class|struct|interface|union|enum|template|alias)\b`)
var dStatementHeader = regexp.MustCompile(`^\s*(?:(?:static|final)\s+)*(?:return|throw|new|delete|if|else|for|foreach|while|switch|try|catch|with|assert|version|debug|scope|mixin|pragma|cast)\b`)

// D has nested /+ comments and token strings. Keep their text out of the
// generic finder; retain positions and restore raw source during extraction.
func prepareDLines(lines []string, config *LanguageConfig) ([]string, map[int]int) {
	lexer := dLineLexer{}
	result := make([]string, len(lines))
	headerEnds := make(map[int]int)
	for i, line := range lines {
		result[i] = normalizeDPrefix(lexer.cleanLine(line))
	}
	// Return types and names can be on separate lines. Move only a recognized
	// header onto its first line, retaining blank lines for original coordinates.
	// The function pattern runs only on text that can hold a header (func_requires, "("): on the other lines,
	// many of them long runs of blanked comments, it backtracked for nothing and took most of the time.
	for i := 0; i+1 < len(result); i++ {
		if strings.TrimSpace(result[i]) == "" || dAttributeLine.MatchString(result[i]) || dFuncHeader(result[i], config) || dStatementHeader.MatchString(result[i]) || dTypeHeader.MatchString(result[i]) || strings.ContainsAny(result[i], ";{}=") {
			continue
		}
		for j := i + 1; j < len(result) && j <= i+4; j++ {
			joined := strings.Join(result[i:j+1], " ")
			if dFuncHeader(joined, config) {
				result[i] = joined
				headerEnds[i] = j
				for k := i + 1; k <= j; k++ {
					result[k] = ""
				}
				break
			}
			if strings.ContainsAny(result[j], ";{}=") {
				break
			}
		}
	}
	pending := false
	parens := 0
	contract := false
	contractDepth := 0
	contractParens := 0
	outSelector := false
	outExpression := false
	for i, line := range result {
		code := []rune(line)
		text := strings.TrimSpace(string(code))
		if dTypeHeader.MatchString(text) {
			for j, ch := range code {
				if ch == '(' {
					code[j] = '['
				} else if ch == ')' {
					code[j] = ']'
				}
			}
		} else if dStatementHeader.MatchString(text) {
			for j, ch := range code {
				if !unicode.IsSpace(ch) {
					code[j] = '?'
					break
				}
			}
		}
		// An attribute line holds at least one attribute word, so a blank line is not one and skips the pattern.
		if text != "" && dAttributeLine.MatchString(string(code)) {
			result[i] = strings.Repeat(" ", len(code))
			continue
		}
		if contractDepth == 0 && dFuncHeader(string(code), config) {
			pending, parens, contract = true, 0, false
		}
		for j := 0; j < len(code); j++ {
			ch := code[j]
			if contractParens > 0 {
				if ch == '(' {
					contractParens++
				} else if ch == ')' {
					contractParens--
					if contractParens == 0 && outSelector && !outExpression {
						contract = true
					}
				} else if ch == ';' {
					outExpression = true
				}
				code[j] = ' '
				continue
			}
			if contractDepth > 0 {
				if ch == '{' {
					contractDepth++
				} else if ch == '}' {
					contractDepth--
				}
				code[j] = ' '
				continue
			}
			if !pending {
				continue
			}
			if unicode.IsLetter(ch) && parens == 0 {
				end := j + 1
				for end < len(code) && (unicode.IsLetter(code[end]) || code[end] == '_') {
					end++
				}
				word := string(code[j:end])
				if word == "in" || word == "out" {
					contract = true
					outSelector, outExpression = word == "out", false
					for k := j; k < end; k++ {
						code[k] = ' '
					}
				} else if word == "do" || word == "body" {
					for k := j; k < end; k++ {
						code[k] = ' '
					}
				}
				j = end - 1
				continue
			}
			switch ch {
			case '(', '[':
				if contract && ch == '(' {
					contractParens, contract = 1, false
					code[j] = ' '
				} else {
					parens++
				}
			case ')', ']':
				parens--
			case '{':
				if parens == 0 {
					if contract {
						contractDepth, contract = 1, false
						code[j] = ' '
					} else {
						pending = false
					}
				}
			case ';':
				if parens == 0 {
					pending = false
				}
			}
		}
		result[i] = string(code)
	}
	return result, headerEnds
}

// dFuncHeader reports whether a prepared D line matches the function pattern. A line without a func_requires
// character cannot, so the pattern is not run on it.
func dFuncHeader(line string, config *LanguageConfig) bool {
	return config.MayStartFunction(line) && config.FuncRegex().MatchString(line)
}

// Conditional declarations and parameterized attributes prefix a declaration,
// but are not its return type. Preserve prefixes that introduce a brace block.
func normalizeDPrefix(line string) string {
	code := []rune(line)
	for i := 0; i < len(code); {
		if unicode.IsSpace(code[i]) {
			i++
			continue
		}
		start := i
		if code[i] == '@' {
			i++
		}
		for i < len(code) && (unicode.IsLetter(code[i]) || unicode.IsDigit(code[i]) || code[i] == '_' || code[i] == '.') {
			i++
		}
		word := string(code[start:i])
		if word == "static" {
			i = skipDSpace(code, i)
			if i+2 <= len(code) && string(code[i:i+2]) == "if" && (i+2 == len(code) || !unicode.IsLetter(code[i+2])) {
				word = "static if"
				i += 2
			}
		}
		i = skipDSpace(code, i)
		parameterized := word == "static if" || word == "version" || word == "debug" || word == "pragma" || word == "extern" || word == "deprecated" || word == "package" || strings.HasPrefix(word, "@")
		if parameterized && i < len(code) && code[i] == '(' {
			depth := 1
			end := i + 1
			for end < len(code) && depth > 0 {
				if code[end] == '(' {
					depth++
				} else if code[end] == ')' {
					depth--
				}
				end++
			}
			if depth != 0 {
				break
			}
			trial := append([]rune(nil), code...)
			for k := start; k < end; k++ {
				trial[k] = ' '
			}
			// Continue through multiple attributes; a later iteration can expose
			// the declaration. Blocks and ordinary statements remain unmodified.
			if strings.TrimSpace(string(code[end:])) != "" && !strings.HasPrefix(strings.TrimSpace(string(code[end:])), "{") {
				code = trial
			}
			i = end
			continue
		}
		if word == "extern" || word == "deprecated" || word == "debug" {
			for k := start; k < i; k++ {
				code[k] = ' '
			}
			continue
		}
		if start == i || !dAttributeLine.MatchString(word) && word != "static" && word != "final" && word != "override" && word != "abstract" && word != "auto" {
			break
		}
	}
	return string(code)
}

func skipDSpace(code []rune, i int) int {
	for i < len(code) && unicode.IsSpace(code[i]) {
		i++
	}
	return i
}

// this(args); inside a method delegates to a constructor; it does not declare
// one. A local class's constructor has its own class scope and must survive.
func filterDFunctions(functions []FunctionBounds, lines []string, startLine int, finder *Finder) []FunctionBounds {
	kept := make([]FunctionBounds, 0, len(functions))
	for _, fn := range functions {
		if !finder.mapMode && !finder.funcNames[fn.Name] {
			continue
		}
		call := false
		signature := sigScanner{}
		if fn.Name == "this" && signature.feed(strings.Join(lines[fn.Start-startLine:fn.End-startLine+1], "\n")) != sigBody {
			for _, parent := range functions {
				if parent.Start < fn.Start && parent.End >= fn.End && parent.ClassName == fn.ClassName {
					call = true
					break
				}
			}
		}
		if !call {
			kept = append(kept, fn)
		}
	}
	return kept
}

type dLineLexer struct {
	commentDepth int
	commentClose string
	quote        rune
	raw          bool
	tokens       int
	qOpen        rune
	qClose       rune
	qDepth       int
	qEnd         string
}

func (s *dLineLexer) cleanLine(line string) string {
	runes := []rune(line)
	code := make([]rune, len(runes))
	for i := range code {
		code[i] = ' '
	}
	for i := 0; i < len(runes); {
		ch := runes[i]
		if s.commentDepth > 0 {
			if s.commentClose == "+/" && dPrefixAt(runes, i, "/+") {
				s.commentDepth++
				i += 2
			} else if dPrefixAt(runes, i, s.commentClose) {
				s.commentDepth--
				i += 2
			} else {
				i++
			}
			continue
		}
		if s.qEnd != "" {
			if dPrefixAt(runes, i, s.qEnd) {
				i += len([]rune(s.qEnd))
				s.qEnd = ""
			} else {
				i++
			}
			continue
		}
		if s.qDepth > 0 {
			if ch == s.qOpen {
				s.qDepth++
			} else if ch == s.qClose {
				s.qDepth--
				if s.qDepth == 0 && i+1 < len(runes) && runes[i+1] == '"' {
					i++
				}
			}
			i++
			continue
		}
		if s.quote != 0 {
			if ch == '\\' && !s.raw {
				i += 2
			} else {
				if ch == s.quote {
					s.quote = 0
				}
				i++
			}
			continue
		}
		if i+1 < len(runes) && ch == '/' {
			switch runes[i+1] {
			case '/':
				return string(code)
			case '*', '+':
				s.commentDepth = 1
				s.commentClose = "*/"
				if runes[i+1] == '+' {
					s.commentClose = "+/"
				}
				i += 2
				continue
			}
		}
		if i+1 < len(runes) && ch == 'q' && (runes[i+1] == '{' || runes[i+1] == '"') {
			if runes[i+1] == '{' {
				s.tokens++
				i += 2
				continue
			}
			if i+2 < len(runes) {
				open := runes[i+2]
				close := map[rune]rune{'[': ']', '(': ')', '{': '}', '<': '>'}[open]
				if close != 0 {
					s.qOpen, s.qClose, s.qDepth = open, close, 1
				} else {
					end := i + 3
					for end < len(runes) && (unicode.IsLetter(runes[end]) || unicode.IsDigit(runes[end]) || runes[end] == '_') {
						end++
					}
					s.qEnd = string(runes[i+2:end]) + `"`
					i = end
					continue
				}
				i += 3
				continue
			}
		}
		if ch == '"' || ch == '\'' || ch == '`' || (ch == 'r' && i+1 < len(runes) && runes[i+1] == '"') {
			s.raw = ch == '`' || ch == 'r'
			if ch == 'r' {
				i++
				ch = '"'
			}
			s.quote = ch
			i++
			continue
		}
		if s.tokens > 0 {
			if ch == '{' {
				s.tokens++
			} else if ch == '}' {
				s.tokens--
			}
		} else {
			code[i] = ch
		}
		i++
	}
	return string(code)
}

func dPrefixAt(runes []rune, pos int, text string) bool {
	for _, ch := range text {
		if pos >= len(runes) || runes[pos] != ch {
			return false
		}
		pos++
	}
	return true
}

func findDClasses(lines []string, offset int, config *LanguageConfig) []ClassBounds {
	var classes []ClassBounds
	for start, line := range lines {
		matches := config.ClassRegex().FindStringSubmatch(line)
		if matches == nil {
			continue
		}
		end := start
		brackets, depth := 0, 0
		opened := false
	scan:
		for i := start; i < len(lines); i++ {
			for _, ch := range lines[i] {
				if !opened {
					switch ch {
					case '(', '[':
						brackets++
					case ')', ']':
						brackets--
					case ';', '=':
						if brackets == 0 {
							end = i
							break scan
						}
					case '{':
						if brackets == 0 {
							opened, depth = true, 1
						}
					}
				} else {
					if ch == '{' {
						depth++
					} else if ch == '}' {
						depth--
						if depth == 0 {
							end = i
							break scan
						}
					}
				}
			}
			end = i
		}
		classes = append(classes, ClassBounds{Name: ExtractFuncName(matches), Start: start + 1 + offset, End: end + 1 + offset})
	}
	return classes
}

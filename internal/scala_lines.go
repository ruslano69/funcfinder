package internal

import "strings"

// A quote with no nearby closing quote is a Scala symbol ('name), not a char.
func cleanScalaLines(lines []string) []string {
	lexer := scalaBoundLexer{}
	result := make([]string, len(lines))
	for i, line := range lines {
		result[i] = lexer.cleanLine(line)
	}
	return result
}

// Scala expression bodies may have no braces and terminate before the next
// member/class. Balanced calls, following chains and indented continuations
// belong to the body; comments and enclosing braces do not.
func scalaFunctionEnd(lines []string, start int, config *LanguageConfig) int {
	return scalaFunctionEndAt(lines, start, 0, config)
}

func scalaFunctionEndAt(lines []string, start, position int, config *LanguageConfig) int {
	indices := config.FuncRegex().FindStringSubmatchIndex(lines[start][position:])
	from := position
	for i := 2; i+1 < len(indices); i += 2 {
		if indices[i] >= 0 {
			from = position + indices[i+1]
		}
	}
	parens, brackets, braces := 0, 0, 0
	body := false
	last := start + 1
	indent := GetIndentLevel(lines[start])
	for i := start; i < len(lines); i++ {
		text := lines[i]
		if i == start {
			text = text[from:]
		}
		trimmed := strings.TrimSpace(text)
		if trimmed == "" {
			continue
		}
		for j := 0; j < len(text); j++ {
			ch := text[j]
			switch ch {
			case '(':
				parens++
			case ')':
				parens--
			case '[':
				brackets++
			case ']':
				brackets--
			case '{':
				if parens == 0 && brackets == 0 && !body {
					body = true // Scala 2 procedure syntax has no '='.
				}
				braces++
			case '}':
				if braces == 0 && parens == 0 && brackets == 0 {
					return last
				}
				braces--
			case '=':
				if parens == 0 && brackets == 0 && braces == 0 && (j+1 == len(text) || text[j+1] != '>') {
					body = true
				}
			}
		}
		last = i + 1
		if parens > 0 || brackets > 0 || braces > 0 {
			continue
		}
		next := i + 1
		for next < len(lines) && strings.TrimSpace(lines[next]) == "" {
			next++
		}
		if next >= len(lines) {
			return last
		}
		nextText := strings.TrimSpace(lines[next])
		if config.FuncRegex().MatchString(lines[next]) || config.ClassRegex().MatchString(lines[next]) || strings.HasPrefix(nextText, "}") {
			return last
		}
		if !body {
			if scalaIncomplete(trimmed) || strings.HasPrefix(nextText, ":") || strings.HasPrefix(nextText, "=") {
				continue
			}
			return last // a bodyless declaration
		}
		if scalaIncomplete(trimmed) || strings.HasPrefix(nextText, ".") || strings.HasPrefix(nextText, "else") || GetIndentLevel(lines[next]) > indent {
			continue
		}
		return last
	}
	return last
}

type scalaBoundLexer struct {
	quote   rune
	triple  bool
	comment int
}

// Literal delimiters become a balanced pair of parentheses. This keeps a
// string-only expression visible and carries multiline strings across lines.
func (s *scalaBoundLexer) cleanLine(line string) string {
	runes := []rune(line)
	code := make([]rune, len(runes))
	for i := range code {
		code[i] = ' '
	}
	for i := 0; i < len(runes); {
		ch := runes[i]
		if s.comment > 0 {
			if dPrefixAt(runes, i, "/*") {
				s.comment++
				i += 2
			} else if dPrefixAt(runes, i, "*/") {
				s.comment--
				i += 2
			} else {
				i++
			}
			continue
		}
		if s.quote != 0 {
			if ch == '\\' && !s.triple {
				i += 2
				continue
			}
			if ch == s.quote && (!s.triple || dPrefixAt(runes, i, `"""`)) {
				code[i] = ')'
				if s.triple {
					i += 3
					for i < len(runes) && runes[i] == '"' {
						i++
					}
				} else {
					i++
				}
				s.quote, s.triple = 0, false
				continue
			}
			i++
			continue
		}
		if dPrefixAt(runes, i, "//") {
			break
		}
		if dPrefixAt(runes, i, "/*") {
			s.comment = 1
			i += 2
			continue
		}
		char := false
		if ch == '\'' && i+2 < len(runes) {
			end := i + 2
			if runes[i+1] == '\\' {
				end++
			}
			char = end < len(runes) && runes[end] == '\''
		}
		if ch == '"' || char {
			s.quote = ch
			s.triple = ch == '"' && dPrefixAt(runes, i, `"""`)
			code[i] = '('
			if s.triple {
				i += 3
			} else {
				i++
			}
			continue
		}
		code[i] = ch
		i++
	}
	return string(code)
}

func scalaIncomplete(text string) bool {
	if text == "" {
		return false
	}
	return strings.ContainsRune("=.,:+-*/|&><!%", rune(text[len(text)-1])) || strings.HasSuffix(text, "match")
}

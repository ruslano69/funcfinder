package internal

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"
)

type kotlinObjectScan struct {
	t []rustObjectToken
	lines, ends []int
}

func kotlinObjectComment(src []byte, at int) (int, error) {
	i, depth := at+2, 1
	for i < len(src) {
		if i+1 < len(src) && string(src[i:i+2]) == "/*" {
			depth++
			i += 2
			continue
		}
		if i+1 < len(src) && string(src[i:i+2]) == "*/" {
			depth--
			i += 2
			if depth == 0 {
				return i, nil
			}
			continue
		}
		i++
	}
	return 0, fmt.Errorf("unclosed Kotlin comment at byte %d", at)
}
func kotlinObjectString(src []byte, at, depth int) (int, error) {
	if depth > 64 {
		return 0, fmt.Errorf("Kotlin literal nesting exceeds 64")
	}
	quote := src[at]
	raw := quote == '"' && at+2 < len(src) && string(src[at:at+3]) == `"""`
	i := at + 1
	if raw {
		i = at + 3
	}
	for i < len(src) {
		if raw && i+2 < len(src) && string(src[i:i+3]) == `"""` {
			for i < len(src) && src[i] == '"' {
				i++
			}
			return i, nil
		}
		if !raw && src[i] == quote {
			return i + 1, nil
		}
		if !raw && src[i] == '\\' {
			i += 2
			continue
		}
		if !raw && (src[i] == '\n' || src[i] == '\r') {
			return 0, fmt.Errorf("newline in Kotlin quoted literal")
		}
		if quote == '"' && i+1 < len(src) && src[i] == '$' && src[i+1] == '{' {
			i += 2
			braces := 1
			for i < len(src) && braces > 0 {
				if i+1 < len(src) && string(src[i:i+2]) == "/*" {
					next, err := kotlinObjectComment(src, i)
					if err != nil {
						return 0, err
					}
					i = next
					continue
				}
				if i+1 < len(src) && string(src[i:i+2]) == "//" {
					for i < len(src) && src[i] != '\n' {
						i++
					}
					continue
				}
				if src[i] == '"' || src[i] == '\'' || src[i] == '`' {
					next, err := kotlinObjectString(src, i, depth+1)
					if err != nil {
						return 0, err
					}
					i = next
					continue
				}
				if src[i] == '{' {
					braces++
				}
				if src[i] == '}' {
					braces--
				}
				i++
			}
			if braces != 0 {
				return 0, fmt.Errorf("unclosed Kotlin interpolation")
			}
			continue
		}
		i++
	}
	return 0, fmt.Errorf("unclosed Kotlin literal at byte %d", at)
}
func kotlinObjectTokens(src []byte) ([]rustObjectToken, error) {
	if !utf8.Valid(src) {
		return nil, fmt.Errorf("Kotlin objects require UTF-8")
	}
	var tokens []rustObjectToken
	var stack []int
	for i := 0; i < len(src); {
		r, size := utf8.DecodeRune(src[i:])
		if unicode.IsSpace(r) || i == 0 && r == '\ufeff' {
			i += size
			continue
		}
		if i == 0 && i+1 < len(src) && string(src[i:i+2]) == "#!" {
			for i < len(src) && src[i] != '\n' {
				i++
			}
			continue
		}
		if i+1 < len(src) && string(src[i:i+2]) == "//" {
			for i < len(src) && src[i] != '\n' {
				i++
			}
			continue
		}
		if i+1 < len(src) && string(src[i:i+2]) == "/*" {
			next, err := kotlinObjectComment(src, i)
			if err != nil {
				return nil, err
			}
			i = next
			continue
		}
		start, literal := i, false
		if src[i] == '"' || src[i] == '\'' || src[i] == '`' {
			literal = src[i] != '`'
			next, err := kotlinObjectString(src, i, 0)
			if err != nil {
				return nil, err
			}
			i = next
		} else {
			i += size
			if r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r) {
				literal = unicode.IsDigit(r)
				for i < len(src) {
					next, n := utf8.DecodeRune(src[i:])
					if next != '_' && !unicode.IsLetter(next) && !unicode.IsDigit(next) && !unicode.IsMark(next) {
						break
					}
					i += n
				}
			} else if i < len(src) && strings.Contains("|?.|?:|!!|::|->|&&||||<=|>=|==|!=|..|+=|-=|*=|/=|++|--|", "|"+string(src[start:i+1])+"|") {
				i++
			}
		}
		token := rustObjectToken{text: string(src[start:i]), start: start, end: i, pair: -1, literal: literal}
		index := len(tokens)
		tokens = append(tokens, token)
		if literal || kotlinObjectWord(token.text) {
			continue
		}
		switch token.text {
		case "(", "[", "{":
			stack = append(stack, index)
		case ")", "]", "}":
			if len(stack) == 0 {
				return nil, fmt.Errorf("unmatched Kotlin delimiter at byte %d", start)
			}
			open := stack[len(stack)-1]
			if tokens[open].text != map[string]string{")": "(", "]": "[", "}": "{"}[token.text] {
				return nil, fmt.Errorf("mismatched Kotlin delimiters")
			}
			stack = stack[:len(stack)-1]
			tokens[open].pair = index
			tokens[index].pair = open
		}
	}
	if len(stack) > 0 {
		return nil, fmt.Errorf("unclosed Kotlin delimiter")
	}
	return tokens, nil
}
func kotlinObjectWord(text string) bool {
	if len(text) >= 2 && text[0] == '`' && text[len(text)-1] == '`' {
		return true
	}
	r, _ := utf8.DecodeRuneInString(text)
	return r == '_' || unicode.IsLetter(r)
}
func kotlinObjectModifier(text string) bool {
	return strings.Contains("|public|private|protected|internal|override|open|abstract|final|suspend|inline|infix|operator|tailrec|external|actual|expect|const|lateinit|data|sealed|enum|annotation|inner|value|companion|crossinline|noinline|reified|", "|"+text+"|")
}
func (s kotlinObjectScan) at(i int, text string) bool {
	return i >= 0 && i < len(s.t) && !s.t[i].literal && s.t[i].text == text
}
func (s kotlinObjectScan) angle(i int) int {
	depth := 1
	for j := i + 1; j < len(s.t); j++ {
		if s.at(j, "<") {
			depth++
		}
		if s.at(j, ">") {
			depth--
			if depth == 0 {
				return j + 1
			}
		}
		if s.at(j, "(") || s.at(j, "[") {
			j = s.t[j].pair
			continue
		}
		if s.at(j, "{") || s.at(j, ";") || s.at(j, "=") {
			return -1
		}
	}
	return -1
}
func (s kotlinObjectScan) annotation(i int) int {
	i++
	if i < len(s.t) && kotlinObjectWord(s.t[i].text) {
		i++
	}
	if s.at(i, ":") {
		i++
		if i < len(s.t) && kotlinObjectWord(s.t[i].text) {
			i++
		}
	}
	if s.at(i, "[") {
		return s.t[i].pair + 1
	}
	for s.at(i, ".") && i+1 < len(s.t) && kotlinObjectWord(s.t[i+1].text) {
		i += 2
	}
	if s.at(i, "<") {
		i = s.angle(i)
		if i < 0 {
			return len(s.t)
		}
	}
	if s.at(i, "(") {
		i = s.t[i].pair + 1
	}
	return i
}
func (s kotlinObjectScan) head(i int) int {
	for i < len(s.t) {
		if s.at(i, "@") {
			i = s.annotation(i)
			continue
		}
		if kotlinObjectModifier(s.t[i].text) {
			i++
			continue
		}
		if s.at(i, "context") && s.at(i+1, "(") {
			i = s.t[i+1].pair + 1
			continue
		}
		break
	}
	return i
}
func (s kotlinObjectScan) declaration(i int) bool {
	if i < 0 || i >= len(s.t) || s.at(i-1, ".") || s.at(i-1, "?.") || s.at(i-1, "::") {
		return false
	}
	j := s.head(i)
	if j >= len(s.t) || s.t[j].literal {
		return false
	}
	return strings.Contains("|fun|val|var|class|interface|object|typealias|constructor|init|package|import|", "|"+s.t[j].text+"|")
}
func (s kotlinObjectScan) canEnd(i int) bool {
	return i >= 0 && i < len(s.t) && (kotlinObjectWord(s.t[i].text) || s.t[i].literal || strings.Contains("|)|]|}|!!|++|--|>|?|", "|"+s.t[i].text+"|"))
}
func (s kotlinObjectScan) continuation(i int) bool {
	return i >= 0 && i < len(s.t) && !s.t[i].literal && strings.Contains("|.|?.|?:|::|+|-|*|/|%|&&||||==|!=|<=|>=|<|>|in|is|as|else|catch|finally|!!|", "|"+s.t[i].text+"|")
}
func (s kotlinObjectScan) expression(first, hi int) int {
	last, need := first-1, true
	for i := first; i < hi; i++ {
		if s.at(i, ";") || s.at(i, "}") || s.at(i, "]") || s.at(i, ")") {
			return last
		}
		if i > first && s.declaration(i) && !s.at(s.head(i), "object") {
			return last
		}
		if i > first && s.lines[i] > s.ends[last] && !need && s.canEnd(last) && !s.continuation(i) && !s.at(i, "{") && !s.at(i, "(") {
			return last
		}
		if s.at(i, "@") {
			i = s.annotation(i) - 1
			last = i
			continue
		}
		if s.at(i, "if") || s.at(i, "when") || s.at(i, "catch") {
			need = true
			if s.at(i+1, "(") {
				i = s.t[i+1].pair
			}
			last = i
			continue
		}
		if s.at(i, "else") || s.at(i, "try") || s.at(i, "finally") || s.at(i, "throw") || s.at(i, "return") {
			need = true
			last = i
			continue
		}
		if s.at(i, "object") {
			body := i + 1
			for body < hi && !s.at(body, "{") && !s.at(body, ";") && !s.at(body, "}") {
				if s.at(body, "(") {
					body = s.t[body].pair
				}
				body++
			}
			if body < hi && s.at(body, "{") {
				i = s.t[body].pair
				last = i
				need = false
				continue
			}
		}
		if s.at(i, "(") || s.at(i, "[") || s.at(i, "{") {
			i = s.t[i].pair
			need = false
		} else if s.at(i, "<") && i > first && kotlinObjectWord(s.t[i-1].text) {
			next := s.angle(i)
			if next > i && (s.at(next, "(") || s.at(next, "{") || s.at(next, ".")) {
				i = next - 1
				need = false
			} else {
				need = true
			}
		} else if kotlinObjectWord(s.t[i].text) && !need && last >= first && s.canEnd(last) {
			need = true
		} else if s.continuation(i) && !s.at(i, "!!") {
			need = true
		} else {
			need = !s.canEnd(i)
		}
		last = i
	}
	return last
}
func (s kotlinObjectScan) parameters(first, hi int) (int, int) {
	for i := first; i < hi; i++ {
		if kotlinObjectWord(s.t[i].text) && s.at(i+1, "(") {
			return i, i + 1
		}
		if s.at(i, "<") {
			next := s.angle(i)
			if next < 0 {
				return -1, -1
			}
			i = next - 1
			continue
		}
		if s.at(i, "(") {
			if s.at(s.t[i].pair+1, ".") {
				i = s.t[i].pair
				continue
			}
			return -1, -1
		}
		if s.at(i, "{") || s.at(i, "=") || s.at(i, ";") || s.at(i, "}") || i > first && s.declaration(i) {
			return -1, -1
		}
	}
	return -1, -1
}
func (s kotlinObjectScan) functionEnd(close, hi int) (int, int) {
	last := close
	for i := close + 1; i < hi; i++ {
		if s.at(i, "=") {
			return s.expression(i+1, hi), -1
		}
		if s.at(i, "{") {
			return s.t[i].pair, i
		}
		if s.at(i, ";") || s.at(i, "}") || s.declaration(i) {
			return last, -1
		}
		if s.at(i, "@") {
			if s.lines[i] > s.ends[last] && !strings.Contains("|:|->|<|,|", "|"+s.t[last].text+"|") {
				return last, -1
			}
			i = s.annotation(i) - 1
		} else if s.at(i, "<") {
			next := s.angle(i)
			if next < 0 {
				return -1, -1
			}
			i = next - 1
		} else if s.at(i, "(") || s.at(i, "[") {
			i = s.t[i].pair
		}
		last = i
	}
	return last, -1
}
func (s kotlinObjectScan) typeEnd(name, hi int) (int, int) {
	last := name
	for i := name + 1; i < hi; i++ {
		if s.at(i, "{") {
			return s.t[i].pair, i
		}
		if s.at(i, ";") || s.at(i, "}") {
			return last, -1
		}
		if s.at(i, "@") {
			next := s.annotation(i)
			if !s.at(next, "constructor") && !s.at(next, "@") && !s.at(last, ":") {
				return last, -1
			}
			i = next - 1
			last = i
			continue
		}
		if s.at(i, "constructor") {
			last = i
			continue
		}
		if kotlinObjectModifier(s.t[i].text) {
			next := i + 1
			for next < hi && kotlinObjectModifier(s.t[next].text) {
				next++
			}
			if s.at(next, "constructor") {
				last = i
				continue
			}
		}
		if s.declaration(i) {
			return last, -1
		}
		if s.lines[i] > s.ends[last] && !strings.Contains("|:|,|where|by|(|<|", "|"+s.t[i].text+"|") && !strings.Contains("|:|,|where|by|constructor|", "|"+s.t[last].text+"|") {
			return last, -1
		}
		if s.at(i, "<") {
			next := s.angle(i)
			if next < 0 {
				return -1, -1
			}
			i = next - 1
		} else if s.at(i, "(") || s.at(i, "[") {
			i = s.t[i].pair
		}
		last = i
	}
	return last, -1
}

func (s kotlinObjectScan) accessorHead(first, hi int) int {
	if first >= hi {
		return -1
	}
	head := s.head(first)
	if head >= hi || !s.at(head, "get") && !s.at(head, "set") {
		return -1
	}
	if s.at(head+1, "(") {
		return head
	}
	if head+1 >= hi || s.at(head+1, ";") || s.at(head+1, "}") || s.lines[head+1] > s.ends[head] || s.declaration(head+1) {
		return head
	}
	return -1
}

func (s kotlinObjectScan) propertyExpression(first, hi int) int {
	for j := first; j < hi; j++ {
		if j > first && (s.at(j, ";") || s.at(j, "}") || s.declaration(j) && !s.at(s.head(j), "object")) {
			break
		}
		if j > first && s.canEnd(j-1) && !s.at(j-1, ".") && !s.at(j-1, "?.") && !s.at(j-1, "::") && s.accessorHead(j, hi) >= 0 {
			return s.expression(first, j)
		}
		if s.t[j].pair > j {
			j = s.t[j].pair
		}
	}
	return s.expression(first, hi)
}

func (s kotlinObjectScan) accessorEnd(first, hi int) (int, int) {
	head := s.accessorHead(first, hi)
	if head < 0 {
		return -1, -1
	}
	if !s.at(head+1, "(") {
		return head, head
	}
	last := s.t[head+1].pair
	for j := last + 1; j < hi; j++ {
		if s.at(j, "=") {
			return head, s.propertyExpression(j+1, hi)
		}
		if s.at(j, "{") {
			return head, s.t[j].pair
		}
		if s.at(j, ";") || s.at(j, "}") || s.declaration(j) || s.accessorHead(j, hi) >= 0 {
			return head, last
		}
		if s.t[j].pair > j {
			j = s.t[j].pair
		}
		if s.at(j, "<") {
			next := s.angle(j)
			if next < 0 {
				return -1, -1
			}
			j = next - 1
		}
		last = j
	}
	return head, last
}

func (s kotlinObjectScan) propertyShape(head, hi int) (int, int, int) {
	name, last, access := -1, head, -1
	j := head + 1
	if s.at(j, "<") {
		j = s.angle(j)
		if j < 0 {
			return -1, -1, -1
		}
	}
	for ; j < hi; j++ {
		if name >= 0 && s.accessorHead(j, hi) >= 0 {
			break
		}
		if s.at(j, ":") || s.at(j, "=") || s.at(j, "by") || s.at(j, ";") || s.at(j, "}") {
			break
		}
		if j > head+1 && s.declaration(j) {
			break
		}
		if s.at(j, "@") {
			j = s.annotation(j) - 1
			continue
		}
		if s.at(j, "<") {
			next := s.angle(j)
			if next < 0 {
				return -1, -1, -1
			}
			j = next - 1
			continue
		}
		if s.at(j, "(") {
			if s.at(s.t[j].pair+1, ".") {
				j = s.t[j].pair
				continue
			}
			return -1, -1, -1
		}
		if kotlinObjectWord(s.t[j].text) {
			name = j
		}
		last = j
	}
	if name < 0 {
		return -1, -1, -1
	}
	for ; j < hi; j++ {
		if s.at(j, ";") || s.at(j, "}") {
			break
		}
		if s.accessorHead(j, hi) >= 0 {
			access = j
			break
		}
		if s.declaration(j) {
			break
		}
		if s.at(j, "=") || s.at(j, "by") {
			last = s.propertyExpression(j+1, hi)
			j = last + 1
			if s.at(j, ";") {
				break
			}
			if s.accessorHead(j, hi) >= 0 {
				access = j
			}
			break
		}
		if s.at(j, "@") {
			j = s.annotation(j) - 1
		} else if s.at(j, "<") {
			next := s.angle(j)
			if next < 0 {
				return -1, -1, -1
			}
			j = next - 1
		} else if s.t[j].pair > j {
			j = s.t[j].pair
		}
		last = j
	}
	if access >= 0 {
		for j = access; j < hi; {
			_, end := s.accessorEnd(j, hi)
			if end < j {
				break
			}
			last = end
			j = end + 1
			if s.at(j, ";") {
				j++
			}
			if s.accessorHead(j, hi) < 0 {
				break
			}
		}
	}
	return name, last, access
}
func kotlinCodeItems(path string, src []byte, scope, context string) ([]rustObjectItem, error) {
	tokens, err := kotlinObjectTokens(src)
	if err != nil {
		return nil, objectError("invalid_source", err.Error())
	}
	// Keys compare LF-normalized payloads with original CRLF literal tokens.
	for i := range tokens {
		tokens[i].text = strings.ReplaceAll(tokens[i].text, "\r\n", "\n")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, objectError("invalid_path", err.Error())
	}
	abs = filepath.Clean(abs)
	starts := []int{0}
	for i, b := range src {
		if b == '\n' {
			starts = append(starts, i+1)
		}
	}
	s := kotlinObjectScan{t: tokens, lines: make([]int, len(tokens)), ends: make([]int, len(tokens))}
	for i, t := range tokens {
		s.lines[i], _ = rustBytePosition(starts, t.start)
		s.ends[i], _ = rustBytePosition(starts, t.end-1)
	}
	revision := codeRevision(src)
	var items []rustObjectItem
	var walk func(int, int, string, string, int) error
	walk = func(lo, hi int, scope, context string, depth int) error {
		if depth > 128 {
			return objectError("invalid_source", "Kotlin scope nesting exceeds 128")
		}
		for first := lo; first < hi; {
			if s.at(first, ";") {
				first++
				continue
			}
			if s.at(first, "@") && s.at(first+1, "file") && s.at(first+2, ":") {
				end := s.annotation(first) - 1
				if end < first || end >= hi {
					return objectError("invalid_source", "incomplete file annotation")
				}
				items = append(items, rustObjectItem{start: tokens[first].start, end: tokens[end].end, key: scope + "/file-annotation:" + rustTokenKey(tokens, first, end+1), scope: scope, context: context, depth: depth})
				first = end + 1
				continue
			}
			head := s.head(first)
			if head >= hi {
				return objectError("invalid_source", "Kotlin prefix without declaration")
			}
			kind, name, last, body, attrs, key := "", "", -1, -1, "", ""
			typeHead := head
			if s.at(head, "fun") && s.at(head+1, "interface") {
				typeHead++
			}
			word := tokens[typeHead].text

			if strings.HasPrefix(context, "property:") {
				accessor, end := s.accessorEnd(first, hi)
				if accessor < 0 || end < first {
					return objectError("invalid_source", "a property scope accepts only get/set accessors")
				}
				name = tokens[accessor].text
				if context == "property:val" && name == "set" {
					return objectError("invalid_source", "val cannot own a setter")
				}
				kind, last = "accessor", end
				attrs = rustTokenKey(tokens, first, accessor+1)
				key = scope + "/accessor:" + name
			} else if s.at(head, "val") || s.at(head, "var") {
				nameAt, end, access := s.propertyShape(head, hi)
				if nameAt < 0 {
					return objectError("invalid_source", "unsupported/destructured Kotlin property")
				}
				kind, name, last, body = "property", tokens[nameAt].text, end, access
				attrs = rustTokenKey(tokens, first, head+1) + "/receiver:" + rustTokenKey(tokens, head+1, nameAt)
				key = scope + "/property:" + name + rustTokenKey(tokens, head+1, nameAt)
			} else if word == "class" || word == "interface" || word == "object" {
				nameAt := typeHead + 1
				if nameAt < hi && kotlinObjectWord(tokens[nameAt].text) && !s.at(nameAt, "where") {
					name = tokens[nameAt].text
				}
				companion := false
				for j := first; j < head; j++ {
					if s.at(j, "companion") {
						companion = true
					}
				}
				if name == "" && !companion {
					return objectError("invalid_source", "anonymous Kotlin type declaration")
				}
				if name == "" {
					nameAt = typeHead
				}
				last, body = s.typeEnd(nameAt, hi)
				kind = "type"
				if name == "" {
					kind = ""
				}
				attrs = rustTokenKey(tokens, first, typeHead+1)
				key = scope + "/" + word + ":" + name
			} else if s.at(head, "typealias") {
				if head+1 >= hi || !kotlinObjectWord(tokens[head+1].text) {
					return objectError("invalid_source", "incomplete Kotlin alias")
				}
				name = tokens[head+1].text
				last = s.expression(head+2, hi)
				kind = "type"
				attrs = rustTokenKey(tokens, first, head+1)
				key = scope + "/alias:" + name
			} else if s.at(head, "fun") {
				nameAt, open := s.parameters(head+1, hi)
				if nameAt < 0 || open < 0 {
					return objectError("invalid_source", "unsupported Kotlin function header")
				}
				close := tokens[open].pair
				last, _ = s.functionEnd(close, hi)
				kind, name = "function", tokens[nameAt].text
				attrs = rustTokenKey(tokens, first, head+1) + "/receiver:" + rustTokenKey(tokens, head+1, nameAt)
				key = scope + "/function:" + name + rustTokenKey(tokens, head+1, close+1)
			} else {
				// Constructors, initializers and other script statements
				// remain opaque. Paired bodies never expose local declarations as targets.
				last = first
				for j := first; j < hi; j++ {
					if j > first && s.declaration(j) {
						break
					}
					if s.at(j, "=") || s.at(j, "by") {
						last = s.expression(j+1, hi)
						break
					}
					if s.at(j, ";") {
						last = j
						break
					}
					if j > first && s.lines[j] > s.ends[last] && (s.at(head, "package") || s.at(head, "import")) {
						break
					}
					if tokens[j].pair > j {
						j = tokens[j].pair
					}
					last = j
				}
				key = scope + "/opaque:" + rustTokenKey(tokens, first, last+1)
			}
			if last < first || last >= hi {
				return objectError("invalid_source", "unsupported Kotlin declaration boundary")
			}
			item := rustObjectItem{start: tokens[first].start, end: tokens[last].end, key: key, scope: scope, context: context, attributes: attrs, depth: depth}
			if kind != "" {
				line, column := rustBytePosition(starts, item.start)
				endLine, _ := rustBytePosition(starts, item.end-1)
				item.object = CodeObject{Protocol: "funcfinder.code/v1", Path: abs, Language: "kotlin", Kind: kind, Name: name, Line: line, Column: column, EndLine: endLine, Revision: revision, Source: strings.ReplaceAll(string(src[item.start:item.end]), "\r\n", "\n")}
				item.object.ID = codeObjectID(abs, "kotlin:"+kind, name, line, column)
			}
			items = append(items, item)
			if body >= 0 {
				child := body + 1
				nextContext := "type"
				childEnd := last
				if kind == "property" {
					childEnd++
					child = body
					nextContext = "property:" + tokens[head].text
				}
				for j := first; j < head; j++ {
					if s.at(j, "enum") {
						child = last
						for k := body + 1; k < last; k++ {
							if s.at(k, ";") {
								child = k + 1
								break
							}
							if tokens[k].pair > k {
								k = tokens[k].pair
							}
						}
						break
					}
				}
				if err := walk(child, childEnd, key, nextContext, depth+1); err != nil {
					return err
				}
			}
			first = last + 1
		}
		return nil
	}
	if err := walk(0, len(tokens), scope, context, 0); err != nil {
		return nil, err
	}
	return items, nil
}
func readKotlinCodeObject(path, kind, name string, line, column int, src []byte) (*CodeObject, error) {
	if (kind != "function" && kind != "type" && kind != "property" && kind != "accessor") || name == "" || line < 0 || column < 0 {
		return nil, objectError("invalid_selector", "select a Kotlin function/type/property/accessor and nonnegative coordinates")
	}
	items, err := kotlinCodeItems(path, src, "", "module")
	if err != nil {
		return nil, err
	}
	var matches []CodeObject
	for _, item := range items {
		o := item.object
		if o.Kind == kind && o.Name == name && (line == 0 || o.Line == line) && (column == 0 || o.Column == column) {
			matches = append(matches, o)
		}
	}
	if len(matches) == 0 {
		return nil, objectError("not_found", "no Kotlin declaration matches the selector")
	}
	if len(matches) > 1 {
		return nil, &ObjectError{Code: "ambiguous", Message: "select a candidate by physical coordinates", Candidates: matches}
	}
	return &matches[0], nil
}

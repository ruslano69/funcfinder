package internal

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"
)

func dObjectComment(src []byte, at int) (int, bool, error) {
	if bytes.HasPrefix(src[at:], []byte("//")) {
		i := at + 2
		for i < len(src) && src[i] != '\n' {
			i++
		}
		return i, true, nil
	}
	if bytes.HasPrefix(src[at:], []byte("/*")) {
		end := bytes.Index(src[at+2:], []byte("*/"))
		if end < 0 {
			return at, true, fmt.Errorf("unclosed comment at byte %d", at)
		}
		return at + end + 4, true, nil
	}
	if bytes.HasPrefix(src[at:], []byte("/+")) {
		i, nesting := at+2, 1
		for i < len(src) && nesting > 0 {
			if bytes.HasPrefix(src[i:], []byte("/+")) {
				nesting++
				i += 2
			} else if bytes.HasPrefix(src[i:], []byte("+/")) {
				nesting--
				i += 2
			} else {
				i++
			}
		}
		if nesting != 0 {
			return at, true, fmt.Errorf("unclosed nesting comment at byte %d", at)
		}
		return i, true, nil
	}
	return at, false, nil
}
func dObjectLiteral(src []byte, at, depth int) (int, bool, error) {
	if depth > 64 {
		return at, false, fmt.Errorf("D literal nesting exceeds 64")
	}
	if at >= len(src) {
		return at, false, nil
	}
	i, raw, quoted := at, false, false
	if bytes.HasPrefix(src[at:], []byte("q{")) {
		i, nesting := at+2, 1
		for i < len(src) && nesting > 0 {
			end, comment, err := dObjectComment(src, i)
			if err != nil {
				return at, true, err
			}
			if comment {
				i = end
				continue
			}
			end, literal, err := dObjectLiteral(src, i, depth+1)
			if err != nil {
				return at, true, err
			}
			if literal {
				i = end
				continue
			}
			if src[i] == '{' {
				nesting++
			}
			if src[i] == '}' {
				nesting--
			}
			i++
		}
		if nesting != 0 {
			return at, true, fmt.Errorf("unclosed token string at byte %d", at)
		}
		return i, true, nil
	}
	if bytes.HasPrefix(src[at:], []byte("q\"")) {
		i = at + 2
		if i == len(src) {
			return at, true, fmt.Errorf("unclosed delimited string")
		}
		open := src[i]
		if close := map[byte]byte{'(': ')', '[': ']', '{': '}', '<': '>'}[open]; close != 0 {
			i++
			nesting := 1
			for i < len(src) && nesting > 0 {
				if src[i] == open {
					nesting++
				}
				if src[i] == close {
					nesting--
				}
				i++
			}
			if nesting != 0 || i >= len(src) || src[i] != '"' {
				return at, true, fmt.Errorf("unclosed paired string at byte %d", at)
			}
			i++
		} else if r, _ := utf8.DecodeRune(src[i:]); unicode.IsLetter(r) || r == '_' {
			start := i
			for i < len(src) {
				r, n := utf8.DecodeRune(src[i:])
				if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' {
					break
				}
				i += n
			}
			label := string(src[start:i])
			if i < len(src) && src[i] == '\r' {
				i++
			}
			if i >= len(src) || src[i] != '\n' {
				return at, true, fmt.Errorf("delimited identifier string requires a newline")
			}
			marker := []byte("\n" + label + "\"")
			end := bytes.Index(src[i:], marker)
			if end < 0 {
				return at, true, fmt.Errorf("unclosed identifier string at byte %d", at)
			}
			i += end + len(marker)
		} else {
			i++
			marker := []byte{open, '"'}
			end := bytes.Index(src[i:], marker)
			if end < 0 {
				return at, true, fmt.Errorf("unclosed delimited string at byte %d", at)
			}
			i += end + 2
		}
		if i < len(src) && (src[i] == 'c' || src[i] == 'w' || src[i] == 'd') {
			i++
		}
		return i, true, nil
	}
	if (src[i] == 'r' || src[i] == 'x') && i+1 < len(src) && src[i+1] == '"' {
		raw = true
		i++
	}
	if src[i] == '`' {
		raw = true
	}
	if src[i] == '"' || src[i] == '\'' || src[i] == '`' {
		quoted = true
	}
	if !quoted {
		return at, false, nil
	}
	quote := src[i]
	i++
	for i < len(src) {
		if src[i] == quote {
			i++
			if quote != '\'' && i < len(src) && (src[i] == 'c' || src[i] == 'w' || src[i] == 'd') {
				i++
			}
			return i, true, nil
		}
		if !raw && src[i] == '\\' {
			i += 2
			continue
		}
		if quote == '\'' && (src[i] == '\n' || src[i] == '\r') {
			return at, true, fmt.Errorf("newline in character literal")
		}
		i++
	}
	return at, true, fmt.Errorf("unclosed literal at byte %d", at)
}
func dObjectTokens(src []byte) ([]rustObjectToken, error) {
	if !utf8.Valid(src) {
		return nil, fmt.Errorf("source must be UTF-8")
	}
	var tokens []rustObjectToken
	var stack []int
	for i := 0; i < len(src); {
		if i == 0 && bytes.HasPrefix(src, []byte(utf8BOM)) {
			i += 3
			continue
		}
		r, width := utf8.DecodeRune(src[i:])
		if unicode.IsSpace(r) {
			i += width
			continue
		}
		end, comment, err := dObjectComment(src, i)
		if err != nil {
			return nil, err
		}
		if comment {
			i = end
			continue
		}
		if src[i] == '#' {
			return nil, fmt.Errorf("D line directives are not supported by object editing")
		}
		start := i
		end, literal, err := dObjectLiteral(src, i, 0)
		if err != nil {
			return nil, err
		}
		if literal {
			i = end
		} else if unicode.IsLetter(r) || r == '_' {
			i += width
			for i < len(src) {
				r, width = utf8.DecodeRune(src[i:])
				if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' {
					break
				}
				i += width
			}
		} else {
			i += width
		}
		t := rustObjectToken{text: string(src[start:i]), start: start, end: i, pair: -1, literal: literal}
		index := len(tokens)
		tokens = append(tokens, t)
		if literal {
			continue
		}
		switch t.text {
		case "(", "[", "{":
			stack = append(stack, index)
		case ")", "]", "}":
			if len(stack) == 0 || tokens[stack[len(stack)-1]].text != map[string]string{")": "(", "]": "[", "}": "{"}[t.text] {
				return nil, fmt.Errorf("mismatched delimiter at byte %d", start)
			}
			open := stack[len(stack)-1]
			tokens[open].pair, tokens[index].pair = index, open
			stack = stack[:len(stack)-1]
		}
	}
	if len(stack) != 0 {
		return nil, fmt.Errorf("unclosed delimiter at byte %d", tokens[stack[0]].start)
	}
	return tokens, nil
}
func dObjectPrefix(tokens []rustObjectToken, start, hi int) int {
	i := start
	for i < hi {
		text := tokens[i].text
		if text == "@" {
			i++
			if i < hi && tokens[i].text != "(" {
				i++
			}
			if i < hi && tokens[i].text == "(" {
				i = tokens[i].pair + 1
			}
			continue
		}
		parameterized := text == "extern" || text == "deprecated" || text == "package" || text == "align" || text == "pragma"
		if parameterized && i+1 < hi && tokens[i+1].text == "(" {
			i = tokens[i+1].pair + 1
			continue
		}
		modifier := parameterized || text == "__gshared"
		for _, word := range dModifiers {
			if word == text && word != "auto" {
				modifier = true
			}
		}
		if !modifier || i+1 < hi && tokens[i+1].text == "(" {
			break
		}
		i++
	}
	return i
}
func dObjectSignature(tokens []rustObjectToken, open, template int) string {
	var key strings.Builder
	if template >= 0 {
		count := 1
		for i := template + 1; i < tokens[template].pair; i++ {
			if tokens[i].text == "," {
				count++
			}
			if tokens[i].pair > i {
				i = tokens[i].pair
			}
		}
		fmt.Fprintf(&key, "`%d", count)
	}
	key.WriteString("(")
	for i, close := open+1, tokens[open].pair; i < close; {
		end := i
		for end < close && tokens[end].text != "," {
			if tokens[end].pair > end {
				end = tokens[end].pair
			}
			end++
		}
		hi := end
		for j := i; j < end; j++ {
			if tokens[j].text == "=" {
				hi = j
				break
			}
			if tokens[j].pair > j {
				j = tokens[j].pair
			}
		}
		lo := i
		for lo < hi && (tokens[lo].text == "scope" || tokens[lo].text == "return" || tokens[lo].text == "in" || tokens[lo].text == "out" || tokens[lo].text == "ref" || tokens[lo].text == "lazy" || tokens[lo].text == "final") {
			if tokens[lo].text == "ref" || tokens[lo].text == "out" || tokens[lo].text == "lazy" {
				key.WriteString(tokens[lo].text + ":")
			}
			lo++
		}
		if hi-lo >= 2 && dIdentEnd(tokens[hi-1].text, 0) == len(tokens[hi-1].text) {
			previous := tokens[hi-2].text
			if previous != "." && previous != "!" && previous != "const" && previous != "immutable" && previous != "shared" && previous != "inout" {
				hi--
			}
		}
		key.WriteString(rustTokenKey(tokens, lo, hi))
		key.WriteString(";")
		i = end + 1
	}
	key.WriteString(")")
	return key.String()
}
func dObjectFunction(tokens []rustObjectToken, start, head, open, hi int) (string, int, string, string) {
	var header strings.Builder
	for i := head; i <= open; i++ {
		if i > head && tokens[i].start > tokens[i-1].end {
			header.WriteByte(' ')
		}
		if tokens[i].literal {
			header.WriteString("0")
		} else {
			header.WriteString(tokens[i].text)
		}
	}
	match := dHeaderSubmatch(header.String())
	if match == nil {
		return "", -1, "", ""
	}
	name := ExtractFuncName(match)
	if name == "function" || name == "delegate" || name == "" {
		return "", -1, "", ""
	}

	switch name {
	case "if", "while", "for", "foreach", "foreach_reverse", "switch", "catch", "with", "scope", "return", "assert", "__traits", "is", "typeof", "typeid", "version", "debug":
		return "", -1, "", ""
	}
	params, template := open, -1
	if tokens[open].pair+1 < hi && tokens[tokens[open].pair+1].text == "(" {
		template, params = open, tokens[open].pair+1
	}
	close := tokens[params].pair
	identity := rustTokenKey(tokens, start, head)
	key := dObjectSignature(tokens, params, template)
	if name == "this" || name == "~this" {
		for i := start; i < head; i++ {
			if tokens[i].text == "static" {
				key += ":static"
			}
			if tokens[i].text == "shared" {
				key += ":shared"
			}
		}
	}
	for i := close + 1; i < hi; {
		text := tokens[i].text
		if text == ";" {
			return name, i, key, identity
		}
		if text == "{" {
			return name, tokens[i].pair, key, identity
		}
		if text == "=" && i+1 < hi && tokens[i+1].text == ">" {
			for j := i + 2; j < hi; j++ {
				if tokens[j].text == ";" {
					return name, j, key, identity
				}
				if tokens[j].pair > j {
					j = tokens[j].pair
				}
			}
			return "", -1, "", ""
		}
		if text == "in" || text == "out" {
			i++
			if i < hi && tokens[i].text == "(" {
				i = tokens[i].pair + 1
			}
			if i < hi && tokens[i].text == "{" {
				i = tokens[i].pair + 1
			}
			continue
		}
		if text == "body" || text == "do" {
			i++
			continue
		}
		if text == "if" && i+1 < hi && tokens[i+1].text == "(" {
			end := tokens[i+1].pair
			key += ":constraint:" + rustTokenKey(tokens, i, end+1)
			i = end + 1
			continue
		}
		if text == "@" {
			end := i + 2
			if i+1 < hi && tokens[i+1].text == "(" {
				end = tokens[i+1].pair + 1
			} else if end < hi && tokens[end].text == "(" {
				end = tokens[end].pair + 1
			}
			identity += rustTokenKey(tokens, i, end)
			i = end
			continue
		}
		if text == "const" || text == "immutable" || text == "shared" || text == "inout" {
			key += ":" + text
		}
		identity += rustTokenKey(tokens, i, i+1)
		if text == "}" || text == "class" || text == "struct" {
			break
		}
		i++
	}
	return "", -1, "", ""
}
func dCodeItems(path string, src []byte, scope, context string) ([]rustObjectItem, error) {
	tokens, err := dObjectTokens(src)
	if err != nil {
		return nil, objectError("invalid_source", err.Error())
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, objectError("invalid_path", err.Error())
	}
	abs = filepath.Clean(abs)
	revision := codeRevision(src)
	lineStarts := []int{0}
	for i, ch := range src {
		if ch == '\n' {
			lineStarts = append(lineStarts, i+1)
		}
	}
	var items []rustObjectItem
	var walk func(int, int, string, string, int) error
	walk = func(lo, hi int, scope, context string, depth int) error {
		if depth > 128 {
			return objectError("invalid_source", "D scope nesting exceeds 128")
		}
		for start := lo; start < hi; {
			if tokens[start].text == ";" {
				start++
				continue
			}
			head := dObjectPrefix(tokens, start, hi)
			if head >= hi {
				return objectError("invalid_source", "attribute without a declaration")
			}
			word := tokens[head].text
			if word == "version" || word == "debug" || word == "if" || word == "mixin" {
				return objectError("invalid_source", "conditional/mixin declaration scopes are not supported by D object editing")
			}
			kind, name, end, body, identity, key := "", "", -1, -1, "", ""
			nextScope, nextContext := scope, context
			if head > start && tokens[head].text == ":" {
				// Access/attribute sections are outside individual objects. The
				// stable scope records the inherited declaration context.
				key = scope + "/section:" + rustTokenKey(tokens, start, head)
				items = append(items, rustObjectItem{start: tokens[start].start, end: tokens[head].end, key: key, scope: scope, context: context, depth: depth})
				scope = key
				start = head + 1
				continue
			}
			if head > start && tokens[head].text == "{" {
				body, end = head, tokens[head].pair
				key = scope + "/attributes:" + rustTokenKey(tokens, start, head)
				nextScope = key
			} else if word == "struct" || word == "class" || word == "interface" || word == "union" || word == "enum" || word == "template" {
				nameAt := head + 1
				if nameAt < hi && dIdentEnd(tokens[nameAt].text, 0) == len(tokens[nameAt].text) {
					name = tokens[nameAt].text
				}
				for j := nameAt; j < hi; j++ {
					if tokens[j].text == "(" || tokens[j].text == "[" {
						j = tokens[j].pair
						continue
					}
					if tokens[j].text == "=" {
						break
					}
					if tokens[j].text == "{" {
						body, end = j, tokens[j].pair
						break
					}
					if tokens[j].text == ";" {
						end = j
						break
					}
				}
				if end >= 0 {
					if name != "" {
						kind = "type"
					}
					identity = rustTokenKey(tokens, start, head+1)
					arity := 0
					if nameAt+1 < hi && tokens[nameAt+1].text == "(" {
						arity = 1
						for j := nameAt + 2; j < tokens[nameAt+1].pair; j++ {
							if tokens[j].text == "," {
								arity++
							}
							if tokens[j].pair > j {
								j = tokens[j].pair
							}
						}
					}
					key = scope + "/" + word + ":" + name + fmt.Sprintf("`%d", arity)
					nextScope = key
					nextContext = "type:" + name
					if word == "enum" {
						nextContext = "opaque"
					}
					if word == "template" {
						kind, nextContext = "", "module"
					}
				}
			} else if word == "alias" {
				for j := head + 1; j < hi; j++ {
					if tokens[j].text == "=" && j == head+2 {
						name = tokens[head+1].text
					}
					if tokens[j].text == ";" {
						end = j
						break
					}
					if tokens[j].pair > j {
						j = tokens[j].pair
					}
				}
				if name != "" && dIdentEnd(name, 0) == len(name) {
					kind, identity, key = "type", rustTokenKey(tokens, start, head+1), scope+"/alias:"+name
				}
			}
			if end < 0 {
				for j := head; j < hi; j++ {
					if tokens[j].literal {
						continue
					}
					if tokens[j].text == "(" {
						fn, last, signature, attrs := dObjectFunction(tokens, start, head, j, hi)
						if fn != "" && last >= j {
							kind, name, end, identity = "function", fn, last, attrs
							key = scope + "/function:" + name + signature
							break
						}
						j = tokens[j].pair
						continue
					}
					if tokens[j].text == "[" {
						j = tokens[j].pair
						continue
					}
					if tokens[j].text == ";" {
						end = j
						break
					}
					if tokens[j].text == "{" {
						end = tokens[j].pair
						break
					}
					if tokens[j].text == "=" {
						for j++; j < hi; j++ {
							if tokens[j].text == ";" {
								end = j
								break
							}
							if tokens[j].pair > j {
								j = tokens[j].pair
							}
						}
						break
					}
				}
			}
			if end < start || end >= hi {
				return objectError("invalid_source", "unsupported or incomplete D declaration")
			}
			if key == "" {
				key = scope + "/opaque:" + rustTokenKey(tokens, start, end+1)
			}
			item := rustObjectItem{start: tokens[start].start, end: tokens[end].end, key: key, scope: scope, context: context, depth: depth, attributes: identity}
			if kind != "" {
				line, column := rustBytePosition(lineStarts, item.start)
				endLine, _ := rustBytePosition(lineStarts, item.end-1)
				item.object = CodeObject{Protocol: "funcfinder.code/v1", Path: abs, Language: "d", Kind: kind, Name: name, Line: line, Column: column, EndLine: endLine, Revision: revision, Source: strings.ReplaceAll(string(src[item.start:item.end]), "\r\n", "\n")}
				item.object.ID = codeObjectID(abs, "d:"+kind, name, line, column)
			}
			items = append(items, item)
			if body >= 0 && nextContext != "opaque" {
				if err := walk(body+1, end, nextScope, nextContext, depth+1); err != nil {
					return err
				}
			}
			start = end + 1
		}
		return nil
	}
	if err := walk(0, len(tokens), scope, context, 0); err != nil {
		return nil, err
	}
	return items, nil
}
func readDCodeObject(path, kind, name string, line, column int, src []byte) (*CodeObject, error) {
	if (kind != "function" && kind != "type") || name == "" || line < 0 || column < 0 {
		return nil, objectError("invalid_selector", "select a function/type name and nonnegative coordinates")
	}
	items, err := dCodeItems(path, src, "", "module")
	if err != nil {
		return nil, err
	}
	var matches []CodeObject
	for _, s := range items {
		o := s.object
		if o.Kind == kind && o.Name == name && (line == 0 || o.Line == line) && (column == 0 || o.Column == column) {
			matches = append(matches, o)
		}
	}
	if len(matches) == 0 {
		return nil, objectError("not_found", "no D declaration matches the selector")
	}
	if len(matches) > 1 {
		return nil, &ObjectError{Code: "ambiguous", Message: "select a candidate by coordinates or edit its object directly", Candidates: matches}
	}
	return &matches[0], nil
}

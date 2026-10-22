package internal

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"
)

func csharpObjectLiteral(src []byte, at, depth int) (int, bool, error) {
	if depth > 64 {
		return at, false, fmt.Errorf("literal nesting exceeds 64 at byte %d", at)
	}
	i, dollars, verbatim := at, 0, false
	for i < len(src) && (src[i] == '$' || src[i] == '@') {
		if src[i] == '$' {
			dollars++
		} else {
			if verbatim {
				return at, false, nil
			}
			verbatim = true
		}
		i++
	}
	if i >= len(src) || (src[i] != '"' && !(i == at && src[i] == '\'')) {
		return at, false, nil
	}
	quote := src[i]
	quotes := 1
	for i+quotes < len(src) && src[i+quotes] == quote {
		quotes++
	}
	raw := quote == '"' && quotes >= 3 && !verbatim
	if !raw {
		quotes = 1
	}
	i += quotes
	for i < len(src) {
		if dollars > 0 && src[i] == '{' {
			run := 1
			for i+run < len(src) && src[i+run] == '{' {
				run++
			}
			if !raw && run >= 2 {
				i += 2
				continue
			}
			if !raw || run >= dollars {
				need := 1
				if raw {
					need = dollars
				}
				i += run
				braces, closed := 0, false
				for i < len(src) {
					if bytes.HasPrefix(src[i:], []byte("//")) {
						for i < len(src) && src[i] != '\n' {
							i++
						}
						continue
					}
					if bytes.HasPrefix(src[i:], []byte("/*")) {
						end := bytes.Index(src[i+2:], []byte("*/"))
						if end < 0 {
							return at, true, fmt.Errorf("unclosed interpolation comment at byte %d", i)
						}
						i += end + 4
						continue
					}
					end, literal, err := csharpObjectLiteral(src, i, depth+1)
					if err != nil {
						return at, true, err
					}
					if literal {
						i = end
						continue
					}
					if src[i] == '{' {
						braces++
					}
					if src[i] == '}' {
						if braces == 0 {
							run := 1
							for i+run < len(src) && src[i+run] == '}' {
								run++
							}
							if run < need {
								return at, true, fmt.Errorf("short interpolation close at byte %d", i)
							}
							i += need
							closed = true
							break
						}
						braces--
					}
					i++
				}
				if !closed {
					return at, true, fmt.Errorf("unclosed interpolation at byte %d", at)
				}
				continue
			}
		}
		if src[i] == quote {
			if raw {
				run := 1
				for i+run < len(src) && src[i+run] == quote {
					run++
				}
				if run < quotes {
					i += run
					continue
				}
				i += quotes
			} else {
				if verbatim && i+1 < len(src) && src[i+1] == '"' {
					i += 2
					continue
				}
				i++
			}
			if quote == '"' && bytes.HasPrefix(src[i:], []byte("u8")) {
				i += 2
			}
			return i, true, nil
		}
		if !raw && !verbatim && src[i] == '\\' {
			i += 2
			continue
		}
		if !raw && !verbatim && (src[i] == '\n' || src[i] == '\r') {
			return at, true, fmt.Errorf("newline in literal at byte %d", at)
		}
		i++
	}
	return at, true, fmt.Errorf("unclosed literal at byte %d", at)
}
func csharpObjectTokens(src []byte) ([]rustObjectToken, error) {
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
		if bytes.HasPrefix(src[i:], []byte("//")) {
			for i < len(src) && src[i] != '\n' {
				i++
			}
			continue
		}
		if bytes.HasPrefix(src[i:], []byte("/*")) {
			end := bytes.Index(src[i+2:], []byte("*/"))
			if end < 0 {
				return nil, fmt.Errorf("unclosed comment at byte %d", i)
			}
			i += end + 4
			continue
		}
		if src[i] == '#' {
			start := i
			for i < len(src) && src[i] != '\n' {
				i++
			}
			directive := strings.Fields(string(src[start+1 : i]))
			if len(directive) == 0 {
				return nil, fmt.Errorf("empty directive")
			}
			switch directive[0] {
			case "nullable", "pragma", "region", "endregion", "line":
				// These do not select declaration branches. Preserve their bytes.
			default:
				return nil, fmt.Errorf("conditional/unsupported directive #%s is not editable", directive[0])
			}

			tokens = append(tokens, rustObjectToken{text: string(src[start:i]), start: start, end: i, pair: -1, literal: true})
			continue
		}
		start := i
		end, literal, err := csharpObjectLiteral(src, i, 0)
		if err != nil {
			return nil, err
		}
		if literal {
			i = end
		} else if unicode.IsLetter(r) || r == '_' || r == '@' {
			i += width
			for i < len(src) {
				r, width = utf8.DecodeRune(src[i:])
				if !unicode.IsLetter(r) && !unicode.IsDigit(r) && !unicode.IsMark(r) && r != '_' {
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
func csharpSignature(tokens []csToken, pairs []int, start, open int) string {
	var key strings.Builder
	// Generic arity is part of an overload's identity; parameter names/defaults are not.
	if open > start && tokens[open-1].text == ">" {
		depth, count := 0, 1
		for i := open - 1; i >= start; i-- {
			if tokens[i].text == ">" {
				depth++
			}
			if tokens[i].text == "<" {
				depth--
				if depth == 0 {
					break
				}
			}
			if depth == 1 && tokens[i].text == "," {
				count++
			}
		}
		fmt.Fprintf(&key, "`%d", count)
	}
	close := pairs[open]
	key.WriteString("(")
	for i := open + 1; i < close; {
		end, generic := i, 0
		for end < close {
			text := tokens[end].text
			if text == "<" {
				generic++
			}
			if text == ">" {
				generic--
			}
			if text == "," && generic == 0 {
				break
			}
			if (text == "(" || text == "[" || text == "{") && pairs[end] > end {
				end = pairs[end]
			}
			end++
		}
		lo := csSkipAttributes(tokens, pairs, i, end)
		byref := false
		for lo < end {
			t := tokens[lo].text
			if t == "ref" || t == "out" || t == "in" {
				byref = true
				lo++
				continue
			}
			if t == "params" || t == "this" || t == "scoped" {
				lo++
				continue
			}
			break
		}
		hi := end
		for j := lo; j < end; j++ {
			if tokens[j].text == "=" {
				hi = j
				break
			}
			if (tokens[j].text == "(" || tokens[j].text == "[") && pairs[j] > j {
				j = pairs[j]
			}
		}
		if hi > lo && csIdentifier(tokens[hi-1].text) {
			hi--
		}
		if byref {
			key.WriteString("ref:")
		}
		for _, token := range tokens[lo:hi] {
			key.WriteString(token.text)
		}
		key.WriteString(";")
		i = end + 1
	}
	key.WriteString(")")
	return key.String()
}
func csharpCodeItems(path string, src []byte, scope, context string) ([]rustObjectItem, error) {
	lexed, err := csharpObjectTokens(src)
	if err != nil {
		return nil, objectError("invalid_source", err.Error())
	}
	tokens, pairs := make([]csToken, len(lexed)), make([]int, len(lexed))
	for i, token := range lexed {
		tokens[i] = csToken{token.text, 0}
		pairs[i] = token.pair
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
			return objectError("invalid_source", "C# scope nesting exceeds 128")
		}
		for start := lo; start < hi; {
			if tokens[start].text == ";" {
				start++
				continue
			}

			if strings.HasPrefix(tokens[start].text, "#") {
				items = append(items, rustObjectItem{start: lexed[start].start, end: lexed[start].end, scope: scope, context: context, depth: depth, key: scope + "/directive:" + tokens[start].text})
				start++
				continue
			}
			if start+2 < hi && tokens[start].text == "[" && (tokens[start+1].text == "assembly" || tokens[start+1].text == "module") && tokens[start+2].text == ":" {
				return objectError("invalid_source", "assembly/module attributes are not supported by the C# editor")
			}
			head := csSkipAttributes(tokens, pairs, start, hi)
			prefix := head
			for prefix < hi && (csModifiers[tokens[prefix].text] || tokens[prefix].text == "file") {
				prefix++
			}
			kind, name, body, end, identity, key := "", "", -1, -1, "", ""
			nextScope, nextContext := scope, context
			if prefix < hi && tokens[prefix].text == "namespace" {
				if context != "module" {
					return objectError("invalid_source", "namespace inside a type")
				}
				j := prefix + 1
				var namespace strings.Builder
				for j < hi && tokens[j].text != "{" && tokens[j].text != ";" {
					namespace.WriteString(tokens[j].text)
					j++
				}
				if j == hi || namespace.Len() == 0 {
					return objectError("invalid_source", "incomplete namespace")
				}
				nextScope = scope + "/namespace:" + namespace.String()
				key = "namespace:" + nextScope
				if tokens[j].text == ";" {
					end = j
					scope = nextScope
				} else {
					body, end = j, pairs[j]
				}
			} else if prefix+1 < hi && (tokens[prefix].text == "class" || tokens[prefix].text == "struct" || tokens[prefix].text == "interface" || tokens[prefix].text == "record" || tokens[prefix].text == "enum") {
				nameAt := prefix + 1
				if tokens[prefix].text == "record" && nameAt < hi && (tokens[nameAt].text == "class" || tokens[nameAt].text == "struct") {
					nameAt++
				}
				if nameAt >= hi || !csIdentifier(tokens[nameAt].text) {
					return objectError("invalid_source", "type name is missing")
				}
				kind, name = "type", strings.TrimPrefix(tokens[nameAt].text, "@")
				for j := nameAt + 1; j < hi; j++ {
					if tokens[j].text == "(" || tokens[j].text == "[" {
						if pairs[j] < j {
							break
						}
						j = pairs[j]
						continue
					}
					if tokens[j].text == "{" {
						body, end = j, pairs[j]
						break
					}
					if tokens[j].text == ";" {
						end = j
						break
					}
				}
				identity = rustTokenKey(lexed, start, head) + ":" + tokens[prefix].text
				if tokens[prefix].text == "record" && nameAt > prefix+1 {
					identity += ":" + tokens[prefix+1].text
				}

				arity := 0
				if nameAt+1 < hi && tokens[nameAt+1].text == "<" {
					arity = 1
					generic := 1
					for j := nameAt + 2; j < hi && generic > 0; j++ {
						if tokens[j].text == "<" {
							generic++
						}
						if tokens[j].text == ">" {
							generic--
						}
						if tokens[j].text == "," && generic == 1 {
							arity++
						}
					}
				}
				key = scope + "/type:" + name + fmt.Sprintf("`%d", arity)
				nextScope, nextContext = key, "type:"+name
				if tokens[prefix].text == "enum" {
					nextContext = "opaque"
				}
			} else {
				for j := head; j < hi; j++ {
					if lexed[j].literal {
						continue
					}
					if tokens[j].text == "[" {
						j = pairs[j]
						continue
					}
					if tokens[j].text == "(" {
						className := strings.TrimPrefix(context, "type:")
						method, _ := csFunctionName(tokens, pairs, start, j, className)
						if method != "" && strings.HasPrefix(context, "type:") {
							end, _ = csFunctionEnd(tokens, pairs, pairs[j])
							kind, name = "function", method
							identity = rustTokenKey(lexed, start, head)
							// Preserve explicit interface qualification and constructor category.
							returnAt := head
							for returnAt < j && csModifiers[tokens[returnAt].text] {
								returnAt++
							}
							qualifier := ""
							if name == className || name == "~"+className {
								qualifier = "constructor"
								for k := head; k < returnAt; k++ {
									if tokens[k].text == "static" {
										qualifier = "static-constructor"
									}
								}
							} else if !strings.Contains(name, "operator ") {
								typeEnd := csSkipType(tokens, pairs, returnAt, j)
								for k := typeEnd; k < j && tokens[k].text != "<"; k++ {
									qualifier += tokens[k].text
								}
							}
							identity += ":" + qualifier
							key = scope + "/function:" + name + ":" + qualifier + csharpSignature(tokens, pairs, head, j)
							break
						}
						j = pairs[j]
						continue
					}
					if tokens[j].text == "{" {
						end = pairs[j]
						break
					}
					if tokens[j].text == ";" {
						end = j
						break
					}
					if tokens[j].text == "=" {
						for j++; j < hi; j++ {
							if tokens[j].text == ";" {
								end = j
								break
							}
							if tokens[j].text == "(" || tokens[j].text == "[" || tokens[j].text == "{" {
								j = pairs[j]
							}
						}
						break
					}
				}
				if end >= 0 && kind == "" && end+1 < hi && tokens[end+1].text == "=" {
					for j := end + 2; j < hi; j++ {
						if tokens[j].text == ";" {
							end = j
							break
						}
						if tokens[j].text == "(" || tokens[j].text == "[" || tokens[j].text == "{" {
							j = pairs[j]
						}
					}
				}
			}
			if end < start || end >= hi {
				return objectError("invalid_source", "unsupported or incomplete C# declaration")
			}
			if key == "" {
				key = scope + "/opaque:" + rustTokenKey(lexed, start, end+1)
			}
			item := rustObjectItem{start: lexed[start].start, end: lexed[end].end, scope: scope, context: context, depth: depth, key: key, attributes: identity}
			if kind != "" {
				line, column := rustBytePosition(lineStarts, item.start)
				endLine, _ := rustBytePosition(lineStarts, item.end-1)
				item.object = CodeObject{Protocol: "funcfinder.code/v1", Path: abs, Language: "cs", Kind: kind, Name: name, Line: line, Column: column, EndLine: endLine, Revision: revision, Source: strings.ReplaceAll(string(src[item.start:item.end]), "\r\n", "\n")}
				item.object.ID = codeObjectID(abs, "cs:"+kind, name, line, column)
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
func readCSharpCodeObject(path, kind, name string, line, column int, src []byte) (*CodeObject, error) {
	if (kind != "function" && kind != "type") || name == "" || line < 0 || column < 0 {
		return nil, objectError("invalid_selector", "select a function/type name and nonnegative coordinates")
	}
	items, err := csharpCodeItems(path, src, "", "module")
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
		return nil, objectError("not_found", "no C# declaration matches the selector")
	}
	if len(matches) > 1 {
		return nil, &ObjectError{Code: "ambiguous", Message: "select a candidate by coordinates or edit its object directly", Candidates: matches}
	}
	return &matches[0], nil
}

package internal

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"
)

type rustObjectToken struct {
	text             string
	start, end, pair int
	literal          bool
}
type rustObjectItem struct {
	object                          CodeObject
	start, end                      int
	key, scope, context, attributes string
	depth                           int
}

// Rust object recognition operates on byte-preserving tokens, not cleaned
// lines. It is a conservative item-structure parser, not compiler validation.
func rustObjectTokens(src []byte) ([]rustObjectToken, error) {
	var tokens []rustObjectToken
	var stack []int
	for i := 0; i < len(src); {
		if i == 0 && bytes.HasPrefix(src, []byte(utf8BOM)) {
			i += 3
			continue
		}
		r, w := utf8.DecodeRune(src[i:])
		if r == utf8.RuneError && w == 1 {
			return nil, fmt.Errorf("invalid UTF-8 at byte %d", i)
		}
		if unicode.IsSpace(r) {
			i += w
			continue
		}
		if i == 0 && bytes.HasPrefix(src, []byte("#!")) && len(src) > 2 && src[2] != '[' {
			for i < len(src) && src[i] != '\n' {
				i++
			}
			continue
		}
		if bytes.HasPrefix(src[i:], []byte("//")) {
			for i < len(src) && src[i] != '\n' {
				i++
			}
			continue
		}
		if bytes.HasPrefix(src[i:], []byte("/*")) {
			start := i
			i += 2
			depth := 1
			for i < len(src) && depth > 0 {
				if bytes.HasPrefix(src[i:], []byte("/*")) {
					depth++
					i += 2
				} else if bytes.HasPrefix(src[i:], []byte("*/")) {
					depth--
					i += 2
				} else {
					i++
				}
			}
			if depth != 0 {
				return nil, fmt.Errorf("unclosed comment at byte %d", start)
			}
			continue
		}
		start := i
		literal := false
		// raw strings: r, br, cr followed by optional hashes and a quote.
		p := i
		if src[p] == 'b' || src[p] == 'c' {
			p++
		}
		if p < len(src) && src[p] == 'r' {
			p++
			hash := p
			for p < len(src) && src[p] == '#' {
				p++
			}
			if p < len(src) && src[p] == '"' {
				close := []byte("\"" + strings.Repeat("#", p-hash))
				n := bytes.Index(src[p+1:], close)
				if n < 0 {
					return nil, fmt.Errorf("unclosed raw string at byte %d", i)
				}
				i = p + 1 + n + len(close)
				literal = true
			}
		}
		if !literal {
			p = i
			if (src[p] == 'b' || src[p] == 'c') && p+1 < len(src) && (src[p+1] == '"' || src[p+1] == '\'') {
				p++
			}
			if src[p] == '"' {
				p++
				for p < len(src) && src[p] != '"' {
					if src[p] == '\\' {
						p++
					}
					p++
				}
				if p >= len(src) {
					return nil, fmt.Errorf("unclosed string at byte %d", i)
				}
				i = p + 1
				literal = true
			} else if src[p] == '\'' {
				q := p + 1
				if q < len(src) {
					if src[q] == '\\' {
						q++
						if q < len(src) && src[q] == 'u' {
							q++
							if q < len(src) && src[q] == '{' {
								for q < len(src) && src[q] != '}' {
									q++
								}
								q++
							}
						} else if q < len(src) && src[q] == 'x' {
							q += 3
						} else {
							q++
						}
					} else {
						_, n := utf8.DecodeRune(src[q:])
						q += n
					}
				}
				if q < len(src) && src[q] == '\'' {
					i = q + 1
					literal = true
				}
			}
		}
		if !literal {
			if bytes.HasPrefix(src[i:], []byte("r#")) && i+2 < len(src) {
				i += 2
				r, w = utf8.DecodeRune(src[i:])
			}
			if unicode.IsLetter(r) || r == '_' || r == '\'' {
				i += w
				for i < len(src) {
					r, w = utf8.DecodeRune(src[i:])
					if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' && !unicode.IsMark(r) {
						break
					}
					i += w
				}
			} else {
				i = start + w
			}
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
			if len(stack) == 0 {
				return nil, fmt.Errorf("unmatched delimiter at byte %d", start)
			}
			open := stack[len(stack)-1]
			want := map[string]string{")": "(", "]": "[", "}": "{"}[t.text]
			if tokens[open].text != want {
				return nil, fmt.Errorf("mismatched delimiter at byte %d", start)
			}
			tokens[open].pair = index
			tokens[index].pair = open
			stack = stack[:len(stack)-1]
		}
	}
	if len(stack) > 0 {
		return nil, fmt.Errorf("unclosed delimiter at byte %d", tokens[stack[0]].start)
	}
	return tokens, nil
}

func rustTokenKey(tokens []rustObjectToken, lo, hi int) string {
	var b strings.Builder
	for _, t := range tokens[lo:hi] {
		fmt.Fprintf(&b, "%d:%s", len(t.text), t.text)
	}
	return b.String()
}

type rustItemParser struct {
	tokens []rustObjectToken
	items  []rustObjectItem
}

func (p *rustItemParser) itemsIn(lo, hi int, scope, context string, depth int) error {
	t := p.tokens
	sawItem := false
	for i := lo; i < hi; {
		if t[i].text == ";" {
			i++
			continue
		}
		start := i
		for i < hi && t[i].text == "#" {
			i++
			inner := false
			if i < hi && t[i].text == "!" {
				inner = true
				i++
			}
			if inner && sawItem {
				return fmt.Errorf("inner attribute after an item at byte %d", t[start].start)
			}
			if i >= hi || t[i].text != "[" {
				return fmt.Errorf("unsupported attribute at byte %d", t[start].start)
			}
			i = t[i].pair + 1
			if inner {
				start = i
			}
		}
		if i >= hi {
			break
		}
		attrs := rustTokenKey(t, start, i)
		for i < hi {
			word := t[i].text
			if word == "pub" {
				i++
				if i < hi && t[i].text == "(" {
					i = t[i].pair + 1
				}
				continue
			}
			if word == "async" || word == "unsafe" || word == "safe" || word == "default" || word == "auto" || (word == "const" && i+1 < hi && (t[i+1].text == "fn" || t[i+1].text == "async" || t[i+1].text == "unsafe")) {
				i++
				continue
			}
			if word == "extern" && i+1 < hi && (t[i+1].literal || t[i+1].text == "fn" || t[i+1].text == "{") {
				i++
				if i < hi && t[i].literal {
					i++
				}
				continue
			}
			break
		}
		if i >= hi {
			return fmt.Errorf("incomplete item at byte %d", t[start].start)
		}
		keyword := t[i].text
		if keyword == "{" {
			isExtern := false
			for q := start; q < i; q++ {
				if t[q].text == "extern" {
					isExtern = true
				}
			}
			if !isExtern {
				return fmt.Errorf("block outside an extern item at byte %d", t[i].start)
			}
		}
		name := ""
		body := -1
		end := -1
		kind := ""
		category := keyword
		if keyword == "fn" {
			if i+1 >= hi || !rustObjectName(t[i+1]) {
				return fmt.Errorf("missing function name at byte %d", t[i].start)
			}
			name = t[i+1].text
			kind = "function"
			angle := 0
			parameters := false
			for j := i + 2; j < hi; j++ {
				switch t[j].text {
				case "<":
					angle++
				case ">":
					if angle > 0 && (j == 0 || t[j-1].text != "-") {
						angle--
					}
				case "(":
					if angle == 0 && !parameters {
						parameters = true
					}
					j = t[j].pair
				case "[":
					j = t[j].pair
				case "{":
					if parameters && angle == 0 {
						body = j
						end = t[j].pair + 1
					} else {
						j = t[j].pair
					}
				case ";":
					if angle == 0 {
						end = j + 1
					}
				}
				if end >= 0 {
					break
				}
			}
			if !parameters {
				return fmt.Errorf("missing parameter list for %s", name)
			}
		} else if keyword == "struct" || keyword == "enum" || keyword == "union" || keyword == "trait" || keyword == "type" || keyword == "mod" || keyword == "impl" || keyword == "{" {
			if keyword != "impl" && keyword != "{" {
				if i+1 >= hi || !rustObjectName(t[i+1]) {
					return fmt.Errorf("missing item name")
				}
				name = t[i+1].text
			}
			if keyword != "mod" && keyword != "impl" && keyword != "{" {
				kind = "type"
			}
			angle := 0
			for j := i + 1; j < hi; j++ {
				if keyword == "{" {
					body = i
					end = t[i].pair + 1
					break
				}
				switch t[j].text {
				case "<":
					angle++
				case ">":
					if angle > 0 && (j == 0 || t[j-1].text != "-") {
						angle--
					}
				case "(", "[":
					j = t[j].pair
				case "{":
					if angle == 0 && keyword != "type" {
						body = j
						end = t[j].pair + 1
					} else {
						j = t[j].pair
					}
				case ";":
					if angle == 0 {
						end = j + 1
					}
				}
				if end >= 0 {
					break
				}
			}
			if keyword == "struct" && body < 0 && end < 0 {
				return fmt.Errorf("unterminated struct")
			}
		} else {
			// Keep opaque non-editable items in the catalogue. Macro token trees
			// are not Rust item lists and must never expose generated functions.
			macro := false
			for j := i; j < hi; j++ {
				if t[j].text == "!" {
					macro = true
				}
				if t[j].text == ";" {
					end = j + 1
					break
				}
				if t[j].text == "(" || t[j].text == "[" || t[j].text == "{" {
					close := t[j].pair
					if macro && t[j].text == "{" {
						end = close + 1
						if end < hi && t[end].text == ";" {
							end++
						}
						break
					}
					j = close
				}
			}
			if end < 0 {
				return fmt.Errorf("unsupported or unterminated item at byte %d", t[start].start)
			}
		}
		if end < 0 || end > hi {
			return fmt.Errorf("unterminated %s item at byte %d", keyword, t[start].start)
		}
		if (context == "impl" || context == "trait" || context == "extern") && kind == "type" && category != "type" {
			return fmt.Errorf("unsupported associated type item %s", category)
		}
		if keyword == "fn" && body < 0 && context != "trait" && context != "extern" {
			return fmt.Errorf("bodyless function outside trait/extern")
		}
		item := rustObjectItem{start: t[start].start, end: t[end-1].end, scope: scope, context: context, attributes: attrs, depth: depth}
		if kind != "" {
			item.object.Kind = kind
			item.object.Name = name
			item.key = scope + "/" + kind + ":" + name
		} else {
			item.key = scope + "/opaque:" + rustTokenKey(t, start, end)
		}
		container := body >= 0 && (keyword == "mod" || keyword == "impl" || keyword == "trait" || keyword == "{")
		childScope := scope + "/" + keyword + ":" + name
		if container {
			if keyword == "impl" || keyword == "{" {
				childScope = scope + "/" + keyword + ":" + rustTokenKey(t, i, body)
			}
			item.key = scope + "/container:" + rustTokenKey(t, start, body)
		}
		p.items = append(p.items, item)
		sawItem = true
		if container {
			childContext := keyword
			if keyword == "{" {
				childContext = "extern"
			}
			if err := p.itemsIn(body+1, t[body].pair, childScope, childContext, depth+1); err != nil {
				return err
			}
		}
		i = end
	}
	return nil
}

func rustObjectName(t rustObjectToken) bool {
	if t.literal || t.text == "" {
		return false
	}
	s := strings.TrimPrefix(t.text, "r#")
	r, _ := utf8.DecodeRuneInString(s)
	return r == '_' || unicode.IsLetter(r)
}

func rustCodeItems(path string, src []byte, scope, context string) ([]rustObjectItem, error) {
	tokens, err := rustObjectTokens(src)
	if err != nil {
		return nil, objectError("invalid_source", err.Error())
	}
	p := rustItemParser{tokens: tokens}
	if err = p.itemsIn(0, len(tokens), scope, context, 0); err != nil {
		return nil, objectError("invalid_source", err.Error())
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, objectError("invalid_path", err.Error())
	}
	abs = filepath.Clean(abs)
	revision := codeRevision(src)
	lineStarts := []int{0}
	for i, b := range src {
		if b == '\n' {
			lineStarts = append(lineStarts, i+1)
		}
	}
	for i := range p.items {
		s := &p.items[i]
		if s.object.Kind == "" {
			continue
		}
		line, col := rustBytePosition(lineStarts, s.start)
		endLine, _ := rustBytePosition(lineStarts, s.end-1)
		s.object = CodeObject{Protocol: "funcfinder.code/v1", Path: abs, Language: "rust", Kind: s.object.Kind, Name: s.object.Name, Line: line, Column: col, EndLine: endLine, Revision: revision, Source: strings.ReplaceAll(string(src[s.start:s.end]), "\r\n", "\n")}
		s.object.ID = codeObjectID(abs, "rust:"+s.object.Kind, s.object.Name, line, col)
	}
	return p.items, nil
}

func rustBytePosition(starts []int, offset int) (int, int) {
	lo, hi := 0, len(starts)
	for lo+1 < hi {
		m := (lo + hi) / 2
		if starts[m] <= offset {
			lo = m
		} else {
			hi = m
		}
	}
	return lo + 1, offset - starts[lo] + 1
}

func readRustCodeObject(path, kind, name string, line, column int, src []byte) (*CodeObject, error) {
	if (kind != "function" && kind != "type") || name == "" || line < 0 || column < 0 {
		return nil, objectError("invalid_selector", "select a function/type name and nonnegative coordinates")
	}
	items, err := rustCodeItems(path, src, "", "module")
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
		return nil, objectError("not_found", "no Rust declaration matches the selector")
	}
	if len(matches) > 1 {
		return nil, &ObjectError{Code: "ambiguous", Message: "select a candidate by coordinates or edit its object directly", Candidates: matches}
	}
	return &matches[0], nil
}

package internal

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// gooseLex keeps physical byte spans. Navigation accepts a partial token stream;
// object editing refuses unterminated literals/comments and mismatched groups.
// Angle brackets are parsed in type contexts, never paired as expression groups.
func gooseLex(src []byte, strict bool) ([]rustObjectToken, error) {
	var ts []rustObjectToken
	var stack []int
	fail := func(at int, message string) ([]rustObjectToken, error) {
		if strict {
			return nil, fmt.Errorf("%s at byte %d", message, at)
		}
		return ts, nil
	}
	if strict && (!utf8.Valid(src) || bytes.IndexByte(src, 0) >= 0) {
		return fail(0, "invalid UTF-8 or NUL in source")
	}
	for i := 0; i < len(src); {
		if i == 0 && bytes.HasPrefix(src, []byte(utf8BOM)) {
			i += 3
			continue
		}
		if src[i] <= ' ' {
			i++
			continue
		}
		if bytes.HasPrefix(src[i:], []byte("//")) {
			for i < len(src) && src[i] != '\n' {
				i++
			}
			continue
		}
		start := i
		if bytes.HasPrefix(src[i:], []byte("/*")) {
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
				return fail(start, "unclosed comment")
			}
			continue
		}
		literal := false
		if bytes.HasPrefix(src[i:], []byte(`"""`)) {
			n := bytes.Index(src[i+3:], []byte(`"""`))
			if n < 0 {
				return fail(start, "unclosed triple string")
			}
			i += 3 + n + 3
			literal = true
		} else if src[i] == '"' || src[i] == '\'' {
			quote := src[i]
			i++
			closed := false
			for i < len(src) && src[i] != '\n' {
				c := src[i]
				i++
				if c == quote {
					closed = true
					break
				}
				if c == '\\' {
					if i >= len(src) || src[i] == '\n' {
						break
					}
					i++
				}
			}
			if !closed && strict {
				return fail(start, "unclosed string or character")
			}
			literal = true
		} else if gooseIdent(string(src[i])) {
			i++
			for i < len(src) && (gooseIdent(string(src[i])) || src[i] >= '0' && src[i] <= '9') {
				i++
			}
		} else {
			i++
			if i < len(src) && (src[start] == ':' && src[i] == ':' || src[start] == '-' && src[i] == '>' || src[start] == '.' && src[i] == '.') {
				i++
			}
		}
		t := rustObjectToken{text: string(src[start:i]), start: start, end: i, pair: -1, literal: literal}
		if !literal {
			switch t.text {
			case "(", "[", "{":
				stack = append(stack, len(ts))
			case ")", "]", "}":
				if len(stack) == 0 {
					if strict {
						return fail(start, "unexpected closing bracket")
					}
				} else {
					j := stack[len(stack)-1]
					if ts[j].text+t.text != "()" && ts[j].text+t.text != "[]" && ts[j].text+t.text != "{}" {
						if strict {
							return fail(start, "mismatched brackets")
						}
					} else {
						t.pair, ts[j].pair = j, len(ts)
						stack = stack[:len(stack)-1]
					}
				}
			}
		}
		ts = append(ts, t)
	}
	if len(stack) != 0 && strict {
		return fail(ts[stack[len(stack)-1]].start, "unclosed bracket")
	}
	return ts, nil
}

func gooseObjectTokens(src []byte) ([]rustObjectToken, error) { return gooseLex(src, true) }

type gooseObjectScan struct{ t []rustObjectToken }

func (s gooseObjectScan) at(i int, text string) bool {
	return i < len(s.t) && i >= 0 && !s.t[i].literal && s.t[i].text == text
}

func (s gooseObjectScan) word(i int) bool {
	return i < len(s.t) && i >= 0 && !s.t[i].literal && gooseIdent(s.t[i].text)
}

func (s gooseObjectScan) name(i int) (string, int) {
	start := i
	if s.at(i, "::") {
		i++
	}
	if !s.word(i) {
		return "", start
	}
	i++
	if s.at(i, "::") && s.word(i+1) {
		i += 2
	}
	return s.spelling(start, i), i
}

// Length prefixes make catalogue keys unambiguous, even for adjacent literals.
func (s gooseObjectScan) text(lo, hi int) string {
	var b strings.Builder
	for _, t := range s.t[lo:hi] {
		text := strings.ReplaceAll(t.text, "\r\n", "\n")
		fmt.Fprintf(&b, "%d:%s", len(text), text)
	}
	return b.String()
}

func (s gooseObjectScan) spelling(lo, hi int) string {
	var b strings.Builder
	for _, t := range s.t[lo:hi] {
		b.WriteString(t.text)
	}
	return b.String()
}

// Generic bounds use the same type grammar as signatures. Array expressions
// are opaque balanced groups: [>..] and [>..<] must not close a generic list.
func (s gooseObjectScan) generics(i, depth int) int {
	if !s.at(i, "<") {
		return i
	}
	i++
	for depth < 128 && s.word(i) {
		i++
		if s.at(i, ":") {
			i = s.typ(i+1, depth+1)
			if i < 0 {
				return -1
			}
		}
		if !s.at(i, ",") {
			break
		}
		i++
	}
	if !s.at(i, ">") {
		return -1
	}
	return i + 1
}

// This parses type boundaries, not type semantics or expression syntax.
func (s gooseObjectScan) typ(i, depth int) int {
	if depth >= 128 {
		return -1
	}
	if s.at(i, "const") {
		i++
	}
	if s.at(i, "fn") {
		i++
		if s.at(i, "(") {
			i = s.t[i].pair + 1
			if s.at(i, "->") {
				i++
				if s.at(i, "(") {
					i = s.t[i].pair + 1
				} else {
					i = s.typ(i, depth+1)
				}
			}
		}
	} else if s.at(i, "(") {
		i = s.t[i].pair + 1
	} else {
		_, next := s.name(i)
		if next == i {
			return -1
		}
		i = next
		if s.at(i, "<") {
			i++
			for {
				i = s.typ(i, depth+1)
				if i < 0 {
					return -1
				}
				if !s.at(i, ",") {
					break
				}
				i++
				if s.at(i, ">") {
					break
				}
			}
			if !s.at(i, ">") {
				return -1
			}
			i++
		}
	}
	if i < 0 {
		return -1
	}
	if s.at(i, "in") || s.at(i, "upto") {
		_, next := s.name(i + 1)
		if next == i+1 {
			return -1
		}
		return next
	}
	for i < len(s.t) {
		switch {
		case s.at(i, "["):
			i = s.t[i].pair + 1
		case s.at(i, "&"):
			i++
			if s.at(i, "<") {
				i++
				if !s.word(i) {
					return -1
				}
				i++
				if s.at(i, "in") {
					_, i = s.name(i + 1)
				}
				if !s.at(i, ">") {
					return -1
				}
				i++
			}
		case s.at(i, "?"), s.at(i, ".."):
			i++
		case s.at(i, ".") && s.word(i+1):
			i += 2
		default:
			return i
		}
	}
	return i
}

// Only top-level functions/types are editable. Bodies, fields and extern blocks
// remain opaque; their outer declaration is still recorded to guard neighbors.
func gooseCodeItems(path string, src []byte, scope, context string) ([]rustObjectItem, error) {
	ts, err := gooseObjectTokens(src)
	if err != nil {
		return nil, objectError("invalid_source", err.Error())
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, objectError("invalid_path", err.Error())
	}
	abs = filepath.Clean(abs)
	s := gooseObjectScan{ts}
	starts := []int{0}
	for i, b := range src {
		if b == '\n' {
			starts = append(starts, i+1)
		}
	}
	revision := codeRevision(src)
	var items []rustObjectItem
	nsSeen, declSeen := false, false
	invalid := func(i int) ([]rustObjectItem, error) {
		at := len(src)
		if i >= 0 && i < len(ts) {
			at = ts[i].start
		}
		return nil, objectError("invalid_source", fmt.Sprintf("unsupported or incomplete Goose declaration at byte %d", at))
	}
	for first := 0; first < len(ts); {
		i := first
		kind, name, attrs, signature := "", "", "", ""
		last := -1
		if s.at(i, "namespace") || s.at(i, "import") {
			if s.at(i, "namespace") {
				if nsSeen || declSeen || !s.word(i+1) || !s.at(i+2, ";") {
					return invalid(i)
				}
				nsSeen = true
				scope = "namespace:" + ts[i+1].text
				last = i + 2
			} else {
				for j := i + 1; j < len(ts); j++ {
					if s.at(j, ";") {
						last = j
						break
					}
					if !ts[j].literal && !s.word(j) && !s.at(j, ".") {
						return invalid(j)
					}
				}
			}
		} else if s.at(i, "let") || s.at(i, "var") || s.at(i, "const") || s.at(i, "reusable") {
			declSeen = true
			for j := i + 1; j < len(ts); j++ {
				if s.at(j, ";") {
					last = j
					break
				}
				if s.at(j, "import") || s.at(j, "namespace") || s.at(j, "struct") || s.at(j, "enum") || s.at(j, "type") || s.at(j, "fn") && s.word(j+1) {
					return invalid(j)
				}
				if ts[j].pair > j {
					j = ts[j].pair
				}
			}
		} else {
			declSeen = true
			external := false
			for s.at(i, "extern") || s.at(i, "export") || s.at(i, "inline") || s.at(i, "checkfree") || s.at(i, "simd") || s.at(i, "recursive") {
				word := ts[i].text
				external = external || word == "extern"
				i++
				if (word == "extern" || word == "export") && i < len(ts) && ts[i].literal {
					i++
				}
			}
			if external && s.at(i, "{") {
				last = ts[i].pair
			} else if s.at(i, "fn") || s.at(i, "thread_fn") || s.at(i, "struct") || s.at(i, "enum") || s.at(i, "type") {
				category := ts[i].text
				if category != "fn" && category != "thread_fn" && i != first {
					return invalid(i)
				}
				attrs = s.text(first, i+1)
				nameAt := i + 1
				_, i = s.name(nameAt)
				if i == nameAt {
					return invalid(i)
				}
				name = s.spelling(nameAt, i)
				sigAt := i
				i = s.generics(i, 0)
				if i < 0 {
					return invalid(i)
				}
				switch category {
				case "fn", "thread_fn":
					kind = "function"
					if !s.at(i, "(") {
						return invalid(i)
					}
					i = ts[i].pair + 1
					if s.at(i, "->") {
						i++
						for {
							i = s.typ(i, 0)
							if i < 0 {
								return invalid(i)
							}
							if !s.at(i, ",") {
								break
							}
							i++
						}
					}
					signature = s.text(sigAt, i)
					if external {
						binding := i
						if s.at(i, "=") && i+1 < len(ts) && ts[i+1].literal {
							i += 2
						}
						attrs += s.text(binding, i)
						if !s.at(i, ";") {
							return invalid(i)
						}
						last = i
					} else if s.at(i, "{") {
						last = ts[i].pair
					}
				case "struct", "enum":
					kind = "type"
					if category == "enum" && s.at(i, ":") {
						i = s.typ(i+1, 0)
					}
					if s.at(i, "{") {
						last = ts[i].pair
					}
				case "type":
					kind = "type"
					if !s.at(i, "=") {
						return invalid(i)
					}
					i = s.typ(i+1, 0)
					if s.at(i, ";") {
						last = i
					}
				}
			}
		}
		if last < first {
			return invalid(i)
		}
		a, b := ts[first].start, ts[last].end
		key := scope + "/" + kind + ":" + name + ":" + attrs + ":" + signature
		if kind == "" {
			key += s.text(first, last+1)
		}
		item := rustObjectItem{start: a, end: b, scope: scope, context: "module", key: key, attributes: attrs}
		if kind != "" {
			line, col := rustBytePosition(starts, a)
			endLine, _ := rustBytePosition(starts, b-1)
			o := CodeObject{Protocol: "funcfinder.code/v1", Path: abs, Language: "goose", Kind: kind, Name: name, Line: line, Column: col, EndLine: endLine, Revision: revision, Source: strings.ReplaceAll(string(src[a:b]), "\r\n", "\n")}
			o.ID = codeObjectID(abs, "goose:"+kind, name, line, col)
			item.object = o
		}
		items = append(items, item)
		first = last + 1
	}
	return items, nil
}

func readGooseCodeObject(path, kind, name string, line, column int, src []byte) (*CodeObject, error) {
	if kind != "function" && kind != "type" {
		return nil, objectError("invalid_kind", "Goose objects support function/type")
	}
	if name == "" || line < 0 || column < 0 {
		return nil, objectError("invalid_selector", "name is required; coordinates must be nonnegative")
	}
	items, err := gooseCodeItems(path, src, "", "module")
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
		return nil, objectError("not_found", "no editable top-level Goose declaration matches the selector")
	}
	if len(matches) > 1 {
		return nil, &ObjectError{Code: "ambiguous", Message: "select a candidate by physical coordinates", Candidates: matches}
	}
	return &matches[0], nil
}

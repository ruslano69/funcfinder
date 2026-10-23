package internal

import (
	"sort"
	"strings"
)

// Goose follows src/lexer.h: nested block comments, escaped single-line
// strings/chars, and triple-quoted strings closed by the first triple quote.
func (s *Sanitizer) cleanGooseLine(line string, state ParserState) (string, ParserState) {
	r := []rune(line)
	out := make([]rune, len(r))
	for i := range out {
		out[i] = ' '
	}
	if state != StateBlockComment {
		s.gooseDepth = 0
	}
	for i := 0; i < len(r); {
		if state == StateMultiLineString {
			if i+2 < len(r) && r[i] == '"' && r[i+1] == '"' && r[i+2] == '"' {
				i += 3
				state = StateNormal
			} else {
				i++
			}
			continue
		}
		if state == StateBlockComment {
			if i+1 < len(r) && r[i] == '/' && r[i+1] == '*' {
				s.gooseDepth++
				i += 2
			} else if i+1 < len(r) && r[i] == '*' && r[i+1] == '/' {
				s.gooseDepth--
				i += 2
				if s.gooseDepth == 0 {
					state = StateNormal
				}
			} else {
				i++
			}
			continue
		}
		if i+1 < len(r) && r[i] == '/' && r[i+1] == '/' {
			break
		}
		if i+1 < len(r) && r[i] == '/' && r[i+1] == '*' {
			state = StateBlockComment
			s.gooseDepth = 1
			i += 2
			continue
		}
		if i+2 < len(r) && r[i] == '"' && r[i+1] == '"' && r[i+2] == '"' {
			state = StateMultiLineString
			i += 3
			continue
		}
		if r[i] == '"' || r[i] == '\'' {
			quote := r[i]
			i++
			for i < len(r) {
				c := r[i]
				i++
				if c == '\\' && i < len(r) {
					i++
				} else if c == quote {
					break
				}
			}
			continue
		}
		out[i] = r[i]
		i++
	}
	return string(out), state
}

type gooseToken struct {
	text               string
	line               int
	startByte, endByte int
}
type gooseDecl struct {
	name, kind                              string
	start, nameLine, end, body, last, first int
}

func gooseIdent(s string) bool {
	if s == "" {
		return false
	}
	for i, c := range s {
		if !(c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || i > 0 && c >= '0' && c <= '9') {
			return false
		}
	}
	return true
}

// gooseDeclarations mirrors ParseFnDecl/ParseStructDecl/ParseEnumDecl and
// ParseTopDeclInner's aliases. It discovers syntax, without type checking,
// imports, specialization or evaluation of compile-time branches.
func gooseDeclarations(lines []string, cfg *LanguageConfig) ([]gooseToken, []gooseDecl) {
	_ = cfg // Language-specific lexer owns Goose lexical rules.
	joined := strings.Join(lines, "\n")
	raw, _ := gooseLex([]byte(joined), false)
	var ts []gooseToken
	line, offset := 0, 0
	for _, t := range raw {
		line += strings.Count(joined[offset:t.start], "\n")
		offset = t.start
		if !t.literal {
			ts = append(ts, gooseToken{text: t.text, line: line, startByte: t.start, endByte: t.end})
		}
	}
	pairs := make(map[int]int)
	var stack []int
	for i, t := range ts {
		switch t.text {
		case "(", "[", "{":
			stack = append(stack, i)
		case ")", "]", "}":
			if len(stack) > 0 {
				j := stack[len(stack)-1]
				if ts[j].text+t.text == "()" || ts[j].text+t.text == "[]" || ts[j].text+t.text == "{}" {
					pairs[j] = i
					stack = stack[:len(stack)-1]
				}
			}
		}
	}
	var ds []gooseDecl
	for i := 0; i < len(ts); i++ {
		kind := ts[i].text
		if kind != "fn" && kind != "thread_fn" && kind != "struct" && kind != "enum" && kind != "type" {
			continue
		}
		j := i + 1
		name := ""
		if j < len(ts) && ts[j].text == "::" {
			name = "::"
			j++
		}
		if j >= len(ts) || !gooseIdent(ts[j].text) {
			continue
		}
		nameLine := ts[j].line
		name += ts[j].text
		j++
		if j+1 < len(ts) && ts[j].text == "::" && gooseIdent(ts[j+1].text) {
			name += "::" + ts[j+1].text
			nameLine = ts[j+1].line
			j += 2
		}
		if j < len(ts) && ts[j].text == "<" {
			depth := 0
			for ; j < len(ts); j++ {
				if ts[j].text == "<" {
					depth++
				}
				if ts[j].text == ">" {
					depth--
					if depth == 0 {
						j++
						break
					}
				}
			}
		}
		isFunc := kind == "fn" || kind == "thread_fn"
		if isFunc {
			if j >= len(ts) || ts[j].text != "(" {
				continue
			}
			end, ok := pairs[j]
			if !ok {
				continue
			}
			j = end + 1
		}
		if kind == "type" && (j >= len(ts) || ts[j].text != "=") {
			continue
		}
		start := i
		if isFunc {
			for start > 0 {
				switch ts[start-1].text {
				case "extern", "export", "checkfree", "simd", "inline", "recursive":
					start--
				default:
					goto prefixDone
				}
			}
		}
	prefixDone:
		body, last := -1, -1
		for ; j < len(ts); j++ {
			if ts[j].text == ";" {
				last = j
				break
			}
			if ts[j].text == "{" {
				if end, ok := pairs[j]; ok {
					if kind == "type" {
						j = end
						continue
					}
					body, last = j, end
				}
				break
			}
			if ts[j].text == "(" || ts[j].text == "[" {
				if end, ok := pairs[j]; ok {
					j = end
				}
			}
			if ts[j].text == "}" {
				break
			}
		}
		if last < 0 {
			continue
		}
		if !isFunc && kind != "type" && body < 0 {
			continue
		}
		if isFunc {
			kind = "function"
		}
		ds = append(ds, gooseDecl{name, kind, ts[start].line, nameLine, ts[last].line, body, last, start})
	}
	return ts, ds
}

// Token ownership distinguishes shared-line bodies and nested declarations.
// Edges remain lexical: overloads, values and namespaces are not resolved.
func gooseCallGraph(path string, lines []string, cfg *LanguageConfig, known map[string]bool, aliases map[string]string) *FileCallGraph {
	ts, ds := gooseDeclarations(lines, cfg)
	r := &FileCallGraph{Path: path, Calls: []CallEdge{}}
	functions := make(map[string]bool)
	for _, d := range ds {
		if d.kind == "function" {
			functions[d.name] = true
		}
	}
	exclude := make(map[string]bool)
	for _, name := range cfg.ExcludeWords {
		exclude[name] = true
	}
	seen := make(map[string]bool)
	for i, tok := range ts {
		if !gooseIdent(tok.text) || exclude[tok.text] {
			continue
		}
		j := i + 1
		if j < len(ts) && ts[j].text == "<" {
			depth := 0
			for ; j < len(ts); j++ {
				if ts[j].text == "<" {
					depth++
				}
				if ts[j].text == ">" {
					depth--
					if depth == 0 {
						j++
						break
					}
				}
				if ts[j].text == ";" || ts[j].text == "{" {
					break
				}
			}
		}
		if j >= len(ts) || ts[j].text != "(" {
			continue
		}
		caller, body, header := "", -1, false
		for _, d := range ds {
			if i >= d.first && (d.kind != "function" && i <= d.last || d.kind == "function" && (d.body < 0 && i <= d.last || d.body >= 0 && i <= d.body)) {
				header = true
				break
			}
			if d.kind == "function" && d.body >= 0 && i > d.body && i < d.last && d.body > body {
				caller, body = d.name, d.body
			}
		}
		if header || caller == "" {
			continue
		}
		callee := tok.text
		if i >= 2 && (ts[i-1].text == "." || ts[i-1].text == "::") && gooseIdent(ts[i-2].text) {
			receiver := ts[i-2].text
			if alias := aliases[receiver]; alias != "" {
				receiver = alias
			}
			callee = receiver + ts[i-1].text + callee
		} else if !functions[callee] && !known[callee] {
			continue
		}
		key := caller + "→" + callee
		if !seen[key] {
			seen[key] = true
			r.Calls = append(r.Calls, CallEdge{Caller: caller, Callee: callee, Line: tok.line + 1})
		}
	}
	sort.Slice(r.Calls, func(i, j int) bool {
		if r.Calls[i].Caller != r.Calls[j].Caller {
			return r.Calls[i].Caller < r.Calls[j].Caller
		}
		return r.Calls[i].Callee < r.Calls[j].Callee
	})
	return r
}

func (f *Finder) findGooseFunctions(lines []string, startLine int, filename string) (*FindResult, error) {
	_, ds := gooseDeclarations(lines, f.config)
	r := &FindResult{Filename: filename, Functions: []FunctionBounds{}, Classes: []ClassBounds{}}
	for _, d := range ds {
		if d.kind != "function" {
			r.Classes = append(r.Classes, ClassBounds{Name: d.name, Kind: d.kind, Start: d.start + startLine, End: d.end + startLine})
			continue
		}
		if !f.mapMode && !f.funcNames[d.name] {
			continue
		}
		name := d.name
		if n := strings.LastIndex(name, "::"); n >= 0 {
			name = name[n+2:]
		}
		fn := FunctionBounds{Name: d.name, Start: d.start + startLine, End: d.end + startLine, SignatureLine: d.nameLine + startLine, SignatureName: name}
		if f.extractMode {
			fn.Lines = append([]string(nil), lines[d.start:d.end+1]...)
		}
		r.Functions = append(r.Functions, fn)
	}
	return r, nil
}

func (f *StructFinder) findGooseStructures(lines []string, startLine int, filename string) (*StructFindResult, error) {
	ts, ds := gooseDeclarations(lines, f.config)
	r := &StructFindResult{Filename: filename, Types: []TypeBounds{}}
	for _, d := range ds {
		if d.kind == "function" || !f.mapMode && !f.typeNames[d.name] {
			continue
		}
		t := TypeBounds{Name: d.name, Kind: d.kind, Start: d.start + startLine, End: d.end + startLine}
		if !f.noFields && d.kind == "struct" {
			depth := 0
			for i := d.body + 1; i+1 < d.last; i++ {
				if depth == 0 && gooseIdent(ts[i].text) && ts[i+1].text == ":" {
					t.Fields = append(t.Fields, FieldBounds{Name: ts[i].text, Line: ts[i].line + startLine})
				}
				switch ts[i].text {
				case "(", "[", "{":
					depth++
				case ")", "]", "}":
					depth--
				}
			}
		}
		r.Types = append(r.Types, t)
	}
	return r, nil
}

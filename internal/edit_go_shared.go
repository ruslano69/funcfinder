package internal

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/scanner"
	"go/token"
	"strings"
)

// editSharedGo uses byte spans only when a function shares a physical line
// with another declaration. Ordinary edits retain their line/comment policy.
func editSharedGo(cfg *LanguageConfig, src []byte, target FuncTarget, old, replacement []byte, op string) (*ReplaceResult, bool, error) {
	if cfg.LangKey != "go" || target.Kind != "" {
		return nil, false, nil
	}
	fs := token.NewFileSet()
	f, err := parser.ParseFile(fs, "", src, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		return nil, false, nil
	}
	var found []*ast.FuncDecl
	for _, d := range f.Decls {
		if fn, ok := d.(*ast.FuncDecl); ok && fn.Name.Name == target.Name && (target.Line == 0 || fs.Position(fn.Pos()).Line == target.Line) {
			found = append(found, fn)
		}
	}
	if len(found) != 1 {
		return nil, false, nil
	}
	fn := found[0]
	a, b := fs.Position(fn.Pos()), fs.Position(fn.End())
	shared := fs.Position(f.Name.End()).Line == a.Line
	for _, d := range f.Decls {
		if d != fn && (fs.Position(d.End()).Line == a.Line || fs.Position(d.Pos()).Line == b.Line) {
			shared = true
		}
	}
	if !shared {
		return nil, false, nil
	}
	fail := func(err error) (*ReplaceResult, bool, error) { return nil, true, err }
	if bytes.Contains(bytes.ReplaceAll(src, []byte("\r\n"), nil), []byte("\n")) && bytes.Contains(src, []byte("\r\n")) {
		return fail(fmt.Errorf("mixed line endings are not supported"))
	}
	start, end := a.Offset, b.Offset
	normal := func(s string) string { return strings.TrimSuffix(strings.ReplaceAll(s, "\r\n", "\n"), "\n") }
	if old != nil {
		want := stripExtractHeader(normal(string(old)), target.Name)
		lo := bytes.LastIndexByte(src[:start], '\n') + 1
		hi := end
		if n := bytes.IndexByte(src[end:], '\n'); n >= 0 {
			hi += n
		} else {
			hi = len(src)
		}
		if want != normal(string(src[start:end])) && want != normal(string(src[lo:hi])) {
			return fail(fmt.Errorf("function %s is not the expected text (it changed since it was read)", target.Name))
		}
	}
	payload := ""
	var added []ast.Decl
	if op != "remove" {
		payload, err = cleanReplacement(normal(string(replacement)), target.Name)
		if err != nil {
			return fail(err)
		}
		pf, parseErr := parser.ParseFile(token.NewFileSet(), "", "package edit\n"+payload+"\n", parser.SkipObjectResolution)
		if parseErr != nil {
			return fail(fmt.Errorf("replacement does not parse as Go: %v", parseErr))
		}
		added = pf.Decls
		if len(added) == 0 {
			return fail(fmt.Errorf("replacement defines no functions"))
		}
		for _, d := range added {
			if _, ok := d.(*ast.FuncDecl); !ok {
				return fail(fmt.Errorf("replacement must contain only functions"))
			}
		}
		if op == "insert" {
			seen := make(map[string]bool)
			for _, d := range f.Decls {
				seen[goSharedIdentity(d)] = true
			}
			for _, d := range added {
				id := goSharedIdentity(d)
				if seen[id] {
					return fail(fmt.Errorf("insertion duplicates %s", id))
				}
				seen[id] = true
			}
		}
		if op == "replace" && (len(added) != 1 || goSharedIdentity(added[0]) != goSharedIdentity(fn)) {
			return fail(fmt.Errorf("replacement must preserve function name and receiver"))
		}
		if bytes.Contains(src, []byte("\r\n")) {
			payload = strings.ReplaceAll(payload, "\n", "\r\n")
		}
	}
	// Scan tokens from the original source: only an explicit outer semicolon
	// belongs to the declaration. Comments and literal contents are never delimiters.
	semi := goFollowingSemicolon(src, end)
	spliceStart, spliceEnd := start, end
	switch op {
	case "remove":
		// Preserve comments between the declaration and its separator byte.
		if semi >= 0 {
			payload = string(src[end:semi])
			spliceEnd = semi + 1
		}
	case "insert":
		spliceStart, spliceEnd = end, end
		if semi >= 0 {
			spliceStart, spliceEnd = semi+1, semi+1
			payload = " " + payload + ";"
		} else {
			payload = "; " + payload + ";"
		}
	}
	out := append([]byte(nil), src[:spliceStart]...)
	out = append(out, payload...)
	out = append(out, src[spliceEnd:]...)
	af, err := parser.ParseFile(token.NewFileSet(), "", out, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		return fail(fmt.Errorf("edit does not give a valid Go file: %v", err))
	}
	var expected []string
	for _, d := range f.Decls {
		if d == fn {
			if op != "remove" {
				if op == "insert" {
					expected = append(expected, goSharedIdentity(d))
				}
				for _, n := range added {
					expected = append(expected, goSharedIdentity(n))
				}
			}
		} else {
			expected = append(expected, goSharedIdentity(d))
		}
	}
	actual := make([]string, 0, len(af.Decls))
	for _, d := range af.Decls {
		actual = append(actual, goSharedIdentity(d))
	}
	if !sameNames(expected, actual) {
		return fail(fmt.Errorf("edit changes neighboring declarations"))
	}
	res := &ReplaceResult{Start: a.Line, End: b.Line, Output: out}
	if op != "insert" {
		res.Removed = strings.Split(normal(string(src[start:end])), "\n")
	}
	if op != "remove" {
		res.Added = strings.Split(normal(payload), "\n")
		res.NewStart = bytes.Count(src[:spliceStart], []byte("\n")) + 1
		res.NewEnd = res.NewStart + strings.Count(payload, "\n")
	}
	return res, true, nil
}

func goSharedIdentity(d ast.Decl) string {
	if fn, ok := d.(*ast.FuncDecl); ok {
		var recv bytes.Buffer
		if fn.Recv != nil {
			for _, field := range fn.Recv.List {
				_ = format.Node(&recv, token.NewFileSet(), field.Type)
			}
		}
		return "func " + recv.String() + "." + fn.Name.Name
	}
	var b bytes.Buffer
	_ = format.Node(&b, token.NewFileSet(), d)
	return b.String()
}

func goFollowingSemicolon(src []byte, end int) int {
	fs := token.NewFileSet()
	f := fs.AddFile("", -1, len(src))
	var s scanner.Scanner
	s.Init(f, src, nil, scanner.ScanComments)
	for {
		p, tok, lit := s.Scan()
		off := f.Offset(p)
		if tok == token.EOF {
			return -1
		}
		if off < end || tok == token.COMMENT {
			continue
		}
		if tok == token.SEMICOLON && lit == ";" {
			return off
		}
		return -1
	}
}

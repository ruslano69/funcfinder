package internal

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
)

// CodeObject is protocol v1's editable document. Change Source only; Revision
// binds the entire file, while ID and coordinates select one declaration.
type CodeObject struct {
	Protocol string `json:"protocol"`
	ID       string `json:"id"`
	Path     string `json:"path"`
	Language string `json:"language"`
	Kind     string `json:"kind"`
	Name     string `json:"name"`
	Line     int    `json:"line"`
	Column   int    `json:"column"`
	EndLine  int    `json:"end_line"`
	Revision string `json:"revision"`
	Source   string `json:"source"`
}

// ObjectError codes are stable protocol values; candidates are directly editable.
type ObjectError struct {
	Code       string       `json:"code"`
	Message    string       `json:"message"`
	Candidates []CodeObject `json:"candidates,omitempty"`
}

func (e *ObjectError) Error() string         { return e.Message }
func objectError(code, message string) error { return &ObjectError{Code: code, Message: message} }

type objectSpan struct {
	object     CodeObject
	start, end int
	decl       ast.Decl
	grouped    bool
}

func codeRevision(src []byte) string { return fmt.Sprintf("sha256:%x", sha256.Sum256(src)) }
func codeObjectID(path, kind, name string, line, column int) string {
	return fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(fmt.Sprintf("%s\x00%s\x00%s\x00%d\x00%d", path, kind, name, line, column))))
}

func goCodeObjects(path string, src []byte) ([]objectSpan, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, objectError("invalid_path", err.Error())
	}
	abs = filepath.Clean(abs)
	fs := token.NewFileSet()
	f, err := parser.ParseFile(fs, abs, src, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		return nil, objectError("invalid_source", err.Error())
	}
	var spans []objectSpan
	revision := codeRevision(src)
	add := func(kind, name string, pos, end token.Pos, decl ast.Decl, grouped bool) {
		a, b := fs.PositionFor(pos, false), fs.PositionFor(end, false)
		source := strings.ReplaceAll(string(src[a.Offset:b.Offset]), "\r\n", "\n")
		if grouped {
			source = "type " + source
		}
		o := CodeObject{Protocol: "funcfinder.code/v1", Path: abs, Language: "go", Kind: kind, Name: name, Line: a.Line, Column: a.Column, EndLine: b.Line, Revision: revision, Source: source}
		o.ID = codeObjectID(o.Path, o.Kind, o.Name, o.Line, o.Column)
		spans = append(spans, objectSpan{object: o, start: a.Offset, end: b.Offset, decl: decl, grouped: grouped})
	}
	for _, d := range f.Decls {
		switch d := d.(type) {
		case *ast.FuncDecl:
			add("function", d.Name.Name, d.Pos(), d.End(), d, false)
		case *ast.GenDecl:
			if d.Tok != token.TYPE {
				continue
			}
			for _, s := range d.Specs {
				ts := s.(*ast.TypeSpec)
				pos, end := ts.Pos(), ts.End()
				if !d.Lparen.IsValid() {
					pos, end = d.Pos(), d.End()
				}
				add("type", ts.Name.Name, pos, end, d, d.Lparen.IsValid())
			}
		}
	}
	return spans, nil
}

// ReadCodeObject returns exact declaration source, never neighboring line text.
// Ambiguity supplies complete candidates rather than choosing a declaration.
func ReadCodeObject(path, language, kind, name string, line, column int, src []byte) (*CodeObject, error) {
	if language == "rust" {
		return readRustCodeObject(path, kind, name, line, column, src)
	}
	if language != "go" {
		return nil, objectError("unsupported_language", "code objects currently support Go and Rust")
	}
	if kind != "function" && kind != "type" {
		return nil, objectError("invalid_kind", "kind must be function or type")
	}
	if name == "" || line < 0 || column < 0 {
		return nil, objectError("invalid_selector", "name is required; line and column must be nonnegative")
	}
	spans, err := goCodeObjects(path, src)
	if err != nil {
		return nil, err
	}
	var matches []CodeObject
	for _, s := range spans {
		o := s.object
		if o.Kind == kind && o.Name == name && (line == 0 || o.Line == line) && (column == 0 || o.Column == column) {
			matches = append(matches, o)
		}
	}
	if len(matches) == 0 {
		return nil, objectError("not_found", "no declaration matches the selector")
	}
	if len(matches) > 1 {
		return nil, &ObjectError{Code: "ambiguous", Message: "select a candidate by line/column, or edit its returned object directly", Candidates: matches}
	}
	return &matches[0], nil
}

// ReplaceCodeObject validates the envelope and grammar, then splices only its
// original byte span. It never writes; the caller owns preview/application.
func ReplaceCodeObject(object CodeObject, src []byte) ([]byte, *CodeObject, error) {
	if object.Language == "rust" {
		out, results, err := planRustCodeEdits([]CodeEdit{{Operation: "replace", Object: object}}, src)
		if err != nil {
			return nil, nil, err
		}
		return out, &results[0].Objects[0], nil
	}
	if object.Protocol != "funcfinder.code/v1" {
		return nil, nil, objectError("unsupported_protocol", "expected funcfinder.code/v1")
	}
	if object.Language != "go" {
		return nil, nil, objectError("unsupported_language", "code objects currently support Go and Rust")
	}
	if object.Revision != codeRevision(src) {
		return nil, nil, objectError("stale_revision", "file changed since the object was read; read it again before editing")
	}
	if bytes.Contains(src, []byte("\r\n")) && bytes.Count(src, []byte("\r\n")) != bytes.Count(src, []byte("\n")) {
		return nil, nil, objectError("mixed_endings", "mixed line endings are not supported")
	}
	spans, err := goCodeObjects(object.Path, src)
	if err != nil {
		return nil, nil, err
	}
	var selected *objectSpan
	for i := range spans {
		o := spans[i].object
		if o.ID == object.ID && o.Path == object.Path && o.Name == object.Name && o.Kind == object.Kind && o.Line == object.Line && o.Column == object.Column && o.EndLine == object.EndLine {
			selected = &spans[i]
			break
		}
	}
	if selected == nil {
		return nil, nil, objectError("invalid_identity", "object identity/coordinates do not match this file")
	}
	payload := strings.ReplaceAll(object.Source, "\r\n", "\n")
	const prefix = "package edit\n"
	fs := token.NewFileSet()
	pf, e := parser.ParseFile(fs, "", prefix+payload+"\n", parser.ParseComments|parser.SkipObjectResolution)
	if e != nil {
		return nil, nil, objectError("invalid_replacement", e.Error())
	}
	if len(pf.Decls) != 1 || len(pf.Imports) != 0 {
		return nil, nil, objectError("invalid_replacement", "source must define exactly one declaration")
	}
	switch object.Kind {
	case "function":
		fn, ok := pf.Decls[0].(*ast.FuncDecl)
		if !ok || goSharedIdentity(fn) != goSharedIdentity(selected.decl) {
			return nil, nil, objectError("contract_changed", "replacement must preserve function name and receiver type")
		}
	case "type":
		gd, ok := pf.Decls[0].(*ast.GenDecl)
		if !ok || gd.Tok != token.TYPE || gd.Lparen.IsValid() || len(gd.Specs) != 1 || gd.Specs[0].(*ast.TypeSpec).Name.Name != object.Name {
			return nil, nil, objectError("contract_changed", "source must define exactly the same named, ungrouped type")
		}
		if selected.grouped {
			ts := gd.Specs[0].(*ast.TypeSpec)
			for _, c := range pf.Comments {
				if c.Pos() < ts.Pos() || c.End() > ts.End() {
					return nil, nil, objectError("invalid_replacement", "cannot relocate declaration comments into a grouped spec")
				}
			}
			payload = payload[fs.PositionFor(ts.Pos(), false).Offset-len(prefix) : fs.PositionFor(ts.End(), false).Offset-len(prefix)]
		}
	default:
		return nil, nil, objectError("invalid_kind", "kind must be function or type")
	}
	if bytes.Contains(src, []byte("\r\n")) {
		payload = strings.ReplaceAll(payload, "\n", "\r\n")
	}
	out := append([]byte(nil), src[:selected.start]...)
	out = append(out, payload...)
	out = append(out, src[selected.end:]...)
	beforeDecls, beforeErr := goDeclSignatures(string(src))
	afterDecls, afterErr := goDeclSignatures(string(out))
	if beforeErr != nil || afterErr != nil || !sameNames(beforeDecls, afterDecls) {
		return nil, nil, objectError("invalid_result", "replacement changes top-level declarations or gives invalid Go syntax")
	}
	// Parsing the complete file catches comments swallowing adjacent declarations
	// and changes in separator context. Check declaration identities in order.
	after, err := goCodeObjects(object.Path, out)
	if err != nil {
		return nil, nil, objectError("invalid_result", err.Error())
	}
	if len(after) != len(spans) {
		return nil, nil, objectError("invalid_result", "replacement changes declaration count")
	}
	var updated *CodeObject
	for i := range spans {
		a, b := spans[i].object, after[i].object
		if a.Kind != b.Kind || a.Name != b.Name {
			return nil, nil, objectError("invalid_result", "replacement changes neighboring declarations")
		}
		if spans[i].start == selected.start {
			copy := b
			updated = &copy
		}
	}
	if updated == nil {
		return nil, nil, objectError("invalid_result", "edited declaration disappeared")
	}
	return out, updated, nil
}

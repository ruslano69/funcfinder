package internal

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
)

// Type targets (Kind "type") use the struct finder for their spans and go/ast to verify them. The finder's functions
// are still compared: a type edit must not add, remove or rename a function.

// unitName is the word for the declarations the edit targets, for messages.
func (e *editFile) unitName() string {
	if e.unit == "type" {
		return "type"
	}
	return "function"
}

// otherKind names the declarations the edit must leave alone, for messages.
func (e *editFile) otherKind() string {
	if e.unit == "type" {
		return "functions"
	}
	return "types"
}

// typeSpans returns the struct finder's top-level types of the file as spans.
func (e *editFile) typeSpans(lines []string) ([]FunctionBounds, error) {
	finder := NewStructFinder(e.cfg, "", true)
	result, err := finder.FindStructuresInLines(lines, 1, "")
	if err != nil {
		return nil, err
	}
	spans := make([]FunctionBounds, 0, len(result.Types))
	for _, t := range result.Types {
		spans = append(spans, FunctionBounds{Name: t.Name, Start: t.Start, End: t.End})
	}
	return spans, nil
}

// declsIn returns the primary declarations of text (functions, or types for a type edit) and the other kind, in order.
func (e *editFile) declsIn(name, text string) ([]string, []string, error) {
	result, err := e.findIn(name, text)
	if err != nil {
		return nil, nil, err
	}
	if e.unit != "type" {
		return funcNames(result), classNames(result), nil
	}
	spans, err := e.typeSpans(strings.Split(text, "\n"))
	if err != nil {
		return nil, nil, err
	}
	types := make([]string, len(spans))
	for i, s := range spans {
		types[i] = s.Name
	}
	return types, funcNames(result), nil
}

// checkDecls requires the primary declarations of text to be want, and the other kind to be unchanged.
func (e *editFile) checkDecls(name, text string, want []string) error {
	primary, other, err := e.declsIn(name, text)
	if err != nil {
		return err
	}
	if !sameNames(want, primary) {
		return fmt.Errorf("replacement changes the %ss in the file: was [%s], now [%s]", e.unitName(),
			strings.Join(want, ", "), strings.Join(primary, ", "))
	}
	if !sameNames(other, e.beforeOther) {
		return fmt.Errorf("replacement changes the %s in the file: was [%s], now [%s]", e.otherKind(),
			strings.Join(e.beforeOther, ", "), strings.Join(other, ", "))
	}
	return nil
}

// checkGoTypeSpanWithAST confirms that the struct finder's span is the span go/ast gives a single, ungrouped type
// declaration. A type declared inside a parenthesised group is refused: its line is only part of the group.
func checkGoTypeSpanWithAST(text string, fn FunctionBounds, name string) error {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "", text, parser.SkipObjectResolution)
	if err != nil {
		return fmt.Errorf("source does not parse as Go: %v", err)
	}
	for _, decl := range file.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.TYPE {
			continue
		}
		for _, spec := range gd.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok || ts.Name.Name != name || fset.Position(ts.Pos()).Line != fn.Start {
				continue
			}
			if gd.Lparen.IsValid() {
				return fmt.Errorf("type %s is in a grouped type declaration (line %d): edit the group by hand", name, fn.Start)
			}
			if end := fset.Position(gd.End()).Line; end != fn.End {
				return fmt.Errorf("type %s: finder span %d-%d disagrees with go/ast span %d-%d", name, fn.Start, fn.End, fn.Start, end)
			}
			return nil
		}
	}
	return fmt.Errorf("type %s at line %d is not a top-level declaration in go/ast", name, fn.Start)
}

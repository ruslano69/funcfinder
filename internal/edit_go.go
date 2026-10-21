package internal

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
)

// checkGoSpanWithAST confirms that the finder's span is the span go/ast gives the same declaration.
func checkGoSpanWithAST(text string, fn FunctionBounds, name string) error {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "", text, parser.SkipObjectResolution)
	if err != nil {
		return fmt.Errorf("source does not parse as Go: %v", err)
	}
	for _, decl := range file.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok || fd.Name.Name != name || fset.Position(fd.Pos()).Line != fn.Start {
			continue
		}
		end := fset.Position(fd.End()).Line
		if end != fn.End {
			return fmt.Errorf("function %s: finder span %d-%d disagrees with go/ast span %d-%d", name, fn.Start, fn.End, fn.Start, end)
		}
		return nil
	}
	return fmt.Errorf("function %s at line %d is not a top-level declaration in go/ast", name, fn.Start)
}

// checkGoDeclarationsKept requires the new file to parse and to keep the kinds and names of its top-level
// declarations. go/ast is the reference here, so this is stricter than the finder comparison in edit.go.
func checkGoDeclarationsKept(oldText, newText, name string) error {
	before, err := goDeclSignatures(oldText)
	if err != nil {
		return fmt.Errorf("original source: %v", err)
	}
	after, err := goDeclSignatures(newText)
	if err != nil {
		return fmt.Errorf("replacement does not give a valid file: %v", err)
	}
	if len(before) != len(after) {
		return fmt.Errorf("replacement changes the number of top-level declarations from %d to %d", len(before), len(after))
	}
	for i := range before {
		if before[i] != after[i] {
			return fmt.Errorf("replacement changes top-level declaration %d from %q to %q", i+1, before[i], after[i])
		}
	}
	return nil
}

// goDeclSignatures lists "kind name" for each top-level declaration; a function's receiver is not part of it.
func goDeclSignatures(text string) ([]string, error) {
	file, err := parser.ParseFile(token.NewFileSet(), "", text, parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	sigs := make([]string, 0, len(file.Decls))
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			sigs = append(sigs, "func "+d.Name.Name)
		case *ast.GenDecl:
			var names []string
			for _, spec := range d.Specs {
				switch s := spec.(type) {
				case *ast.ValueSpec:
					for _, n := range s.Names {
						names = append(names, n.Name)
					}
				case *ast.TypeSpec:
					names = append(names, s.Name.Name)
				case *ast.ImportSpec:
					names = append(names, "import "+s.Path.Value)
				}
			}
			sigs = append(sigs, d.Tok.String()+" "+strings.Join(names, ","))
		}
	}
	return sigs, nil
}

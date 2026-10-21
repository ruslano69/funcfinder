package internal

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
)

// GoFuncTarget selects one top-level Go function or method to replace. Line is the 1-based line of its
// func keyword; zero selects by name, which must then match exactly one function.
type GoFuncTarget struct {
	Name string
	Line int
}

// GoReplaceResult is a validated replacement. Output is the complete new file, with the original line endings.
type GoReplaceResult struct {
	Start  int // first replaced line (1-based)
	End    int // last replaced line (1-based, inclusive)
	Output []byte
}

// ReplaceGoFunction replaces one top-level Go function in src with replacement. Nothing is returned unless every
// check passes: the target is found exactly once, the finder's span agrees with go/ast, the current text equals
// expectOld (when non-nil; line endings are ignored), the replacement holds exactly one function with the same
// name and no other top-level declaration changes, and the new file parses. Lines outside the span are kept
// byte for byte. Mixed line endings are refused, because the file is rewritten with one line ending.
func ReplaceGoFunction(cfg *LanguageConfig, src []byte, target GoFuncTarget, expectOld []byte, replacement []byte) (*GoReplaceResult, error) {
	if target.Name == "" {
		return nil, fmt.Errorf("function name is empty")
	}
	crlf := bytes.Count(src, []byte("\r\n"))
	if crlf > 0 && crlf != bytes.Count(src, []byte("\n")) {
		return nil, fmt.Errorf("mixed line endings: refusing to rewrite the file")
	}
	eol := "\n"
	if crlf > 0 {
		eol = "\r\n"
	}
	text := strings.ReplaceAll(string(src), "\r\n", "\n")
	if text == "" {
		return nil, fmt.Errorf("source is empty")
	}
	finalNewline := strings.HasSuffix(text, "\n")
	lines := strings.Split(strings.TrimSuffix(text, "\n"), "\n")

	fn, err := locateGoFunction(cfg, lines, target)
	if err != nil {
		return nil, err
	}
	if err := checkGoSpanWithAST(text, fn, target.Name); err != nil {
		return nil, err
	}

	oldText := strings.Join(lines[fn.Start-1:fn.End], "\n")
	if expectOld != nil && oldText != strings.ReplaceAll(string(expectOld), "\r\n", "\n") {
		return nil, fmt.Errorf("function %s at lines %d-%d is not the expected text (it changed since it was read)",
			target.Name, fn.Start, fn.End)
	}

	newText := strings.TrimSuffix(strings.ReplaceAll(string(replacement), "\r\n", "\n"), "\n")
	if strings.TrimSpace(newText) == "" {
		return nil, fmt.Errorf("replacement is empty")
	}
	newLines := strings.Split(newText, "\n")

	out := make([]string, 0, len(lines)-(fn.End-fn.Start+1)+len(newLines))
	out = append(out, lines[:fn.Start-1]...)
	out = append(out, newLines...)
	out = append(out, lines[fn.End:]...)
	outText := strings.Join(out, "\n")
	if finalNewline {
		outText += "\n"
	}

	if err := checkGoDeclarationsKept(text, outText, target.Name); err != nil {
		return nil, err
	}

	return &GoReplaceResult{
		Start:  fn.Start,
		End:    fn.End,
		Output: []byte(strings.ReplaceAll(outText, "\n", eol)),
	}, nil
}

// locateGoFunction uses the production finder and returns the one function the target selects.
func locateGoFunction(cfg *LanguageConfig, lines []string, target GoFuncTarget) (FunctionBounds, error) {
	result, err := NewFinder(cfg, nil, true, false, false).FindFunctionsInLines(lines, 1, "")
	if err != nil {
		return FunctionBounds{}, err
	}
	var matches []FunctionBounds
	for _, fn := range result.Functions {
		if fn.Name == target.Name && (target.Line == 0 || fn.Start == target.Line) {
			matches = append(matches, fn)
		}
	}
	switch len(matches) {
	case 0:
		if target.Line != 0 {
			return FunctionBounds{}, fmt.Errorf("no function %s starts at line %d", target.Name, target.Line)
		}
		return FunctionBounds{}, fmt.Errorf("no function named %s", target.Name)
	case 1:
		return matches[0], nil
	}
	starts := make([]string, len(matches))
	for i, fn := range matches {
		starts[i] = fmt.Sprint(fn.Start)
	}
	return FunctionBounds{}, fmt.Errorf("%d functions named %s start at lines %s: select one with the start line",
		len(matches), target.Name, strings.Join(starts, ", "))
}

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
// declarations, with the target still a function of the same name.
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

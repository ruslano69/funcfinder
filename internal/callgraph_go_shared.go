package internal

import (
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
	"strings"
)

// goSharedLineCallGraph resolves call ownership within overlapping physical lines.
func goSharedLineCallGraph(path string, lines []string, funcs []FunctionBounds, knownFuncs map[string]bool, importAliases map[string]string) (*FileCallGraph, bool) {
	fs := token.NewFileSet()
	file, err := parser.ParseFile(fs, path, strings.Join(lines, "\n"), parser.SkipObjectResolution)
	if err != nil {
		return nil, false
	}
	// Keep the finder's selection authoritative; the AST supplies body ownership.
	selected := make(map[string]map[int]bool)
	fileFuncs := make(map[string]bool, len(funcs))
	for _, f := range funcs {
		fileFuncs[f.Name] = true
		if selected[f.Name] == nil {
			selected[f.Name] = make(map[int]bool)
		}
		selected[f.Name][f.Start] = true
	}
	graph := &FileCallGraph{Path: path}
	seen := make(map[string]bool)
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil || !selected[fn.Name.Name][fs.PositionFor(fn.Pos(), false).Line] {
			continue
		}
		ast.Inspect(fn.Body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			fun := call.Fun
			for {
				switch expr := fun.(type) {
				case *ast.IndexExpr:
					fun = expr.X
				case *ast.IndexListExpr:
					fun = expr.X
				case *ast.ParenExpr:
					fun = expr.X
				default:
					goto resolved
				}
			}
		resolved:
			callee := ""
			switch expr := fun.(type) {
			case *ast.Ident:
				if fileFuncs[expr.Name] || knownFuncs[expr.Name] {
					callee = expr.Name
				}
			case *ast.SelectorExpr:
				receiver := ""
				switch recv := expr.X.(type) {
				case *ast.Ident:
					receiver = recv.Name
					if alias, ok := importAliases[receiver]; ok {
						receiver = alias
					}
				case *ast.SelectorExpr:
					// Match the existing graph's final receiver-name convention.
					// A field receiver is not an import alias.
					receiver = recv.Sel.Name
				}
				if receiver != "" {
					callee = receiver + "." + expr.Sel.Name
				}
			}
			key := fn.Name.Name + "→" + callee
			if callee != "" && !seen[key] {
				seen[key] = true
				graph.Calls = append(graph.Calls, CallEdge{Caller: fn.Name.Name, Callee: callee, Line: fs.PositionFor(call.Pos(), false).Line})
			}
			return true
		})
	}
	sort.Slice(graph.Calls, func(i, j int) bool {
		if graph.Calls[i].Caller != graph.Calls[j].Caller {
			return graph.Calls[i].Caller < graph.Calls[j].Caller
		}
		return graph.Calls[i].Callee < graph.Calls[j].Callee
	})
	return graph, true
}

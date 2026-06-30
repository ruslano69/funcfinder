// astoracle — ground-truth symbol extractor for Go, built on go/ast.
//
// It is the "ruler" against which funcfinder's regex output is measured: a real
// parser, zero heuristics. It emits the exact same JSON shape as
// `funcfinder --dir <d> --all --json`, so the two can be diffed directly
// (see benchmarks/specsheet.py).
//
// Scope: Go only, on purpose. The oracle must be unimpeachable on the flagship
// language; other languages get their own native oracle (Python `ast`, the TS
// compiler API, …) or tree-sitter as a second tier.
//
// Usage: astoracle <dir>   (defaults to ".")
package main

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

type sym struct {
	Name string `json:"name"`
	Line int    `json:"line"`
}

type fileOut struct {
	Path      string `json:"path"`
	Functions []sym  `json:"functions"`
	Classes   []sym  `json:"classes"`
}

type out struct {
	Files          []fileOut `json:"files"`
	TotalFiles     int       `json:"total_files"`
	TotalFunctions int       `json:"total_functions"`
	TotalClasses   int       `json:"total_classes"`
	// Unparseable counts files go/parser rejected — the oracle only vouches for
	// what actually compiles, so these are reported rather than silently dropped.
	Unparseable int `json:"unparseable"`
}

func main() {
	root := "."
	if len(os.Args) > 1 {
		root = os.Args[1]
	}

	var o out
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			base := info.Name()
			// Mirror funcfinder's traversal: skip vcs/vendored/hidden dirs.
			if base == ".git" || base == "vendor" || base == "node_modules" {
				return filepath.SkipDir
			}
			if len(base) > 0 && base[0] == '.' && path != root {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}

		fset := token.NewFileSet()
		f, perr := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if perr != nil {
			o.Unparseable++
			return nil
		}

		fo := fileOut{Path: path, Functions: []sym{}, Classes: []sym{}}
		for _, decl := range f.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				// Both plain funcs and methods; name is the bare identifier, the
				// way funcfinder reports it. Line is the `func` keyword.
				fo.Functions = append(fo.Functions, sym{d.Name.Name, fset.Position(d.Pos()).Line})
			case *ast.GenDecl:
				if d.Tok == token.TYPE {
					for _, spec := range d.Specs {
						if ts, ok := spec.(*ast.TypeSpec); ok {
							fo.Classes = append(fo.Classes, sym{ts.Name.Name, fset.Position(ts.Pos()).Line})
						}
					}
				}
			}
		}

		if len(fo.Functions) > 0 || len(fo.Classes) > 0 {
			o.Files = append(o.Files, fo)
			o.TotalFiles++
			o.TotalFunctions += len(fo.Functions)
			o.TotalClasses += len(fo.Classes)
		}
		return nil
	})

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(o)
}

// names.go - the names listing (--names): every named declaration of a file or a directory, one per line.
package internal

import (
	"fmt"
	"sort"
	"strings"
)

// nameLine is one declaration in the names listing.
type nameLine struct {
	Kind string
	Name string
	Line int
}

// ProcessSingleFile runs the directory job on one file, so a file and a directory give the same names.
func ProcessSingleFile(config Config, path, langKey, workMode string) DirResult {
	dp := NewDirProcessor(config, 1, false, false, workMode)
	return dp.processFile(Job{Path: path, LangKey: langKey})
}

// functionKind is "method" for a function inside a class, and "function" otherwise.
func functionKind(fn FunctionBounds) string {
	if fn.ClassName != "" {
		return "method"
	}
	return "function"
}

// classKind is the kind of a type from the struct finder, and "class" for one only the class finder found.
func classKind(cl ClassBounds) string {
	if cl.Kind == "" {
		return "class"
	}
	return cl.Kind
}

// FormatNames lists the names of the results, one per line: kind, name and file:line, tab-separated. Files are in path
// order and the names of a file in line order. Kinds: "function"; "method" for a function inside a class; the kind of a
// type ("class" when the class finder found it); "alias" for a Go type alias.
func FormatNames(results []DirResult) string {
	results = append([]DirResult(nil), results...)
	sort.SliceStable(results, func(i, j int) bool { return results[i].Path < results[j].Path })

	var b strings.Builder
	for _, r := range results {
		var names []nameLine
		for _, fn := range r.Functions {
			names = append(names, nameLine{Kind: functionKind(fn), Name: fn.Name, Line: fn.Start})
		}
		for _, cl := range r.Classes {
			names = append(names, nameLine{Kind: classKind(cl), Name: cl.Name, Line: cl.Start})
		}
		for _, a := range r.Aliases {
			names = append(names, nameLine{Kind: "alias", Name: a.Name, Line: a.Start})
		}
		sort.SliceStable(names, func(i, j int) bool { return names[i].Line < names[j].Line })
		for _, n := range names {
			fmt.Fprintf(&b, "%s\t%s\t%s:%d\n", n.Kind, n.Name, r.Path, n.Line)
		}
	}
	return b.String()
}

// gomodule.go - Go module paths for the plain listing: an import under the project's own module is internal.
package main

import (
	"path/filepath"
	"strings"

	"github.com/ruslano69/funcfinder/internal"
)

// findGoModulePath returns the module path of the nearest go.mod at or above dir, or "" when there is none. A run on a
// subdirectory still knows its module, so the imports of the project's own packages are not taken for external ones.
func findGoModulePath(dir string) string {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return ""
	}
	for {
		if mod := internal.DetectModulePrefix(abs); mod != "" {
			return mod
		}
		parent := filepath.Dir(abs)
		if parent == abs {
			return ""
		}
		abs = parent
	}
}

// underModule reports whether a Go import path is one of the module paths in localPkgs or a package under one.
func underModule(dep string, localPkgs map[string]bool) bool {
	for mod := range localPkgs {
		if dep == mod || strings.HasPrefix(dep, mod+"/") {
			return true
		}
	}
	return false
}

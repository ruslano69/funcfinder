// exclude.go - the -exclude globs of the complexity command: the files of an analysed tree to leave out.
package internal

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
)

// FilterExcluded returns the paths that no pattern excludes. A pattern is a glob in path.Match syntax, and it matches a
// file by its base name (`*_test.go`) or by its path under the analysed directory, with forward slashes
// (`internal/gen/*.go`). root is the directory or file the analysis started from; for a file, paths are taken relative
// to its directory. A malformed pattern is an error, reported before any file is looked at.
func FilterExcluded(paths []string, root string, patterns []string) ([]string, error) {
	if len(patterns) == 0 {
		return paths, nil
	}
	for _, pattern := range patterns {
		if _, err := path.Match(pattern, ""); err != nil {
			return nil, fmt.Errorf("bad -exclude pattern %q: %v", pattern, err)
		}
	}

	base := root
	if info, err := os.Stat(root); err == nil && !info.IsDir() {
		base = filepath.Dir(root)
	}
	kept := make([]string, 0, len(paths))
	for _, file := range paths {
		if !isExcluded(file, base, patterns) {
			kept = append(kept, file)
		}
	}
	return kept, nil
}

// isExcluded reports whether one pattern matches the file's base name or its path under base.
func isExcluded(file, base string, patterns []string) bool {
	rel, err := filepath.Rel(base, file)
	if err != nil {
		rel = file
	}
	rel = filepath.ToSlash(rel)
	name := path.Base(rel)
	for _, pattern := range patterns {
		if ok, _ := path.Match(pattern, name); ok {
			return true
		}
		if ok, _ := path.Match(pattern, rel); ok {
			return true
		}
	}
	return false
}

// detect.go - picks the language of a directory or file when the caller does not force one (complexity without -l).
package internal

import (
	"os"
	"sort"
)

// DetectLanguage returns the language of root. For a file, it is the language its extension names. For a directory,
// it is the language with the most source files under it: recursive, and respecting .gitignore like CollectSourceFiles.
// A tie goes to the language key that sorts first, so the same tree always gives the same answer. The returned counts
// are the source files per language key. The language is nil when no supported source file is found.
func DetectLanguage(config Config, root string) (*LanguageConfig, map[string]int, error) {
	info, err := os.Stat(root)
	if err != nil {
		return nil, nil, err
	}

	var paths []string
	if info.IsDir() {
		paths, err = CollectSourceFiles(root, nil, true)
		if err != nil {
			return nil, nil, err
		}
	} else {
		paths = []string{root}
	}

	counts := make(map[string]int)
	byKey := make(map[string]*LanguageConfig)
	for _, path := range paths {
		if lc := config.GetLanguageByExtension(path); lc != nil {
			counts[lc.LangKey]++
			byKey[lc.LangKey] = lc
		}
	}

	keys := make([]string, 0, len(counts))
	for key := range counts {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	best := ""
	for _, key := range keys {
		if best == "" || counts[key] > counts[best] {
			best = key
		}
	}
	if best == "" {
		return nil, counts, nil
	}
	return byKey[best], counts, nil
}

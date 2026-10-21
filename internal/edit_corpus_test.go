package internal

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// TestEditCorpusIdentity replaces every function of a real project with its own text. The output must be the same
// file, byte for byte. It is skipped unless FF_EDIT_CORPUS names a directory, and FF_EDIT_LANG optionally names
// one language key. Refusals are counted and printed: a refusal of an identity edit is a finder or check problem.
func TestEditCorpusIdentity(t *testing.T) {
	root := os.Getenv("FF_EDIT_CORPUS")
	if root == "" {
		t.Skip("set FF_EDIT_CORPUS to a project directory")
	}
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	only := os.Getenv("FF_EDIT_LANG")

	stats := map[string]int{}
	refusals := map[string]int{}
	var changed []string
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if path != root && strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		lc := cfg.GetLanguageByExtension(path)
		if lc == nil || (only != "" && lc.LangKey != only) {
			return nil
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		stats["files"]++
		text := strings.TrimSuffix(strings.ReplaceAll(string(src), "\r\n", "\n"), "\n")
		lines := strings.Split(text, "\n")
		finder := CreateFinder(lc, "", "map", false, false)
		result, err := finder.FindFunctions(path)
		if err != nil {
			stats["unreadable"]++
			return nil
		}
		for _, fn := range result.Functions {
			end := fn.End
			if end > len(lines) {
				end = len(lines)
			}
			if fn.Start < 1 || fn.Start > end {
				continue
			}
			stats["functions"]++
			body := strings.Join(lines[fn.Start-1:end], "\n") + "\n"
			out, err := ReplaceFunction(lc, src, FuncTarget{Name: fn.Name, Line: fn.Start}, nil, []byte(body))
			if err != nil {
				refusals[firstWords(err.Error())]++
				continue
			}
			if !bytes.Equal(out.Output, src) {
				stats["changed"]++
				if len(changed) < 20 {
					changed = append(changed, fmt.Sprintf("%s:%d %s", path, fn.Start, fn.Name))
				}
				continue
			}
			stats["identical"]++
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}

	t.Logf("stats: %v", stats)
	keys := make([]string, 0, len(refusals))
	for k := range refusals {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return refusals[keys[i]] > refusals[keys[j]] })
	for _, k := range keys {
		t.Logf("refused %5d: %s", refusals[k], k)
	}
	for _, c := range changed {
		t.Errorf("identity edit changed the file: %s", c)
	}
}

// firstWords shortens an error message to its first few words, so refusals group together.
func firstWords(msg string) string {
	words := strings.Fields(msg)
	if len(words) > 6 {
		words = words[:6]
	}
	return strings.Join(words, " ")
}

// uncalled.go - functions that no code line of an analysed tree names: the candidates for deletion (funcstats --uncalled).
package internal

import (
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"
)

// UncalledFunction is one candidate: where it is declared, and whether its name is exported (Go only). An exported name
// may be used by code outside the analysed tree, so it is marked.
type UncalledFunction struct {
	File     string `json:"file"`
	Line     int    `json:"line"`
	Name     string `json:"name"`
	Exported bool   `json:"exported"`
}

// UncalledReport is the result of FindUncalled: how many functions were considered, and the candidates among them.
type UncalledReport struct {
	Considered int                `json:"considered"`
	Candidates []UncalledFunction `json:"candidates"`
}

// identifierRe matches an identifier in a cleaned line.
var identifierRe = regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]*`)

// goImplicitMethods are methods that the standard library or the runtime calls through an interface, so no line of the
// tree names them: fmt calls String and Error, sort calls Len, Less and Swap, and so on.
var goImplicitMethods = map[string]bool{
	"String": true, "Error": true, "Len": true, "Less": true, "Swap": true, "Close": true, "Read": true, "Write": true,
	"MarshalJSON": true, "UnmarshalJSON": true, "ServeHTTP": true,
}

// entryPointNames are started by the program, not called by name.
var entryPointNames = map[string]bool{"main": true, "init": true}

// isTestFile reports whether a file holds tests. Their functions run under a test runner, so they are never candidates,
// but a call from a test still counts as a use of the function it calls.
func isTestFile(path, langKey string) bool {
	base := filepath.Base(path)
	switch langKey {
	case "go":
		return strings.HasSuffix(base, "_test.go")
	case "py":
		return strings.HasPrefix(base, "test_") || strings.HasSuffix(base, "_test.py")
	case "js", "ts":
		return strings.Contains(base, ".test.") || strings.Contains(base, ".spec.")
	}
	return false
}

// declaration is one function found in a file, with what decides whether it can be a candidate.
type declaration struct {
	file     string
	line     int
	name     string
	exported bool
	excluded bool
}

// FindUncalled returns the functions declared in files that no code line of files names. root is the analysed
// directory: it is where the internal-package rule for exported names starts. A name counts as used when it
// appears as an identifier in cleaned code: a comment or a string is not a use. A declaration's own name does not count,
// but another name on the same line does (`func A() { B() }` uses B). Functions in test files, the entry points (main,
// init) and the Go interface methods in goImplicitMethods are not candidates.
//
// The candidates are a list to check, not a verdict. A name used only by reflection, by a dispatch table keyed on a
// string, or by another repository looks uncalled. A recursive function counts its own calls as uses, and a function
// called only by an uncalled function is not a candidate, so the list is conservative.
func FindUncalled(config *LanguageConfig, root string, files []string) (UncalledReport, error) {
	uses := make(map[string]int)
	var decls []declaration
	langKey := config.LangKey
	finder := CreateFinder(config, "", "map", false, false)

	for _, file := range files {
		lines, err := readFileLines(file)
		if err != nil {
			return UncalledReport{}, err
		}

		// The names declared on each line, so the declared name is left out of that line's uses.
		declaredOn := make(map[int][]string)
		testFile := isTestFile(file, langKey)
		if fr, ferr := finder.FindFunctions(file); ferr == nil {
			for _, fn := range fr.Functions {
				declaredOn[fn.Start] = append(declaredOn[fn.Start], fn.Name)
				decls = append(decls, declaration{
					file:     file,
					line:     fn.Start,
					name:     fn.Name,
					exported: langKey == "go" && unicode.IsUpper(initialRune(fn.Name)) && !underInternal(root, file),
					excluded: testFile || entryPointNames[fn.Name] || (langKey == "go" && goImplicitMethods[fn.Name]),
				})
			}
		}

		sanitizer := NewSanitizer(config, false)
		state := StateNormal
		for i, line := range lines {
			cleaned, next := sanitizer.CleanLine(line, state)
			state = next
			tokens := identifierRe.FindAllString(cleaned, -1)
			for _, name := range declaredOn[i+1] {
				tokens = dropFirst(tokens, name)
			}
			for _, token := range tokens {
				uses[token]++
			}
		}
	}

	report := UncalledReport{Candidates: []UncalledFunction{}}
	for _, d := range decls {
		if d.excluded {
			continue
		}
		report.Considered++
		if uses[d.name] == 0 {
			report.Candidates = append(report.Candidates, UncalledFunction{
				File: d.file, Line: d.line, Name: d.name, Exported: d.exported,
			})
		}
	}
	sort.SliceStable(report.Candidates, func(i, j int) bool {
		a, b := report.Candidates[i], report.Candidates[j]
		if a.File != b.File {
			return a.File < b.File
		}
		return a.Line < b.Line
	})
	return report, nil
}

// underInternal reports whether a file lies under an internal directory below root. Go does not let code outside the
// module import such a package, so an exported name there cannot be used from outside.
func underInternal(root, file string) bool {
	rel, err := filepath.Rel(root, file)
	if err != nil {
		rel = file
	}
	for _, part := range strings.Split(filepath.ToSlash(filepath.Dir(rel)), "/") {
		if part == "internal" {
			return true
		}
	}
	return false
}

// dropFirst returns tokens without its first element equal to name.
func dropFirst(tokens []string, name string) []string {
	for i, token := range tokens {
		if token == name {
			return append(tokens[:i:i], tokens[i+1:]...)
		}
	}
	return tokens
}

// initialRune returns the first rune of s, or 0 for an empty name.
func initialRune(s string) rune {
	for _, r := range s {
		return r
	}
	return 0
}

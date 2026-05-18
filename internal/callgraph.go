package internal

import (
	"bufio"
	"os"
	"regexp"
	"sort"
	"strings"
)

// CallEdge represents a directed call from one function to another.
type CallEdge struct {
	Caller string `json:"caller"`
	Callee string `json:"callee"`
	Line   int    `json:"line"` // line number of the call site (1-based)
}

// FileCallGraph is the call graph for a single source file.
type FileCallGraph struct {
	Path  string     `json:"path"`
	Calls []CallEdge `json:"calls"`
}

// CallGraphResult is the full result for a directory or file.
type CallGraphResult struct {
	Files          []FileCallGraph `json:"files"`
	TotalCalls     int             `json:"total_calls"`
	TotalFunctions int             `json:"total_functions"`
}

// callIdentRe matches a bare call candidate: word( or word.word(
// Captures: 1=receiver (optional), 2=function name
var callIdentRe = regexp.MustCompile(`\b([A-Za-z_][A-Za-z0-9_]*)\.([A-Za-z_][A-Za-z0-9_]*)\s*\(|` +
	`\b([A-Za-z_][A-Za-z0-9_]*)\s*\(`)

// BuildFileCallGraph extracts the call graph from a single file.
//
// knownFuncs is the set of function names defined in this file (and optionally
// same-package neighbours). importAliases maps import alias → package short name
// so "alias.Func" → "pkg.Func".
func BuildFileCallGraph(
	path string,
	langConfig *LanguageConfig,
	knownFuncs map[string]bool,
	importAliases map[string]string, // alias → package base name
) (*FileCallGraph, error) {
	// 1. Read all lines
	lines, err := readAllLines(path)
	if err != nil {
		return nil, err
	}

	// 2. Map function name → (start, end) from funcfinder
	finder := CreateFinder(langConfig, "", "map", false, false)
	fr, err := finder.FindFunctions(path)
	if err != nil {
		return nil, err
	}

	// Build a combined known set: file-local functions + provided knownFuncs
	localFuncs := make(map[string]bool, len(fr.Functions)+len(knownFuncs))
	for _, f := range fr.Functions {
		localFuncs[f.Name] = true
	}
	for k := range knownFuncs {
		localFuncs[k] = true
	}

	// Build line → caller name index
	type funcRange struct {
		name  string
		start int
		end   int
	}
	var ranges []funcRange
	for _, f := range fr.Functions {
		ranges = append(ranges, funcRange{f.Name, f.Start, f.End})
	}
	sort.Slice(ranges, func(i, j int) bool { return ranges[i].start < ranges[j].start })

	callerAt := func(lineNo int) string {
		for _, r := range ranges {
			if lineNo >= r.start && lineNo <= r.end {
				return r.name
			}
		}
		return ""
	}

	// 3. Sanitize and scan for calls
	sanitizer := NewSanitizer(langConfig, false)
	state := StateNormal

	cgFile := &FileCallGraph{Path: path}
	seen := make(map[string]bool)

	for i, raw := range lines {
		lineNo := i + 1
		clean, newState := sanitizer.CleanLine(raw, state)
		state = newState

		caller := callerAt(lineNo)
		if caller == "" {
			continue // outside any known function
		}

		matches := callIdentRe.FindAllStringSubmatch(clean, -1)
		for _, m := range matches {
			var callee string
			if m[1] != "" && m[2] != "" {
				// pkg.Func( form
				receiver := m[1]
				fname := m[2]
				if pkg, ok := importAliases[receiver]; ok {
					callee = pkg + "." + fname
				} else {
					callee = receiver + "." + fname
				}
			} else if m[3] != "" {
				// bare Func( form
				fname := m[3]
				if !localFuncs[fname] {
					continue // not a known function → skip
				}
				callee = fname
			}
			if callee == "" || callee == caller {
				continue
			}
			key := caller + "→" + callee
			if seen[key] {
				continue
			}
			seen[key] = true
			cgFile.Calls = append(cgFile.Calls, CallEdge{
				Caller: caller,
				Callee: callee,
				Line:   lineNo,
			})
		}
	}

	sort.Slice(cgFile.Calls, func(i, j int) bool {
		if cgFile.Calls[i].Caller != cgFile.Calls[j].Caller {
			return cgFile.Calls[i].Caller < cgFile.Calls[j].Caller
		}
		return cgFile.Calls[i].Callee < cgFile.Calls[j].Callee
	})

	return cgFile, nil
}

// BuildDirCallGraph processes all files in results and builds the call graph.
// It collects all function names across the directory first, then resolves calls.
func BuildDirCallGraph(
	results []DirResult,
	config Config,
	importsByFile map[string]map[string]string, // path → alias→pkg
) *CallGraphResult {
	// Global function name set (all functions in the dir)
	globalFuncs := make(map[string]bool)
	for _, r := range results {
		for _, f := range r.Functions {
			globalFuncs[f.Name] = true
		}
	}

	cg := &CallGraphResult{}
	cg.TotalFunctions = len(globalFuncs)

	for _, r := range results {
		if len(r.Functions) == 0 {
			continue
		}
		langConfig := config.GetLanguageByExtension(r.Path)
		if langConfig == nil {
			continue
		}
		aliases := importsByFile[r.Path]
		fcg, err := BuildFileCallGraph(r.Path, langConfig, globalFuncs, aliases)
		if err != nil || len(fcg.Calls) == 0 {
			continue
		}
		cg.Files = append(cg.Files, *fcg)
		cg.TotalCalls += len(fcg.Calls)
	}
	return cg
}

// ReverseCallGraph returns a map callee → []callers across all files.
func ReverseCallGraph(cg *CallGraphResult) map[string][]string {
	rev := make(map[string][]string)
	seen := make(map[string]bool)
	for _, f := range cg.Files {
		for _, e := range f.Calls {
			key := e.Callee + "←" + e.Caller
			if seen[key] {
				continue
			}
			seen[key] = true
			rev[e.Callee] = append(rev[e.Callee], e.Caller)
		}
	}
	return rev
}

// FormatCallGraphText returns a human-readable call graph.
func FormatCallGraphText(cg *CallGraphResult) string {
	var sb strings.Builder
	for _, f := range cg.Files {
		sb.WriteString(f.Path + "\n")
		for _, e := range f.Calls {
			sb.WriteString("  " + e.Caller + " → " + e.Callee + "\n")
		}
	}
	return strings.TrimRight(sb.String(), "\n")
}

// readAllLines reads a file and returns all lines (without newline characters).
func readAllLines(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var lines []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		lines = append(lines, sc.Text())
	}
	return lines, sc.Err()
}

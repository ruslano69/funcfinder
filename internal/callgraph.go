package internal

import (
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
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
// Captures: 1=receiver, 2=method (for the pkg.Func form); 3=function name (bare).
//
// Built from the shared identifier classes (identStart/identClass) so call-site
// detection recognises exactly the same identifiers as the language patterns
// that detect definitions — including non-ASCII names. Each identifier must
// start with a letter/underscore (identStart) so numeric literals like `123(`
// are not mistaken for calls. The leading `\b` of the old ASCII regex is
// dropped because RE2 word boundaries are ASCII-only (they never fire before a
// non-ASCII letter); leftmost-longest matching of the receiver form already
// prevents a method name from also matching as a separate bare call.
var ident = identStart + identClass + `*`
var callIdentRe = regexp.MustCompile(`(` + ident + `)\.(` + ident + `)\s*\(|` +
	`(` + ident + `)\s*\(`)

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

	return buildFileCallGraph(path, lines, fr.Functions, langConfig, knownFuncs, importAliases), nil
}

// buildFileCallGraph scans the lines of one file for calls, given the file's function bounds. BuildFileCallGraph
// finds the bounds itself; BuildDirCallGraph passes the ones the directory job already found.
func buildFileCallGraph(
	path string,
	lines []string,
	funcs []FunctionBounds,
	langConfig *LanguageConfig,
	knownFuncs map[string]bool,
	importAliases map[string]string,
) *FileCallGraph {
	// A bare call counts when it names a function of this file or one of knownFuncs. The two sets are looked up in
	// turn, not merged: knownFuncs is every function of a directory run, and copying it once per file was most of
	// the time of a callgraph run on a large tree.
	fileFuncs := make(map[string]bool, len(funcs))
	signatures := make(map[int]map[string]string)
	for _, f := range funcs {
		fileFuncs[f.Name] = true
		if f.SignatureLine > 0 {
			if signatures[f.SignatureLine] == nil {
				signatures[f.SignatureLine] = make(map[string]string)
			}
			name := f.SignatureName
			if name == "" {
				name = f.Name
			}
			signatures[f.SignatureLine][f.Name] = name
		}
	}

	// Build line → caller name index
	type funcRange struct {
		name  string
		start int
		end   int
	}
	var ranges []funcRange
	for _, f := range funcs {
		ranges = append(ranges, funcRange{f.Name, f.Start, f.End})
	}
	sort.Slice(ranges, func(i, j int) bool { return ranges[i].start < ranges[j].start })

	callerAt := func(lineNo int) string {
		if langConfig.LangKey == "cs" || langConfig.LangKey == "java" {
			for i := len(ranges) - 1; i >= 0; i-- {
				if ranges[i].start <= lineNo && ranges[i].end >= lineNo {
					return ranges[i].name
				}
			}
			return ""
		}
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
	funcRegex := langConfig.FuncRegex()

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

		// C# and Java supply declaration coordinates, including split headers.
		// C# also names destructors/conversions. Other finders retain the regex
		// fallback: a decorated declaration may start before its name line.
		signatureName, isOwnSignatureLine := signatures[lineNo][caller]
		if langConfig.LangKey != "cs" && langConfig.LangKey != "java" && funcRegex != nil && strings.Contains(clean, caller) {
			if m := funcRegex.FindStringSubmatch(clean); m != nil {
				if ExtractFuncName(m) == caller {
					isOwnSignatureLine = true
				}
			}
		}

		matches := callIdentRe.FindAllStringSubmatch(clean, -1)
		skippedOwnSignature := false
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
				if !fileFuncs[fname] && !knownFuncs[fname] {
					continue // not a known function → skip
				}
				callee = fname
			}
			if callee == "" {
				continue
			}
			ownName := callee == caller
			if langConfig.LangKey == "cs" || langConfig.LangKey == "java" {
				ownName = m[2] == signatureName || m[3] == signatureName
			}
			if ownName && isOwnSignatureLine && !skippedOwnSignature {
				skippedOwnSignature = true
				// Skip the declaration token once. A recursive call later on
				// this same line, or elsewhere in the body, still counts.
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

	return cgFile
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

	// Files are scanned in parallel, each into its own slot, and reported in path order: results come in the order the
	// directory workers finished, and the output must not depend on that. The maps are only read by the workers.
	results = append([]DirResult(nil), results...)
	sort.SliceStable(results, func(i, j int) bool { return results[i].Path < results[j].Path })
	files := make([]*FileCallGraph, len(results))
	sem := make(chan struct{}, runtime.NumCPU())
	var wg sync.WaitGroup
	for i, r := range results {
		if len(r.Functions) == 0 {
			continue
		}
		langConfig := config.GetLanguageByExtension(r.Path)
		if langConfig == nil {
			continue
		}
		wg.Add(1)
		go func(i int, r DirResult, langConfig *LanguageConfig) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			files[i] = dirFileCallGraph(r, langConfig, globalFuncs, importsByFile[r.Path])
		}(i, r, langConfig)
	}
	wg.Wait()

	for _, fcg := range files {
		if fcg == nil || len(fcg.Calls) == 0 {
			continue
		}
		cg.Files = append(cg.Files, *fcg)
		cg.TotalCalls += len(fcg.Calls)
	}
	return cg
}

// dirFileCallGraph builds the call graph of one file of a directory job. It reuses the function bounds the job found
// when the job read the file with the same language that scans it for calls; finding them again was most of the time
// of a callgraph run. The two differ when -l restricts the job to a language whose extension the full config gives to
// another one (a .h file under -l c is scanned as cpp): then the bounds are found again with the scanning language, as
// before. It returns nil when the file cannot be read.
func dirFileCallGraph(r DirResult, langConfig *LanguageConfig, globalFuncs map[string]bool, aliases map[string]string) *FileCallGraph {
	if r.LangKey != langConfig.LangKey {
		fcg, err := BuildFileCallGraph(r.Path, langConfig, globalFuncs, aliases)
		if err != nil {
			return nil
		}
		return fcg
	}
	lines, err := readAllLines(r.Path)
	if err != nil {
		return nil
	}
	return buildFileCallGraph(r.Path, lines, r.Functions, langConfig, globalFuncs, aliases)
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

// ReverseCallers returns sorted, unique callers of name. Qualified names match
// exactly; bare names also match the final dot-separated callee component.
// Bare lookup finds name-based candidates, not type-resolved method identities.
func ReverseCallers(rev map[string][]string, name string) []string {
	var callers []string
	seen := make(map[string]bool)
	for callee, candidates := range rev {
		if callee != name && (strings.Contains(name, ".") || !strings.HasSuffix(callee, "."+name)) {
			continue
		}
		for _, caller := range candidates {
			if !seen[caller] {
				seen[caller] = true
				callers = append(callers, caller)
			}
		}
	}
	sort.Strings(callers)
	return callers
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
	return readFileLines(path)
}

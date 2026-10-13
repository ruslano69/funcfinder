// callgraph - function call relationship analyzer
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/ruslano69/funcfinder/internal"
)

func main() {
	showVersion := false
	dir := ""
	inp := ""
	lang := ""
	jsonOut := false
	reverseMode := false
	funcFilter := ""
	depth := 0
	noGitignore := false

	for i := 1; i < len(os.Args); i++ {
		arg := os.Args[i]
		switch {
		case arg == "-h" || arg == "--help":
			printHelp()
			return
		case arg == "--version":
			showVersion = true
		case arg == "--dir" && i+1 < len(os.Args):
			dir = os.Args[i+1]
			i++
		case arg == "--inp" && i+1 < len(os.Args):
			inp = os.Args[i+1]
			i++
		case arg == "-l" && i+1 < len(os.Args):
			lang = os.Args[i+1]
			i++
		case arg == "--json" || arg == "-j":
			jsonOut = true
		case arg == "--reverse" || arg == "-r":
			reverseMode = true
		case arg == "--func" && i+1 < len(os.Args):
			funcFilter = os.Args[i+1]
			i++
		case arg == "--depth" && i+1 < len(os.Args):
			fmt.Sscanf(os.Args[i+1], "%d", &depth)
			i++
		case arg == "--no-gitignore":
			noGitignore = true
		case !strings.HasPrefix(arg, "-") && dir == "" && inp == "":
			dir = arg
		}
	}

	if showVersion {
		internal.PrintVersion("callgraph")
	}

	config, err := internal.LoadConfig()
	if err != nil {
		internal.FatalError("loading config: %v", err)
	}

	if inp != "" {
		runFileMode(config, inp, lang, jsonOut, reverseMode, funcFilter, depth)
	} else if dir != "" {
		runDirMode(config, dir, lang, jsonOut, reverseMode, funcFilter, depth, noGitignore)
	} else {
		internal.FatalError("either --dir or --inp must be specified")
	}
}

func runFileMode(config internal.Config, inp, lang string, jsonOut, reverseMode bool, funcFilter string, depth int) {
	if lang == "" {
		internal.FatalError("--inp mode requires -l <lang>")
	}
	langConfig, err := config.GetLanguageConfig(lang)
	if err != nil {
		internal.FatalError("%v", err)
	}

	aliases := collectImports(inp, langConfig)
	fcg, err := internal.BuildFileCallGraph(inp, langConfig, nil, aliases)
	if err != nil {
		internal.FatalError("building call graph: %v", err)
	}

	cg := &internal.CallGraphResult{Files: []internal.FileCallGraph{*fcg}, TotalCalls: len(fcg.Calls)}
	output(cg, jsonOut, reverseMode, funcFilter, depth)
}

func runDirMode(config internal.Config, dir, lang string, jsonOut, reverseMode bool, funcFilter string, depth int, noGitignore bool) {
	var langConfig *internal.LanguageConfig
	var err error
	if lang != "" {
		langConfig, err = config.GetLanguageConfig(lang)
		if err != nil {
			internal.FatalError("%v", err)
		}
	}

	// Collect files. The list is only used to read import aliases, so the walk runs while the directory job below
	// finds the functions, and its result is awaited before the imports are read.
	var allFiles []string
	var walkErr error
	walkDone := make(chan struct{})
	go func() {
		defer close(walkDone)
		if langConfig != nil {
			allFiles, walkErr = internal.CollectSourceFiles(dir, langConfig, true, !noGitignore)
		} else {
			// Auto-detect: collect all supported files
			for _, lc := range config {
				files, _ := internal.CollectSourceFiles(dir, lc, true, !noGitignore)
				allFiles = append(allFiles, files...)
			}
		}
	}()

	// Run funcfinder on each file to get function boundaries. When -l is set,
	// scope this to that single language too — otherwise DirProcessor
	// auto-detects and processes every supported language in dir regardless
	// of -l, silently mixing in e.g. Python functions/edges on a "-l rust"
	// run over a Python backend (TODO.md "-l is a hint, not a filter" bug).
	procConfig := config
	if langConfig != nil {
		procConfig = internal.Config{lang: langConfig}
	}
	processor := internal.NewDirProcessor(procConfig, 0, true, !noGitignore, "functions")
	results, err := processor.ProcessDirectory(dir)
	<-walkDone
	if walkErr != nil {
		internal.FatalError("collecting files: %v", walkErr)
	}
	if err != nil {
		internal.FatalError("processing directory: %v", err)
	}

	// Collect per-file import aliases, in parallel, for the files the call graph scans: BuildDirCallGraph reads
	// importsByFile only for a result with functions. A file the walk did not list keeps no aliases, as before.
	listed := make(map[string]bool, len(allFiles))
	for _, path := range allFiles {
		listed[path] = true
	}
	var importPaths []string
	var importLangs []*internal.LanguageConfig
	for _, r := range results {
		if len(r.Functions) == 0 || !listed[r.Path] {
			continue
		}
		if lc := config.GetLanguageByExtension(r.Path); lc != nil {
			importPaths = append(importPaths, r.Path)
			importLangs = append(importLangs, lc)
		}
	}
	aliases := make([]map[string]string, len(importPaths))
	internal.ForEachParallel(len(importPaths), func(i int) {
		aliases[i] = collectImports(importPaths[i], importLangs[i])
	})
	importsByFile := make(map[string]map[string]string, len(importPaths))
	for i, path := range importPaths {
		importsByFile[path] = aliases[i]
	}

	cg := internal.BuildDirCallGraph(results, config, importsByFile)
	output(cg, jsonOut, reverseMode, funcFilter, depth)

	internal.InfoMessage("Analyzed %d files, found %d call edges across %d functions",
		len(results), cg.TotalCalls, cg.TotalFunctions)
}

// output renders the call graph result.
func output(cg *internal.CallGraphResult, jsonOut, reverseMode bool, funcFilter string, depth int) {
	if reverseMode {
		rev := internal.ReverseCallGraph(cg)
		if funcFilter != "" {
			// Single function reverse lookup
			callers := internal.ReverseCallers(rev, funcFilter)
			if jsonOut {
				out, _ := json.MarshalIndent(map[string]any{
					"function": funcFilter,
					"callers":  callers,
				}, "", "  ")
				fmt.Println(string(out))
			} else {
				if len(callers) == 0 {
					fmt.Printf("%s: not called by any known function\n", funcFilter)
				} else {
					fmt.Printf("%s is called by:\n", funcFilter)
					for _, c := range callers {
						fmt.Printf("  %s\n", c)
					}
				}
			}
			return
		}
		// Full reverse graph
		if jsonOut {
			type revEntry struct {
				Callee  string   `json:"callee"`
				Callers []string `json:"callers"`
			}
			var entries []revEntry
			for callee, callers := range rev {
				sort.Strings(callers)
				entries = append(entries, revEntry{callee, callers})
			}
			sort.Slice(entries, func(i, j int) bool { return entries[i].Callee < entries[j].Callee })
			out, _ := json.MarshalIndent(map[string]any{"reverse": entries}, "", "  ")
			fmt.Println(string(out))
		} else {
			type revEntry struct {
				callee  string
				callers []string
			}
			var entries []revEntry
			for callee, callers := range rev {
				sort.Strings(callers)
				entries = append(entries, revEntry{callee, callers})
			}
			sort.Slice(entries, func(i, j int) bool { return entries[i].callee < entries[j].callee })
			for _, e := range entries {
				fmt.Printf("%s ← %s\n", e.callee, strings.Join(e.callers, ", "))
			}
		}
		return
	}

	// Forward graph — optional func filter + depth limit
	if funcFilter != "" {
		filtered := filterCallGraph(cg, funcFilter, depth)
		if jsonOut {
			out, _ := json.MarshalIndent(filtered, "", "  ")
			fmt.Println(string(out))
		} else {
			printCallTree(filtered, funcFilter)
		}
		return
	}

	if jsonOut {
		out, _ := json.MarshalIndent(cg, "", "  ")
		fmt.Println(string(out))
	} else {
		fmt.Println(internal.FormatCallGraphText(cg))
	}
}

// filterCallGraph returns only calls reachable from root within depth hops.
func filterCallGraph(cg *internal.CallGraphResult, root string, maxDepth int) *internal.CallGraphResult {
	// Build a flat callee map: caller → []callee
	callees := make(map[string][]string)
	for _, f := range cg.Files {
		for _, e := range f.Calls {
			callees[e.Caller] = append(callees[e.Caller], e.Callee)
		}
	}

	visited := make(map[string]bool)
	var edges []internal.CallEdge
	var walk func(name string, d int)
	walk = func(name string, d int) {
		if visited[name] {
			return
		}
		visited[name] = true
		if maxDepth > 0 && d >= maxDepth {
			return
		}
		for _, callee := range callees[name] {
			edges = append(edges, internal.CallEdge{Caller: name, Callee: callee})
			walk(callee, d+1)
		}
	}
	walk(root, 0)

	fake := &internal.CallGraphResult{
		Files:      []internal.FileCallGraph{{Path: "(filtered)", Calls: edges}},
		TotalCalls: len(edges),
	}
	return fake
}

// printCallTree prints a simple indented call tree from root. A callee that is
// on the path from root to here calls itself, directly or through others: that
// is recursion. A callee already printed in another branch is not expanded
// again; it is marked "(see above)".
func printCallTree(cg *internal.CallGraphResult, root string) {
	callees := make(map[string][]string)
	for _, f := range cg.Files {
		for _, e := range f.Calls {
			callees[e.Caller] = append(callees[e.Caller], e.Callee)
		}
	}
	onPath := make(map[string]bool)
	shown := make(map[string]bool)
	var print func(name string, prefix string)
	print = func(name string, prefix string) {
		if onPath[name] {
			fmt.Printf("%s%s (↑ recursive)\n", prefix, name)
			return
		}
		if shown[name] {
			fmt.Printf("%s%s (see above)\n", prefix, name)
			return
		}
		shown[name] = true
		onPath[name] = true
		fmt.Printf("%s%s\n", prefix, name)
		for _, c := range callees[name] {
			print(c, prefix+"  ")
		}
		onPath[name] = false
	}
	print(root, "")
}

// collectImports parses a file and returns alias → package base name.
// blockImportRe reads one line of an import block: an optional alias and the quoted path.
var blockImportRe = regexp.MustCompile(`^\s*(?:(\w+)\s+)?"([^"]+)"`)

func collectImports(path string, langConfig *internal.LanguageConfig) map[string]string {
	aliases := make(map[string]string)

	lines, err := internal.ReadLines(path)
	if err != nil {
		internal.WarnError("%s: %v", path, err)
		return aliases
	}

	importRe := langConfig.ImportRegex()
	if importRe == nil {
		return aliases
	}

	inBlock := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if langConfig.MultiLineBlock != "" && strings.HasPrefix(trimmed, langConfig.MultiLineBlock) {
			inBlock = true
			continue
		}
		if inBlock {
			if trimmed == ")" {
				inBlock = false
			} else if m := blockImportRe.FindStringSubmatch(line); len(m) >= 3 {
				alias := m[1]
				pkg := m[2]
				base := filepath.Base(pkg)
				if alias != "" && alias != "_" && alias != "." {
					aliases[alias] = base
				} else {
					aliases[base] = base
				}
			}
			continue
		}

		if m := importRe.FindStringSubmatch(line); len(m) >= 2 {
			pkg := m[len(m)-1]
			base := filepath.Base(pkg)
			aliases[base] = base
		}
	}
	return aliases
}

func printHelp() {
	fmt.Println("Usage: callgraph [OPTIONS]")
	fmt.Println("  --dir <path>       Analyze directory")
	fmt.Println("  --inp <file>       Analyze single file")
	fmt.Println("  -l <lang>          Language (required with --inp; with --dir, restricts the scan to this language only — omit to auto-detect every supported language)")
	fmt.Println("  --json, -j         JSON output")
	fmt.Println("  --reverse, -r      Reverse graph: who calls each function")
	fmt.Println("  --func <name>      Focus on one function (with optional --depth)")
	fmt.Println("  --depth <n>        Limit traversal depth (default: unlimited)")
	fmt.Println("  --no-gitignore     Ignore .gitignore rules")
	fmt.Println("  --version          Print version")
}

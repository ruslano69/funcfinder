// callgraph - function call relationship analyzer
package main

import (
	"bufio"
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

	// Collect files
	var allFiles []string
	if langConfig != nil {
		allFiles, err = internal.CollectSourceFiles(dir, langConfig, true, !noGitignore)
	} else {
		// Auto-detect: collect all supported files
		for _, lc := range config {
			files, _ := internal.CollectSourceFiles(dir, lc, true, !noGitignore)
			allFiles = append(allFiles, files...)
		}
	}
	if err != nil {
		internal.FatalError("collecting files: %v", err)
	}

	// Run funcfinder on each file to get function boundaries
	processor := internal.NewDirProcessor(config, 0, true, !noGitignore, "functions")
	results, err := processor.ProcessDirectory(dir)
	if err != nil {
		internal.FatalError("processing directory: %v", err)
	}

	// Collect per-file import aliases
	importsByFile := make(map[string]map[string]string)
	for _, path := range allFiles {
		lc := config.GetLanguageByExtension(path)
		if lc == nil {
			continue
		}
		importsByFile[path] = collectImports(path, lc)
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
			callers := rev[funcFilter]
			sort.Strings(callers)
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
			type revEntry struct{ callee string; callers []string }
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

// printCallTree prints a simple indented call tree from root.
func printCallTree(cg *internal.CallGraphResult, root string) {
	callees := make(map[string][]string)
	for _, f := range cg.Files {
		for _, e := range f.Calls {
			callees[e.Caller] = append(callees[e.Caller], e.Callee)
		}
	}
	var print func(name string, prefix string, visited map[string]bool)
	print = func(name string, prefix string, visited map[string]bool) {
		if visited[name] {
			fmt.Printf("%s%s (↑ recursive)\n", prefix, name)
			return
		}
		visited[name] = true
		fmt.Printf("%s%s\n", prefix, name)
		for _, c := range callees[name] {
			print(c, prefix+"  ", visited)
		}
	}
	print(root, "", make(map[string]bool))
}

// collectImports parses a file and returns alias → package base name.
func collectImports(path string, langConfig *internal.LanguageConfig) map[string]string {
	aliases := make(map[string]string)

	f, err := os.Open(path)
	if err != nil {
		return aliases
	}
	defer f.Close()

	importRe := langConfig.ImportRegex()
	if importRe == nil {
		return aliases
	}

	blockImportRe := regexp.MustCompile(`^\s*(?:(\w+)\s+)?"([^"]+)"`)
	inBlock := false

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
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
	fmt.Println("  -l <lang>          Language (required with --inp)")
	fmt.Println("  --json, -j         JSON output")
	fmt.Println("  --reverse, -r      Reverse graph: who calls each function")
	fmt.Println("  --func <name>      Focus on one function (with optional --depth)")
	fmt.Println("  --depth <n>        Limit traversal depth (default: unlimited)")
	fmt.Println("  --no-gitignore     Ignore .gitignore rules")
	fmt.Println("  --version          Print version")
}

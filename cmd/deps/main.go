// deps.go - Module dependency analyzer
// Uses shared configuration for multiple languages
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

type DepInfo struct {
	Module string   `json:"module"`
	Count  int      `json:"count"`
	Files  []string `json:"files"`
}

type DepResult struct {
	Language           string         `json:"language"`
	TotalImports       int            `json:"total_imports"`
	UniqueModules      int            `json:"unique_modules"`
	Dependencies       []DepInfo      `json:"dependencies"`
	ExternalVsInternal map[string]int `json:"external_vs_internal"`
}

type fileSet map[string]bool

// Stdlib detection for common languages
var stdlibPrefixes = map[string][]string{
	"py":    {"", "builtins.", "sys.", "os.", "json.", "re.", "collections.", "typing.", "__future__."},
	"go":    {"fmt", "os", "io", "strings", "math", "regexp", "encoding/json", "testing", "bytes", "errors"},
	"rs":    {"std::", "core::"},
	"js":    {"assert", "buffer", "crypto", "fs", "http", "path", "url"},
	"ts":    {"assert", "buffer", "crypto", "fs", "http", "path", "url"},
	"java":  {"java.", "javax."},
	"cs":    {"System.", "Microsoft."},
	"c":     {"stdio", "stdlib", "string", "math"},
	"cpp":   {"iostream", "vector", "string", "algorithm"},
	"d":     {"std."},
	"swift": {"Swift", "Foundation"},
}

func isStdlib(module, langKey string) bool {
	prefixes := stdlibPrefixes[langKey]
	for _, p := range prefixes {
		if strings.HasPrefix(module, p) {
			return true
		}
	}
	return false
}

// collectFileImports returns all imports per file as map[absPath][]importedModule
func collectFileImports(filename string, config *internal.LanguageConfig, excludeREs []*regexp.Regexp) []string {
	var imports []string

	file, err := os.Open(filename)
	if err != nil {
		return imports
	}
	defer file.Close()

	importRe := config.ImportRegex()
	if importRe == nil {
		return imports
	}

	blockImportRe := regexp.MustCompile(`^\s*"([^"]+)"`)
	inBlock := false

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		// Handle multi-line import blocks (Go-style)
		if config.MultiLineBlock != "" && strings.HasPrefix(trimmed, config.MultiLineBlock) {
			inBlock = true
			continue
		}
		if inBlock {
			if trimmed == ")" {
				inBlock = false
			} else if match := blockImportRe.FindStringSubmatch(line); len(match) >= 2 {
				dep := match[1]
				if dep != "" && !strings.Contains(dep, "://") {
					imports = append(imports, dep)
				}
			}
			continue
		}

		skip := false
		for _, re := range excludeREs {
			if re.MatchString(trimmed) {
				skip = true
				break
			}
		}
		if skip {
			continue
		}

		if match := importRe.FindStringSubmatch(line); len(match) >= 2 {
			for i := 1; i < len(match); i++ {
				if match[i] != "" && !strings.Contains(match[i], "://") {
					dep := match[i]
					excluded := false
					for _, re := range excludeREs {
						if re.MatchString(dep) {
							excluded = true
							break
						}
					}
					if excluded {
						continue
					}
					if !strings.HasSuffix(dep, "/") {
						imports = append(imports, dep)
					}
				}
			}
		}
	}
	return imports
}

// analyzeDeps returns aggregated deps map (used by standard flat mode).
func analyzeDeps(filename string, config *internal.LanguageConfig, excludeREs []*regexp.Regexp) map[string]fileSet {
	deps := make(map[string]fileSet)
	for _, imp := range collectFileImports(filename, config, excludeREs) {
		if deps[imp] == nil {
			deps[imp] = make(fileSet)
		}
		deps[imp][filename] = true
	}
	return deps
}

func main() {
	showVersion := false
	dir := "."
	lang := ""
	topN := 0
	jsonOut := false
	shardsMode := false
	splitBy := "dir"
	updateManifest := ""
	noGitignore := false

	for i := 1; i < len(os.Args); i++ {
		arg := os.Args[i]
		switch {
		case arg == "-h" || arg == "--help":
			fmt.Println("Usage: deps [OPTIONS] <dir>")
			fmt.Println("  --version              Show version and exit")
			fmt.Println("  -l <lang>              Force language (py, go, rs, js, ts, java, cs, swift, c, cpp, d)")
			fmt.Println("  -n <num>               Show top N dependencies")
			fmt.Println("  -j, --json             Output JSON")
			fmt.Println("  --shards               Output inter-shard dependency graph")
			fmt.Println("  --split-by dir|file    Shard granularity (default: dir)")
			fmt.Println("  --update-manifest <p>  Write depends_on into existing manifest.json")
			fmt.Println("  --no-gitignore         Do not respect .gitignore rules")
			return
		case arg == "--version":
			showVersion = true
		case arg == "-l" && i+1 < len(os.Args):
			lang = os.Args[i+1]
			i++
		case arg == "-n" && i+1 < len(os.Args):
			fmt.Sscanf(os.Args[i+1], "%d", &topN)
			i++
		case arg == "-j" || arg == "--json":
			jsonOut = true
		case arg == "--shards":
			shardsMode = true
		case arg == "--split-by" && i+1 < len(os.Args):
			splitBy = os.Args[i+1]
			i++
		case arg == "--update-manifest" && i+1 < len(os.Args):
			updateManifest = os.Args[i+1]
			i++
		case arg == "--no-gitignore":
			noGitignore = true
		case !strings.HasPrefix(arg, "-"):
			dir = arg
		}
	}

	if showVersion {
		internal.PrintVersion("deps")
	}

	// Load shared configuration
	config, err := internal.LoadConfig()
	if err != nil {
		internal.FatalError("loading config: %v", err)
	}

	var langConfig *internal.LanguageConfig
	if lang != "" {
		langConfig, err = config.GetLanguageConfig(lang)
		if err != nil {
			internal.FatalError("%v\nSupported languages: %s", err, strings.Join(config.GetSupportedLanguages(), ", "))
		}
	} else {
		for _, lc := range config {
			for _, ext := range lc.Extensions {
				files, _ := filepath.Glob(filepath.Join(dir, "*"+ext))
				if len(files) > 0 {
					langConfig = lc
					break
				}
			}
			if langConfig != nil {
				break
			}
		}
	}

	if langConfig == nil {
		internal.FatalError("no supported files found in directory\nSupported languages: %s", strings.Join(config.GetSupportedLanguages(), ", "))
	}

	// Pre-compile ExcludePatterns
	var excludeREs []*regexp.Regexp
	for _, pattern := range langConfig.ExcludePatterns {
		excludeREs = append(excludeREs, regexp.MustCompile(pattern))
	}

	dirFiles, walkErr := internal.CollectSourceFiles(dir, langConfig, true, !noGitignore)
	if walkErr != nil {
		internal.FatalError("walking directory: %v", walkErr)
	}

	// ── Shard dependency graph mode ─────────────────────────────────────────
	if shardsMode || updateManifest != "" {
		absDir, err := filepath.Abs(dir)
		if err != nil {
			internal.FatalError("resolving directory: %v", err)
		}

		// Collect per-file imports
		fileImports := make(map[string][]string, len(dirFiles))
		for _, path := range dirFiles {
			abs, _ := filepath.Abs(path)
			fileImports[abs] = collectFileImports(path, langConfig, excludeREs)
		}

		// Auto-detect module prefix / aliases per language
		modulePrefix := ""
		var aliases map[string]string
		switch langConfig.LangKey {
		case "go":
			modulePrefix = internal.DetectModulePrefix(absDir)
		case "ts", "js":
			aliases = internal.DetectTSAliases(absDir)
			if len(aliases) == 0 {
				if tscPath := internal.DetectTSConfigAbove(absDir); tscPath != "" {
					fmt.Fprintf(os.Stderr,
						"WARNING: found %s above %s, but --shards only looks in the analyzed "+
							"root and one level below — path aliases from it won't resolve. "+
							"Re-run --shards from its directory.\n", tscPath, absDir)
				}
			}
		}

		graph, stats := internal.BuildShardGraph(absDir, splitBy, modulePrefix, fileImports, aliases)
		list := internal.ShardGraphToList(graph)

		if warning := stats.Warning(); warning != "" {
			fmt.Fprintf(os.Stderr, "WARNING: %s\n", warning)
		}

		if updateManifest != "" {
			if err := applyGraphToManifest(updateManifest, graph); err != nil {
				internal.FatalError("updating manifest: %v", err)
			}
			fmt.Fprintf(os.Stderr, "INFO: Updated %s with depends_on for %d shards\n", updateManifest, len(graph))
			if !jsonOut {
				return
			}
		}

		if jsonOut || shardsMode {
			out, _ := json.MarshalIndent(map[string]any{"shards": list}, "", "  ")
			fmt.Println(string(out))
			return
		}

		// Plain text fallback
		for _, sd := range list {
			if len(sd.DependsOn) == 0 {
				continue
			}
			fmt.Printf("%s → %s\n", sd.Shard, strings.Join(sd.DependsOn, ", "))
		}
		return
	}

	// ── Standard flat deps mode ──────────────────────────────────────────────
	allDeps := make(map[string]fileSet)
	for _, path := range dirFiles {
		fileDeps := analyzeDeps(path, langConfig, excludeREs)
		for dep, files := range fileDeps {
			if allDeps[dep] == nil {
				allDeps[dep] = make(fileSet)
			}
			for f := range files {
				allDeps[dep][f] = true
			}
		}
	}

	var deps []DepInfo
	stdlib, external, internalCount := 0, 0, 0

	for dep, files := range allDeps {
		fileList := make([]string, 0, len(files))
		for f := range files {
			fileList = append(fileList, f)
		}
		info := DepInfo{Module: dep, Count: len(fileList), Files: fileList}
		deps = append(deps, info)

		if isStdlib(dep, langConfig.LangKey) {
			stdlib++
		} else if strings.Contains(dep, "internal/") || strings.Contains(dep, "vendor/") {
			internalCount++
		} else if strings.Contains(dep, "/") || strings.Contains(dep, ".") {
			external++
		} else {
			internalCount++
		}
	}

	sort.Slice(deps, func(i, j int) bool { return deps[i].Count > deps[j].Count })

	if jsonOut {
		result := DepResult{
			Language:      langConfig.Name,
			TotalImports:  len(allDeps),
			UniqueModules: len(deps),
			Dependencies:  deps,
			ExternalVsInternal: map[string]int{
				"stdlib":   stdlib,
				"external": external,
				"internal": internalCount,
			},
		}
		jsonBytes, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(jsonBytes))
		return
	}

	fmt.Printf("Language: %s\n", langConfig.Name)
	fmt.Printf("Total imports: %d\n", len(allDeps))
	fmt.Printf("Unique modules: %d\n", len(deps))
	fmt.Println(strings.Repeat("-", 35))
	fmt.Printf("stdlib: %d, external: %d, internal: %d\n", stdlib, external, internalCount)
	fmt.Println(strings.Repeat("-", 35))

	printCount := len(deps)
	if topN > 0 && topN < printCount {
		printCount = topN
	}
	for i := 0; i < printCount; i++ {
		kind := "ext"
		if isStdlib(deps[i].Module, langConfig.LangKey) {
			kind = "std"
		} else if strings.Contains(deps[i].Module, "internal/") || strings.Contains(deps[i].Module, "vendor/") {
			kind = "int"
		}
		fmt.Printf("%-30s %3d (%s)\n", deps[i].Module, deps[i].Count, kind)
	}
}

// applyGraphToManifest reads manifest.json, sets DependsOn per shard, rewrites.
func applyGraphToManifest(manifestPath string, graph internal.ShardGraph) error {
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("reading manifest: %w", err)
	}

	var m internal.Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return fmt.Errorf("parsing manifest: %w", err)
	}

	for i, s := range m.Shards {
		if deps, ok := graph[s.Path]; ok {
			list := make([]string, 0, len(deps))
			for d := range deps {
				list = append(list, d)
			}
			sort.Strings(list)
			m.Shards[i].DependsOn = list
		}
	}

	out, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling manifest: %w", err)
	}
	return os.WriteFile(manifestPath, append(out, '\n'), 0644)
}

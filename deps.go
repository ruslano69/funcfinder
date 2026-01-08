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
	"py":   {"", "builtins.", "sys.", "os.", "json.", "re.", "collections.", "typing.", "__future__."},
	"go":   {"fmt", "os", "io", "strings", "math", "regexp", "encoding/json", "testing", "bytes", "errors"},
	"rs":   {"std::", "core::"},
	"js":   {"assert", "buffer", "crypto", "fs", "http", "path", "url"},
	"ts":   {"assert", "buffer", "crypto", "fs", "http", "path", "url"},
	"java": {"java.", "javax."},
	"cs":   {"System.", "Microsoft."},
	"c":    {"stdio", "stdlib", "string", "math"},
	"cpp":  {"iostream", "vector", "string", "algorithm"},
	"d":    {"std."},
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

func analyzeDeps(filename string, config *LanguageConfig) map[string]fileSet {
	deps := make(map[string]fileSet)

	file, err := os.Open(filename)
	if err != nil {
		return deps
	}
	defer file.Close()

	importRe := config.ImportRegex()
	if importRe == nil {
		return deps
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
					if deps[dep] == nil {
						deps[dep] = make(fileSet)
					}
					deps[dep][filename] = true
				}
			}
			continue
		}

		// Check exclusion patterns
		skip := false
		for _, pattern := range config.ExcludePatterns {
			re := regexp.MustCompile(pattern)
			if re.MatchString(trimmed) {
				skip = true
				break
			}
		}
		if skip {
			continue
		}

		// Extract imports using regex
		if match := importRe.FindStringSubmatch(line); len(match) >= 2 {
			for i := 1; i < len(match); i++ {
				if match[i] != "" && !strings.Contains(match[i], "://") {
					dep := match[i]

					// Additional exclusion check on extracted module
					excluded := false
					for _, pattern := range config.ExcludePatterns {
						re := regexp.MustCompile(pattern)
						if re.MatchString(dep) {
							excluded = true
							break
						}
					}
					if excluded {
						continue
					}

					if !strings.HasSuffix(dep, "/") {
						if deps[dep] == nil {
							deps[dep] = make(fileSet)
						}
						deps[dep][filename] = true
					}
				}
			}
		}
	}
	return deps
}

func main() {
	showVersion := false
	dir := "."
	lang := ""
	topN := 0
	jsonOut := false

	for i := 1; i < len(os.Args); i++ {
		arg := os.Args[i]
		switch {
		case arg == "-h" || arg == "--help":
			fmt.Println("Usage: deps [OPTIONS] <dir>")
			fmt.Println("  --version   Show version and exit")
			fmt.Println("  -l <lang>   Force language (py, go, rs, js, ts, java, cs, swift, c, cpp, d)")
			fmt.Println("  -n <num>    Show top N dependencies")
			fmt.Println("  -j, --json  Output JSON")
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
		case !strings.HasPrefix(arg, "-"):
			dir = arg
		}
	}

	if showVersion {
		PrintVersion("deps")
	}

	// Load shared configuration
	config, err := LoadConfig()
	if err != nil {
		FatalError("loading config: %v", err)
	}

	var langConfig *LanguageConfig
	if lang != "" {
		langConfig, err = config.GetLanguageConfig(lang)
		if err != nil {
			FatalError("%v\nSupported languages: %s", err, strings.Join(config.GetSupportedLanguages(), ", "))
		}
	} else {
		// Auto-detect by finding files with supported extensions
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
		FatalError("no supported files found in directory\nSupported languages: %s", strings.Join(config.GetSupportedLanguages(), ", "))
	}

	allDeps := make(map[string]fileSet)
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		for _, ext := range langConfig.Extensions {
			if strings.HasSuffix(path, ext) {
				deps := analyzeDeps(path, langConfig)
				for dep, files := range deps {
					if allDeps[dep] == nil {
						allDeps[dep] = make(fileSet)
					}
					for f := range files {
						allDeps[dep][f] = true
					}
				}
				break
			}
		}
		return nil
	})

	var deps []DepInfo
	stdlib, external, internal := 0, 0, 0

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
			internal++
		} else if strings.Contains(dep, "/") || strings.Contains(dep, ".") {
			external++
		} else {
			internal++
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
				"internal": internal,
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
	fmt.Printf("stdlib: %d, external: %d, internal: %d\n", stdlib, external, internal)
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

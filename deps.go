// deps.go - Module dependency analyzer
// v1.2 - 350-380 lines, 0 dependencies, 9 languages
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

type DependencyConfig struct {
	Name            string
	LangKey         string
	Extensions      []string
	ImportPattern   string
	AliasPattern    string
	MultiLineBlock  string
	ExcludePatterns []string
}

type DepInfo struct {
	Module string   `json:"module"`
	Count  int      `json:"count"`
	Files  []string `json:"files"`
}

type DepResult struct {
	Language          string            `json:"language"`
	TotalImports      int               `json:"total_imports"`
	UniqueModules     int               `json:"unique_modules"`
	Dependencies      []DepInfo         `json:"dependencies"`
	ExternalVsInternal map[string]int   `json:"external_vs_internal"`
}

type fileSet map[string]bool

var languages = map[string]*DependencyConfig{
	"py": {
		Name:       "Python",
		LangKey:    "py",
		Extensions: []string{".py", ".pyw"},
		ImportPattern:   `^\s*(?:from\s+(\S+)\s+import|import\s+(\S+))`,
		ExcludePatterns: []string{"^\\.", "^\\.\\.", "^__future__"},
	},
	"go": {
		Name:          "Go",
		LangKey:       "go",
		Extensions:    []string{".go"},
		ImportPattern: `^\s*import\s+"(.*)"`,
		MultiLineBlock: "import (",
	},
	"rs": {
		Name:       "Rust",
		LangKey:    "rs",
		Extensions: []string{".rs"},
		ImportPattern:   `^\s*use\s+(\S+)`,
		ExcludePatterns: []string{"^super::", "^self::"},
	},
	"js": {
		Name:       "JavaScript/TypeScript",
		LangKey:    "js",
		Extensions: []string{".js", ".jsx", ".ts", ".tsx"},
		ImportPattern:   `^\s*(?:import\s+(?:\{[^}]*\}|\*|[\w$]+)\s+from\s+["']|require\s*\(\s*["'])([^"'\n]+)`,
		ExcludePatterns: []string{"^\\.", "^\\./"},
	},
	"java": {
		Name:       "Java",
		LangKey:    "java",
		Extensions: []string{".java"},
		ImportPattern: `^\s*import\s+(?:static\s+)?([\w.]+)`,
	},
	"cs": {
		Name:       "C#",
		LangKey:    "cs",
		Extensions: []string{".cs", ".csx"},
		ImportPattern: `^\s*using\s+([\w.]+)`,
	},
	"sw": {
		Name:       "Swift",
		LangKey:    "sw",
		Extensions: []string{".swift"},
		ImportPattern: `^\s*import\s+(\S+)`,
	},
	"c": {
		Name:       "C/C++",
		LangKey:    "c",
		Extensions: []string{".c", ".cpp", ".h", ".hpp"},
		ImportPattern: `^\s*#\s*include\s*[<"]([^>"]+)[>"]`,
	},
}

func isStdlib(module, langKey string) bool {
	stdlibPrefixes := map[string][]string{
		"py":   {"", "builtins.", "sys.", "os.", "json.", "re.", "collections.", "typing.", "__future__."},
		"go":   {"fmt", "os", "io", "strings", "math", "regexp", "encoding/json", "testing", "bytes", "errors"},
		"rs":   {"std::", "core::"},
		"js":   {"assert", "buffer", "crypto", "fs", "http", "path", "url"},
		"java": {"java.", "javax."},
		"cs":   {"System.", "Microsoft."},
		"sw":   {"Swift"},
	}
	prefixes := stdlibPrefixes[langKey]
	for _, p := range prefixes {
		if strings.HasPrefix(module, p) {
			return true
		}
	}
	return false
}

func analyzeDeps(filename string, config *DependencyConfig) map[string]fileSet {
	deps := make(map[string]fileSet)

	file, err := os.Open(filename)
	if err != nil {
		return deps
	}
	defer file.Close()

	importRe := regexp.MustCompile(config.ImportPattern)
	blockImportRe := regexp.MustCompile(`^\s*"([^"]+)"`)
	inBlock := false

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

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

		// Check exclusions with regex
		skip := false
		for _, pattern := range config.ExcludePatterns {
			re := regexp.MustCompile(pattern)
			if re.MatchString(trimmed) || re.MatchString(moduleFromImport(trimmed, config)) {
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

func moduleFromImport(line string, config *DependencyConfig) string {
	re := regexp.MustCompile(config.ImportPattern)
	if match := re.FindStringSubmatch(line); len(match) >= 2 {
		for i := 1; i < len(match); i++ {
			if match[i] != "" {
				return match[i]
			}
		}
	}
	return ""
}

func main() {
	dir := "."
	lang := ""
	topN := 0
	jsonOut := false

	for i := 1; i < len(os.Args); i++ {
		arg := os.Args[i]
		switch {
		case arg == "-h" || arg == "--help":
			fmt.Println("Usage: deps [OPTIONS] <dir>")
			fmt.Println("  -l <lang>   Force language")
			fmt.Println("  -n <num>    Show top N dependencies")
			fmt.Println("  -j, --json  Output JSON")
			return
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

	var config *DependencyConfig
	if lang != "" {
		config = languages[lang]
	} else {
		for _, l := range languages {
			for _, ext := range l.Extensions {
				files, _ := filepath.Glob(filepath.Join(dir, "*"+ext))
				if len(files) > 0 {
					config = l
					break
				}
			}
			if config != nil {
				break
			}
		}
	}

	if config == nil {
		fmt.Fprintln(os.Stderr, "No supported files found")
		os.Exit(1)
	}

	allDeps := make(map[string]fileSet)
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		for _, ext := range config.Extensions {
			if strings.HasSuffix(path, ext) {
				deps := analyzeDeps(path, config)
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

		if isStdlib(dep, config.LangKey) {
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
			Language: config.Name,
			TotalImports: len(allDeps),
			UniqueModules: len(deps),
			Dependencies: deps,
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

	fmt.Printf("Language: %s\n", config.Name)
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
		if isStdlib(deps[i].Module, config.LangKey) {
			kind = "std"
		} else if strings.Contains(deps[i].Module, "internal/") || strings.Contains(deps[i].Module, "vendor/") {
			kind = "int"
		}
		fmt.Printf("%-30s %3d (%s)\n", deps[i].Module, deps[i].Count, kind)
	}
}

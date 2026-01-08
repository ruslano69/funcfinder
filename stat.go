// stat.go - Unified function call counter for multiple languages
// Counts function calls in source files using configurable patterns
package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// LanguageConfig holds parsing patterns for a specific language
type LanguageConfig struct {
	Name             string
	Extensions       []string
	CommentPatterns  []string
	BlockComment     *BlockCommentConfig
	StringPatterns   []string
	CallPattern      string
	ImportPattern    string
	ExcludeWords     []string
	DecoratorPattern string
}

// BlockCommentConfig defines multi-line comment handling
type BlockCommentConfig struct {
	Start string
	End   string
}

// FileMetrics holds statistics about a source file
type FileMetrics struct {
	TotalLines    int
	CodeLines     int
	CommentLines  int
	BlankLines    int
	Imports       []string
	Decorators    []string
	FileSize      int64
}

// Pre-defined language configurations
var languages = map[string]*LanguageConfig{
	"py": {
		Name:       "Python",
		Extensions: []string{".py", ".pyw"},
		CommentPatterns: []string{
			`^\s*#!`, // Shebang line
			`#.*`,    // Line comments
		},
		StringPatterns: []string{
			`"[^"\\]*(?:\\.[^"\\]*)*"`,
			`'[^'\\]*(?:\\.[^'\\]*)*'`,
		},
		CallPattern:      `(\w+)\s*\(`,
		ImportPattern:    `^\s*(?:from\s+(\S+)\s+import|import\s+(\S+))`,
		DecoratorPattern: `^\s*@(\w+)`,
	},
	"go": {
		Name:       "Go",
		Extensions: []string{".go"},
		CommentPatterns: []string{
			`//.*`,
		},
		BlockComment: &BlockCommentConfig{
			Start: `/\*`,
			End:   `\*/`,
		},
		StringPatterns: []string{
			`"[^"\\]*(?:\\.[^"\\]*)*"`,
		},
		CallPattern:   `(\w+)\s*\(`,
		ImportPattern: `^\s*(?:import\s+"([^"]+)"|"([^"]+)"$)`,
		ExcludeWords:  []string{"func"},
	},
	"rs": {
		Name:       "Rust",
		Extensions: []string{".rs"},
		CommentPatterns: []string{
			`//.*`,
		},
		BlockComment: &BlockCommentConfig{
			Start: `/\*`,
			End:   `\*/`,
		},
		StringPatterns: []string{
			`"[^"\\]*(?:\\.[^"\\]*)*"`,
			`r#"[^"]*"#`,
			`r#*[^"]*#*`,
		},
		CallPattern:   `(\w+)\s*\(`,
		ImportPattern: `^\s*use\s+([^;]+)`,
	},
	"js": {
		Name:       "JavaScript/TypeScript",
		Extensions: []string{".js", ".jsx", ".mjs", ".ts", ".tsx"},
		CommentPatterns: []string{
			`//.*`,
		},
		BlockComment: &BlockCommentConfig{
			Start: `/\*`,
			End:   `\*/`,
		},
		StringPatterns: []string{
			`"[^"\\]*(?:\\.[^"\\]*)*"`,
			`'[^'\\]*(?:\\.[^'\\]*)*'`,
			"`[^`\\\\]*(?:\\\\.[^`\\\\]*)*`",
		},
		CallPattern:   `(\w+)\s*\(`,
		ImportPattern: `^\s*(?:import\s+.*?from\s+["']([^"']+)["']|require\s*\(\s*["']([^"']+)["'])`,
	},
	"sw": {
		Name:       "Swift",
		Extensions: []string{".swift"},
		CommentPatterns: []string{
			`//.*`,
		},
		BlockComment: &BlockCommentConfig{
			Start: `/\*`,
			End:   `\*/`,
		},
		StringPatterns: []string{
			`"[^"\\]*(?:\\.[^"\\]*)*"`,
			`""".*?"""`,
		},
		CallPattern:   `(\w+)\s*\(`,
		ImportPattern: `^\s*import\s+(\S+)`,
		ExcludeWords: []string{
			"func", "if", "while", "for", "guard",
			"switch", "catch", "return", "throw",
		},
	},
	"c": {
		Name:       "C/C++",
		Extensions: []string{".c", ".cpp", ".cc", ".h", ".hpp"},
		CommentPatterns: []string{
			`//.*`,
		},
		BlockComment: &BlockCommentConfig{
			Start: `/\*`,
			End:   `\*/`,
		},
		StringPatterns: []string{
			`"[^"\\]*(?:\\.[^"\\]*)*"`,
			`'[^'\\]*(?:\\.[^'\\]*)*'`,
		},
		CallPattern:   `(\w+)\s*\(`,
		ImportPattern: `^\s*#\s*include\s*[<"]([^>"]+)[>"]`,
		ExcludeWords: []string{
			"if", "else", "while", "for", "switch",
			"case", "return", "sizeof",
		},
	},
	"java": {
		Name:       "Java",
		Extensions: []string{".java"},
		CommentPatterns: []string{
			`//.*`,
		},
		BlockComment: &BlockCommentConfig{
			Start: `/\*`,
			End:   `\*/`,
		},
		StringPatterns: []string{
			`"[^"\\]*(?:\\.[^"\\]*)*"`,
		},
		CallPattern:      `(\w+)\s*\(`,
		ImportPattern:    `^\s*import\s+(?:static\s+)?([\w.]+)`,
		DecoratorPattern: `^\s*@(\w+)`,
		ExcludeWords: []string{
			"if", "else", "while", "for", "switch",
			"case", "return",
		},
	},
	"d": {
		Name:       "D",
		Extensions: []string{".d"},
		CommentPatterns: []string{
			`//.*`,
		},
		BlockComment: &BlockCommentConfig{
			Start: `/\*`,
			End:   `\*/`,
		},
		StringPatterns: []string{
			`"[^"\\]*(?:\\.[^"\\]*)*"`,
			`r"[^"]*"`,
		},
		CallPattern:   `(\w+)\s*\(`,
		ImportPattern: `^\s*import\s+([\w.]+)`,
		ExcludeWords: []string{
			"if", "else", "while", "foreach", "for",
			"switch", "case", "return",
		},
	},
	"cs": {
		Name:       "C#",
		Extensions: []string{".cs", ".csx"},
		CommentPatterns: []string{
			`//.*`,
		},
		BlockComment: &BlockCommentConfig{
			Start: `/\*`,
			End:   `\*/`,
		},
		StringPatterns: []string{
			`"[^"\\]*(?:\\.[^"\\]*)*"`,
			`@"[^"]*"`,
		},
		CallPattern:      `(\w+)\s*\(`,
		ImportPattern:    `^\s*using\s+([\w.]+)`,
		DecoratorPattern: `^\s*\[(\w+)`,
		ExcludeWords: []string{
			"if", "else", "while", "foreach", "for",
			"switch", "case", "return",
		},
	},
}

// getLanguageByExtension determines language config from file extension
func getLanguageByExtension(filename string) *LanguageConfig {
	ext := filepath.Ext(filename)
	for _, lang := range languages {
		for _, e := range lang.Extensions {
			if ext == e {
				return lang
			}
		}
	}
	return nil
}

// cleanLine removes comments and strings from a line based on config
func cleanLine(line string, config *LanguageConfig) (string, bool) {
	if strings.HasPrefix(line, "#!") {
		return "", true
	}

	if config.BlockComment != nil {
		inBlock := false
		for {
			if inBlock {
				idx := strings.Index(line, config.BlockComment.End)
				if idx == -1 {
					return "", true
				}
				line = line[idx+len(config.BlockComment.End):]
				inBlock = false
			}

			idx := strings.Index(line, config.BlockComment.Start)
			if idx == -1 {
				break
			}

			endIdx := strings.Index(line[idx+len(config.BlockComment.Start):], config.BlockComment.End)
			if endIdx == -1 {
				line = line[:idx]
				inBlock = true
			} else {
				line = line[:idx] + line[idx+len(config.BlockComment.Start)+endIdx+len(config.BlockComment.End):]
			}
		}
	}

	for _, pattern := range config.CommentPatterns {
		re := regexp.MustCompile(pattern)
		if re.MatchString(line) {
			loc := re.FindStringIndex(line)
			if loc != nil && loc[0] == 0 {
				return "", true
			}
			line = re.ReplaceAllString(line, "")
		}
	}

	for _, pattern := range config.StringPatterns {
		re := regexp.MustCompile(pattern)
		line = re.ReplaceAllString(line, `""`)
	}

	return line, false
}

// analyzeFile analyzes a source file and returns function calls and metrics
func analyzeFile(filename string, config *LanguageConfig) (map[string]int, *FileMetrics) {
	file, err := os.Open(filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening file: %v\n", err)
		os.Exit(1)
	}
	defer file.Close()

	// Get file size
	fileInfo, _ := file.Stat()
	metrics := &FileMetrics{
		FileSize: fileInfo.Size(),
		Imports:  []string{},
		Decorators: []string{},
	}

	pattern := regexp.MustCompile(config.CallPattern)
	callCounts := make(map[string]int)

	var decoratorPattern *regexp.Regexp
	if config.DecoratorPattern != "" {
		decoratorPattern = regexp.MustCompile(config.DecoratorPattern)
	}

	var importPattern *regexp.Regexp
	if config.ImportPattern != "" {
		importPattern = regexp.MustCompile(config.ImportPattern)
	}

	importSet := make(map[string]bool)
	decoratorSet := make(map[string]bool)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		metrics.TotalLines++

		trimmed := strings.TrimSpace(line)

		// Count blank lines
		if trimmed == "" {
			metrics.BlankLines++
			continue
		}

		// Check for imports (before cleaning)
		if importPattern != nil {
			match := importPattern.FindStringSubmatch(line)
			if len(match) >= 2 {
				for i := 1; i < len(match); i++ {
					if match[i] != "" && !importSet[match[i]] {
						importSet[match[i]] = true
						metrics.Imports = append(metrics.Imports, match[i])
					}
				}
			}
		}

		// Check for decorators
		if decoratorPattern != nil {
			match := decoratorPattern.FindStringSubmatch(line)
			if len(match) >= 2 {
				if !decoratorSet[match[1]] {
					decoratorSet[match[1]] = true
					metrics.Decorators = append(metrics.Decorators, match[1])
				}
			}
		}

		cleanedLine, isComment := cleanLine(line, config)

		// Count comment lines
		if isComment {
			metrics.CommentLines++
			continue
		}

		// Count code lines (non-blank, non-comment)
		if cleanedLine != "" {
			metrics.CodeLines++
		}

		// Count function calls
		if cleanedLine == "" {
			continue
		}

		matches := pattern.FindAllStringSubmatch(cleanedLine, -1)
		for _, match := range matches {
			if len(match) >= 2 {
				funcName := match[1]

				excluded := false
				for _, exclude := range config.ExcludeWords {
					if funcName == exclude {
						excluded = true
						break
					}
				}
				if !excluded {
					callCounts[funcName]++
				}
			}
		}
	}

	return callCounts, metrics
}

func main() {
	filename := ""
	langFlag := ""
	topN := 0

	for i := 1; i < len(os.Args); i++ {
		arg := os.Args[i]
		if arg == "-h" || arg == "--help" {
			fmt.Println("Usage: stat [OPTIONS] <source_file>")
			fmt.Println("  -l <lang>   Force language (py, go, rs, js, sw, c, java, d, cs)")
			fmt.Println("  -n <num>    Show top N functions")
			return
		} else if arg == "-l" && i+1 < len(os.Args) {
			langFlag = os.Args[i+1]
			i++
		} else if arg == "-n" && i+1 < len(os.Args) {
			fmt.Sscanf(os.Args[i+1], "%d", &topN)
			i++
		} else if !strings.HasPrefix(arg, "-") {
			filename = arg
		}
	}

	if filename == "" {
		fmt.Fprintln(os.Stderr, "Usage: stat [OPTIONS] <source_file>")
		os.Exit(1)
	}

	var config *LanguageConfig
	if langFlag != "" {
		config = languages[langFlag]
		if config == nil {
			fmt.Fprintf(os.Stderr, "Unknown language: %s\n", langFlag)
			os.Exit(1)
		}
	} else {
		config = getLanguageByExtension(filename)
		if config == nil {
			fmt.Fprintf(os.Stderr, "Warning: Unknown file extension, using generic patterns\n")
			config = &LanguageConfig{
				Name:            "Generic",
				CallPattern:     `(\w+)\s*\(`,
				CommentPatterns: []string{},
			}
		}
	}

	callCounts, metrics := analyzeFile(filename, config)

	type pair struct{ name string; count int }
	var calls []pair
	for name, count := range callCounts {
		calls = append(calls, pair{name, count})
	}
	sort.Slice(calls, func(i, j int) bool { return calls[i].count > calls[j].count })

	// Output file metrics
	fmt.Printf("Language: %s\n", config.Name)
	fmt.Printf("File: %s (%.1f KB)\n", filepath.Base(filename), float64(metrics.FileSize)/1024)
	fmt.Println(strings.Repeat("-", 35))

	fmt.Printf("Lines: %d\n", metrics.TotalLines)
	if metrics.TotalLines > 0 {
		fmt.Printf("  Code:     %d (%.1f%%)\n", metrics.CodeLines, float64(metrics.CodeLines)*100/float64(metrics.TotalLines))
		fmt.Printf("  Comments: %d (%.1f%%)\n", metrics.CommentLines, float64(metrics.CommentLines)*100/float64(metrics.TotalLines))
		fmt.Printf("  Blank:    %d (%.1f%%)\n", metrics.BlankLines, float64(metrics.BlankLines)*100/float64(metrics.TotalLines))
	}

	if len(metrics.Imports) > 0 {
		fmt.Printf("Imports: %d", len(metrics.Imports))
		if len(metrics.Imports) <= 5 {
			fmt.Printf(" (%s)\n", strings.Join(metrics.Imports, ", "))
		} else {
			fmt.Printf(" (%s, ...)\n", strings.Join(metrics.Imports[:5], ", "))
		}
	}

	if len(metrics.Decorators) > 0 {
		fmt.Printf("Decorators: %d (%s)\n", len(metrics.Decorators), strings.Join(metrics.Decorators, ", "))
	}

	fmt.Println(strings.Repeat("-", 35))
	fmt.Printf("Function calls: %d unique\n", len(calls))
	fmt.Println(strings.Repeat("-", 35))

	printCount := len(calls)
	if topN > 0 && topN < printCount {
		printCount = topN
	}
	for i := 0; i < printCount; i++ {
		fmt.Printf("%-25s %d\n", calls[i].name, calls[i].count)
	}
}

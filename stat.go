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
	ExcludeWords     []string
	DecoratorPattern string
}

// BlockCommentConfig defines multi-line comment handling
type BlockCommentConfig struct {
	Start string
	End   string
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
		CallPattern: `(\w+)\s*\(`,
		ExcludeWords: []string{"func"},
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
		CallPattern: `(\w+)\s*\(`,
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
		CallPattern: `(\w+)\s*\(`,
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
		CallPattern: `(\w+)\s*\(`,
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
		CallPattern: `(\w+)\s*\(`,
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
		CallPattern: `(\w+)\s*\(`,
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

// countFunctionCalls counts function calls in a source file
func countFunctionCalls(filename string, config *LanguageConfig) map[string]int {
	file, err := os.Open(filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening file: %v\n", err)
		os.Exit(1)
	}
	defer file.Close()

	pattern := regexp.MustCompile(config.CallPattern)
	callCounts := make(map[string]int)

	var decoratorPattern *regexp.Regexp
	if config.DecoratorPattern != "" {
		decoratorPattern = regexp.MustCompile(config.DecoratorPattern)
	}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()

		cleanedLine, skip := cleanLine(line, config)
		if skip || cleanedLine == "" {
			continue
		}

		if decoratorPattern != nil {
			match := decoratorPattern.FindStringSubmatch(line)
			if len(match) >= 2 {
				callCounts[match[1]]++
			}
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

	return callCounts
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

	callCounts := countFunctionCalls(filename, config)

	type pair struct{ name string; count int }
	var calls []pair
	for name, count := range callCounts {
		calls = append(calls, pair{name, count})
	}
	sort.Slice(calls, func(i, j int) bool { return calls[i].count > calls[j].count })

	fmt.Printf("Language: %s\n", config.Name)
	fmt.Printf("Functions: %d\n", len(calls))
	fmt.Println(strings.Repeat("-", 35))
	printCount := len(calls)
	if topN > 0 && topN < printCount {
		printCount = topN
	}
	for i := 0; i < printCount; i++ {
		fmt.Printf("%-25s %d\n", calls[i].name, calls[i].count)
	}
}

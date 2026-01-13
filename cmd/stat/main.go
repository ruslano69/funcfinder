// stat.go - Unified function call counter for multiple languages
// Counts function calls in source files using shared configuration
package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/ruslano69/funcfinder/internal"
)

// FileMetrics holds statistics about a source file
type FileMetrics struct {
	TotalLines   int
	CodeLines    int
	CommentLines int
	BlankLines   int
	Imports      []string
	Decorators   []string
	FileSize     int64
}

// cleanLine removes comments and strings from a line based on config
func cleanLine(line string, config *internal.LanguageConfig) (string, bool) {
	if strings.HasPrefix(line, "#!") {
		return "", true
	}

	// Handle block comments
	if config.BlockCommentRegex() != nil {
		blockRe := config.BlockCommentRegex()
		if blockRe.MatchString(line) {
			line = blockRe.ReplaceAllString(line, "")
			if strings.TrimSpace(line) == "" {
				return "", true
			}
		}
	}

	// Handle line comments
	if config.LineComment != "" {
		commentIdx := strings.Index(line, config.LineComment)
		if commentIdx >= 0 {
			before := line[:commentIdx]
			if strings.TrimSpace(before) == "" {
				return "", true
			}
			line = before
		}
	}

	// Remove string literals
	for _, sc := range config.StringChars {
		pattern := regexp.QuoteMeta(sc) + `[^` + regexp.QuoteMeta(sc) + `\\]*(?:\\.[^` + regexp.QuoteMeta(sc) + `\\]*)*` + regexp.QuoteMeta(sc)
		re := regexp.MustCompile(pattern)
		line = re.ReplaceAllString(line, `""`)
	}

	return line, false
}

// analyzeFile analyzes a source file and returns function calls and metrics
func analyzeFile(filename string, config *internal.LanguageConfig) (map[string]int, *FileMetrics) {
	file, err := os.Open(filename)
	if err != nil {
		internal.FatalError("opening file: %v", err)
	}
	defer file.Close()

	// Get file size
	fileInfo, _ := file.Stat()
	metrics := &FileMetrics{
		FileSize:   fileInfo.Size(),
		Imports:    []string{},
		Decorators: []string{},
	}

	callRegex := config.CallRegex()
	if callRegex == nil {
		internal.FatalError("no call pattern defined for language")
	}

	callCounts := make(map[string]int)

	var decoratorRegex *regexp.Regexp
	if config.DecoratorPattern != "" {
		decoratorRegex = config.DecoratorRegex()
	}

	var importRegex *regexp.Regexp
	if config.ImportPattern != "" {
		importRegex = config.ImportRegex()
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
		if importRegex != nil {
			match := importRegex.FindStringSubmatch(line)
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
		if decoratorRegex != nil {
			match := decoratorRegex.FindStringSubmatch(line)
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

		matches := callRegex.FindAllStringSubmatch(cleanedLine, -1)
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
	showVersion := false
	filename := ""
	langFlag := ""
	topN := 0

	for i := 1; i < len(os.Args); i++ {
		arg := os.Args[i]
		if arg == "-h" || arg == "--help" {
			fmt.Println("Usage: stat [OPTIONS] <source_file>")
			fmt.Println("  --version   Show version and exit")
			fmt.Println("  -l <lang>   Force language (py, go, rs, js, ts, sw, c, cpp, java, d, cs)")
			fmt.Println("  -n <num>    Show top N functions")
			return
		} else if arg == "--version" {
			showVersion = true
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

	if showVersion {
		internal.PrintVersion("stat")
	}

	if filename == "" {
		internal.FatalError("source file is required\nUsage: stat [OPTIONS] <source_file>")
	}

	// Load shared configuration
	config, err := internal.LoadConfig()
	if err != nil {
		internal.FatalError("loading config: %v", err)
	}

	var langConfig *internal.LanguageConfig
	if langFlag != "" {
		langConfig, err = config.GetLanguageConfig(langFlag)
		if err != nil {
			internal.FatalError("%v\nSupported languages: %s", err, strings.Join(config.GetSupportedLanguages(), ", "))
		}
	} else {
		langConfig = config.GetLanguageByExtension(filename)
		if langConfig == nil {
			internal.FatalError("cannot detect language from file extension\nSupported languages: %s", strings.Join(config.GetSupportedLanguages(), ", "))
		}
	}

	callCounts, metrics := analyzeFile(filename, langConfig)

	type pair struct {
		name  string
		count int
	}
	var calls []pair
	for name, count := range callCounts {
		calls = append(calls, pair{name, count})
	}
	sort.Slice(calls, func(i, j int) bool { return calls[i].count > calls[j].count })

	// Output file metrics
	fmt.Printf("Language: %s\n", langConfig.Name)
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

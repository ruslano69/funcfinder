// stat.go - Unified function call counter for multiple languages
// Counts function calls in source files using shared configuration
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

// StatResult is the JSON output structure for a single file.
type StatResult struct {
	Language     string      `json:"language"`
	File         string      `json:"file"`
	FileSizeKB   float64     `json:"file_size_kb"`
	TotalLines   int         `json:"total_lines"`
	CodeLines    int         `json:"code_lines"`
	CommentLines int         `json:"comment_lines"`
	BlankLines   int         `json:"blank_lines"`
	Imports      []string    `json:"imports"`
	Decorators   []string    `json:"decorators"`
	UniqueCalls  int         `json:"unique_calls"`
	TopCalls     []CallEntry `json:"top_calls"`
}

// DirStatResult is the JSON output structure for directory mode.
type DirStatResult struct {
	Language     string      `json:"language"`
	Dir          string      `json:"dir"`
	TotalFiles   int         `json:"total_files"`
	TotalLines   int         `json:"total_lines"`
	CodeLines    int         `json:"code_lines"`
	CommentLines int         `json:"comment_lines"`
	BlankLines   int         `json:"blank_lines"`
	UniqueCalls  int         `json:"unique_calls"`
	TopCalls     []CallEntry `json:"top_calls"`
	Files        []StatResult `json:"files"`
}

type CallEntry struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

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

// cleanLine removes comments and strings using enhanced_sanitizer
func cleanLine(line string, sanitizer *internal.Sanitizer, state *internal.ParserState) (string, bool) {
	// Skip shebang lines
	if strings.HasPrefix(line, "#!") {
		return "", true
	}

	// Use enhanced_sanitizer for proper string/comment removal
	cleaned, newState := sanitizer.CleanLine(line, *state)
	*state = newState

	// Check if line is entirely comment/string (all spaces after cleaning)
	if strings.TrimSpace(cleaned) == "" {
		return "", true
	}

	return cleaned, false
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

	// Create sanitizer for proper string/comment removal
	sanitizer := internal.NewSanitizer(config, false)
	state := internal.StateNormal

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

		cleanedLine, isComment := cleanLine(line, sanitizer, &state)

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

// sortedCalls converts a callCounts map to a sorted slice of pairs.
func sortedCalls(callCounts map[string]int) []struct{ name string; count int } {
	type pair struct {
		name  string
		count int
	}
	var calls []pair
	for name, count := range callCounts {
		calls = append(calls, pair{name, count})
	}
	sort.Slice(calls, func(i, j int) bool { return calls[i].count > calls[j].count })
	result := make([]struct{ name string; count int }, len(calls))
	for i, c := range calls {
		result[i].name = c.name
		result[i].count = c.count
	}
	return result
}

// toCallEntries converts a sorted calls slice to JSON-ready CallEntry slice, capped at topN.
func toCallEntries(calls []struct{ name string; count int }, topN int) []CallEntry {
	entries := calls
	if topN > 0 && topN < len(entries) {
		entries = entries[:topN]
	}
	out := make([]CallEntry, len(entries))
	for i, c := range entries {
		out[i] = CallEntry{Name: c.name, Count: c.count}
	}
	return out
}

// printFileStats prints text output for a single analyzed file.
func printFileStats(filename string, langName string, calls []struct{ name string; count int }, metrics *FileMetrics, topN int) {
	fmt.Printf("Language: %s\n", langName)
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

func main() {
	showVersion := false
	filename := ""
	dirMode := ""
	langFlag := ""
	topN := 0
	jsonOut := false

	for i := 1; i < len(os.Args); i++ {
		arg := os.Args[i]
		if arg == "-h" || arg == "--help" {
			fmt.Println("Usage: stat [OPTIONS] <source_file>")
			fmt.Println("       stat [OPTIONS] --dir <directory>")
			fmt.Println("  --version      Show version and exit")
			fmt.Println("  --dir <path>   Analyze all source files in directory recursively")
			fmt.Println("  -l <lang>      Force language (py, go, rs, js, ts, sw, c, cpp, java, d, cs)")
			fmt.Println("  -n <num>       Show top N functions")
			fmt.Println("  -j, --json     Output JSON")
			return
		} else if arg == "--version" {
			showVersion = true
		} else if arg == "--dir" && i+1 < len(os.Args) {
			dirMode = os.Args[i+1]
			i++
		} else if arg == "-l" && i+1 < len(os.Args) {
			langFlag = os.Args[i+1]
			i++
		} else if arg == "-n" && i+1 < len(os.Args) {
			fmt.Sscanf(os.Args[i+1], "%d", &topN)
			i++
		} else if arg == "-j" || arg == "--json" {
			jsonOut = true
		} else if !strings.HasPrefix(arg, "-") {
			filename = arg
		}
	}

	if showVersion {
		internal.PrintVersion("stat")
	}

	if dirMode == "" && filename == "" {
		internal.FatalError("source file or --dir is required\nUsage: stat [OPTIONS] <source_file>\n       stat [OPTIONS] --dir <directory>")
	}

	// Load shared configuration
	config, err := internal.LoadConfig()
	if err != nil {
		internal.FatalError("loading config: %v", err)
	}

	// ── DIRECTORY MODE ────────────────────────────────────────────────────────
	if dirMode != "" {
		var langConfig *internal.LanguageConfig
		if langFlag != "" {
			langConfig, err = config.GetLanguageConfig(langFlag)
			if err != nil {
				internal.FatalError("%v\nSupported languages: %s", err, strings.Join(config.GetSupportedLanguages(), ", "))
			}
		} else {
			// Auto-detect by finding files with supported extensions in the dir.
			for _, lc := range config {
				for _, ext := range lc.Extensions {
					files, _ := filepath.Glob(filepath.Join(dirMode, "*"+ext))
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

		// Single pass: collect per-file results and aggregate.
		type perFile struct {
			path  string
			calls []struct{ name string; count int }
			m     FileMetrics
		}
		aggregateCounts := make(map[string]int)
		var aggMetrics FileMetrics
		var collected []perFile

		dirFiles, walkErr := internal.CollectSourceFiles(dirMode, langConfig, true)
		if walkErr != nil {
			internal.FatalError("walking directory: %v", walkErr)
		}
		for _, path := range dirFiles {
			counts, m := analyzeFile(path, langConfig)
			for fn, cnt := range counts {
				aggregateCounts[fn] += cnt
			}
			aggMetrics.TotalLines += m.TotalLines
			aggMetrics.CodeLines += m.CodeLines
			aggMetrics.CommentLines += m.CommentLines
			aggMetrics.BlankLines += m.BlankLines
			aggMetrics.FileSize += m.FileSize
			collected = append(collected, perFile{path: path, calls: sortedCalls(counts), m: *m})
		}

		aggCalls := sortedCalls(aggregateCounts)

		if jsonOut {
			fileResults := make([]StatResult, len(collected))
			for i, pf := range collected {
				fileResults[i] = StatResult{
					Language:     langConfig.Name,
					File:         pf.path,
					FileSizeKB:   float64(pf.m.FileSize) / 1024,
					TotalLines:   pf.m.TotalLines,
					CodeLines:    pf.m.CodeLines,
					CommentLines: pf.m.CommentLines,
					BlankLines:   pf.m.BlankLines,
					Imports:      pf.m.Imports,
					Decorators:   pf.m.Decorators,
					UniqueCalls:  len(pf.calls),
					TopCalls:     toCallEntries(pf.calls, topN),
				}
			}
			result := DirStatResult{
				Language:     langConfig.Name,
				Dir:          dirMode,
				TotalFiles:   len(collected),
				TotalLines:   aggMetrics.TotalLines,
				CodeLines:    aggMetrics.CodeLines,
				CommentLines: aggMetrics.CommentLines,
				BlankLines:   aggMetrics.BlankLines,
				UniqueCalls:  len(aggregateCounts),
				TopCalls:     toCallEntries(aggCalls, topN),
				Files:        fileResults,
			}
			jsonBytes, _ := json.MarshalIndent(result, "", "  ")
			fmt.Println(string(jsonBytes))
			return
		}

		// Text output: per-file summary + aggregate totals.
		fmt.Printf("Language: %s  Dir: %s\n", langConfig.Name, dirMode)
		fmt.Println(strings.Repeat("=", 45))
		for _, pf := range collected {
			printCount := len(pf.calls)
			if topN > 0 && topN < printCount {
				printCount = topN
			}
			fmt.Printf("\n%s (%.1f KB, %d lines, %d unique calls)\n",
				pf.path, float64(pf.m.FileSize)/1024, pf.m.TotalLines, len(pf.calls))
			for i := 0; i < printCount; i++ {
				fmt.Printf("  %-23s %d\n", pf.calls[i].name, pf.calls[i].count)
			}
		}
		fmt.Printf("\n%s\n", strings.Repeat("=", 45))
		fmt.Printf("TOTAL  files: %d  lines: %d  unique calls: %d\n",
			len(collected), aggMetrics.TotalLines, len(aggregateCounts))
		fmt.Println(strings.Repeat("-", 45))
		printCount := len(aggCalls)
		if topN > 0 && topN < printCount {
			printCount = topN
		}
		for i := 0; i < printCount; i++ {
			fmt.Printf("%-25s %d\n", aggCalls[i].name, aggCalls[i].count)
		}
		return
	}

	// ── SINGLE FILE MODE ──────────────────────────────────────────────────────
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
	calls := sortedCalls(callCounts)

	if jsonOut {
		result := StatResult{
			Language:     langConfig.Name,
			File:         filename,
			FileSizeKB:   float64(metrics.FileSize) / 1024,
			TotalLines:   metrics.TotalLines,
			CodeLines:    metrics.CodeLines,
			CommentLines: metrics.CommentLines,
			BlankLines:   metrics.BlankLines,
			Imports:      metrics.Imports,
			Decorators:   metrics.Decorators,
			UniqueCalls:  len(callCounts),
			TopCalls:     toCallEntries(calls, topN),
		}
		jsonBytes, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(jsonBytes))
		return
	}

	printFileStats(filename, langConfig.Name, calls, metrics, topN)
}

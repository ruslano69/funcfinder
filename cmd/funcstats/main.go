// funcstats - Unified function call counter for multiple languages
// Counts function calls in source files using shared configuration
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
	Language     string       `json:"language"`
	Dir          string       `json:"dir"`
	TotalFiles   int          `json:"total_files"`
	TotalLines   int          `json:"total_lines"`
	CodeLines    int          `json:"code_lines"`
	CommentLines int          `json:"comment_lines"`
	BlankLines   int          `json:"blank_lines"`
	UniqueCalls  int          `json:"unique_calls"`
	TopCalls     []CallEntry  `json:"top_calls"`
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
	lines, err := internal.ReadLines(filename)
	if err != nil {
		internal.FatalError("reading file: %v", err)
	}
	return analyzeLines(filename, lines, config)
}

// analyzeLines analyzes the lines of a source file, already read, and returns function calls and metrics.
func analyzeLines(filename string, lines []string, config *internal.LanguageConfig) (map[string]int, *FileMetrics) {
	// Get file size
	fileInfo, _ := os.Stat(filename)
	metrics := &FileMetrics{
		FileSize:   fileInfo.Size(),
		Imports:    []string{},
		Decorators: []string{},
	}

	callRegex := config.CallRegex()
	if callRegex == nil {
		internal.FatalError("no call pattern defined for language")
	}

	// Function names known to the shared finder (the same one funcfinder,
	// complexity and callgraph already use) — a call-site scan must not
	// count a function's own signature line as a call to itself. Best
	// effort: an error here just means no self-definition line is skipped,
	// same as funcstats' behavior before this existed.
	definedNames := make(map[string]bool)
	signatures := make(map[int]map[string]bool)
	if fr, ferr := internal.FindFunctionsInFile(internal.CreateFinder(config, "", "map", false, false), filename, lines); ferr == nil {
		for _, fn := range fr.Functions {
			definedNames[fn.Name] = true
			if fn.SignatureLine > 0 {
				if signatures[fn.SignatureLine] == nil {
					signatures[fn.SignatureLine] = make(map[string]bool)
				}
				name := fn.SignatureName
				if name == "" {
					name = fn.Name
				}
				signatures[fn.SignatureLine][name] = true
			}
		}
	}
	funcRegex := config.FuncRegex()

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

	for lineIndex, line := range lines {
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

		// Prefer C#/Java/JavaScript declaration coordinates from the finder. Other languages
		// identify signature lines through their configured function pattern.
		// Skip only the declaration token, retaining real recursive calls.
		selfDefName := ""
		if config.LangKey != "cs" && config.LangKey != "java" && config.LangKey != "js" && funcRegex != nil {
			if m := funcRegex.FindStringSubmatch(cleanedLine); m != nil {
				if name := internal.ExtractFuncName(m); name != "" && definedNames[name] {
					selfDefName = name
				}
			}
		}

		matches := callRegex.FindAllStringSubmatch(cleanedLine, -1)
		skippedSelfDef := false
		var skippedSignatures map[string]bool
		if len(signatures[lineIndex+1]) > 0 {
			skippedSignatures = make(map[string]bool)
		}
		for _, match := range matches {
			if len(match) >= 2 {
				funcName := match[1]
				if (config.LangKey == "cs" || config.LangKey == "java" || config.LangKey == "js") && signatures[lineIndex+1][funcName] && !skippedSignatures[funcName] {
					skippedSignatures[funcName] = true
					continue
				}

				if funcName == selfDefName && !skippedSelfDef {
					skippedSelfDef = true
					continue
				}

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

// sortedCalls converts a callCounts map to a slice of pairs, most calls first and names in order among equal counts.
// The name breaks the tie because the map is read in random order: without it the output changed from run to run, and
// with -n so did which of the tied names made the cut.
func sortedCalls(callCounts map[string]int) []struct {
	name  string
	count int
} {
	type pair struct {
		name  string
		count int
	}
	var calls []pair
	for name, count := range callCounts {
		calls = append(calls, pair{name, count})
	}
	sort.Slice(calls, func(i, j int) bool {
		if calls[i].count != calls[j].count {
			return calls[i].count > calls[j].count
		}
		return calls[i].name < calls[j].name
	})
	result := make([]struct {
		name  string
		count int
	}, len(calls))
	for i, c := range calls {
		result[i].name = c.name
		result[i].count = c.count
	}
	return result
}

// toCallEntries converts a sorted calls slice to JSON-ready CallEntry slice, capped at topN.
func toCallEntries(calls []struct {
	name  string
	count int
}, topN int) []CallEntry {
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
func printFileStats(filename string, langName string, calls []struct {
	name  string
	count int
}, metrics *FileMetrics, topN int) {
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
	uncalledMode := false
	var excludes []string

	for i := 1; i < len(os.Args); i++ {
		arg := os.Args[i]
		if arg == "-h" || arg == "--help" {
			fmt.Println("Usage: funcstats [OPTIONS] <source_file>")
			fmt.Println("       funcstats [OPTIONS] --dir <directory>")
			fmt.Println("  --version      Show version and exit")
			fmt.Println("  --dir <path>   Analyze all source files in directory recursively")
			fmt.Println("  -l <lang>      Force language (c, cpp, cs, d, go, java, js, kotlin, php, py, ruby, rust, scala, swift, ts)")
			fmt.Println("  -n <num>       Show top N functions")
			fmt.Println("  -j, --json     Output JSON")
			fmt.Println("  --uncalled     Dir mode: list functions no code line names (candidates to delete)")
			fmt.Println("  --exclude GLOB Dir mode: leave out files whose name or path under <dir> matches (repeatable)")
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
		} else if arg == "--exclude" && i+1 < len(os.Args) {
			excludes = append(excludes, os.Args[i+1])
			i++
		} else if arg == "--uncalled" {
			uncalledMode = true
		} else if arg == "-j" || arg == "--json" {
			jsonOut = true
		} else if !strings.HasPrefix(arg, "-") {
			filename = arg
		}
	}

	if showVersion {
		internal.PrintVersion("funcstats")
	}

	if uncalledMode && dirMode == "" {
		internal.FatalError("--uncalled needs --dir: one file cannot show what is unused elsewhere")
	}
	if dirMode == "" && filename == "" {
		internal.FatalError("source file or --dir is required\nUsage: funcstats [OPTIONS] <source_file>\n       funcstats [OPTIONS] --dir <directory>")
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
			// The language of the tree: the one with the most source files under it, found recursively.
			langConfig, _, err = internal.DetectLanguage(config, dirMode)
			if err != nil {
				internal.FatalError("detecting language: %v", err)
			}
		}
		if langConfig == nil {
			internal.FatalError("no supported files found in directory\nSupported languages: %s", strings.Join(config.GetSupportedLanguages(), ", "))
		}

		// Single pass: collect per-file results and aggregate.
		type perFile struct {
			path  string
			calls []struct {
				name  string
				count int
			}
			m FileMetrics
		}
		aggregateCounts := make(map[string]int)
		var aggMetrics FileMetrics
		var collected []perFile

		dirFiles, walkErr := internal.CollectSourceFiles(dirMode, langConfig, true)
		if walkErr != nil {
			internal.FatalError("walking directory: %v", walkErr)
		}
		dirFiles, err = internal.FilterExcluded(dirFiles, dirMode, excludes)
		if err != nil {
			internal.FatalError("%v", err)
		}
		if uncalledMode {
			report, err := internal.FindUncalled(langConfig, dirMode, dirFiles)
			if err != nil {
				internal.FatalError("finding uncalled functions: %v", err)
			}
			printUncalled(report, jsonOut)
			return
		}
		// Files are analysed in parallel, each into its own slot, and aggregated in the order of dirFiles, so the output
		// and the first unreadable file reported are the same as in a sequential run.
		type analysed struct {
			counts map[string]int
			m      *FileMetrics
			err    error
		}
		results := make([]analysed, len(dirFiles))
		internal.ForEachParallel(len(dirFiles), func(i int) {
			lines, err := internal.ReadLines(dirFiles[i])
			if err != nil {
				results[i].err = err
				return
			}
			results[i].counts, results[i].m = analyzeLines(dirFiles[i], lines, langConfig)
		})
		for i, path := range dirFiles {
			if results[i].err != nil {
				internal.FatalError("reading file: %v", results[i].err)
			}
			counts, m := results[i].counts, results[i].m
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

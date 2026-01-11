



// complexity.go - Nesting Depth Complexity Analyzer
// Analyzes code complexity based on NESTING DEPTH, not decision point count
// Philosophy: Deep nesting is harder to understand than flat code with many branches
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"github.com/yourusername/funcfinder/internal"
)

// ComplexityLevel represents the complexity classification
type ComplexityLevel int

const (
	LevelSimple ComplexityLevel = iota
	LevelModerate
	LevelHigh
	LevelVeryHigh
	LevelCritical
)

// ComplexityMetrics contains complexity analysis results for a function
type ComplexityMetrics struct {
	Name             string   `json:"name"`
	File             string   `json:"file"`
	StartLine        int      `json:"start_line"`
	EndLine          int      `json:"end_line"`
	LinesOfCode      int      `json:"lines_of_code"`
	Complexity       int      `json:"complexity"`
	Level            string   `json:"level"`
	MaxNestingDepth  int      `json:"max_nesting_depth"`
	NestingHistory   []int    `json:"nesting_history"`
}

// FileComplexity contains complexity metrics for a single file
type FileComplexity struct {
	Filename          string              `json:"filename"`
	Language          string              `json:"language"`
	TotalFunctions    int                 `json:"total_functions"`
	AverageComplexity float64             `json:"average_complexity"`
	MaxComplexity     int                 `json:"max_complexity"`
	Functions         []ComplexityMetrics `json:"functions"`
}

// ComplexityResult contains the complete analysis result
type ComplexityResult struct {
	Language          string          `json:"language"`
	TotalFiles        int             `json:"total_files"`
	TotalFunctions    int             `json:"total_functions"`
	AverageComplexity float64         `json:"average_complexity"`
	Files             []FileComplexity `json:"files"`
}

// Nesting thresholds based on cognitive load
const (
	DepthSimple    = 2  // flat code
	DepthModerate  = 3  // one level of nesting
	DepthHigh      = 4  // two levels of nesting
	DepthVeryHigh  = 5  // three levels of nesting
	DepthCritical  = 6  // four or more levels
)

// getComplexityLevel returns the complexity level based on nesting depth
func getComplexityLevel(maxDepth int) ComplexityLevel {
	switch {
	case maxDepth <= DepthSimple:
		return LevelSimple
	case maxDepth <= DepthModerate:
		return LevelModerate
	case maxDepth <= DepthHigh:
		return LevelHigh
	case maxDepth <= DepthVeryHigh:
		return LevelVeryHigh
	default:
		return LevelCritical
	}
}

// calculateNestingComplexity computes complexity from max nesting depth
// Formula: NDC = 2^(maxDepth - 1)
// This reflects exponential cognitive load with each nesting level
func calculateNestingComplexity(maxDepth int) int {
	if maxDepth <= 1 {
		return 1
	}
	return 1 << (maxDepth - 1) // 2^(maxDepth-1)
}

// getComplexityColor returns ANSI color code for complexity level
func getComplexityColor(level ComplexityLevel) string {
	switch level {
	case LevelSimple:
		return "\033[32m" // Green
	case LevelModerate:
		return "\033[33m" // Yellow
	case LevelHigh:
		return "\033[35m" // Magenta
	case LevelVeryHigh:
		return "\033[31m" // Red
	case LevelCritical:
		return "\033[31;1m" // Bold Red
	default:
		return "\033[0m" // Default
	}
}

// getLevelName returns human-readable level name
func getLevelName(level ComplexityLevel) string {
	switch level {
	case LevelSimple:
		return "SIMPLE"
	case LevelModerate:
		return "MODERATE"
	case LevelHigh:
		return "HIGH"
	case LevelVeryHigh:
		return "VERY_HIGH"
	case LevelCritical:
		return "CRITICAL"
	default:
		return "UNKNOWN"
	}
}

// Nesting patterns that increase depth (keywords followed by conditions)
// Flat constructs (else, elif, case) are handled separately
var nestingPatterns = map[string]*regexp.Regexp{
	"py": regexp.MustCompile(`^\s*(if|elif|for|while|except|with)\s*[(a-zA-Z]`),
	"go": regexp.MustCompile(`^\s*(if|for|switch)\s*[(a-zA-Z]`),
	"rs": regexp.MustCompile(`^\s*(if|else|for|while|match|loop)\s*[(a-zA-Z_]`),
	"js": regexp.MustCompile(`^\s*(if|else|for|while|do|switch|catch|finally)\s*[(a-zA-Z_]`),
	"ts": regexp.MustCompile(`^\s*(if|else|for|while|do|switch|catch|finally)\s*[(a-zA-Z_]`),
	"sw": regexp.MustCompile(`^\s*(if|else|guard|for|while|repeat)\s*[(a-zA-Z_]`),
	"c":  regexp.MustCompile(`^\s*(if|else|for|while|do|switch|case|default)\s*[(a-zA-Z_]`),
	"java": regexp.MustCompile(`^\s*(if|else|for|while|do|switch|catch|finally)\s*[(a-zA-Z_]`),
	"d":  regexp.MustCompile(`^\s*(if|else|for|foreach|while|do|switch|catch|finally)\s*[(a-zA-Z_]`),
	"cs": regexp.MustCompile(`^\s*(if|else|for|foreach|while|do|switch|catch|finally)\s*[(a-zA-Z_]`),
}

// Flat patterns that continue current depth (else, elif, case without brace)
var flatPatterns = map[string]*regexp.Regexp{
	"py": regexp.MustCompile(`^\s*elif\s+|^\s*else\s*:|^\s*except\s+`),
	"go": regexp.MustCompile(`^\s*else\s*\{?\s*$|^\s*case\s+`),
	"rs": regexp.MustCompile(`^\s*else\s*\{|^\s*case\s+`),
	"js": regexp.MustCompile(`^\s*else\s*\{|^\s*case\s+:|^\s*default\s*:`),
	"ts": regexp.MustCompile(`^\s*else\s*\{|^\s*case\s+:|^\s*default\s*:`),
	"sw": regexp.MustCompile(`^\s*else\s*\{|^\s*case\s+`),
	"c":  regexp.MustCompile(`^\s*else\s*\{|^\s*case\s+:|^\s*default\s*:`),
	"java": regexp.MustCompile(`^\s*else\s*\{|^\s*case\s+:|^\s*default\s*:`),
	"d":  regexp.MustCompile(`^\s*else\s*\{|^\s*case\s+:|^\s*default\s*:`),
	"cs": regexp.MustCompile(`^\s*else\s*\{|^\s*case\s+:|^\s*default\s*:`),
}

func main() {
	// Define flags
	showVersion := flag.Bool("version", false, "Show version")
	langFlag := flag.String("l", "", "Force language")
	jsonOut := flag.Bool("j", false, "Output JSON")
	thresholdFlag := flag.Int("t", DepthHigh, "Threshold for high nesting depth")
	topN := flag.Int("n", 0, "Show top N most complex functions")
	showDetails := flag.Bool("v", false, "Show detailed nesting analysis")
	noSimple := flag.Bool("nosimple", false, "Hide SIMPLE level functions (depth <= 2)")
	flag.Parse()

	// Handle version flag
	if *showVersion {
		internal.PrintVersion("complexity")
	}

	// Use threshold flag to avoid unused error
	_ = thresholdFlag

	// Check for positional args
	args := flag.Args()
	dir := "."
	if len(args) >= 1 {
		dir = args[0]
	}

	// Load configuration
	config, err := internal.LoadConfig()
	if err != nil {
		internal.FatalError("loading config: %v", err)
	}

	var langConfig *internal.LanguageConfig
	if *langFlag != "" {
		langConfig, err = config.GetLanguageConfig(*langFlag)
		if err != nil {
			internal.FatalError("%v", err)
		}
	} else {
		for _, l := range config {
			files, _ := filepath.Glob(filepath.Join(dir, "*"+l.Extensions[0]))
			if len(files) > 0 {
				langConfig = l
				break
			}
		}
	}

	if langConfig == nil {
		internal.FatalErrorMsg("No supported files found")
	}

	// Walk directory and analyze files
	var allFiles []FileComplexity
	totalComplexity := 0
	totalFunctions := 0

	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		for _, ext := range langConfig.Extensions {
			if strings.HasSuffix(path, ext) {
				fileComplexity := analyzeFileComplexity(path, langConfig)
				if fileComplexity.TotalFunctions > 0 {
					allFiles = append(allFiles, fileComplexity)
					totalFunctions += fileComplexity.TotalFunctions
					totalComplexity += fileComplexity.MaxComplexity
				}
				break
			}
		}
		return nil
	})

	if len(allFiles) == 0 {
		internal.FatalErrorMsg("No functions found")
	}

	// Calculate overall average (using max complexity per file)
	avgComplexity := float64(totalComplexity) / float64(len(allFiles))

	// Sort files by average complexity
	sort.Slice(allFiles, func(i, j int) bool {
		return allFiles[i].MaxComplexity > allFiles[j].MaxComplexity
	})

	if *jsonOut {
		result := ComplexityResult{
			Language:          langConfig.Name,
			TotalFiles:        len(allFiles),
			TotalFunctions:    totalFunctions,
			AverageComplexity: avgComplexity,
			Files:             allFiles,
		}
		jsonBytes, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(jsonBytes))
		return
	}

	// Text output
	internal.InfoMessage(fmt.Sprintf("Language: %s", langConfig.Name))
	internal.InfoMessage(fmt.Sprintf("Files analyzed: %d", len(allFiles)))
	internal.InfoMessage(fmt.Sprintf("Total functions: %d", totalFunctions))
	fmt.Printf("Average max complexity: %.2f\n", avgComplexity)
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println("Philosophy: Deep nesting (not branch count) is the real complexity")
	fmt.Println(strings.Repeat("=", 60))

	// Collect all functions for sorting
	var allFunctions []ComplexityMetrics
	for _, fc := range allFiles {
		allFunctions = append(allFunctions, fc.Functions...)
	}

	// Sort by complexity
	sort.Slice(allFunctions, func(i, j int) bool {
		return allFunctions[i].Complexity > allFunctions[j].Complexity
	})

	// Filter out SIMPLE functions if --nosimple flag is set
	if *noSimple {
		filtered := make([]ComplexityMetrics, 0, len(allFunctions))
		for _, fn := range allFunctions {
			level := getComplexityLevel(fn.MaxNestingDepth)
			if level != LevelSimple {
				filtered = append(filtered, fn)
			}
		}
		allFunctions = filtered
	}

	// Show top N or all
	printCount := len(allFunctions)
	if *topN > 0 && *topN < printCount {
		printCount = *topN
	}

	// Get terminal color support
	colorsEnabled := checkColorSupport()

	printFunc := func(metrics ComplexityMetrics, rank int) {
		level := getComplexityLevel(metrics.MaxNestingDepth)
		levelName := getLevelName(level)

		if colorsEnabled {
			color := getComplexityColor(level)
			fmt.Printf("%s#%d %s:%d %s() depth=%d complexity=%d level=%s%s\033[0m\n",
				color, rank, filepath.Base(metrics.File), metrics.StartLine,
				metrics.Name, metrics.MaxNestingDepth, metrics.Complexity, levelName, resetColor())
		} else {
			fmt.Printf("#%d %s:%d %s() depth=%d complexity=%d level=%s\n",
				rank, filepath.Base(metrics.File), metrics.StartLine,
				metrics.Name, metrics.MaxNestingDepth, metrics.Complexity, levelName)
		}

		if *showDetails && len(metrics.NestingHistory) > 0 {
			fmt.Printf("  Nesting history: %v\n", metrics.NestingHistory)
		}
		fmt.Printf("  Lines: %d, File: %s\n", metrics.LinesOfCode, metrics.File)
		fmt.Println()
	}

	for i := 0; i < printCount; i++ {
		printFunc(allFunctions[i], i+1)
	}

	// Summary by level
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println("Complexity distribution (by nesting depth):")

	levelCounts := make(map[ComplexityLevel]int)
	for _, f := range allFiles {
		for _, fn := range f.Functions {
			level := getComplexityLevel(fn.MaxNestingDepth)
			levelCounts[level]++
		}
	}

	levelOrder := []ComplexityLevel{LevelSimple, LevelModerate, LevelHigh, LevelVeryHigh, LevelCritical}
	for _, level := range levelOrder {
		count := levelCounts[level]
		if count > 0 {
			name := getLevelName(level)
			bar := strings.Repeat("█", count*20/totalFunctions)
			if colorsEnabled {
				color := getComplexityColor(level)
				fmt.Printf("%s%s: %d %s (depth > %d)%s\033[0m\n", color, name, count, bar, getDepthThreshold(level), resetColor())
			} else {
				fmt.Printf("%s: %d %s (depth > %d)\n", name, count, bar, getDepthThreshold(level))
			}
		}
	}
}

// getDepthThreshold returns the minimum depth for a level
func getDepthThreshold(level ComplexityLevel) int {
	switch level {
	case LevelSimple:
		return 1
	case LevelModerate:
		return DepthSimple + 1
	case LevelHigh:
		return DepthModerate + 1
	case LevelVeryHigh:
		return DepthHigh + 1
	case LevelCritical:
		return DepthVeryHigh + 1
	default:
		return 1
	}
}

// analyzeFileComplexity calculates nesting complexity for all functions in a file
func analyzeFileComplexity(filename string, langConfig *internal.LanguageConfig) FileComplexity {
	file, err := os.Open(filename)
	if err != nil {
		return FileComplexity{Filename: filename}
	}
	defer file.Close()

	// Read all lines
	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	// Use finder to get function bounds (auto-selects PythonFinder for Python)
	finder := internal.CreateFinder(langConfig, "", "map", false, false)
	result, err := finder.FindFunctions(filename)
	if err != nil {
		return FileComplexity{Filename: filename}
	}

	// Get patterns for language
	nestingRe := getNestingPattern(langConfig.LangKey)
	flatRe := getFlatPattern(langConfig.LangKey)

	var functions []ComplexityMetrics
	maxFileComplexity := 0

	for _, fn := range result.Functions {
		// Extract function body
		startIdx := fn.Start - 1
		endIdx := fn.End
		if endIdx > len(lines) {
			endIdx = len(lines)
		}

		funcBody := lines[startIdx:endIdx]
		linesOfCode := countLinesOfCode(funcBody)

		// Calculate nesting depth
		nestingResult := calculateNestingDepth(funcBody, nestingRe, flatRe)
		maxDepth := nestingResult.maxDepth
		complexity := calculateNestingComplexity(maxDepth)

		metrics := ComplexityMetrics{
			Name:            fn.Name,
			File:            filename,
			StartLine:       fn.Start,
			EndLine:         fn.End,
			LinesOfCode:     linesOfCode,
			Complexity:      complexity,
			Level:           getLevelName(getComplexityLevel(maxDepth)),
			MaxNestingDepth: maxDepth,
			NestingHistory:  nestingResult.history,
		}

		functions = append(functions, metrics)
		if complexity > maxFileComplexity {
			maxFileComplexity = complexity
		}
	}

	avgComplexity := 0.0
	if len(functions) > 0 {
		total := 0
		for _, fn := range functions {
			total += fn.Complexity
		}
		avgComplexity = float64(total) / float64(len(functions))
	}

	return FileComplexity{
		Filename:          filename,
		Language:          langConfig.Name,
		TotalFunctions:    len(functions),
		AverageComplexity: avgComplexity,
		MaxComplexity:     maxFileComplexity,
		Functions:         functions,
	}
}

// nestingResult holds the result of nesting analysis
type nestingResult struct {
	maxDepth int
	history  []int
}

// calculateNestingDepth computes maximum nesting depth and history
// Uses BRACE-BASED depth tracking for accurate measurement
func calculateNestingDepth(lines []string, nestingRe, flatRe *regexp.Regexp) nestingResult {
	result := nestingResult{
		maxDepth: 0,
		history:  []int{},
	}

	currentDepth := 0
	inBlock := false // Track if we're inside a block that started with "{"

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Skip empty lines and comments
		if trimmed == "" || isCommentOnly(trimmed) {
			result.history = append(result.history, currentDepth)
			continue
		}

		// Remove inline comments for accurate detection
		codeLine := removeComments(trimmed)

		// Count braces on this line
		openBraces := strings.Count(codeLine, "{")
		closeBraces := strings.Count(codeLine, "}")

		// Check if this line contains a nesting keyword
		hasNestingKeyword := false
		hasFlatKeyword := false

		if nestingRe != nil && nestingRe.MatchString(codeLine) {
			hasNestingKeyword = true
		}
		if flatRe != nil && flatRe.MatchString(codeLine) {
			hasFlatKeyword = true
		}

		// Handle nesting constructs
		if hasNestingKeyword && !hasFlatKeyword {
			// This line starts a new block
			// If it has opening brace, depth increases
			if openBraces > 0 {
				currentDepth += openBraces
			} else {
				// Multi-line definition (e.g., if (cond) { on next line)
				currentDepth++
				inBlock = true
			}
		} else if hasFlatKeyword {
			// Flat construct (else, elif, case) - doesn't increase depth
			// But we're still at current depth level
		}

		// Handle closing braces
		if closeBraces > 0 {
			currentDepth -= closeBraces
			if currentDepth < 0 {
				currentDepth = 0
			}
			inBlock = false
		}

		// Handle standalone opening braces (not after keywords)
		if openBraces > closeBraces && !hasNestingKeyword && !inBlock {
			// Standalone block - rare but possible
			currentDepth += openBraces - closeBraces
		}

		result.history = append(result.history, currentDepth)

		if currentDepth > result.maxDepth {
			result.maxDepth = currentDepth
		}
	}

	// Sanity check: max depth shouldn't exceed reasonable limits
	if result.maxDepth > 15 {
		result.maxDepth = 15 // Cap at reasonable level
	}

	return result
}

// getNestingPattern returns the nesting pattern for a language
func getNestingPattern(langKey string) *regexp.Regexp {
	if pattern, ok := nestingPatterns[langKey]; ok {
		return pattern
	}
	// Default pattern
	return regexp.MustCompile(`\b(if|for|while|switch)\b`)
}

// getFlatPattern returns the flat pattern for a language
func getFlatPattern(langKey string) *regexp.Regexp {
	if pattern, ok := flatPatterns[langKey]; ok {
		return pattern
	}
	// Default pattern
	return regexp.MustCompile(`\b(else|elif|case|default)\b`)
}

// countLinesOfCode counts non-empty, non-comment-only lines
func countLinesOfCode(lines []string) int {
	count := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && !isCommentOnly(trimmed) {
			count++
		}
	}
	return count
}

// isCommentOnly checks if a line is only a comment
func isCommentOnly(line string) bool {
	trimmed := strings.TrimSpace(line)
	return strings.HasPrefix(trimmed, "//") ||
		strings.HasPrefix(trimmed, "#") ||
		strings.HasPrefix(trimmed, "/*") ||
		strings.HasPrefix(trimmed, "'") ||
		strings.HasPrefix(trimmed, "\"\"\"") ||
		strings.HasPrefix(trimmed, "'''")
}

// removeComments removes comments from a line for accurate counting
func removeComments(line string) string {
	// Remove single-line comments
	if idx := strings.Index(line, "//"); idx != -1 {
		line = line[:idx]
	}
	if idx := strings.Index(line, "#"); idx != -1 {
		line = line[:idx]
	}
	return line
}

// checkColorSupport checks if terminal supports colors
func checkColorSupport() bool {
	term := os.Getenv("TERM")
	noColor := os.Getenv("NO_COLOR")
	return term != "dumb" && noColor == "" && isTerminal()
}

// isTerminal checks if stdout is a terminal
func isTerminal() bool {
	fi, _ := os.Stdout.Stat()
	return (fi.Mode() & os.ModeCharDevice) != 0
}

// resetColor returns the ANSI reset code
func resetColor() string {
	return "\033[0m"
}

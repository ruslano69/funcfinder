



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

	"github.com/ruslano69/funcfinder/internal"
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

// reorderArgs moves flags before positional arguments so flag.Parse() works
// regardless of argument order (e.g. "complexity file.go -l js" becomes
// "complexity -l js file.go").
func reorderArgs(args []string) []string {
	// Flags that consume the next argument as their value.
	valueFlags := map[string]bool{"l": true, "t": true, "n": true}

	var flags []string
	var positional []string

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "-") {
			flags = append(flags, arg)
			name := strings.TrimLeft(arg, "-")
			// Handle -flag=value form (value already included).
			if strings.Contains(name, "=") {
				continue
			}
			// Consume next token as value if this flag expects one.
			if valueFlags[name] && i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				i++
				flags = append(flags, args[i])
			}
		} else {
			positional = append(positional, arg)
		}
	}
	return append(flags, positional...)
}

func main() {
	os.Args = append([]string{os.Args[0]}, reorderArgs(os.Args[1:])...)

	// Define flags
	showVersion := flag.Bool("version", false, "Show version")
	langFlag := flag.String("l", "", "Force language")
	jsonOut := flag.Bool("j", false, "Output JSON")
	thresholdFlag := flag.Int("t", 0, "Show only functions with nesting depth >= N (0 = show all)")
	topN := flag.Int("n", 0, "Show top N most complex functions")
	showDetails := flag.Bool("v", false, "Show detailed nesting analysis")
	noSimple := flag.Bool("nosimple", false, "Hide SIMPLE level functions (depth <= 2)")
	flag.Parse()

	// Handle version flag
	if *showVersion {
		internal.PrintVersion("complexity")
	}

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

	dirFiles, walkErr := internal.CollectSourceFiles(dir, langConfig, true)
	if walkErr != nil {
		internal.FatalError("walking directory: %v", walkErr)
	}
	for _, path := range dirFiles {
		fileComplexity := analyzeFileComplexity(path, langConfig)
		if fileComplexity.TotalFunctions > 0 {
			allFiles = append(allFiles, fileComplexity)
			totalFunctions += fileComplexity.TotalFunctions
			totalComplexity += fileComplexity.MaxComplexity
		}
	}

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

	// Apply -t threshold filter
	if *thresholdFlag > 0 {
		filtered := make([]ComplexityMetrics, 0, len(allFunctions))
		for _, fn := range allFunctions {
			if fn.MaxNestingDepth >= *thresholdFlag {
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

	// One sanitizer per file, driven by the language config — the same one the
	// finder, stat and callgraph already use. Built here rather than per
	// function so the language lookup happens once.
	sanitizer := internal.NewSanitizer(langConfig, false)

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

		// Strip comments and literals once, and measure both metrics on the
		// result. A function body always starts in normal state, so each one
		// gets a fresh parser state.
		cleaned := cleanBody(funcBody, sanitizer)
		linesOfCode := countLinesOfCode(cleaned)

		// Calculate nesting depth
		nestingResult := calculateNestingDepth(cleaned, langConfig, nestingRe, flatRe)
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

// calculateNestingDepth computes maximum nesting depth and history.
//
// How a block is delimited is a property of the language, and languages.json
// already states it: indent_based for Python, block_end_keyword for Ruby,
// neither for the thirteen brace languages. Dispatch follows that, the same way
// finder_factory and struct_finder_factory already do — no list of language
// keys to keep in sync here.
//
// Lines arrive sanitized (see cleanBody), so braces, "end" and indentation seen
// below are real syntax and never string or comment contents.
func calculateNestingDepth(lines []string, langConfig *internal.LanguageConfig, nestingRe, flatRe *regexp.Regexp) nestingResult {
	var result nestingResult

	switch {
	case langConfig.IndentBased:
		result = nestingByIndent(lines)
	case langConfig.BlockEndKeyword != "":
		result = nestingByBlockKeyword(lines, langConfig.BlockEndKeyword, nestingRe, flatRe)
	default:
		result = nestingByBraces(lines)
	}

	// Sanity check: max depth shouldn't exceed reasonable limits
	if result.maxDepth > 15 {
		result.maxDepth = 15 // Cap at reasonable level
	}

	return result
}

// nestingByBraces tracks depth for the thirteen brace languages by walking the
// braces themselves.
//
// No keyword heuristics: in a brace language the braces *are* the block
// structure, so reading them is both simpler and exact. The previous code
// mixed a keyword regex with brace counting and, more damagingly, subtracted
// every closing brace before considering the opening ones — so a balanced line
// such as `x := T{A{{...}}}` or `} else {` drove the depth *down*. That was
// invisible while comments were stripped by cutting at "//", because the
// closers on those lines were being thrown away too.
//
// A line is credited with the greater of the depth entering it and the depth
// leaving it — not with the peak reached inside it. A brace that opens and
// closes within one line encloses nothing the reader has to hold: `m :=
// map[string]int{"a": 1}` reads at its own level, and crediting the momentary
// peak there inflated 316 of 2090 functions in a real repository by exactly
// one. Blocks that do enclose something still register, because the lines they
// enclose are themselves measured one level deeper.
func nestingByBraces(lines []string) nestingResult {
	result := nestingResult{history: make([]int, 0, len(lines))}
	depth := 0

	for _, line := range lines {
		before := depth

		for _, ch := range line {
			switch ch {
			case '{':
				depth++
			case '}':
				depth--
				if depth < 0 {
					// Unbalanced input (a partial body, or a brace the
					// sanitizer could not classify). Clamp rather than go
					// negative and skew everything after it.
					depth = 0
				}
			}
		}

		credited := before
		if depth > credited {
			credited = depth
		}

		result.history = append(result.history, credited)
		if credited > result.maxDepth {
			result.maxDepth = credited
		}
	}

	return result
}

// nestingByIndent tracks depth for indentation-delimited languages (Python).
//
// This path did not exist before: Python went through the brace code, where its
// blocks have no braces to count, so `if`/`for` incremented via the keyword
// branch and *nothing ever decremented*. Sequential statements accumulated —
// four consecutive ifs at one level reported depth 4, the same as four nested
// ones, making the two indistinguishable.
//
// Depth is the INDENT/DEDENT stack, which needs no assumption about the indent
// unit: any consistent width works, and mixed widths still order correctly. The
// `def` line sits at the base and is level 0, so a body statement is 1 — the
// same level a body statement gets in a brace language, where the function's
// own brace opened level 1.
//
// Continuation lines inside (), [] or {} are skipped: their indentation is
// alignment, not nesting, and would otherwise push a spurious level.
func nestingByIndent(lines []string) nestingResult {
	result := nestingResult{history: make([]int, 0, len(lines))}

	var stack []int // indent widths, strictly increasing
	depth := 0
	brackets := 0

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			result.history = append(result.history, depth)
			continue
		}

		// A line opened inside brackets continues the previous logical line.
		if brackets > 0 {
			brackets += bracketBalance(line)
			result.history = append(result.history, depth)
			continue
		}

		width := indentWidth(line)

		for len(stack) > 0 && width < stack[len(stack)-1] {
			stack = stack[:len(stack)-1]
		}
		if len(stack) == 0 || width > stack[len(stack)-1] {
			stack = append(stack, width)
		}

		// stack[0] is the def line's own indentation, hence the -1.
		depth = len(stack) - 1
		brackets += bracketBalance(line)

		result.history = append(result.history, depth)
		if depth > result.maxDepth {
			result.maxDepth = depth
		}
	}

	return result
}

// nestingByBlockKeyword tracks depth for languages that close blocks with a
// keyword rather than a brace (Ruby's `end`).
//
// Like Python, this had no path of its own and inherited the brace code, where
// nothing decremented — so depth only ever grew. Braces still count, since Ruby
// writes single-line blocks as `{ ... }`.
//
// An opener is recognised by the language's own nesting pattern; `end` closes.
// Approximate by nature — a modifier form (`x = 1 if y`) has no `end` to match
// and is deliberately not treated as an opener, which is why the pattern is
// anchored at line start for the languages that define one.
//
// Not reachable today: CreateFinder splits on IndentBased versus braces, so for
// Ruby it looks for a brace-delimited body, finds none, and reports no
// functions at all — nothing ever gets here to measure. Kept and tested
// directly all the same, because the alternative is Ruby silently falling into
// the brace path and resuming the grows-forever behaviour the moment the finder
// learns to locate `def ... end`.
func nestingByBlockKeyword(lines []string, endKeyword string, nestingRe, flatRe *regexp.Regexp) nestingResult {
	result := nestingResult{history: make([]int, 0, len(lines))}
	endRe := regexp.MustCompile(`(^|\s)` + regexp.QuoteMeta(endKeyword) + `(\s|$)`)
	depth := 0

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			result.history = append(result.history, depth)
			continue
		}

		opens := strings.Count(trimmed, "{")
		closes := strings.Count(trimmed, "}") + len(endRe.FindAllString(trimmed, -1))

		isFlat := flatRe != nil && flatRe.MatchString(trimmed)
		if !isFlat && nestingRe != nil && nestingRe.MatchString(trimmed) {
			opens++
		}

		peak := depth + opens
		depth = peak - closes
		if depth < 0 {
			depth = 0
		}

		result.history = append(result.history, peak)
		if peak > result.maxDepth {
			result.maxDepth = peak
		}
	}

	return result
}

// indentWidth measures leading whitespace in columns, expanding tabs to the
// next multiple of 8 as Python itself does. Only the ordering of widths
// matters to the stack, but tab expansion keeps a file that mixes tabs and
// spaces from ordering wrongly.
func indentWidth(line string) int {
	width := 0

	for _, ch := range line {
		switch ch {
		case ' ':
			width++
		case '\t':
			width += 8 - width%8
		default:
			return width
		}
	}

	return width
}

// bracketBalance returns opened-minus-closed brackets on a line. Used to detect
// continuation lines; safe because literals are already blanked.
func bracketBalance(line string) int {
	balance := 0

	for _, ch := range line {
		switch ch {
		case '(', '[', '{':
			balance++
		case ')', ']', '}':
			balance--
		}
	}

	return balance
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

// cleanBody strips comments and literal contents from a function body, using
// the language's own definition of what those are.
//
// This replaces a pair of hand-rolled helpers that cut every line at the first
// "//" or "#" regardless of language and regardless of context. Both markers
// occur inside ordinary string literals — a URL ("http://example.com"), an XML
// character reference ("&#xD;"), a spreadsheet error string ("#N/A") — and
// cutting there discarded the rest of the line, closing brace included. Each
// such line then raised the nesting depth by one and never gave it back, so a
// table-driven test of eight URLs reported depth 9 instead of 2 and landed at
// the top of the CRITICAL list.
//
// Sanitizer answers this from languages.json (line_comment, block_comment_*,
// string_chars, raw_string_chars, escape_char) and is what finder, stat and
// callgraph already use; complexity was the one tool doing it by hand. It
// blanks literal contents to spaces rather than deleting them, so brace
// counting and the nesting/flat keyword patterns still see correct columns —
// and no longer match an "if" that lives inside a string.
//
// State is threaded across lines so multi-line block comments and raw strings
// survive, which the old prefix test could not represent at all.
func cleanBody(lines []string, sanitizer *internal.Sanitizer) []string {
	cleaned := make([]string, 0, len(lines))
	state := internal.StateNormal

	for _, line := range lines {
		// A shebang is not code, and its "#" is not a comment marker either.
		if strings.HasPrefix(line, "#!") {
			cleaned = append(cleaned, "")
			continue
		}

		out, newState := sanitizer.CleanLine(line, state)
		state = newState
		cleaned = append(cleaned, out)
	}

	return cleaned
}

// countLinesOfCode counts non-empty lines. Expects sanitized input (see
// cleanBody), where comment-only lines have already become empty.
func countLinesOfCode(lines []string) int {
	count := 0
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			count++
		}
	}
	return count
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

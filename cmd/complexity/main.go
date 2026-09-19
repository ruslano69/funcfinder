// complexity.go - Cognitive Complexity Analyzer
// Analyzes code complexity as an additive score over control-flow decision
// points (if/for/while/switch/...), weighted by how deep each one sits —
// not the exponential 2^(maxDepth-1) this replaced, and not raw nesting
// depth alone either.
// Philosophy: a decision costs more the deeper it's nested (SonarSource-
// style cognitive complexity), but several sibling branches at the same
// level (if/elif/elif/else) cost only what they visibly add — they don't
// compound like real nesting does. MaxNestingDepth is still reported
// separately as a plain "how deep does this go" signal.
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
	Name            string `json:"name"`
	File            string `json:"file"`
	StartLine       int    `json:"start_line"`
	EndLine         int    `json:"end_line"`
	LinesOfCode     int    `json:"lines_of_code"`
	Complexity      int    `json:"complexity"`
	Level           string `json:"level"`
	MaxNestingDepth int    `json:"max_nesting_depth"`
	// MaxLoopNestingDepth counts only loop-in-loop nesting (for/while/...
	// inside another loop), independent of Complexity/Level/MaxNestingDepth:
	// a loop nested in another loop carries a distinct risk (algorithmic
	// blowup, iteration-interaction bugs) that nesting a conditional simply
	// doesn't, so it's surfaced as its own signal rather than weighted into
	// the general branching score.
	MaxLoopNestingDepth int   `json:"max_loop_nesting_depth"`
	NestingHistory      []int `json:"nesting_history"`
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
	Language          string           `json:"language"`
	TotalFiles        int              `json:"total_files"`
	TotalFunctions    int              `json:"total_functions"`
	AverageComplexity float64          `json:"average_complexity"`
	Files             []FileComplexity `json:"files"`
}

// Score thresholds for the additive cognitive-complexity total (see
// nestingResult.score). Derived by running the old depth-based cutoffs
// (simple<=2, moderate<=3, high<=4, veryhigh<=5) through score's own
// formula for the case they were actually measuring — a straight chain of
// nested decisions with no flat siblings, score = D*(D+1)/2 at depth D —
// so a function that used to sit exactly on an old boundary for pure
// nesting lands in the equivalent new bucket. A function with the same
// score built instead from flat branching (several sibling ifs rather
// than nested ones) now correctly lands in a lower bucket than depth
// alone would have suggested, which is the point of the whole rework.
const (
	ScoreSimple   = 3  // D=2: flat code, at most one level of real nesting
	ScoreModerate = 6  // D=3
	ScoreHigh     = 10 // D=4
	ScoreVeryHigh = 15 // D=5
)

// getComplexityLevel returns the complexity level for the additive score.
func getComplexityLevel(score int) ComplexityLevel {
	switch {
	case score <= ScoreSimple:
		return LevelSimple
	case score <= ScoreModerate:
		return LevelModerate
	case score <= ScoreHigh:
		return LevelHigh
	case score <= ScoreVeryHigh:
		return LevelVeryHigh
	default:
		return LevelCritical
	}
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

// genericNestingPattern/genericFlatPattern are the fallback used when a
// language's config carries no nesting_pattern/flat_pattern of its own
// (see getNestingPattern/getFlatPattern below).
var genericNestingPattern = regexp.MustCompile(`\b(if|for|while|switch)\b`)
var genericFlatPattern = regexp.MustCompile(`\b(else|elif|case|default)\b`)
var genericLoopPattern = regexp.MustCompile(`\b(for|while)\b`)

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
	noSimple := flag.Bool("nosimple", false, "Hide SIMPLE level functions (score <= 3)")
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
	fmt.Println("Philosophy: cost = sum of (1 + depth) per decision — nesting compounds, sibling branches don't")
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
			level := getComplexityLevel(fn.Complexity)
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
		level := getComplexityLevel(metrics.Complexity)
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
		if metrics.MaxLoopNestingDepth >= 2 {
			// A distinct signal, not folded into depth/complexity/level above:
			// loop-in-loop carries algorithmic (O(n^2)+) and iteration-
			// interaction risk that nesting a conditional doesn't.
			fmt.Printf("  loop-nesting=%d (nested loops — check for O(n^%d)+ cost and iteration-interaction bugs)\n",
				metrics.MaxLoopNestingDepth, metrics.MaxLoopNestingDepth)
		}
		fmt.Printf("  Lines: %d, File: %s\n", metrics.LinesOfCode, metrics.File)
		fmt.Println()
	}

	for i := 0; i < printCount; i++ {
		printFunc(allFunctions[i], i+1)
	}

	// Summary by level
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println("Complexity distribution (by cognitive score):")

	levelCounts := make(map[ComplexityLevel]int)
	for _, f := range allFiles {
		for _, fn := range f.Functions {
			level := getComplexityLevel(fn.Complexity)
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
				fmt.Printf("%s%s: %d %s (score > %d)%s\033[0m\n", color, name, count, bar, getScoreThreshold(level), resetColor())
			} else {
				fmt.Printf("%s: %d %s (score > %d)\n", name, count, bar, getScoreThreshold(level))
			}
		}
	}
}

// getScoreThreshold returns the minimum score for a level, for the
// distribution histogram's "(score > N)" label.
func getScoreThreshold(level ComplexityLevel) int {
	switch level {
	case LevelSimple:
		return 0
	case LevelModerate:
		return ScoreSimple
	case LevelHigh:
		return ScoreModerate
	case LevelVeryHigh:
		return ScoreHigh
	case LevelCritical:
		return ScoreVeryHigh
	default:
		return 0
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
	nestingRe := getNestingPattern(langConfig)
	flatRe := getFlatPattern(langConfig)
	loopRe := getLoopPattern(langConfig)

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

		// Calculate nesting depth and the additive cognitive-complexity score
		// (nestingResult.score — see its doc comment for the formula; this
		// replaced the old 2^(maxDepth-1) exponential).
		nestingResult := calculateNestingDepth(cleaned, langConfig, nestingRe, flatRe)
		maxDepth := nestingResult.maxDepth
		complexity := nestingResult.score

		// Loop-in-loop depth is tracked separately from general branching
		// depth: reuses the exact same depth machinery (calculateNestingDepth
		// dispatches on the same brace/indent/block-keyword strategy either
		// way), just classifying "opens a level" against loopRe instead of
		// nestingRe/flatRe. A loop nested in another loop compounds cost per
		// iteration in a way a conditional doesn't (O(n^2) risk, iteration-
		// interaction bugs), so it gets its own signal rather than folding
		// into the same score as a plain `if` — see the loop_pattern doc in
		// internal/config.go.
		loopResult := calculateNestingDepth(cleaned, langConfig, loopRe, nil)

		metrics := ComplexityMetrics{
			Name:                fn.Name,
			File:                filename,
			StartLine:           fn.Start,
			EndLine:             fn.End,
			LinesOfCode:         linesOfCode,
			Complexity:          complexity,
			Level:               getLevelName(getComplexityLevel(complexity)),
			MaxNestingDepth:     maxDepth,
			MaxLoopNestingDepth: loopResult.maxDepth,
			NestingHistory:      nestingResult.history,
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
	// score is the additive cognitive-complexity total (replaces the old
	// 2^(maxDepth-1) formula): every decision line (nestingRe, not flatRe)
	// adds 1+depth-at-that-point; every flat sibling (elif/else/case/
	// default/...) adds a flat 1 with no depth bonus, since it's an
	// alternate branch of the same decision, not a fresh one.
	score int
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
		result = nestingByIndent(lines, nestingRe, flatRe)
	case langConfig.BlockEndKeyword != "":
		result = nestingByBlockKeyword(lines, langConfig.BlockEndKeyword, nestingRe, flatRe)
	default:
		result = nestingByBraces(lines, nestingRe, flatRe)
	}

	// Sanity check: max depth shouldn't exceed reasonable limits
	if result.maxDepth > 15 {
		result.maxDepth = 15 // Cap at reasonable level
	}

	return result
}

// nestingByBraces tracks depth for the thirteen brace languages by walking the
// braces themselves, crediting only the ones a control-flow decision opens.
//
// Every brace still has to be counted to know when one closes (rawDepth), but
// only a brace whose line matches the language's nestingRe — and isn't a flat
// sibling continuation per flatRe — adds to the *credited* depth that gets
// reported. A struct literal, a closure body, or any other purely structural
// `{` no longer inflates "cognitive" nesting just for existing: a function
// built entirely of nested literals and no branching used to score as deep as
// several layers of real if/for, which is exactly backwards.
//
// A decision is classified once per line (matching flatRe/nestingRe against
// the line's own text) and applied to every brace that line opens — the same
// per-line granularity nestingByBlockKeyword already uses for Ruby, not a
// finer per-character one. That means a multi-line condition whose `{` lands
// on a continuation line (`if x &&\n    y {`) isn't credited, since only the
// first line starts with the keyword; accepted as a known gap rather than
// tracking classification across line continuations, consistent with the
// same simplification nestingByBlockKeyword already made.
//
// A line is still credited with the greater of the depth entering it and the
// depth leaving it, per the original fix this replaces: a brace that opens
// and closes within one line encloses nothing the reader has to hold, so the
// momentary peak inside it isn't reported — but now measured on the credited
// count, not the raw brace count.
func nestingByBraces(lines []string, nestingRe, flatRe *regexp.Regexp) nestingResult {
	result := nestingResult{history: make([]int, 0, len(lines))}
	rawDepth := 0
	credited := 0
	var creditStack []bool // per currently-open brace: did a decision open it?

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		isFlat := flatRe != nil && flatRe.MatchString(trimmed)
		isDecision := !isFlat && nestingRe != nil && nestingRe.MatchString(trimmed)

		before := credited

		// Score: a decision costs 1 plus however deep it already sits
		// (before its own brace opens, i.e. its ancestors only, not
		// itself); a flat sibling costs a flat 1 regardless of depth.
		if isDecision {
			result.score += 1 + before
		} else if isFlat {
			result.score++
		}

		// A flat line ("} else {") closes the previous branch's frame and
		// reopens one on the very same line — poppedCredit carries what was
		// just closed across to that reopen, so the new frame keeps the
		// same credited level instead of dropping to uncredited: the else
		// branch reads at the same depth as the if it belongs to, not one
		// shallower.
		poppedAny := false
		poppedCredit := false

		for _, ch := range line {
			switch ch {
			case '{':
				creditThisBrace := isDecision
				if isFlat && poppedAny {
					creditThisBrace = poppedCredit
				}
				rawDepth++
				creditStack = append(creditStack, creditThisBrace)
				if creditThisBrace {
					credited++
				}
			case '}':
				rawDepth--
				if rawDepth < 0 {
					// Unbalanced input (a partial body, or a brace the
					// sanitizer could not classify). Clamp rather than go
					// negative and skew everything after it.
					rawDepth = 0
				}
				if len(creditStack) > 0 {
					wasCredited := creditStack[len(creditStack)-1]
					creditStack = creditStack[:len(creditStack)-1]
					poppedAny = true
					poppedCredit = wasCredited
					if wasCredited {
						credited--
						if credited < 0 {
							credited = 0
						}
					}
				}
			}
		}

		creditedLine := before
		if credited > creditedLine {
			creditedLine = credited
		}

		result.history = append(result.history, creditedLine)
		if creditedLine > result.maxDepth {
			result.maxDepth = creditedLine
		}
	}

	return result
}

// nestingByIndent tracks depth for indentation-delimited languages (Python),
// crediting only the indent levels a control-flow decision actually opened.
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
// Only a level opened by a decision line — matching nestingRe or flatRe — is
// credited; a nested `def`/`class`, or any other block that merely deepens
// the indent without branching, no longer inflates cognitive depth. The
// classification has to be read off the line *before* the indent increases,
// not the indented line itself: "if x:" sits at the shallower width, and the
// deeper width only appears on its body's first line, which usually isn't
// itself a decision keyword at all.
//
// flatRe alone still credits: "elif"/"else" don't add a NEW level relative
// to the "if" they continue (same width, no push happens for the elif/else
// line itself, so that distinction needs no code here) — but the elif/else
// line's OWN body is one real level deeper than the elif/else line, exactly
// like the if's body is, and must be measured the same. Excluding flatRe
// lines from crediting (as an early version of this code did) made an
// elif/else body come out one level shallower than its sibling if-body.
//
// Continuation lines inside (), [] or {} are skipped: their indentation is
// alignment, not nesting, and would otherwise push a spurious level.
func nestingByIndent(lines []string, nestingRe, flatRe *regexp.Regexp) nestingResult {
	result := nestingResult{history: make([]int, 0, len(lines))}

	var stack []int        // indent widths, strictly increasing
	var creditStack []bool // per stack level: did a decision line open it?
	credited := 0
	brackets := 0
	prevWasDecision := false // classification of the last real statement line

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			result.history = append(result.history, credited)
			continue
		}

		// A line opened inside brackets continues the previous logical line.
		if brackets > 0 {
			brackets += bracketBalance(line)
			result.history = append(result.history, credited)
			continue
		}

		width := indentWidth(line)

		for len(stack) > 0 && width < stack[len(stack)-1] {
			stack = stack[:len(stack)-1]
			if len(creditStack) > 0 {
				wasCredited := creditStack[len(creditStack)-1]
				creditStack = creditStack[:len(creditStack)-1]
				if wasCredited {
					credited--
					if credited < 0 {
						credited = 0
					}
				}
			}
		}
		if len(stack) == 0 || width > stack[len(stack)-1] {
			stack = append(stack, width)
			creditStack = append(creditStack, prevWasDecision)
			if prevWasDecision {
				credited++
			}
		}

		brackets += bracketBalance(line)

		trimmed := strings.TrimSpace(line)
		isNesting := nestingRe != nil && nestingRe.MatchString(trimmed)
		isFlat := flatRe != nil && flatRe.MatchString(trimmed)
		prevWasDecision = isNesting || isFlat

		// Score, using the same `credited` this line resolved to above: for
		// Python a decision line's own indent increase (its body) and the
		// credit for entering whatever decision opened *this* line's level
		// are two different things that land on the same physical line, so
		// (unlike nestingByBraces, which uses the depth from BEFORE this
		// line's own brace) `credited` here already reflects ancestors only
		// — this line's own body hasn't pushed anything yet.
		if isNesting && !isFlat {
			result.score += 1 + credited
		} else if isFlat {
			result.score++
		}

		result.history = append(result.history, credited)
		if credited > result.maxDepth {
			result.maxDepth = credited
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
		isDecision := !isFlat && nestingRe != nil && nestingRe.MatchString(trimmed)
		if isDecision {
			opens++
		}

		// Score, using depth from before this line's own opens (its
		// ancestors only) — same convention as nestingByBraces, since a
		// keyword and its increment are resolved on the same line here too.
		if isDecision {
			result.score += 1 + depth
		} else if isFlat {
			result.score++
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

// getNestingPattern returns the language's nesting_pattern from its shared
// config (internal/languages.json, via LanguageConfig.NestingRegex()) —
// the same regex source complexity, and eventually other tools, draw from —
// falling back to a generic keyword pattern for a language whose config
// doesn't define one.
func getNestingPattern(langConfig *internal.LanguageConfig) *regexp.Regexp {
	if re := langConfig.NestingRegex(); re != nil {
		return re
	}
	return genericNestingPattern
}

// getFlatPattern mirrors getNestingPattern for the flat_pattern side.
func getFlatPattern(langConfig *internal.LanguageConfig) *regexp.Regexp {
	if re := langConfig.FlatRegex(); re != nil {
		return re
	}
	return genericFlatPattern
}

// getLoopPattern mirrors getNestingPattern for the loop-only subset used to
// compute MaxLoopNestingDepth (for/while/foreach/do/loop — never if/switch).
func getLoopPattern(langConfig *internal.LanguageConfig) *regexp.Regexp {
	if re := langConfig.LoopRegex(); re != nil {
		return re
	}
	return genericLoopPattern
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

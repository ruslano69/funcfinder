// breakdown.go - the figures behind a function's score (--breakdown). The score folds branching cost into one number;
// these show what makes it up: the size, how many decisions there are, how deep the branches nest, how deep the loops
// nest, and how many lines combine conditions with && / || or and / or.
package main

import (
	"fmt"
	"strings"
)

// breakdownThresholds are the limits behind the --breakdown flags. The defaults are the common warning levels rather
// than the loosest ones: a flag that fires early costs a glance, a flag that fires late is missed. Each limit is set on
// the command line, and 0 turns its flag off.
type breakdownThresholds struct {
	Lines     int // flag above this many lines of code
	Decisions int // flag above this many decisions (if / elif / match / switch / for / while / loop)
	Bool      int // flag a line with more than this many boolean operators (&&, ||, and, or)
	Depth     int // flag at this branch depth of if / match or more
	Loops     int // flag at this loop depth or more
}

// defaultThresholds: about 100 lines is the Clippy too_many_lines default; 10 decisions is McCabe's classic limit; 2
// boolean operators on a line is four conditions, where a condition starts to be hard to read; 3 to 4 levels of
// branching is the usual limit, so 4 is flagged; a loop in a loop is common in ordinary code (files, then their
// lines), so the loop flag starts at three deep. The plain output's loop-nesting line still starts at two.
var defaultThresholds = breakdownThresholds{Lines: 100, Decisions: 10, Bool: 2, Depth: 4, Loops: 3}

// The breakdown counts scan each cleaned line by hand instead of with regular expressions: a pattern with \b and an
// alternation has no literal prefix, so Go's regexp backtracks at every position of every line, and these counts took
// half of the analysis time. The scanners keep the regexp semantics exactly (breakdown_scan_test.go compares them with
// the patterns): a word is a maximal run of ASCII letters, digits and _, as \b sees it.

// isWordByte reports whether b is a word character for \b: ASCII letter, digit or underscore. A byte of a multi-byte
// UTF-8 rune is not, as the rune is not a word character for \b either.
func isWordByte(b byte) bool {
	return b == '_' || '0' <= b && b <= '9' || 'a' <= b && b <= 'z' || 'A' <= b && b <= 'Z'
}

// boolOps counts the boolean operators on a cleaned line: && or || (C-like), and / or as whole words (Python, Ruby),
// left to right without overlap, as FindAllString(`&&|\|\||\band\b|\bor\b`) does. Operators in strings and comments
// are blanked by the sanitizer and do not count. A closure taking no arguments (|| expr) in Rust counts too; that is
// the known false positive of this count.
func boolOps(line string) int {
	n := 0
	for i := 0; i < len(line); {
		c := line[i]
		if (c == '&' || c == '|') && i+1 < len(line) && line[i+1] == c {
			n++
			i += 2
			continue
		}
		if !isWordByte(c) {
			i++
			continue
		}
		j := i
		for j < len(line) && isWordByte(line[j]) {
			j++
		}
		if w := line[i:j]; w == "and" || w == "or" {
			n++
		}
		i = j
	}
	return n
}

// hasDecision reports whether a cleaned line holds a keyword that opens a decision: if, elif, match, switch, for,
// while, loop, as whole words. An else is a sibling branch and is not one; "else if" counts once, as its if. Whole
// words only, so loop_count does not.
func hasDecision(line string) bool {
	for i := 0; i < len(line); {
		if !isWordByte(line[i]) {
			i++
			continue
		}
		j := i
		for j < len(line) && isWordByte(line[j]) {
			j++
		}
		switch line[i:j] {
		case "if", "elif", "match", "switch", "for", "while", "loop":
			return true
		}
		i = j
	}
	return false
}

// countDecisions counts the cleaned lines that contain a decision keyword, wherever it sits on the line. The score
// reads the keyword only at the start of a line (the nesting pattern), so an if or match inside an expression is
// counted here and not there: this count is what the code shows, the score is what its pattern sees.
func countDecisions(lines []string) int {
	n := 0
	for _, line := range lines {
		if hasDecision(line) {
			n++
		}
	}
	return n
}

// countBooleanLines counts the cleaned lines that combine conditions with && / || or and / or. A line with several
// operators counts once: the figure is how many lines carry a condition chain, not how many operators they hold.
func countBooleanLines(lines []string) int {
	n := 0
	for _, line := range lines {
		if boolOps(line) > 0 {
			n++
		}
	}
	return n
}

// countMaxBoolOps returns the most boolean operators on one cleaned line: && / || and and / or. Four conditions joined
// by three operators give 3. It is the figure the -bool limit reads, where boolean_lines counts lines.
func countMaxBoolOps(lines []string) int {
	most := 0
	for _, line := range lines {
		if n := boolOps(line); n > most {
			most = n
		}
	}
	return most
}

// breakdownCheck is one limit of --breakdown: its key in the JSON output (triggered_flags), its label in the text, and
// whether the function passes it.
type breakdownCheck struct {
	key   string
	label string
	fires bool
}

// breakdownChecks evaluates the four limits for a function, in reading order. A limit of 0 is off and never fires.
func breakdownChecks(m ComplexityMetrics, th breakdownThresholds) []breakdownCheck {
	return []breakdownCheck{
		{"len", fmt.Sprintf("long (>%d lines)", th.Lines), th.Lines > 0 && m.LinesOfCode > th.Lines},
		{"if", fmt.Sprintf("many decisions (>%d)", th.Decisions), th.Decisions > 0 && m.DecisionCount > th.Decisions},
		{"bool", fmt.Sprintf("long boolean chain (>%d operators on a line)", th.Bool), th.Bool > 0 && m.MaxBoolOps > th.Bool},
		{"deep", fmt.Sprintf("deep branches (depth >=%d)", th.Depth), th.Depth > 0 && m.MaxNestingDepth >= th.Depth},
		{"loop", fmt.Sprintf("nested loops (depth >=%d)", th.Loops), th.Loops > 0 && m.MaxLoopNestingDepth >= th.Loops},
	}
}

// breakdownFlags names the thresholds a function passes, in reading order. It is empty when none is passed.
func breakdownFlags(m ComplexityMetrics, th breakdownThresholds) []string {
	var flags []string
	for _, c := range breakdownChecks(m, th) {
		if c.fires {
			flags = append(flags, c.label)
		}
	}
	return flags
}

// triggeredFlags returns the keys (len, if, bool, deep, loop) of the thresholds a function passes: the JSON form of
// breakdownFlags. It is never nil, so a function that passes none has [] in the output, not null.
func triggeredFlags(m ComplexityMetrics, th breakdownThresholds) []string {
	keys := []string{}
	for _, c := range breakdownChecks(m, th) {
		if c.fires {
			keys = append(keys, c.key)
		}
	}
	return keys
}

// printBreakdown prints the figures of one function under its header, and the thresholds it passes.
func printBreakdown(m ComplexityMetrics, th breakdownThresholds) {
	fmt.Printf("  size=%d lines  decisions=%d  branch-depth=%d  loop-depth=%d  boolean-lines=%d\n",
		m.LinesOfCode, m.DecisionCount, m.MaxNestingDepth, m.MaxLoopNestingDepth, m.BooleanLines)
	if flags := breakdownFlags(m, th); len(flags) > 0 {
		fmt.Printf("  over: %s\n", strings.Join(flags, ", "))
	}
}

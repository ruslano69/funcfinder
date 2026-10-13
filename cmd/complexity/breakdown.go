// breakdown.go - the figures behind a function's score (--breakdown). The score folds branching cost into one number;
// these show what makes it up: the size, how many decisions there are, how deep the branches nest, how deep the loops
// nest, and how many lines combine conditions with && / || or and / or.
package main

import (
	"fmt"
	"regexp"
	"strings"
)

// Thresholds for the --breakdown flags. They are heuristics for where a function gets hard to hold in one reading, not
// limits of any language. The loop threshold is the one the loop-nesting signal already uses.
const (
	breakdownLongLines     = 300 // lines of code
	breakdownManyDecisions = 50  // if / else if / match / for / while / loop
	breakdownDeepBranches  = 5   // nesting depth of if / match
	breakdownLoopNesting   = 2   // loops inside loops
)

// booleanRe matches a line that combines conditions: && or || (C-like), and / or (Python, Ruby). It is applied to
// cleaned lines, so operators in strings and comments do not count. A closure taking no arguments (|| expr) in Rust
// matches too; that is the known false positive of this count.
var booleanRe = regexp.MustCompile(`&&|\|\||\band\b|\bor\b`)

// decisionRe matches the keywords that open a decision: if, elif, match, switch, for, while, loop, as whole words. An
// else is a sibling branch and is not one; "else if" counts once, as its if. Whole words only, so loop_count does not.
var decisionRe = regexp.MustCompile(`\b(if|elif|match|switch|for|while|loop)\b`)

// countDecisions counts the cleaned lines that contain a decision keyword, wherever it sits on the line. The score
// reads the keyword only at the start of a line (the nesting pattern), so an if or match inside an expression is
// counted here and not there: this count is what the code shows, the score is what its pattern sees.
func countDecisions(lines []string) int {
	n := 0
	for _, line := range lines {
		if decisionRe.MatchString(line) {
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
		if booleanRe.MatchString(line) {
			n++
		}
	}
	return n
}

// breakdownFlags names the thresholds a function passes, in reading order. It is empty when none is passed.
func breakdownFlags(m ComplexityMetrics) []string {
	var flags []string
	if m.LinesOfCode > breakdownLongLines {
		flags = append(flags, fmt.Sprintf("long (>%d lines)", breakdownLongLines))
	}
	if m.DecisionCount > breakdownManyDecisions {
		flags = append(flags, fmt.Sprintf("many decisions (>%d)", breakdownManyDecisions))
	}
	if m.MaxNestingDepth >= breakdownDeepBranches {
		flags = append(flags, fmt.Sprintf("deep branches (depth >=%d)", breakdownDeepBranches))
	}
	if m.MaxLoopNestingDepth >= breakdownLoopNesting {
		flags = append(flags, "nested loops")
	}
	return flags
}

// printBreakdown prints the figures of one function under its header, and the thresholds it passes.
func printBreakdown(m ComplexityMetrics) {
	fmt.Printf("  size=%d lines  decisions=%d  branch-depth=%d  loop-depth=%d  boolean-lines=%d\n",
		m.LinesOfCode, m.DecisionCount, m.MaxNestingDepth, m.MaxLoopNestingDepth, m.BooleanLines)
	if flags := breakdownFlags(m); len(flags) > 0 {
		fmt.Printf("  over: %s\n", strings.Join(flags, ", "))
	}
}

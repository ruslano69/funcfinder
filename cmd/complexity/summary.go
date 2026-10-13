// summary.go - the -summary counts: how many functions sit at each level, how many each --breakdown limit flags, and the
// total score. They are the figures to track over time, so the limits in effect are printed with them.
package main

import (
	"fmt"
	"strings"
)

// summaryLevels lists the level names from the simplest up, in the order they are printed.
var summaryLevels = []string{"SIMPLE", "MODERATE", "HIGH", "VERY_HIGH", "CRITICAL"}

// complexitySummary holds the counts behind -summary, over the functions that were analysed.
type complexitySummary struct {
	Files        int
	Functions    int
	TotalScore   int            // the score of every function
	Levels       map[string]int // functions per level name
	Flagged      int            // functions that pass at least one limit
	FlaggedScore int            // the score of the flagged functions
	ByLimit      map[string]int // functions each limit flags, keyed len, if, bool, deep, loop
	LimitOrder   []string       // the limit keys in printing order
}

// summarize counts the functions of the analysed files against the limits th.
func summarize(files []FileComplexity, th breakdownThresholds) complexitySummary {
	s := complexitySummary{
		Files:   len(files),
		Levels:  make(map[string]int),
		ByLimit: make(map[string]int),
	}
	for _, c := range breakdownChecks(ComplexityMetrics{}, th) {
		s.LimitOrder = append(s.LimitOrder, c.key)
	}

	for _, f := range files {
		for _, fn := range f.Functions {
			s.Functions++
			s.TotalScore += fn.Complexity
			s.Levels[getLevelName(getComplexityLevel(fn.Complexity))]++

			flagged := false
			for _, c := range breakdownChecks(fn, th) {
				if c.fires {
					s.ByLimit[c.key]++
					flagged = true
				}
			}
			if flagged {
				s.Flagged++
				s.FlaggedScore += fn.Complexity
			}
		}
	}
	return s
}

// limitsLine lists the limits in effect, in the flags' spelling. A limit of 0 is off.
func limitsLine(th breakdownThresholds) string {
	return fmt.Sprintf("-len %s  -if %s  -bool %s  -deep %s  -loop %s",
		limitValue(th.Lines), limitValue(th.Decisions), limitValue(th.Bool), limitValue(th.Depth), limitValue(th.Loops))
}

// limitValue spells one limit: its number, or off.
func limitValue(n int) string {
	if n <= 0 {
		return "off"
	}
	return fmt.Sprint(n)
}

// printSummary prints the counts and the limits they were taken with: the same settings give the same figures.
func printSummary(files []FileComplexity, th breakdownThresholds) {
	s := summarize(files, th)

	pct := 0.0
	if s.Functions > 0 {
		pct = 100 * float64(s.Flagged) / float64(s.Functions)
	}

	levels := make([]string, 0, len(summaryLevels))
	for _, name := range summaryLevels {
		levels = append(levels, fmt.Sprintf("%s %d", name, s.Levels[name]))
	}

	limits := make([]string, 0, len(s.LimitOrder))
	for _, key := range s.LimitOrder {
		limits = append(limits, fmt.Sprintf("%s %d", key, s.ByLimit[key]))
	}

	fmt.Printf("Limits: %s\n", limitsLine(th))
	fmt.Printf("Functions: %d in %d files, total score %d\n", s.Functions, s.Files, s.TotalScore)
	fmt.Printf("Levels: %s\n", strings.Join(levels, "  "))
	fmt.Printf("Flagged: %d of %d functions (%.1f%%), score in them %d\n", s.Flagged, s.Functions, pct, s.FlaggedScore)
	fmt.Printf("By limit: %s (a function can count under several)\n", strings.Join(limits, "  "))
}

package main

import (
	"io/fs"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The patterns the hand scanners replace. They stay here as the reference: boolOps and hasDecision must give what
// these give on every line.
var (
	refBooleanRe  = regexp.MustCompile(`&&|\|\||\band\b|\bor\b`)
	refDecisionRe = regexp.MustCompile(`\b(if|elif|match|switch|for|while|loop)\b`)
)

func checkScanLine(t *testing.T, where, line string) {
	t.Helper()
	if got, want := boolOps(line), len(refBooleanRe.FindAllString(line, -1)); got != want {
		t.Errorf("%s: boolOps(%q) = %d, regexp gives %d", where, line, got, want)
	}
	if got, want := hasDecision(line), refDecisionRe.MatchString(line); got != want {
		t.Errorf("%s: hasDecision(%q) = %v, regexp gives %v", where, line, got, want)
	}
}

// TestBreakdownScanMatchesRegexp compares the scanners with the patterns on every line of the fixtures and of the
// repository's own sources, on edge cases, and on random lines built from the characters that matter.
func TestBreakdownScanMatchesRegexp(t *testing.T) {
	edge := []string{"", "&", "&&", "&&&", "&&&&", "|", "||", "|||", "a&&b||c", "and", "or", "andor", "_and", "and_",
		"x.and(y)", "1or2", "or1", "if", "elif", "else if", "loop_count", "for_each", "(if)", "élif", "éif", "if€",
		"orand", "a and b or c", "&&and||or", "while(x)", "match", "switches", "Match", "IF"}
	for _, line := range edge {
		checkScanLine(t, "edge", line)
	}

	lines := 0
	for _, root := range []string{"../../test_examples", "../../test_files", "../../internal", "."} {
		filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			data, err := os.ReadFile(p)
			if err != nil {
				return nil
			}
			for _, line := range strings.Split(string(data), "\n") {
				checkScanLine(t, p, line)
				lines++
			}
			return nil
		})
	}
	if lines < 1000 {
		t.Fatalf("only %d fixture lines read; the corpus is missing", lines)
	}

	alphabet := []string{"&", "|", "a", "n", "d", "o", "r", "i", "f", "e", "l", "s", "w", "h", "m", "t", "c", "p",
		"_", "1", " ", "(", "é", "\t", "and", "or", "if", "for", "loop", "elif"}
	rng := rand.New(rand.NewSource(1))
	for k := 0; k < 20000; k++ {
		var b strings.Builder
		for n := rng.Intn(12); n > 0; n-- {
			b.WriteString(alphabet[rng.Intn(len(alphabet))])
		}
		checkScanLine(t, "random", b.String())
	}
}

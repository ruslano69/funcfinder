package internal

import (
	"fmt"
	"regexp"
	"strings"
)

// Text an agent typically pastes around the code: the header line that --extract prints, and a markdown fence.
// Both are removed, so a mistake that the tool can recognise does not reach the file.

// stripExtractHeader removes the first line when it is the header --extract prints for the named function
// ("// name: start-end" or "# name: start-end").
func stripExtractHeader(text, name string) string {
	lines := strings.SplitN(text, "\n", 2)
	if len(lines) == 0 {
		return text
	}
	re := regexp.MustCompile(`^(//|#) ` + regexp.QuoteMeta(name) + `: \d+-\d+$`)
	if !re.MatchString(strings.TrimSpace(lines[0])) {
		return text
	}
	if len(lines) == 1 {
		return ""
	}
	return lines[1]
}

// stripFences removes a markdown code fence that wraps the text: a first line starting with ``` and a last line
// that is ```.
func stripFences(text string) string {
	lines := strings.Split(text, "\n")
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) >= 2 && strings.HasPrefix(strings.TrimSpace(lines[0]), "```") && strings.TrimSpace(lines[len(lines)-1]) == "```" {
		lines = lines[1 : len(lines)-1]
	}
	return strings.Join(lines, "\n")
}

// cleanReplacement applies both removals and reports a replacement that still has a fence on one side only.
func cleanReplacement(text, name string) (string, error) {
	text = stripExtractHeader(text, name)
	text = stripFences(text)
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			return "", fmt.Errorf("replacement has a markdown code fence line (%q); send only the code", strings.TrimSpace(line))
		}
	}
	return text, nil
}

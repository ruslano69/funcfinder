package internal

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// readFileLines reads every line of a file. Its errors name the step that failed; the finders
// that open a file themselves report the same messages.
func readFileLines(filename string) ([]string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	var lines []string
	err = forEachLine(file, func(line string) bool {
		lines = append(lines, line)
		return true
	})
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}
	return lines, nil
}

// ReadLines reads every line of a file, with no line-length limit. Its errors name the step that failed. The CLI
// tools read files through it: a scanner with the default limit stops at a long line and drops the rest silently.
func ReadLines(filename string) ([]string, error) {
	return readFileLines(filename)
}

// forEachLine calls fn with each line of r, in order, until fn returns false. The line rules are bufio.Scanner's,
// without its 64 KB line limit (minified bundles have longer lines): a CR before the LF is dropped, and a last line
// without a newline is a line; a file that ends with a newline has no extra empty line.
func forEachLine(r io.Reader, fn func(line string) bool) error {
	reader := bufio.NewReader(r)
	for {
		line, err := reader.ReadString('\n')
		if err != nil && err != io.EOF {
			return err
		}
		if err == io.EOF && line == "" {
			return nil
		}
		line = strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
		if !fn(line) || err == io.EOF {
			return nil
		}
	}
}

// LineRange represents a range of lines to extract
type LineRange struct {
	Start int // 1-based, inclusive
	End   int // 1-based, inclusive
}

// ParseLineRange parses line range string like "100:150", "100", ":50", "100:"
func ParseLineRange(rangeStr string) (LineRange, error) {
	if rangeStr == "" {
		return LineRange{}, fmt.Errorf("empty line range")
	}

	// Single line: "100"
	if !strings.Contains(rangeStr, ":") {
		line, err := strconv.Atoi(rangeStr)
		if err != nil {
			return LineRange{}, fmt.Errorf("invalid line number: %s", rangeStr)
		}
		if line < 1 {
			return LineRange{}, fmt.Errorf("line number must be >= 1, got %d", line)
		}
		return LineRange{Start: line, End: line}, nil
	}

	// Range: "100:150", ":50", "100:"
	parts := strings.SplitN(rangeStr, ":", 2)

	var start, end int
	var err error

	// Parse start
	if parts[0] == "" {
		start = 1 // From beginning
	} else {
		start, err = strconv.Atoi(parts[0])
		if err != nil {
			return LineRange{}, fmt.Errorf("invalid start line: %s", parts[0])
		}
		if start < 1 {
			return LineRange{}, fmt.Errorf("start line must be >= 1, got %d", start)
		}
	}

	// Parse end
	if parts[1] == "" {
		end = -1 // To the end of file
	} else {
		end, err = strconv.Atoi(parts[1])
		if err != nil {
			return LineRange{}, fmt.Errorf("invalid end line: %s", parts[1])
		}
		if end < 1 {
			return LineRange{}, fmt.Errorf("end line must be >= 1, got %d", end)
		}
	}

	// Validate range
	if end != -1 && start > end {
		return LineRange{}, fmt.Errorf("start line (%d) must be <= end line (%d)", start, end)
	}

	return LineRange{Start: start, End: end}, nil
}

// ReadFileLines reads specific lines from file according to range
// Returns lines with original line numbers preserved
func ReadFileLines(filename string, lineRange LineRange) ([]string, int, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, 0, err
	}
	defer file.Close()

	var lines []string
	lineNum := 1
	actualEnd := lineRange.End

	err = forEachLine(file, func(line string) bool {
		if lineNum >= lineRange.Start {
			if lineRange.End == -1 || lineNum <= lineRange.End {
				lines = append(lines, line)
			}
		}

		if lineRange.End != -1 && lineNum > lineRange.End {
			return false
		}

		lineNum++
		return true
	})
	if err != nil {
		return nil, 0, err
	}

	// Update actual end if it was "to the end"
	if actualEnd == -1 {
		actualEnd = lineNum - 1
	}

	if len(lines) == 0 {
		return nil, 0, fmt.Errorf("no lines found in range %d:%d (file has %d lines)", lineRange.Start, actualEnd, lineNum-1)
	}

	return lines, lineRange.Start, nil
}

// CheckPartialFunctions checks if line range cuts through function bodies
// Returns warning message if functions are partially included
func CheckPartialFunctions(functions []FunctionBounds, lineRange LineRange, totalLines int) string {
	actualEnd := lineRange.End
	if actualEnd == -1 {
		actualEnd = totalLines
	}

	var warnings []string

	for _, fn := range functions {
		// Function starts before range but ends inside
		if fn.Start < lineRange.Start && fn.End >= lineRange.Start && fn.End <= actualEnd {
			warnings = append(warnings, fmt.Sprintf("%s (starts at line %d, cut at %d)", fn.Name, fn.Start, lineRange.Start))
		}

		// Function starts inside range but ends after
		if fn.Start >= lineRange.Start && fn.Start <= actualEnd && fn.End > actualEnd {
			warnings = append(warnings, fmt.Sprintf("%s (ends at line %d, cut at %d)", fn.Name, fn.End, actualEnd))
		}

		// Function spans across entire range
		if fn.Start < lineRange.Start && fn.End > actualEnd {
			warnings = append(warnings, fmt.Sprintf("%s (spans %d-%d, range is %d-%d)", fn.Name, fn.Start, fn.End, lineRange.Start, actualEnd))
		}
	}

	if len(warnings) > 0 {
		return fmt.Sprintf("WARNING: --lines range may cut through function bodies:\n  %s", strings.Join(warnings, "\n  "))
	}

	return ""
}

// OutputPlainLines outputs lines in plain text format with line numbers
func OutputPlainLines(lines []string, startLine int) {
	for i, line := range lines {
		fmt.Printf("%d: %s\n", startLine+i, line)
	}
}

// OutputJSONLines outputs lines in JSON format
func OutputJSONLines(lines []string, startLine int, lineRange LineRange) {
	actualEnd := startLine + len(lines) - 1

	fmt.Println("{")
	fmt.Printf("  \"range\": {\"start\": %d, \"end\": %d},\n", lineRange.Start, actualEnd)
	fmt.Printf("  \"line_count\": %d,\n", len(lines))
	fmt.Println("  \"lines\": [")

	for i, line := range lines {
		// Escape special characters for JSON
		escaped := strings.ReplaceAll(line, "\\", "\\\\")
		escaped = strings.ReplaceAll(escaped, "\"", "\\\"")
		escaped = strings.ReplaceAll(escaped, "\n", "\\n")
		escaped = strings.ReplaceAll(escaped, "\r", "\\r")
		escaped = strings.ReplaceAll(escaped, "\t", "\\t")

		comma := ","
		if i == len(lines)-1 {
			comma = ""
		}

		fmt.Printf("    {\"line\": %d, \"content\": \"%s\"}%s\n", startLine+i, escaped, comma)
	}

	fmt.Println("  ]")
	fmt.Println("}")
}

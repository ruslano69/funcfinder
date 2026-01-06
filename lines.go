package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

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
	scanner := bufio.NewScanner(file)
	lineNum := 1
	actualEnd := lineRange.End

	for scanner.Scan() {
		if lineNum >= lineRange.Start {
			if lineRange.End == -1 || lineNum <= lineRange.End {
				lines = append(lines, scanner.Text())
			}
		}

		if lineRange.End != -1 && lineNum > lineRange.End {
			break
		}

		lineNum++
	}

	if err := scanner.Err(); err != nil {
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

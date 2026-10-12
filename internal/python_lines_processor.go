package internal

import (
	"os"
	"strconv"
	"strings"
)

// PythonScope represents a scope in Python code (function, class, or global)
type PythonScope struct {
	Name        string
	Kind        string // "function", "class", "global"
	StartLine   int
	EndLine     int
	StartIndent int
	Parent      *PythonScope
}

// LineAdjustment describes how a line range was adjusted
type LineAdjustment struct {
	OriginalStart int
	OriginalEnd   int
	FixedStart    int
	FixedEnd      int
	Reason        string
	ScopeName     string
	ScopeKind     string
}

// AnalyzePythonScopes performs Pass 1: analyzes indentation and builds scope map
// This scans the ENTIRE file first, ignoring any --lines boundaries
func AnalyzePythonScopes(filePath string) ([]PythonScope, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	config, err := LoadConfig()
	if err != nil {
		return nil, err
	}
	pyConfig, err := config.GetLanguageConfig("py")
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(content), "\n")
	logical := pythonLogicalLines(lines)
	var stack []*PythonScope
	var all []*PythonScope
	for i, line := range logical {
		if !line.statementStart {
			continue
		}
		kind := "function"
		matches := pyConfig.FuncRegex().FindStringSubmatch(line.code)
		if matches == nil {
			kind = "class"
			matches = pyConfig.ClassRegex().FindStringSubmatch(line.code)
		}
		if matches == nil {
			continue
		}
		indent := GetIndentLevel(lines[i])
		for len(stack) > 0 && (i+1 > stack[len(stack)-1].EndLine || indent <= stack[len(stack)-1].StartIndent) {
			stack = stack[:len(stack)-1]
		}
		start := i + 1
		if _, decoratorStart := pythonDecorators(lines, logical, i); decoratorStart > 0 {
			start = decoratorStart
		}
		scope := &PythonScope{
			Name:        ExtractFuncName(matches),
			Kind:        kind,
			StartLine:   start,
			EndLine:     pythonDefinitionEnd(lines, logical, i),
			StartIndent: indent,
		}
		if len(stack) > 0 {
			scope.Parent = stack[len(stack)-1]
		}
		stack = append(stack, scope)
		all = append(all, scope)
	}
	result := make([]PythonScope, len(all))
	for i, scope := range all {
		result[i] = *scope
	}
	return result, nil
}

// ValidateAndFixLineRange performs Pass 2: validates and adjusts line range
func ValidateAndFixLineRange(scopes []PythonScope, requestedStart, requestedEnd int) (int, int, []LineAdjustment) {
	adjustments := []LineAdjustment{}

	// Build a map of line -> scope for quick lookup
	lineScopeMap := make(map[int]*PythonScope)
	for i := range scopes {
		s := &scopes[i]
		for line := s.StartLine; line <= s.EndLine; line++ {
			lineScopeMap[line] = s
		}
	}

	// Find scope at requested start
	startScope := lineScopeMap[requestedStart]

	fixedStart := requestedStart
	fixedEnd := requestedEnd

	if startScope == nil {
		// Start is in global scope
		// Find all scopes that contain or start after requested start
		var candidateScopes []PythonScope
		for _, s := range scopes {
			// Scope is relevant if it starts after requested start
			// or if the requested range overlaps with it
			if s.StartLine > requestedStart {
				candidateScopes = append(candidateScopes, s)
			}
		}

		if len(candidateScopes) > 0 {
			// Find the FIRST scope that comes after requested start
			nextScope := candidateScopes[0]
			for _, s := range candidateScopes {
				if s.StartLine < nextScope.StartLine {
					nextScope = s
				}
			}

			// Expand range to include the next function/class
			adjustments = append(adjustments, LineAdjustment{
				OriginalStart: requestedStart,
				OriginalEnd:   requestedEnd,
				FixedStart:    nextScope.StartLine,
				FixedEnd:      nextScope.EndLine,
				Reason:        "Requested range is in global scope, expanded to include " + nextScope.Kind + " '" + nextScope.Name + "'",
				ScopeName:     nextScope.Name,
				ScopeKind:     nextScope.Kind,
			})
			fixedStart = nextScope.StartLine
			fixedEnd = nextScope.EndLine
			return fixedStart, fixedEnd, adjustments
		}

		// No next scope found, just check if end is in a scope
		endScope := lineScopeMap[requestedEnd]
		if endScope != nil {
			// Adjust to include the entire scope at end
			adjustments = append(adjustments, LineAdjustment{
				OriginalStart: requestedStart,
				OriginalEnd:   requestedEnd,
				FixedStart:    requestedStart,
				FixedEnd:      endScope.EndLine,
				Reason:        "End line is inside " + endScope.Kind + " '" + endScope.Name + "', expanded to include full scope",
				ScopeName:     endScope.Name,
				ScopeKind:     endScope.Kind,
			})
			fixedEnd = endScope.EndLine
		}
		return fixedStart, fixedEnd, adjustments
	}

	// Check if we're in a nested function (can't slice inside function body)
	if startScope.Kind == "function" {
		if requestedStart != startScope.StartLine {
			adjustments = append(adjustments, LineAdjustment{
				OriginalStart: requestedStart,
				OriginalEnd:   requestedEnd,
				FixedStart:    startScope.StartLine,
				FixedEnd:      startScope.EndLine, // Also expand end to full function
				Reason:        "Requested range is inside function body, expanded to full function",
				ScopeName:     startScope.Name,
				ScopeKind:     startScope.Kind,
			})
			fixedStart = startScope.StartLine
			fixedEnd = startScope.EndLine
		} else if requestedEnd > startScope.EndLine {
			// Start is correct, but end exceeds function - clip it
			adjustments = append(adjustments, LineAdjustment{
				OriginalStart: requestedStart,
				OriginalEnd:   requestedEnd,
				FixedStart:    fixedStart,
				FixedEnd:      startScope.EndLine,
				Reason:        "Requested end exceeds function body, clipped to function end",
				ScopeName:     startScope.Name,
				ScopeKind:     startScope.Kind,
			})
			fixedEnd = startScope.EndLine
		}
	}

	// Check if end is in a different scope
	endScope := lineScopeMap[requestedEnd]
	if endScope != nil && endScope != startScope {
		// If end is deeper in the same tree, clip to parent scope
		if endScope.Kind == "function" && startScope.Kind == "class" {
			// We're showing a class, but end is inside a method
			adjustments = append(adjustments, LineAdjustment{
				OriginalStart: requestedStart,
				OriginalEnd:   requestedEnd,
				FixedStart:    fixedStart,
				FixedEnd:      startScope.EndLine,
				Reason:        "End line is inside a method of this class, clipped to class end",
				ScopeName:     startScope.Name,
				ScopeKind:     startScope.Kind,
			})
			fixedEnd = startScope.EndLine
		}
	}

	return fixedStart, fixedEnd, adjustments
}

// FormatLineAdjustmentReport creates a human-readable report of adjustments
func FormatLineAdjustmentReport(adjustments []LineAdjustment, originalStart, originalEnd, fixedStart, fixedEnd int) string {
	if len(adjustments) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString("+------------------------------------------------------------------+\n")
	sb.WriteString("|           PYTHON LINES RANGE ADJUSTMENT REPORT                  |\n")
	sb.WriteString("+------------------------------------------------------------------+\n")
	sb.WriteString("| Requested range: " + formatLineRange(originalStart, originalEnd) + padToLen(formatLineRange(originalStart, originalEnd), 55) + "|\n")
	sb.WriteString("| Adjusted range:  " + formatLineRange(fixedStart, fixedEnd) + padToLen(formatLineRange(fixedStart, fixedEnd), 55) + "|\n")
	sb.WriteString("+------------------------------------------------------------------+\n")

	for i, adj := range adjustments {
		sb.WriteString("| Adjustment #" + strconv.Itoa(i+1) + ":\n")
		sb.WriteString("|   Scope: " + adj.ScopeKind + " '" + adj.ScopeName + "'\n")
		sb.WriteString("|   Reason: " + adj.Reason + "\n")
	}

	sb.WriteString("+------------------------------------------------------------------+\n")
	return sb.String()
}

func formatLineRange(start, end int) string {
	if end == -1 {
		return strconv.Itoa(start) + ":EOF"
	}
	return strconv.Itoa(start) + ":" + strconv.Itoa(end)
}

func padToLen(s string, length int) string {
	if len(s) >= length {
		return strings.Repeat(" ", length)
	}
	return strings.Repeat(" ", length-len(s))
}

func countLeadingSpaces(line string) int {
	count := 0
	for _, c := range line {
		if c == ' ' {
			count++
		} else if c == '\t' {
			count += 8 // tab is 8 spaces
		} else {
			break
		}
	}
	return count
}

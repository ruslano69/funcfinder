package internal

import (
	"bufio"
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
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Use pointers for heap-allocated scopes that can be updated
	scopeStack := []*PythonScope{} // stack of parent scopes (pointers to heap)
	allScopes := []*PythonScope{}  // track all scopes we created
	currentIndent := 0
	lineNum := 0
	pendingDecorators := 0 // count of decorators before current def/class

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		// Skip empty lines
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		// Calculate indent level (tabs or spaces)
		indent := countLeadingSpaces(line)
		if indent == len(line) {
			continue // empty or whitespace-only line
		}

		// Skip lines that are only comments
		if len(trimmed) > 0 && trimmed[0] == '#' {
			continue
		}

		// Handle decorators: count them, they don't end scopes
		if strings.HasPrefix(trimmed, "@") {
			pendingDecorators++
			continue
		}

		// Detect function or class definition
		// Note: "async def" functions also need to be detected
		isDef := strings.HasPrefix(trimmed, "def ") || strings.HasPrefix(trimmed, "async def ")
		isClass := strings.HasPrefix(trimmed, "class ")

		if isDef || isClass {
			// Pop scopes that ended before this line
			for len(scopeStack) > 0 && indent <= scopeStack[len(scopeStack)-1].StartIndent {
				closed := scopeStack[len(scopeStack)-1]
				closed.EndLine = lineNum - 1
				scopeStack = scopeStack[:len(scopeStack)-1]
			}

			// Create new scope on heap
			scopePtr := new(PythonScope)
			if isDef {
				// Handle both "def" and "async def"
				nameStart := 4 // length of "def "
				if strings.HasPrefix(trimmed, "async ") {
					nameStart = 10 // length of "async def "
				}
				parts := strings.SplitN(trimmed[nameStart:], "(", 2)
				scopePtr.Name = parts[0]
				scopePtr.Kind = "function"
			} else {
				parts := strings.SplitN(trimmed[6:], ":", 2)
				scopePtr.Name = parts[0]
				scopePtr.Kind = "class"
			}
			scopePtr.StartLine = lineNum - pendingDecorators
			scopePtr.StartIndent = indent
			pendingDecorators = 0 // reset after creating scope

			// Set parent
			if len(scopeStack) > 0 {
				scopePtr.Parent = scopeStack[len(scopeStack)-1]
			}

			// Push to stack and track
			scopeStack = append(scopeStack, scopePtr)
			allScopes = append(allScopes, scopePtr)
		} else if indent <= currentIndent && len(scopeStack) > 0 {
			// Line is at or before parent's indent level - close completed scopes
			// BUT: don't close if we're inside a docstring (""")
			isDocstringLine := strings.HasPrefix(trimmed, "\"\"\"") || strings.HasPrefix(trimmed, "    \"\"\"")

			for len(scopeStack) > 0 && indent <= scopeStack[len(scopeStack)-1].StartIndent {
				if isDocstringLine {
					// Don't close scopes when inside docstrings
					break
				}
				closed := scopeStack[len(scopeStack)-1]
				closed.EndLine = lineNum - 1
				scopeStack = scopeStack[:len(scopeStack)-1]
			}
		}

		currentIndent = indent
	}

	// Close remaining scopes at EOF
	for len(scopeStack) > 0 {
		closed := scopeStack[len(scopeStack)-1]
		closed.EndLine = lineNum
		scopeStack = scopeStack[:len(scopeStack)-1]
	}

	// Second pass: fix EndLine for scopes that were closed prematurely
	// Use the StartLine of the next scope (or EOF) as EndLine
	for i := 0; i < len(allScopes)-1; i++ {
		current := allScopes[i]
		next := allScopes[i+1]
		// If current EndLine is before next StartLine, use next StartLine - 1
		if current.EndLine < next.StartLine-1 {
			current.EndLine = next.StartLine - 1
		}
	}

	// Convert pointers to values for return
	result := make([]PythonScope, len(allScopes))
	for i, sp := range allScopes {
		result[i] = *sp
	}

	return result, scanner.Err()
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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
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

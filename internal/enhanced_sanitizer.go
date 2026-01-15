package internal

import (
	"strings"
)

type ParserState int

const (
	StateNormal ParserState = iota
	StateLineComment
	StateBlockComment
	StateString
	StateRawString
	StateCharLiteral
	StateMultiLineString
)

type State = ParserState

// State classification maps for O(1) lookups
var (
	stringStates = map[ParserState]bool{
		StateString:          true,
		StateRawString:       true,
		StateMultiLineString: true,
		StateCharLiteral:     true,
	}

	commentStates = map[ParserState]bool{
		StateLineComment:  true,
		StateBlockComment: true,
	}

	literalStates = map[ParserState]bool{
		StateString:          true,
		StateRawString:       true,
		StateMultiLineString: true,
		StateCharLiteral:     true,
		StateLineComment:     true,
		StateBlockComment:    true,
	}

	validStates = map[ParserState]bool{
		StateNormal:          true,
		StateLineComment:     true,
		StateBlockComment:    true,
		StateString:          true,
		StateRawString:       true,
		StateCharLiteral:     true,
		StateMultiLineString: true,
	}
)

func (s ParserState) String() string {
	switch s {
	case StateNormal:
		return "Normal"
	case StateLineComment:
		return "LineComment"
	case StateBlockComment:
		return "BlockComment"
	case StateString:
		return "String"
	case StateRawString:
		return "RawString"
	case StateCharLiteral:
		return "CharLiteral"
	case StateMultiLineString:
		return "MultiLineString"
	default:
		return "Unknown"
	}
}

// Sanitizer provides code sanitization by removing comments and string literals
type Sanitizer struct {
	config *LanguageConfig
	useRaw bool
}

func NewSanitizer(config *LanguageConfig, useRaw bool) *Sanitizer {
	return &Sanitizer{
		config: config,
		useRaw: useRaw,
	}
}

// Helper functions for filling result buffer with spaces
func fillWithSpaces(result []rune, startIdx, length int) {
	endIdx := startIdx + length
	if endIdx > len(result) {
		endIdx = len(result)
	}
	for i := startIdx; i < endIdx && i < len(result); i++ {
		result[i] = ' '
	}
}

func fillToEndWithSpaces(result []rune, startIdx int) {
	for i := startIdx; i < len(result); i++ {
		result[i] = ' '
	}
}

func replaceCharWithSpace(result []rune, idx int) {
	if idx < len(result) {
		result[idx] = ' '
	}
}

// Pattern matching functions
func (s *Sanitizer) matchesAt(runes []rune, pos int, pattern string) bool {
	if pattern == "" {
		return true
	}
	patternRunes := []rune(pattern)
	if pos+len(patternRunes) > len(runes) {
		return false
	}
	return string(runes[pos:pos+len(patternRunes)]) == pattern
}

func (s *Sanitizer) matchesStringDelimiter(runes []rune, pos int) bool {
	for _, char := range s.config.StringChars {
		if s.matchesAt(runes, pos, char) {
			return true
		}
	}
	return false
}

func (s *Sanitizer) matchesRawStringDelimiter(runes []rune, pos int) bool {
	for _, char := range s.config.RawStringChars {
		if s.matchesAt(runes, pos, char) {
			return true
		}
	}
	return false
}

func (s *Sanitizer) matchesCharDelimiter(runes []rune, pos int) bool {
	if s.config.CharDelimiters != nil {
		for _, char := range s.config.CharDelimiters {
			if s.matchesAt(runes, pos, char) {
				return true
			}
		}
	}
	return false
}

func (s *Sanitizer) matchesDocStringStart(runes []rune, pos int) bool {
	for _, marker := range s.config.DocStringMarkers {
		if s.matchesAt(runes, pos, marker) {
			return true
		}
	}
	return false
}

// State handlers
func (s *Sanitizer) handleBlockComment(line string, runes []rune, result []rune, idx int) (int, ParserState) {
	remaining := line[idx:]
	if pos := strings.Index(remaining, s.config.BlockCommentEnd); pos >= 0 {
		// Found closing - replace entire block
		fillWithSpaces(result, idx, pos+len([]rune(s.config.BlockCommentEnd)))
		return idx + pos + len([]rune(s.config.BlockCommentEnd)), StateNormal
	}
	// No closing found - replace rest of line
	fillToEndWithSpaces(result, idx)
	return len(runes), StateBlockComment
}

func (s *Sanitizer) handleString(runes []rune, result []rune, idx int) (int, ParserState) {
	if s.config.EscapeChar != "" && runes[idx] == []rune(s.config.EscapeChar)[0] && idx+1 < len(runes) {
		replaceCharWithSpace(result, idx)
		replaceCharWithSpace(result, idx+1)
		return idx + 2, StateString
	} else if s.matchesStringDelimiter(runes, idx) {
		replaceCharWithSpace(result, idx)
		return idx + 1, StateNormal
	}
	return idx + 1, StateString
}

func (s *Sanitizer) handleRawString(runes []rune, result []rune, idx int) (int, ParserState) {
	if s.matchesRawStringDelimiter(runes, idx) {
		return idx + 1, StateNormal
	}
	if s.useRaw {
		result[idx] = runes[idx]
	}
	return idx + 1, StateRawString
}

func (s *Sanitizer) handleCharLiteral(runes []rune, result []rune, idx int) (int, ParserState) {
	if s.config.EscapeChar != "" && runes[idx] == []rune(s.config.EscapeChar)[0] && idx+1 < len(runes) {
		replaceCharWithSpace(result, idx)
		replaceCharWithSpace(result, idx+1)
		return idx + 2, StateCharLiteral
	} else if s.matchesCharDelimiter(runes, idx) {
		replaceCharWithSpace(result, idx)
		return idx + 1, StateNormal
	}
	replaceCharWithSpace(result, idx)
	return idx + 1, StateCharLiteral
}

func (s *Sanitizer) handleMultiLineString(line string, runes []rune, result []rune, idx int) (int, ParserState) {
	remaining := line[idx:]
	foundEnd := -1
	foundDelim := ""
	newState := StateMultiLineString

	// Special case: C# verbatim strings end with " not @"
	// Check if we started with @" by looking for " as closing
	if pos := strings.Index(remaining, `"`); pos >= 0 {
		// Check if it's unescaped (not "")
		if pos+1 >= len(remaining) || remaining[pos+1] != '"' {
			foundEnd = pos
			foundDelim = `"`
		}
	}

	// Standard docstring markers
	if foundEnd < 0 {
		for _, marker := range s.config.DocStringMarkers {
			if pos := strings.Index(remaining, marker); pos >= 0 {
				if foundEnd == -1 || pos < foundEnd {
					foundEnd = pos
					foundDelim = marker
				}
			}
		}
	}

	if foundEnd >= 0 {
		fillWithSpaces(result, idx, foundEnd+len([]rune(foundDelim)))
		idx += foundEnd + len([]rune(foundDelim))
		newState = StateNormal
	}
	// Always replace rest of line with spaces (even if closing delimiter found)
	fillToEndWithSpaces(result, idx)
	return len(runes), newState
}

// StateNormal helper functions - return (newIdx, newState, handled)
func (s *Sanitizer) tryHandleCharDelimiter(runes []rune, result []rune, idx int) (int, ParserState, bool) {
	if len(s.config.CharDelimiters) == 0 {
		return idx, StateNormal, false
	}
	for _, char := range s.config.CharDelimiters {
		if char != "" && s.matchesAt(runes, idx, char) {
			replaceCharWithSpace(result, idx)
			return idx + len([]rune(char)), StateCharLiteral, true
		}
	}
	return idx, StateNormal, false
}

func (s *Sanitizer) tryHandleLineComment(runes []rune, idx int) (int, bool) {
	if s.config.LineComment != "" && s.matchesAt(runes, idx, s.config.LineComment) {
		return len(runes), true
	}
	return idx, false
}

func (s *Sanitizer) tryHandleRegularStrings(runes []rune, idx int) (ParserState, bool) {
	if !s.useRaw && s.matchesRawStringDelimiter(runes, idx) {
		return StateRawString, true
	} else if s.matchesStringDelimiter(runes, idx) {
		return StateString, true
	}
	return StateNormal, false
}

func (s *Sanitizer) tryHandleMultiLineString(line string, runes []rune, result []rune, idx int) (int, ParserState, bool) {
	if !s.matchesDocStringStart(runes, idx) {
		return idx, StateNormal, false
	}

	// Find which marker matched
	var matchedMarker string
	for _, marker := range s.config.DocStringMarkers {
		if s.matchesAt(runes, idx, marker) {
			matchedMarker = marker
			break
		}
	}

	if matchedMarker == "" {
		return idx, StateNormal, false
	}

	afterStart := line[idx+len([]rune(matchedMarker)):]
	foundEnd := -1
	closingDelim := matchedMarker

	// Special case: C# verbatim strings @"..." close with " not @"
	if matchedMarker == `@"` {
		// Search for unescaped " (in verbatim, "" is escaped ")
		searchPos := 0
		for searchPos < len(afterStart) {
			pos := strings.Index(afterStart[searchPos:], `"`)
			if pos < 0 {
				break
			}
			// Check if it's "" (escaped)
			actualPos := searchPos + pos
			if actualPos+1 < len(afterStart) && afterStart[actualPos+1] == '"' {
				searchPos = actualPos + 2 // Skip ""
				continue
			}
			// Found unescaped "
			foundEnd = actualPos
			closingDelim = `"`
			break
		}
	} else {
		// Standard docstring markers - search for same marker
		if pos := strings.Index(afterStart, matchedMarker); pos >= 0 {
			foundEnd = pos
		}
	}

	if foundEnd >= 0 {
		// Found closing in same line
		endIdx := idx + len([]rune(matchedMarker)) + foundEnd + len([]rune(closingDelim))
		fillWithSpaces(result, idx, endIdx-idx)
		return endIdx, StateNormal, true
	} else {
		// No closing - fill rest of line and transition to StateMultiLineString
		fillToEndWithSpaces(result, idx)
		return len(runes), StateMultiLineString, true
	}
}

func (s *Sanitizer) tryHandleBlockComment(line string, runes []rune, result []rune, idx int) (int, ParserState, bool) {
	if s.config.BlockCommentStart == "" || !s.matchesAt(runes, idx, s.config.BlockCommentStart) {
		return idx, StateNormal, false
	}

	afterStart := line[idx+len([]rune(s.config.BlockCommentStart)):]

	// Search for closing delimiter considering nesting
	depth := 1
	searchPos := 0
	foundEnd := -1

	for searchPos < len(afterStart) {
		// Check for nested comment start
		if strings.HasPrefix(afterStart[searchPos:], s.config.BlockCommentStart) {
			depth++
			searchPos += len([]rune(s.config.BlockCommentStart))
			continue
		}

		// Check for comment closing
		if strings.HasPrefix(afterStart[searchPos:], s.config.BlockCommentEnd) {
			depth--
			if depth == 0 {
				foundEnd = searchPos
				break
			}
			searchPos += len([]rune(s.config.BlockCommentEnd))
			continue
		}

		searchPos++
	}

	if foundEnd >= 0 {
		// Found closing in same line
		fillWithSpaces(result, idx, len([]rune(s.config.BlockCommentStart))+foundEnd+len([]rune(s.config.BlockCommentEnd)))
		return idx + len([]rune(s.config.BlockCommentStart)) + foundEnd + len([]rune(s.config.BlockCommentEnd)), StateNormal, true
	} else {
		// No closing - fill rest of line and transition to StateBlockComment
		fillToEndWithSpaces(result, idx)
		return len(runes), StateBlockComment, true
	}
}

func (s *Sanitizer) CleanLine(line string, state ParserState) (string, ParserState) {
	if len(line) == 0 {
		return line, state
	}

	result := make([]rune, len(line))
	for idx := range result {
		result[idx] = ' '
	}

	runes := []rune(line)
	idx := 0

	for idx < len(runes) {
		switch state {
		case StateBlockComment:
			idx, state = s.handleBlockComment(line, runes, result, idx)
			continue

		case StateString:
			idx, state = s.handleString(runes, result, idx)
			continue

		case StateRawString:
			idx, state = s.handleRawString(runes, result, idx)
			continue

		case StateCharLiteral:
			idx, state = s.handleCharLiteral(runes, result, idx)
			continue

		case StateMultiLineString:
			idx, state = s.handleMultiLineString(line, runes, result, idx)
			continue

		case StateNormal:
			// Try handlers in priority order
			var handled bool

			// 1. Char delimiters (highest priority)
			if idx, state, handled = s.tryHandleCharDelimiter(runes, result, idx); handled {
				continue
			}

			// 2. Multi-line strings (check before regular strings)
			if idx, state, handled = s.tryHandleMultiLineString(line, runes, result, idx); handled {
				continue
			}

			// 3. Block comments
			if idx, state, handled = s.tryHandleBlockComment(line, runes, result, idx); handled {
				continue
			}

			// 4. Line comments
			if idx, handled = s.tryHandleLineComment(runes, idx); handled {
				continue
			}

			// 5. Regular strings and raw strings
			if state, handled = s.tryHandleRegularStrings(runes, idx); handled {
				// State changed, continue to next iteration
			}

			// 6. Copy character if still in StateNormal
			if state == StateNormal && idx < len(result) && idx < len(runes) {
				result[idx] = runes[idx]
			}
			idx++
		}
	}

	return string(result), state
}

func (s *Sanitizer) CleanLines(lines []string) []string {
	result := make([]string, len(lines))
	state := StateNormal

	for i, line := range lines {
		result[i], state = s.CleanLine(line, state)
	}

	return result
}

func (s *Sanitizer) CleanCode(code string) string {
	lines := strings.Split(code, "\n")
	cleanedLines := s.CleanLines(lines)
	return strings.Join(cleanedLines, "\n")
}

func (s *Sanitizer) IsInString(state State) bool {
	return stringStates[state]
}

func (s *Sanitizer) IsInComment(state State) bool {
	return commentStates[state]
}

func (s *Sanitizer) IsInLiteral(state State) bool {
	return literalStates[state]
}

func (s *Sanitizer) GetConfig() *LanguageConfig {
	return s.config
}

func (s *Sanitizer) Reset() {}

// Backward compatibility aliases for tests
type EnhancedSanitizer = Sanitizer

func NewEnhancedSanitizer(config *LanguageConfig) *EnhancedSanitizer {
	return NewSanitizer(config, false)
}

// Utility functions
func ValidState(state ParserState) bool {
	return validStates[state]
}

func CountBraces(line string) int {
	count := 0
	for _, ch := range line {
		if ch == '{' {
			count++
		} else if ch == '}' {
			count--
		}
	}
	return count
}

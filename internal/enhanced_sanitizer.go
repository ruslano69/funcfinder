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

// runeLen returns the number of runes in s without allocating a []rune.
func runeLen(s string) int {
	n := 0
	for range s {
		n++
	}
	return n
}

// firstRune returns the first rune of s (or utf8.RuneError-equivalent 0 for
// an empty string), without allocating a []rune.
func firstRune(s string) rune {
	for _, r := range s {
		return r
	}
	return 0
}

// indexRunesFrom returns the absolute rune index >= start of the first
// occurrence of needle in runes, or -1. Matching is rune-by-rune and
// allocation-free (it ranges over needle's runes directly).
func indexRunesFrom(runes []rune, start int, needle string) int {
	if needle == "" {
		return start
	}
	nlen := runeLen(needle)
	for i := start; i+nlen <= len(runes); i++ {
		match := true
		j := i
		for _, nr := range needle {
			if runes[j] != nr {
				match = false
				break
			}
			j++
		}
		if match {
			return i
		}
	}
	return -1
}

// Pattern matching functions
//
// matchesAt reports whether runes, starting at pos, matches pattern rune for
// rune. It ranges over pattern's runes directly so it allocates nothing — this
// matters because it runs for (nearly) every character of every line of every
// file scanned, making it the single hottest path in the toolkit.
func (s *Sanitizer) matchesAt(runes []rune, pos int, pattern string) bool {
	if pattern == "" {
		return true
	}
	i := pos
	for _, pr := range pattern {
		if i >= len(runes) || runes[i] != pr {
			return false
		}
		i++
	}
	return true
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
	_ = line // retained for signature symmetry; scanning is rune-based
	endLen := runeLen(s.config.BlockCommentEnd)
	if pos := indexRunesFrom(runes, idx, s.config.BlockCommentEnd); pos >= 0 {
		// Found closing - blank from idx through the end delimiter (rune-based).
		fillWithSpaces(result, idx, pos+endLen-idx)
		return pos + endLen, StateNormal
	}
	// No closing found - replace rest of line
	fillToEndWithSpaces(result, idx)
	return len(runes), StateBlockComment
}

func (s *Sanitizer) handleString(runes []rune, result []rune, idx int) (int, ParserState) {
	if s.config.EscapeChar != "" && runes[idx] == firstRune(s.config.EscapeChar) && idx+1 < len(runes) {
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
	if s.config.EscapeChar != "" && runes[idx] == firstRune(s.config.EscapeChar) && idx+1 < len(runes) {
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
	_ = line // retained for signature symmetry; scanning is rune-based
	foundEnd := -1 // absolute rune index of the closing delimiter
	foundDelim := ""
	newState := StateMultiLineString

	// Special case: C# verbatim strings end with " not @"
	// Check if we started with @" by looking for " as closing
	if pos := indexRunesFrom(runes, idx, `"`); pos >= 0 {
		// Check if it's unescaped (not "")
		if pos+1 >= len(runes) || runes[pos+1] != '"' {
			foundEnd = pos
			foundDelim = `"`
		}
	}

	// Standard docstring markers
	if foundEnd < 0 {
		for _, marker := range s.config.DocStringMarkers {
			if pos := indexRunesFrom(runes, idx, marker); pos >= 0 {
				if foundEnd == -1 || pos < foundEnd {
					foundEnd = pos
					foundDelim = marker
				}
			}
		}
	}

	if foundEnd >= 0 {
		end := foundEnd + runeLen(foundDelim)
		fillWithSpaces(result, idx, end-idx)
		idx = end
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
			return idx + runeLen(char), StateCharLiteral, true
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

	startSearch := idx + runeLen(matchedMarker)
	foundEnd := -1 // absolute rune index of the closing delimiter
	closingDelim := matchedMarker

	// Special case: C# verbatim strings @"..." close with " not @"
	if matchedMarker == `@"` {
		// Search for unescaped " (in verbatim, "" is escaped ")
		p := startSearch
		for p < len(runes) {
			pos := indexRunesFrom(runes, p, `"`)
			if pos < 0 {
				break
			}
			// Check if it's "" (escaped)
			if pos+1 < len(runes) && runes[pos+1] == '"' {
				p = pos + 2 // Skip ""
				continue
			}
			// Found unescaped "
			foundEnd = pos
			closingDelim = `"`
			break
		}
	} else {
		// Standard docstring markers - search for same marker
		if pos := indexRunesFrom(runes, startSearch, matchedMarker); pos >= 0 {
			foundEnd = pos
		}
	}

	if foundEnd >= 0 {
		// Found closing in same line
		endIdx := foundEnd + runeLen(closingDelim)
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

	startLen := runeLen(s.config.BlockCommentStart)
	endLen := runeLen(s.config.BlockCommentEnd)

	// Search for closing delimiter considering nesting (rune-based).
	depth := 1
	p := idx + startLen
	foundEnd := -1 // absolute rune index where the closing delimiter starts

	for p < len(runes) {
		// Check for nested comment start
		if s.matchesAt(runes, p, s.config.BlockCommentStart) {
			depth++
			p += startLen
			continue
		}

		// Check for comment closing
		if s.matchesAt(runes, p, s.config.BlockCommentEnd) {
			depth--
			if depth == 0 {
				foundEnd = p
				break
			}
			p += endLen
			continue
		}

		p++
	}

	if foundEnd >= 0 {
		// Found closing in same line
		endIdx := foundEnd + endLen
		fillWithSpaces(result, idx, endIdx-idx)
		return endIdx, StateNormal, true
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

	// Size the buffer by rune count, not byte count: a byte-sized buffer would
	// leave (bytes-runes) trailing spaces on lines containing multibyte runes.
	runes := []rune(line)
	result := make([]rune, len(runes))
	for i := range result {
		result[i] = ' '
	}

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

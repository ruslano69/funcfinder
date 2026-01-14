package internal

import (
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"
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

const (
	LegacyStateNormal       = StateNormal
	LegacyStateLineComment  = StateLineComment
	LegacyStateBlockComment = StateBlockComment
	LegacyStateString       = StateString
	LegacyStateRawString    = StateRawString
)

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

type StringDelimiter struct {
	Start       string
	End         string
	EscapeChar  string
	IsRaw       bool
	IsMultiLine bool
	IsDocString bool
	Priority    int
}

type SanitizerConfig struct {
	LanguageConfig   *LanguageConfig
	StringDelimiters []StringDelimiter // Keep for backward compatibility and priority ordering
	CharDelimiters   []string

	// Map-based lookups for O(1) performance
	DelimiterMap        map[string]*StringDelimiter
	RegularStringDelims map[string]*StringDelimiter // !IsMultiLine
	RawStringDelims     map[string]*StringDelimiter // IsRaw && !IsMultiLine
	MultiLineDelims     map[string]*StringDelimiter // IsMultiLine
}

type EnhancedSanitizer struct {
	config          *LanguageConfig
	sanitizerConfig *SanitizerConfig
	useRaw          bool
	multiLineDepth  int
	blockCommentDepth int
}

func NewEnhancedSanitizer(config *LanguageConfig) *EnhancedSanitizer {
	sanitizerConfig := buildSanitizerConfig(config)

	return &EnhancedSanitizer{
		config:          config,
		sanitizerConfig: sanitizerConfig,
		useRaw:          false,
	}
}

func buildSanitizerConfig(config *LanguageConfig) *SanitizerConfig {
	delimiters := buildStringDelimiters(config)

	// Build delimiter maps for O(1) lookups
	delimiterMap := make(map[string]*StringDelimiter)
	regularMap := make(map[string]*StringDelimiter)
	rawMap := make(map[string]*StringDelimiter)
	multiLineMap := make(map[string]*StringDelimiter)

	for i := range delimiters {
		delim := &delimiters[i]

		// Add to main map
		delimiterMap[delim.Start] = delim

		// Add to type-specific maps
		if delim.IsMultiLine {
			multiLineMap[delim.Start] = delim
		} else if delim.IsRaw {
			rawMap[delim.Start] = delim
		} else {
			regularMap[delim.Start] = delim
		}
	}

	return &SanitizerConfig{
		LanguageConfig:      config,
		StringDelimiters:    delimiters,
		CharDelimiters:      buildCharDelimiters(config),
		DelimiterMap:        delimiterMap,
		RegularStringDelims: regularMap,
		RawStringDelims:     rawMap,
		MultiLineDelims:     multiLineMap,
	}
}

func buildStringDelimiters(config *LanguageConfig) []StringDelimiter {
	var delimiters []StringDelimiter

	for _, char := range config.StringChars {
		delimiters = append(delimiters, StringDelimiter{
			Start:      char,
			End:        char,
			EscapeChar: config.EscapeChar,
			Priority:   10,
		})
	}

	for _, char := range config.RawStringChars {
		delimiters = append(delimiters, StringDelimiter{
			Start:    char,
			End:      char,
			IsRaw:    true,
			Priority: 20,
		})
	}

	for _, marker := range config.DocStringMarkers {
		delimiters = append(delimiters, StringDelimiter{
			Start:       marker,
			End:         marker,
			EscapeChar:  config.EscapeChar,
			IsMultiLine: true,
			Priority:    30,
		})
	}

	sort.Slice(delimiters, func(i, j int) bool {
		return delimiters[i].Priority > delimiters[j].Priority
	})

	return delimiters
}

func buildCharDelimiters(config *LanguageConfig) []string {
	if config.CharDelimiters != nil {
		return config.CharDelimiters
	}
	return []string{""}
}

// Helper functions for replacing characters with spaces (DRY principle)

// fillWithSpaces replaces a range [startIdx, startIdx+length) with spaces (bounds-checked)
func fillWithSpaces(result []rune, startIdx, length int) {
	endIdx := startIdx + length
	if endIdx > len(result) {
		endIdx = len(result)
	}
	for i := startIdx; i < endIdx && i < len(result); i++ {
		result[i] = ' '
	}
}

// fillToEndWithSpaces replaces from startIdx to end of line with spaces
func fillToEndWithSpaces(result []rune, startIdx int) {
	for i := startIdx; i < len(result); i++ {
		result[i] = ' '
	}
}

// replaceCharWithSpace replaces a single character at index with space (bounds-checked)
func replaceCharWithSpace(result []rune, idx int) {
	if idx < len(result) {
		result[idx] = ' '
	}
}

func (s *EnhancedSanitizer) CleanLine(line string, state ParserState) (string, ParserState) {
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
		// Control flags for managing processing flow
		skipIdxIncrement := false
		skipRemainingProcessing := false

		switch state {
		case StateBlockComment:
			// Fast scan: search for block comment closing delimiter
			remaining := line[idx:]
			if pos := strings.Index(remaining, s.config.BlockCommentEnd); pos >= 0 {
				// Found closing - replace entire block
				fillWithSpaces(result, idx, pos+len([]rune(s.config.BlockCommentEnd)))
				idx += pos + len([]rune(s.config.BlockCommentEnd))
				state = StateNormal
				continue
			}
			// No closing found - replace rest of line
			fillToEndWithSpaces(result, idx)
			idx = len(runes)
			continue

		case StateString:
			if s.config.EscapeChar != "" && runes[idx] == []rune(s.config.EscapeChar)[0] && idx+1 < len(runes) {
				// Replace both characters (escape and next) with spaces
				replaceCharWithSpace(result, idx)
				replaceCharWithSpace(result, idx+1)
				idx += 2
			} else if s.matchesAnyStringDelimiter(runes, idx) {
				// Replace closing delimiter with space
				replaceCharWithSpace(result, idx)
				idx++
				state = StateNormal
				continue
			} else {
				idx++
			}

		case StateRawString:
			if s.matchesAnyRawStringDelimiter(runes, idx) {
				idx++
				state = StateNormal
				continue
			} else {
				if s.useRaw {
					result[idx] = runes[idx]
				}
				idx++
			}

		case StateCharLiteral:
			// Check escape character first (to avoid confusing escaped quote with closing delimiter)
			if s.config.EscapeChar != "" && runes[idx] == []rune(s.config.EscapeChar)[0] && idx+1 < len(runes) {
				// Replace escape character and next character with spaces
				if idx < len(result) {
					result[idx] = ' '
				}
				if idx+1 < len(result) {
					result[idx+1] = ' '
				}
				idx += 2
			} else if s.matchesAnyAt(runes, idx, s.sanitizerConfig.CharDelimiters) {
				// Found closing delimiter - replace it with space!
				if idx < len(result) {
					result[idx] = ' '
				}
				idx++
				state = StateNormal
			} else {
				// Replace current character with space
				if idx < len(result) {
					result[idx] = ' '
				}
				idx++
			}

		case StateMultiLineString:
			// Fast scan: search for multi-line string closing delimiter
			remaining := line[idx:]
			foundEnd := -1
			foundDelim := ""

			for _, delim := range s.sanitizerConfig.StringDelimiters {
				if delim.IsMultiLine && delim.End != "" {
					if pos := strings.Index(remaining, delim.End); pos >= 0 {
						if foundEnd == -1 || pos < foundEnd {
							foundEnd = pos
							foundDelim = delim.End
						}
					}
				}
			}

			if foundEnd >= 0 {
				// Found closing - replace entire block from current position to end of closing delimiter
				fillWithSpaces(result, idx, foundEnd+len([]rune(foundDelim)))
				idx += foundEnd + len([]rune(foundDelim))
				state = StateNormal
				// Skip remaining processing of this line
				skipRemainingProcessing = true
			}
			// No closing found - replace rest of line
			fillToEndWithSpaces(result, idx)
			idx = len(runes)
			continue

		case StateNormal:
			if len(s.sanitizerConfig.CharDelimiters) > 0 {
				for _, char := range s.sanitizerConfig.CharDelimiters {
					if char != "" && s.matchesAt(runes, idx, char) {
						// Replace opening delimiter with space!
						if idx < len(result) {
							result[idx] = ' '
						}
						state = StateCharLiteral
						idx += len([]rune(char))
						// Skip standard idx++ at end of loop
						skipIdxIncrement = true
						break
					}
				}
			}

			if state == StateNormal {
				// Fast scan for multi-line blocks: check in priority order
				foundMultiLineString := false

				// 1. Multi-line strings (""", ''', @" etc.) - CHECK BEFORE regular strings!
				for _, delim := range s.sanitizerConfig.StringDelimiters {
					if delim.IsMultiLine && delim.Start != "" && s.matchesAt(runes, idx, delim.Start) {
						// Start searching for closing delimiter
						afterStart := line[idx+len([]rune(delim.Start)):]
						foundEnd := -1

						// First search for @" as closing delimiter (for @"..." ending with @")
						if pos := strings.Index(afterStart, delim.Start); pos >= 0 {
							foundEnd = pos
						}

						// If @" not found as closing, search for " for @"..." case (regular end)
						// To search correctly, find the last " that is not part of ""
						if foundEnd < 0 {
							// Search for all " positions and take the last one
							lastPos := -1
							searchPos := 0
							for {
								pos := strings.Index(afterStart[searchPos:], "\"")
								if pos < 0 {
									break
								}
								lastPos = searchPos + pos
								searchPos = lastPos + 1
							}
							if lastPos >= 0 {
								foundEnd = lastPos
							}
						}

						if foundEnd >= 0 {
							// Found closing in same line - replace entire block
							// endIdx should point to character AFTER closing delimiter
							// foundEnd is the position of closing delimiter in afterStart
							endIdx := idx + len([]rune(delim.Start)) + foundEnd
							fillWithSpaces(result, idx, endIdx-idx+1)
							idx = endIdx + 1 // Move to character after closing delimiter
							if idx >= len(runes) {
								break
							}
							continue
						} else {
							// No closing found - replace from start to end of line and transition to StateMultiLineString
							fillToEndWithSpaces(result, idx)
							idx = len(runes)
							state = StateMultiLineString
							skipRemainingProcessing = true
						}
						foundMultiLineString = true
						break
					}
				}

				// Skip remaining checks if multi-line string was found
				if foundMultiLineString {
					if skipRemainingProcessing {
						continue
					}
					// If closing was in same line, skip remaining checks
					if idx < len(runes) && state == StateNormal {
						// Continue from current position but skip regular string checks
						// Move to copying character if needed
						if idx < len(result) && idx < len(runes) {
							result[idx] = runes[idx]
						}
						if !skipIdxIncrement {
							idx++
						}
						if idx >= len(runes) {
							break
						}
						continue
					}
				}

				// 2. Block comments (with nesting support)
				if s.config.BlockCommentStart != "" && s.matchesAt(runes, idx, s.config.BlockCommentStart) {
					afterStart := line[idx+len([]rune(s.config.BlockCommentStart)):]
					
					// Search for closing delimiter considering nesting
					depth := 1
					searchPos := 0
					foundEnd := -1
					
					for searchPos < len(afterStart) {
						// Check for start of new nested comment
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
						idx += len([]rune(s.config.BlockCommentStart)) + foundEnd + len([]rune(s.config.BlockCommentEnd))
					} else {
						// No closing found - replace from start to end of line and transition to StateBlockComment
						fillToEndWithSpaces(result, idx)
						idx = len(runes)
						state = StateBlockComment
						skipRemainingProcessing = true
					}
				}

				// 3. Single-line comments
				if s.matchesAt(runes, idx, s.config.LineComment) {
					idx = len(runes)
					skipRemainingProcessing = true
				}

				// 4. Regular strings and raw strings (only if not multi-line)
				if !s.useRaw && s.matchesAnyRawStringDelimiter(runes, idx) {
					state = StateRawString
				} else if s.matchesAnyStringDelimiter(runes, idx) {
					state = StateString
				}
			}

			if state == StateNormal {
				if idx < len(result) && idx < len(runes) {
					result[idx] = runes[idx]
				}
			}
			// Обработка флагов пропуска
			if skipRemainingProcessing {
				// Пропускаем idx++ и переходим к следующей итерации
				continue
			}
			if !skipIdxIncrement {
				idx++
			}
			// Check if we went out of bounds after increment
			if idx >= len(runes) {
				break
			}
		}
	}

	return string(result), state
}

func (s *EnhancedSanitizer) matchesAnyStringDelimiter(runes []rune, pos int) bool {
	for _, delim := range s.sanitizerConfig.StringDelimiters {
		if !delim.IsMultiLine && s.matchesAt(runes, pos, delim.Start) {
			return true
		}
	}
	return false
}

func (s *EnhancedSanitizer) matchesAnyRawStringDelimiter(runes []rune, pos int) bool {
	for _, delim := range s.sanitizerConfig.StringDelimiters {
		if delim.IsRaw && !delim.IsMultiLine && s.matchesAt(runes, pos, delim.Start) {
			return true
		}
	}
	return false
}

func (s *EnhancedSanitizer) matchesAt(runes []rune, pos int, pattern string) bool {
	if pattern == "" {
		return true
	}
	patternRunes := []rune(pattern)
	if pos+len(patternRunes) > len(runes) {
		return false
	}
	return string(runes[pos:pos+len(patternRunes)]) == pattern
}

func (s *EnhancedSanitizer) matchesAnyAt(runes []rune, pos int, patterns []string) bool {
	for _, pattern := range patterns {
		if len(pattern) == 0 {
			// Empty pattern matches anywhere, but we don't want that for delimiter matching
			continue
		}
		if s.matchesAt(runes, pos, pattern) {
			return true
		}
	}
	return false
}

func (s *EnhancedSanitizer) CleanLines(lines []string) []string {
	result := make([]string, len(lines))
	state := StateNormal

	for i, line := range lines {
		result[i], state = s.CleanLine(line, state)
	}

	return result
}

func (s *EnhancedSanitizer) CleanCode(code string) string {
	lines := strings.Split(code, "\n")
	cleanedLines := s.CleanLines(lines)
	return strings.Join(cleanedLines, "\n")
}

func (s *EnhancedSanitizer) Reset() {
	s.multiLineDepth = 0
	s.blockCommentDepth = 0
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

func CountAngleBrackets(line string) int {
	count := 0
	for _, ch := range line {
		if ch == '<' {
			count++
		} else if ch == '>' {
			count--
		}
	}
	return count
}

func (s *EnhancedSanitizer) IsInString(state ParserState) bool {
	return stringStates[state]
}

func (s *EnhancedSanitizer) IsInComment(state ParserState) bool {
	return commentStates[state]
}

func (s *EnhancedSanitizer) IsInLiteral(state ParserState) bool {
	return literalStates[state]
}

func (s *EnhancedSanitizer) GetCurrentState() string {
	return fmt.Sprintf("EnhancedSanitizer with %d delimiters", len(s.sanitizerConfig.StringDelimiters))
}

func SkipToNextLine(state ParserState) ParserState {
	if state == StateLineComment {
		return StateNormal
	}
	return state
}

func ValidState(state ParserState) bool {
	return validStates[state]
}

func IsTransitionValid(from, to ParserState) bool {
	validTransitions := map[ParserState][]ParserState{
		StateNormal:         {StateLineComment, StateBlockComment, StateString, StateRawString, StateCharLiteral, StateMultiLineString},
		StateLineComment:    {StateNormal},
		StateBlockComment:   {StateNormal, StateBlockComment},
		StateString:         {StateNormal, StateString},
		StateRawString:      {StateNormal, StateRawString},
		StateCharLiteral:    {StateNormal, StateCharLiteral},
		StateMultiLineString: {StateNormal, StateMultiLineString},
	}

	allowed, exists := validTransitions[from]
	if !exists {
		return false
	}

	for _, st := range allowed {
		if st == to {
			return true
		}
	}
	return false
}

func RuneCount(s string) int {
	return utf8.RuneCountInString(s)
}

func TruncateToRunes(s string, maxRunes int) string {
	if maxRunes <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	return string(runes[:maxRunes])
}

type Sanitizer struct {
	enhanced *EnhancedSanitizer
}

func NewSanitizer(config *LanguageConfig, useRaw bool) *Sanitizer {
	return &Sanitizer{
		enhanced: NewEnhancedSanitizer(config),
	}
}

func (s *Sanitizer) CleanLine(line string, state State) (string, State) {
	return s.enhanced.CleanLine(line, state)
}

func (s *Sanitizer) CleanLines(lines []string) []string {
	return s.enhanced.CleanLines(lines)
}

func (s *Sanitizer) CleanCode(code string) string {
	return s.enhanced.CleanCode(code)
}

func (s *Sanitizer) GetConfig() *LanguageConfig {
	return s.enhanced.config
}

func (s *Sanitizer) IsInString(state State) bool {
	return s.enhanced.IsInString(state)
}

func (s *Sanitizer) IsInComment(state State) bool {
	return s.enhanced.IsInComment(state)
}

func (s *Sanitizer) IsInLiteral(state State) bool {
	return s.enhanced.IsInLiteral(state)
}

func (s *Sanitizer) Reset() {
	s.enhanced.Reset()
}

func (s *Sanitizer) GetMultiLineDepth() int {
	return s.enhanced.multiLineDepth
}

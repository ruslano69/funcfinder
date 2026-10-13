package internal

import (
	"regexp"
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
	// stringDelim is the delimiter that opened the current StateString: only
	// the same delimiter closes it ("it's" is one string, not two).
	stringDelim string
	// runeBuf and resultBuf are reused from line to line by cleanLineSlow. A Sanitizer belongs to one finder
	// at a time (it already keeps stringDelim), so the buffers are not shared between goroutines.
	runeBuf   []rune
	resultBuf []rune
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
	} else if s.closesString(runes, idx) {
		replaceCharWithSpace(result, idx)
		s.stringDelim = ""
		return idx + 1, StateNormal
	}
	return idx + 1, StateString
}

// closesString reports whether the rune at pos ends the current string: the
// delimiter that opened it, or — when that is unknown (state carried in from a
// caller) — any string delimiter.
func (s *Sanitizer) closesString(runes []rune, pos int) bool {
	if s.stringDelim != "" {
		return s.matchesAt(runes, pos, s.stringDelim)
	}
	return s.matchesStringDelimiter(runes, pos)
}

// rawDelimiterLen returns the length of the raw-string delimiter that starts at pos,
// or 0 if none does.
func (s *Sanitizer) rawDelimiterLen(runes []rune, pos int) int {
	n := 0
	for _, char := range s.config.RawStringChars {
		if s.matchesAt(runes, pos, char) && runeLen(char) > n {
			n = runeLen(char)
		}
	}
	return n
}

// templateExprEnd returns the index just past the } that closes a template expression whose
// text starts at i, just after its "${", or -1 when the expression does not close on this
// line. The strings and nested template literals inside it are skipped whole.
func templateExprEnd(runes []rune, i int) int {
	depth := 0
	for i < len(runes) {
		switch runes[i] {
		case '{':
			depth++
		case '}':
			if depth == 0 {
				return i + 1
			}
			depth--
		case '\'', '"':
			i = quotedEnd(runes, i)
			continue
		case '`':
			if i = templateEnd(runes, i+1); i < 0 {
				return -1
			}
			continue
		}
		i++
	}
	return -1
}

// templateEnd returns the index just past the backtick that closes a template literal whose
// text starts at i, or -1 when it does not close on this line.
func templateEnd(runes []rune, i int) int {
	for i < len(runes) {
		switch runes[i] {
		case '\\':
			i += 2
			continue
		case '`':
			return i + 1
		case '$':
			if i+1 < len(runes) && runes[i+1] == '{' {
				if i = templateExprEnd(runes, i+2); i < 0 {
					return -1
				}
				continue
			}
		}
		i++
	}
	return -1
}

// quotedEnd returns the index just past the quote that closes the string opened at i, or the
// end of the line when it does not close here.
func quotedEnd(runes []rune, i int) int {
	q := runes[i]
	for j := i + 1; j < len(runes); j++ {
		switch runes[j] {
		case '\\':
			j++
		case q:
			return j + 1
		}
	}
	return len(runes)
}

func (s *Sanitizer) handleRawString(runes []rune, result []rune, idx int) (int, ParserState) {
	// A template expression is code: a string or a nested template inside it does not end the
	// template, and its braces are not text.
	if s.config.TemplateExpressions && runes[idx] == '$' && idx+1 < len(runes) && runes[idx+1] == '{' {
		if end := templateExprEnd(runes, idx+2); end > 0 {
			for k := idx; k < end; k++ {
				if s.useRaw {
					result[k] = runes[k]
				} else {
					result[k] = ' '
				}
			}
			return end, StateRawString
		}
	}
	// A raw string with its own closing delimiter (r", r#", @") closes only at that delimiter.
	if s.stringDelim != "" {
		if s.stringDelim == `@"` && runes[idx] == '"' {
			if idx+1 < len(runes) && runes[idx+1] == '"' {
				// a doubled quote in a verbatim string is one quote, and the string goes on
				if s.useRaw {
					result[idx], result[idx+1] = runes[idx], runes[idx+1]
				}
				return idx + 2, StateRawString
			}
			s.stringDelim = ""
			return idx + 1, StateNormal
		}
		if s.stringDelim != `@"` && s.matchesAt(runes, idx, s.stringDelim) {
			end := idx + runeLen(s.stringDelim)
			s.stringDelim = ""
			return end, StateNormal
		}
		if s.useRaw {
			result[idx] = runes[idx]
		}
		return idx + 1, StateRawString
	}
	if n := s.rawDelimiterLen(runes, idx); n > 0 {
		// A multi-quote delimiter (Kotlin """) closes at the end of its run: a longer run
		// keeps its extra quotes as content, so the closing quotes are consumed whole.
		end := idx + n
		if n > 1 {
			for end < len(runes) && runes[end] == runes[idx] {
				end++
			}
		}
		return end, StateNormal
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
	_ = line       // retained for signature symmetry; scanning is rune-based
	foundEnd := -1 // absolute rune index of the closing delimiter
	foundDelim := ""
	newState := StateMultiLineString

	if s.stringDelim == `@"` {
		// C# verbatim string: it ends at a " that is not doubled (`""` is an escaped quote).
		if pos := indexRunesFrom(runes, idx, `"`); pos >= 0 {
			if pos+1 >= len(runes) || runes[pos+1] != '"' {
				foundEnd = pos
				foundDelim = `"`
			}
		}
	} else if s.stringDelim != "" {
		// A docstring ends only with the marker that opened it: a single quote inside `"""` is text.
		if pos := indexRunesFrom(runes, idx, s.stringDelim); pos >= 0 {
			foundEnd = pos
			foundDelim = s.stringDelim
		}
	} else {
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
		s.stringDelim = ""
		// The rest of the line is code again: the caller goes on from the closing delimiter.
		return end, StateNormal
	}
	// Still open: the rest of the line is text.
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
			if s.config.CharLiteralLookahead {
				// A quote that does not close on this line is not a char literal (a Rust lifetime): it is code.
				end := charLiteralEnd(runes, idx)
				if end < 0 {
					return idx, StateNormal, false
				}
				return end, StateNormal, true
			}
			replaceCharWithSpace(result, idx)
			return idx + runeLen(char), StateCharLiteral, true
		}
	}
	return idx, StateNormal, false
}

// charLiteralEnd returns the index just past the char literal that starts with the quote at idx, or -1 when the
// quote does not start one. A literal is one character ('x', 'À') or an escape ('\n', '\'', '\u{a0}'),
// closed on the same line. The result cells of a literal stay blank, as the sanitizer leaves them.
func charLiteralEnd(runes []rune, idx int) int {
	if idx+2 < len(runes) && runes[idx+1] != '\\' && runes[idx+2] == '\'' {
		return idx + 3
	}
	if idx+3 < len(runes) && runes[idx+1] == '\\' {
		for j := idx + 3; j < len(runes) && j < idx+12; j++ {
			if runes[j] == '\'' {
				return j + 1
			}
		}
	}
	return -1
}

func (s *Sanitizer) tryHandleLineComment(runes []rune, idx int) (int, bool) {
	if s.config.LineComment != "" && s.matchesAt(runes, idx, s.config.LineComment) {
		return len(runes), true
	}
	return idx, false
}

// rawOpenerRe matches a raw-string opener of the forms r", r', r#", @" and a backtick.
var rawOpenerRe = regexp.MustCompile("^([r@]?)(#*)([\"'`])$")

// rawClosing returns the delimiter that closes a raw string opened with the entry of raw_string_chars: `"` for r",
// `"#` for r#", and `@"` for a C# verbatim string (its doubled quote is one quote). A backtick closes at its own run,
// and so do the forms that match no rule here (`"""`): those return "" and keep the run-based rule.
func rawClosing(opener string) string {
	m := rawOpenerRe.FindStringSubmatch(opener)
	if m == nil {
		return ""
	}
	if m[1] == "@" {
		return `@"`
	}
	return m[3] + m[2]
}

// rawOpenerAt returns the longest raw_string_chars entry that matches at pos, or "".
func (s *Sanitizer) rawOpenerAt(runes []rune, pos int) string {
	best := ""
	for _, char := range s.config.RawStringChars {
		if s.matchesAt(runes, pos, char) && len(char) > len(best) {
			best = char
		}
	}
	return best
}

// tripleQuoteAt reports whether a raw opener with a prefix (r", r') is the start of a triple-quoted string: its
// quote is followed by two more of the same quote.
func tripleQuoteAt(runes []rune, idx int, opener string) bool {
	if len(opener) < 2 || opener[0] == opener[len(opener)-1] {
		return false
	}
	q := rune(opener[len(opener)-1])
	if q != '"' && q != '\'' {
		return false
	}
	n := idx + runeLen(opener)
	return n+1 < len(runes) && runes[n] == q && runes[n+1] == q
}

// tryHandleRegularStrings opens a string at idx. It returns how many runes the opener takes: a raw opener such as r"
// is read whole, so that its quote is not taken for the closing delimiter.
func (s *Sanitizer) tryHandleRegularStrings(runes []rune, idx int) (ParserState, int, bool) {
	if !s.useRaw && s.matchesRawStringDelimiter(runes, idx) {
		opener := s.rawOpenerAt(runes, idx)
		if tripleQuoteAt(runes, idx, opener) {
			// r""" is a triple-quoted string with a prefix: the prefix is code, and the quotes are a docstring's.
			return StateNormal, 1, false
		}
		s.stringDelim = rawClosing(opener)
		if s.stringDelim == "" {
			return StateRawString, 1, true
		}
		return StateRawString, runeLen(opener), true
	}
	for _, char := range s.config.StringChars {
		if s.matchesAt(runes, idx, char) {
			s.stringDelim = char
			return StateString, 1, true
		}
	}
	return StateNormal, 1, false
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
		s.stringDelim = matchedMarker // the marker that closes this string
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

// noLiteralStarts reports whether no string, character, comment, or percent or regex literal can start anywhere on
// the line. Every handler in CleanLine fires only where one of the configured markers occurs, so when none occurs,
// a line in the normal state cleans to itself. An empty marker matches everywhere: it disables the check.
func (s *Sanitizer) noLiteralStarts(line string) bool {
	c := s.config
	for _, list := range [][]string{c.CharDelimiters, c.StringChars, c.RawStringChars, c.DocStringMarkers} {
		for _, marker := range list {
			if marker == "" || strings.Contains(line, marker) {
				return false
			}
		}
	}
	for _, marker := range []string{c.LineComment, c.BlockCommentStart} {
		if marker != "" && strings.Contains(line, marker) {
			return false
		}
	}
	if c.PercentLiterals && strings.Contains(line, "%") {
		return false
	}
	if c.RegexLiterals && strings.Contains(line, "/") {
		return false
	}
	return true
}

func (s *Sanitizer) CleanLine(line string, state ParserState) (string, ParserState) {
	if len(line) == 0 {
		return line, state
	}

	// Fast path: nothing in a valid line in the normal state can start a literal or a comment, so the line is its own
	// cleaned form and the rune conversion below is not needed.
	if state == StateNormal && utf8.ValidString(line) && s.noLiteralStarts(line) {
		return line, state
	}

	return s.cleanLineSlow(line, state)
}

// cleanLineSlow is the rune-by-rune cleaner that CleanLine's fast path must agree with.
func (s *Sanitizer) cleanLineSlow(line string, state ParserState) (string, ParserState) {
	// Size the buffer by rune count, not byte count: a byte-sized buffer would
	// leave (bytes-runes) trailing spaces on lines containing multibyte runes.
	// The rune buffers are the sanitizer's own, reused from line to line. Ranging over the string yields the same
	// runes as []rune(line), including U+FFFD for each invalid byte.
	runes := s.runeBuf[:0]
	for _, r := range line {
		runes = append(runes, r)
	}
	s.runeBuf = runes
	if cap(s.resultBuf) < len(runes) {
		s.resultBuf = make([]rune, len(runes))
	}
	result := s.resultBuf[:len(runes)]
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

			// 4a. Ruby percent literals (%q{...}, %r{...}): their brackets nest, and what is in them is text
			if idx, handled = s.tryHandlePercentLiteral(runes, result, idx); handled {
				continue
			}

			// 4b. Regular expression literals (JavaScript): their quotes are not strings
			if idx, handled = s.tryHandleRegexLiteral(runes, result, idx); handled {
				continue
			}

			// 5. Regular strings and raw strings
			var consumed int
			if state, consumed, handled = s.tryHandleRegularStrings(runes, idx); handled {
				// the opener is read whole: its rest is not a closing delimiter
				idx += consumed - 1
			}

			// 6. Copy character if still in StateNormal
			if state == StateNormal && idx < len(result) && idx < len(runes) {
				result[idx] = runes[idx]
			}
			idx++
		}
	}

	if state == StateString && s.config.SingleLineStrings {
		state = StateNormal
		s.stringDelim = ""
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

// LineFacts is what the sanitizer gives a finder about one source line, computed in one pass over the file.
// Decisions read these fields. The raw line is kept for the output, and for the few facts the sanitizer cannot
// give (heredoc terminators, quoted method names).
type LineFacts struct {
	Cleaned    string      // the line with comments and literals blanked
	StartState ParserState // the state the line starts in
	Indent     int         // indentation of the raw line (GetIndentLevel)
	CodeText   bool        // the raw line holds text outside comments; a literal-only line has it with a blank Cleaned
}

// CleanLinesFacts cleans a file line by line and returns the facts of each line.
func (s *Sanitizer) CleanLinesFacts(lines []string) []LineFacts {
	facts := make([]LineFacts, len(lines))
	state := StateNormal
	for i, line := range lines {
		facts[i], state = s.lineFacts(line, state)
	}
	return facts
}

// lineFacts returns the facts of one line that starts in state, and the state the next line starts in. A caller that
// skips some lines (a heredoc body is not cleaned) passes on the state of the last line it cleaned.
func (s *Sanitizer) lineFacts(line string, state ParserState) (LineFacts, ParserState) {
	fact := LineFacts{
		StartState: state,
		Indent:     GetIndentLevel(line),
		CodeText:   hasCodeText(line),
	}
	// A line that starts outside any literal and opens with the language's line comment (`# note` in Python and
	// Ruby) is a comment: hasCodeText knows only the C-family markers. Inside a literal the same text is its content.
	if state == StateNormal && s.config.LineComment != "" && strings.HasPrefix(strings.TrimSpace(line), s.config.LineComment) {
		fact.CodeText = false
	}
	var next ParserState
	fact.Cleaned, next = s.CleanLine(line, state)
	return fact, next
}

// lexerFacts returns the facts of lines that a language lexer has cleaned already (D, Scala): the lexer's text is the
// cleaned text, and the sanitizer does not run on it again. The raw lines give the indentation. The lexer's text holds
// no literal or comment start, so every line starts in the normal state.
func lexerFacts(cleaned, raw []string) []LineFacts {
	facts := make([]LineFacts, len(cleaned))
	for i := range cleaned {
		facts[i] = LineFacts{
			Cleaned:    cleaned[i],
			StartState: StateNormal,
			Indent:     GetIndentLevel(raw[i]),
			CodeText:   hasCodeText(cleaned[i]),
		}
	}
	return facts
}

// cleanedTexts returns the cleaned text of each line, for the helpers that look across lines.
func cleanedTexts(facts []LineFacts) []string {
	texts := make([]string, len(facts))
	for i := range facts {
		texts[i] = facts[i].Cleaned
	}
	return texts
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

func (s *Sanitizer) Reset() { s.stringDelim = "" }

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

// regexPrecededBy lists the characters after which a slash starts a regular expression
// rather than being a division.
const regexPrecededBy = "(,=:[!&|?{};+-*%<>~^"

// regexKeywords are the words after which a slash starts a regular expression.
var regexKeywords = map[string]bool{"return": true, "typeof": true, "case": true, "in": true, "of": true, "yield": true, "await": true, "if": true, "elsif": true, "unless": true, "when": true}

// endsJSXAttribute reports whether the last non-blank character before idx ends a JSX attribute value or a
// tag name: `}` of an expression, a closing quote, `]`, or a name character.
func endsJSXAttribute(result []rune, idx int) bool {
	k := idx - 1
	for k >= 0 && (result[k] == ' ' || result[k] == '\t') {
		k--
	}
	if k < 0 {
		return false
	}
	c := result[k]
	return c == '}' || c == '"' || c == '\'' || c == '`' || c == ']' || c == '_' || c == '$' ||
		(c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

// regexPositionAllowed reports whether a slash at idx starts a regular expression: the
// last non-blank character before it is an operator or an opening bracket, or the word
// before it is one of regexKeywords.
func regexPositionAllowed(result []rune, idx int) bool {
	// A slash right after '<' is a closing tag (</h4>) in JSX, not the start of a regular expression.
	if idx > 0 && result[idx-1] == '<' {
		return false
	}
	k := idx - 1
	for k >= 0 && (result[k] == ' ' || result[k] == '\t') {
		k--
	}
	if k < 0 {
		return true
	}
	if strings.ContainsRune(regexPrecededBy, result[k]) {
		return true
	}
	end := k + 1
	start := k
	for start >= 0 && (result[start] == '_' || result[start] == '$' || (result[start] >= 'a' && result[start] <= 'z') || (result[start] >= 'A' && result[start] <= 'Z')) {
		start--
	}
	return regexKeywords[string(result[start+1:end])]
}

// isRegexFlag reports whether a rune is a regular expression flag.
func isRegexFlag(c rune) bool {
	return strings.ContainsRune("dgimsuvy", c)
}

// percentClose maps the opening bracket of a Ruby percent literal to its closing bracket.
var percentClose = map[rune]rune{'(': ')', '[': ']', '{': '}', '<': '>'}

// tryHandlePercentLiteral blanks a Ruby percent literal, %q{...}, %Q{...}, %r{...},
// %w[...] and so on, that starts at idx. Its brackets nest. A literal that does not close
// on the line takes the rest of the line: the quotes in it are text, not a string.
func (s *Sanitizer) tryHandlePercentLiteral(runes []rune, result []rune, idx int) (int, bool) {
	if !s.config.PercentLiterals || runes[idx] != '%' || !regexPositionAllowed(result, idx) {
		return idx, false
	}
	j := idx + 1
	if j < len(runes) && strings.ContainsRune("qQrwWiIs", runes[j]) {
		j++
	}
	if j >= len(runes) {
		return idx, false
	}
	open := runes[j]
	closing, ok := percentClose[open]
	if !ok {
		return idx, false
	}
	depth := 0
	for k := j; k < len(runes); k++ {
		switch runes[k] {
		case '\\':
			k++
		case open:
			depth++
		case closing:
			depth--
			if depth == 0 {
				fillWithSpaces(result, idx, k+1-idx)
				return k + 1, true
			}
		}
	}
	fillWithSpaces(result, idx, len(runes)-idx)
	return len(runes), true
}

// tryHandleRegexLiteral blanks a regular expression literal, /.../flags, that starts at
// idx. A slash that is a division, or has no closing slash on the line, is not handled.
func (s *Sanitizer) tryHandleRegexLiteral(runes []rune, result []rune, idx int) (int, bool) {
	if !s.config.RegexLiterals || runes[idx] != '/' {
		return idx, false
	}
	if idx+1 < len(runes) && (runes[idx+1] == '/' || runes[idx+1] == '*') {
		return idx, false
	}
	// `/>` ends a self-closing JSX tag (`<input ... />`, `}/>`): the slash is not a regular expression. A regex
	// may be `/>/` after '(' or ',', so only a slash after a tag's last attribute value or name is exempt.
	if idx+1 < len(runes) && runes[idx+1] == '>' && endsJSXAttribute(result, idx) {
		return idx, false
	}
	if !regexPositionAllowed(result, idx) {
		return idx, false
	}
	inClass := false
	for j := idx + 1; j < len(runes); j++ {
		c := runes[j]
		switch {
		case c == '\\':
			j++
		case inClass:
			if c == ']' {
				inClass = false
			}
		case c == '[':
			inClass = true
		case c == '/':
			end := j + 1
			for end < len(runes) && isRegexFlag(runes[end]) {
				end++
			}
			fillWithSpaces(result, idx, end-idx)
			return end, true
		}
	}
	return idx, false
}

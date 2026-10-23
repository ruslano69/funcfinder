package internal

import "unicode"

// cppRawLiteralOpener recognizes a complete C++ raw-string opening token.
// Its delimiter is at most 16 basic characters, excluding whitespace, ( ) and \.
// The returned closing token is independent of quotes and escapes in the body.
func cppRawLiteralOpener(runes []rune, idx int) (int, string) {
	if idx > 0 {
		prev := runes[idx-1]
		if prev == '_' || unicode.IsLetter(prev) || unicode.IsDigit(prev) {
			return 0, ""
		}
	}
	start := idx
	if idx < len(runes) && runes[idx] == 'u' {
		idx++
		if idx < len(runes) && runes[idx] == '8' {
			idx++
		}
	} else if idx < len(runes) && (runes[idx] == 'U' || runes[idx] == 'L') {
		idx++
	}
	if idx+1 >= len(runes) || runes[idx] != 'R' || runes[idx+1] != '"' {
		return 0, ""
	}
	delim := idx + 2
	for end := delim; end < len(runes) && end-delim <= 16; end++ {
		c := runes[end]
		if c == '(' {
			return end + 1 - start, ")" + string(runes[delim:end]) + `"`
		}
		if c < '!' || c > '~' || c == ')' || c == '\\' {
			return 0, ""
		}
	}
	return 0, ""
}

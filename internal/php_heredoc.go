package internal

import (
	"strings"
	"unicode"
)

// Keep PHP heredoc/nowdoc text out of the generic brace/string parser. Line and
// rune positions are preserved; code following an indented closing label stays
// visible. The caller restores original source when extracting function bodies.
func maskPHPHeredocs(lines []string, config *LanguageConfig) []string {
	result := make([]string, len(lines))
	sanitizer := NewSanitizer(config, false)
	state := StateNormal
	label := ""
	for i, line := range lines {
		runes := []rune(line)
		if label != "" {
			end := phpHeredocClose(runes, label)
			if end < 0 {
				result[i] = strings.Repeat(" ", len(runes))
				continue
			}
			for j := 0; j < end; j++ {
				runes[j] = ' '
			}
			label = ""
			line = string(runes)
		}
		cleaned, nextState := sanitizer.CleanLine(line, state)
		code := []rune(cleaned)
		for j := 0; j+2 < len(code); j++ {
			if code[j] != '<' || code[j+1] != '<' || code[j+2] != '<' {
				continue
			}
			if name := phpHeredocOpen(runes[j+3:]); name != "" {
				label = name
				for k := j; k < len(runes); k++ {
					runes[k] = ' '
				}
				line = string(runes)
				break
			}
		}
		result[i] = line
		state = nextState
	}
	return result
}

func phpHeredocOpen(tail []rune) string {
	i := 0
	for i < len(tail) && (tail[i] == ' ' || tail[i] == '\t') {
		i++
	}
	var quote rune
	if i < len(tail) && (tail[i] == '\'' || tail[i] == '"') {
		quote = tail[i]
		i++
	}
	start := i
	if i >= len(tail) || !phpLabelStart(tail[i]) {
		return ""
	}
	for i < len(tail) && phpLabelChar(tail[i]) {
		i++
	}
	name := string(tail[start:i])
	if quote != 0 {
		if i >= len(tail) || tail[i] != quote {
			return ""
		}
		i++
	}
	rest := strings.TrimSpace(string(tail[i:]))
	if rest != "" && !strings.HasPrefix(rest, "//") && !strings.HasPrefix(rest, "#") {
		return ""
	}
	return name
}

func phpHeredocClose(line []rune, label string) int {
	i := 0
	for i < len(line) && (line[i] == ' ' || line[i] == '\t') {
		i++
	}
	name := []rune(label)
	if i+len(name) > len(line) || string(line[i:i+len(name)]) != label {
		return -1
	}
	end := i + len(name)
	if end < len(line) && phpLabelChar(line[end]) {
		return -1
	}
	return end
}

func phpLabelStart(ch rune) bool { return ch == '_' || unicode.IsLetter(ch) }
func phpLabelChar(ch rune) bool  { return phpLabelStart(ch) || unicode.IsDigit(ch) }

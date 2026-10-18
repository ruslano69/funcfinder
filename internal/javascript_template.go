package internal

import (
	"strings"
	"unicode"
)

// The same incremental stack reads a template in the line sanitizer and in
// whole-file token recovery. Frames survive line breaks without rescanning.
type jsTemplateFrame struct {
	kind    rune
	braces  int
	operand bool
}
type jsTemplateScanner struct{ frames []jsTemplateFrame }

func (s *jsTemplateScanner) feed(text []rune, i int) (int, bool) {
	for i < len(text) && len(s.frames) > 0 {
		frame := &s.frames[len(s.frames)-1]
		c := text[i]
		switch frame.kind {
		case '`':
			if c == '\\' {
				i = min(len(text), i+2)
				continue
			}
			if c == '`' {
				s.frames = s.frames[:len(s.frames)-1]
				i++
				continue
			}
			if c == '$' && i+1 < len(text) && text[i+1] == '{' {
				s.frames = append(s.frames, jsTemplateFrame{kind: '{', operand: true})
				i += 2
				continue
			}
		case '\'', '"':
			if c == '\\' {
				i = min(len(text), i+2)
				continue
			}
			if c == frame.kind {
				s.frames = s.frames[:len(s.frames)-1]
			}
		case '*':
			if c == '*' && i+1 < len(text) && text[i+1] == '/' {
				s.frames = s.frames[:len(s.frames)-1]
				i += 2
				continue
			}
		case '{':
			if unicode.IsSpace(c) {
				i++
				continue
			}
			if jsWord(c) {
				start := i
				for i < len(text) && jsWord(text[i]) {
					i++
				}
				frame.operand = regexKeywords[string(text[start:i])]
				continue
			}
			switch c {
			case '\'', '"', '`':
				frame.operand = false
				s.frames = append(s.frames, jsTemplateFrame{kind: c})
			case '{':
				frame.braces++
				frame.operand = true
			case '}':
				if frame.braces == 0 {
					s.frames = s.frames[:len(s.frames)-1]
				} else {
					frame.braces--
					frame.operand = false
				}
			case '/':
				if i+1 < len(text) && text[i+1] == '/' {
					for i < len(text) && text[i] != '\n' {
						i++
					}
					continue
				}
				if i+1 < len(text) && text[i+1] == '*' {
					s.frames = append(s.frames, jsTemplateFrame{kind: '*'})
					i += 2
					continue
				}
				if frame.operand {
					if end := regexLiteralEnd(text, i, true); end >= 0 {
						frame.operand = false
						i = end
						continue
					}
				}
				frame.operand = true
			default:
				frame.operand = strings.ContainsRune(regexPrecededBy, c)
			}
		}
		i++
	}
	return i, len(s.frames) == 0
}

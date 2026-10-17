package internal

import (
	"strings"
	"unicode"
)

type csToken struct {
	text string
	line int
}

var csModifiers = map[string]bool{
	"public": true, "private": true, "protected": true, "internal": true,
	"static": true, "abstract": true, "virtual": true, "override": true,
	"sealed": true, "async": true, "unsafe": true, "extern": true,
	"partial": true, "readonly": true, "new": true, "ref": true, "scoped": true,
}

var csStatements = map[string]bool{
	"return": true, "throw": true, "await": true, "yield": true, "new": true,
	"if": true, "else": true, "for": true, "foreach": true, "while": true,
	"do": true, "switch": true, "case": true, "catch": true, "finally": true,
	"try": true, "lock": true, "using": true, "fixed": true, "checked": true,
	"unchecked": true, "nameof": true, "typeof": true, "sizeof": true,
	"class": true, "struct": true, "interface": true, "record": true,
	"enum": true, "namespace": true, "delegate": true,
	"base": true, "this": true, "default": true,
}

func csIdentifier(text string) bool {
	text = strings.TrimPrefix(text, "@")
	if text == "" {
		return false
	}
	for i, ch := range text {
		if ch != '_' && !unicode.IsLetter(ch) && !(i > 0 && unicode.IsDigit(ch)) {
			return false
		}
	}
	return true
}

func csTokens(facts []LineFacts) ([]csToken, []int) {
	return braceTokens(facts, false)
}

// Java additionally permits dollar signs in identifiers. Keep the C# token
// rules unchanged while sharing bracket pairing and source coordinates.
func braceTokens(facts []LineFacts, java bool) ([]csToken, []int) {
	var tokens []csToken
	var pairs []int
	stacks := map[string][]int{"(": {}, "[": {}, "{": {}}
	for line, fact := range facts {
		if strings.HasPrefix(strings.TrimSpace(fact.Cleaned), "#") {
			continue
		}
		chars := []rune(fact.Cleaned)
		for i := 0; i < len(chars); {
			if unicode.IsSpace(chars[i]) {
				i++
				continue
			}
			end := i + 1
			if unicode.IsLetter(chars[i]) || chars[i] == '_' || chars[i] == '@' || java && chars[i] == '$' {
				for end < len(chars) && (unicode.IsLetter(chars[end]) || unicode.IsDigit(chars[end]) || chars[end] == '_' || java && chars[end] == '$') {
					end++
				}
			}
			text := string(chars[i:end])
			index := len(tokens)
			tokens = append(tokens, csToken{text, line})
			pairs = append(pairs, -1)
			if _, ok := stacks[text]; ok {
				stacks[text] = append(stacks[text], index)
			} else if open := map[string]string{")": "(", "]": "[", "}": "{"}[text]; open != "" {
				stack := stacks[open]
				if len(stack) > 0 {
					start := stack[len(stack)-1]
					pairs[start], pairs[index] = index, start
					stacks[open] = stack[:len(stack)-1]
				}
			}
			i = end
		}
	}
	return tokens, pairs
}

func csSkipAttributes(tokens []csToken, pairs []int, start, end int) int {
	for start < end && tokens[start].text == "[" {
		if pairs[start] < start || pairs[start] >= end {
			return end
		}
		start = pairs[start] + 1
	}
	return start
}

func csClassName(tokens []csToken, pairs []int, start, end int) string {
	start = csSkipAttributes(tokens, pairs, start, end)
	for start < end && csModifiers[tokens[start].text] {
		start++
	}
	if start >= end {
		return ""
	}
	switch tokens[start].text {
	case "class", "struct", "interface", "record", "enum":
		start++
		if start < end && (tokens[start].text == "class" || tokens[start].text == "struct") {
			start++
		}
		if start < end && csIdentifier(tokens[start].text) {
			return strings.TrimPrefix(tokens[start].text, "@")
		}
	}
	return ""
}

// A return type is a qualified identifier (with type arguments/suffixes), or
// a tuple. An arbitrary balanced call/cast is not a type.
func csSkipType(tokens []csToken, pairs []int, start, end int) int {
	i := start
	if i >= end {
		return start
	}
	if tokens[i].text == "(" {
		close := pairs[i]
		if close < i || close >= end {
			return start
		}
		comma := false
		for j := i + 1; j < close; j++ {
			if tokens[j].text == "," {
				comma = true
			}
		}
		if !comma {
			return start
		}
		i = close + 1
	} else if csIdentifier(tokens[i].text) {
		i++
	} else {
		return start
	}
	for i < end {
		switch tokens[i].text {
		case "<":
			depth := 1
			i++
			for i < end && depth > 0 {
				if tokens[i].text == "<" {
					depth++
				} else if tokens[i].text == ">" {
					depth--
				}
				i++
			}
			if depth != 0 {
				return start
			}
		case "?", "*":
			i++
		case "[":
			if pairs[i] < i || pairs[i] >= end {
				return start
			}
			i = pairs[i] + 1
		case ".":
			if i+1 >= end || !csIdentifier(tokens[i+1].text) {
				return i
			}
			i += 2
		case ":":
			if i+2 >= end || tokens[i+1].text != ":" || !csIdentifier(tokens[i+2].text) {
				return i
			}
			i += 3
		default:
			return i
		}
	}
	return i
}

func csPropertyBody(tokens []csToken, pairs []int, start, end int) bool {
	start = csSkipAttributes(tokens, pairs, start, end)
	for start < end && (csModifiers[tokens[start].text] || tokens[start].text == "event") {
		start++
	}
	typeEnd := csSkipType(tokens, pairs, start, end)
	if typeEnd <= start || typeEnd >= end {
		return false
	}
	if typeEnd+1 == end && csIdentifier(tokens[typeEnd].text) {
		return true
	}
	nameAt := end - 1
	if tokens[nameAt].text == "]" {
		nameAt = pairs[nameAt] - 1
		if nameAt < typeEnd || tokens[nameAt].text != "this" {
			return false
		}
	}
	if !csIdentifier(tokens[nameAt].text) {
		return false
	}
	return nameAt == typeEnd || csSkipType(tokens, pairs, typeEnd, nameAt) == nameAt-1 && tokens[nameAt-1].text == "."
}

func csFunctionName(tokens []csToken, pairs []int, start, open int, className string) (string, int) {
	start = csSkipAttributes(tokens, pairs, start, open)
	if start >= open || csStatements[tokens[start].text] && tokens[start].text != "new" {
		return "", start
	}
	operatorAt := -1
	for i := start; i < open; i++ {
		if tokens[i].text == "operator" {
			operatorAt = i
		}
	}
	for i := start; i < open; i++ {
		switch tokens[i].text {
		case "=":
			if operatorAt < 0 || i < operatorAt {
				return "", start
			}
		case ";", "{", "}", "class", "struct", "interface", "record", "enum", "namespace", "delegate":
			return "", start
		case "(", "[":
			if pairs[i] < i || pairs[i] >= open {
				return "", start
			}
		}
	}
	for i := start; i < open; i++ {
		if tokens[i].text == "operator" {
			var suffix strings.Builder
			for _, token := range tokens[i+1 : open] {
				suffix.WriteString(token.text)
			}
			if i > start && (tokens[i-1].text == "implicit" || tokens[i-1].text == "explicit") {
				return tokens[i-1].text + " operator " + suffix.String(), start
			}
			return "operator " + suffix.String(), start
		}
	}
	nameAt := open - 1
	if tokens[nameAt].text == ">" {
		depth := 1
		nameAt--
		for nameAt >= start && depth > 0 {
			if tokens[nameAt].text == ">" {
				depth++
			} else if tokens[nameAt].text == "<" {
				depth--
			}
			nameAt--
		}
	}
	if nameAt < start || !csIdentifier(tokens[nameAt].text) || csStatements[tokens[nameAt].text] || csModifiers[tokens[nameAt].text] {
		return "", start
	}
	name := strings.TrimPrefix(tokens[nameAt].text, "@")
	prefixEnd := nameAt
	if prefixEnd > start && tokens[prefixEnd-1].text == "~" {
		name = "~" + name
		prefixEnd--
	}
	returnType := start
	for returnType < prefixEnd && csModifiers[tokens[returnType].text] {
		returnType++
	}
	if returnType == prefixEnd {
		if tokens[start].text != "new" && (name == className || name == "~"+className) {
			return name, start
		}
		return "", start
	}
	typeEnd := csSkipType(tokens, pairs, returnType, prefixEnd)
	if typeEnd == returnType {
		return "", start
	}
	if typeEnd != prefixEnd {
		// An explicit interface qualifier follows a complete return type.
		if !csIdentifier(tokens[typeEnd].text) {
			return "", start
		}
		qualifierEnd := csSkipType(tokens, pairs, typeEnd, prefixEnd)
		if qualifierEnd != prefixEnd-1 || tokens[qualifierEnd].text != "." {
			return "", start
		}
	}
	return name, start
}

// After the parameter list, constraints and constructor initializers precede
// either a block, an expression or a semicolon. Braces in an expression may be
// object initializers or lambda bodies, so only its outer semicolon ends it.
func csFunctionEnd(tokens []csToken, pairs []int, close int) (int, int) {
	for i := close + 1; i < len(tokens); i++ {
		switch tokens[i].text {
		case "(", "[":
			if pairs[i] < i {
				return -1, -1
			}
			i = pairs[i]
		case "{":
			return pairs[i], i
		case ";":
			return i, -1
		case "=":
			if i+1 < len(tokens) && tokens[i+1].text == ">" {
				for j := i + 2; j < len(tokens); j++ {
					if tokens[j].text == ";" {
						return j, -1
					}
					if tokens[j].text == "(" || tokens[j].text == "[" || tokens[j].text == "{" {
						if pairs[j] < j {
							return -1, -1
						}
						j = pairs[j]
					}
				}
				return -1, -1
			}
		case "}":
			return -1, -1
		}
	}
	return -1, -1
}

type csConditional struct{ start, end int }

func csConditionals(facts []LineFacts) []csConditional {
	var stack []int
	var ranges []csConditional
	for line, fact := range facts {
		text := strings.TrimSpace(fact.Cleaned)
		if strings.HasPrefix(text, "#if ") || strings.HasPrefix(text, "#if\t") {
			stack = append(stack, line)
		} else if strings.HasPrefix(text, "#endif") && len(stack) > 0 {
			ranges = append(ranges, csConditional{stack[len(stack)-1], line})
			stack = stack[:len(stack)-1]
		}
	}
	return ranges
}

// An expression with #if alternatives occupies the source through its last
// branch. A conditional around a whole declaration does not extend its span.
func csExpressionAlternatives(tokens []csToken, pairs []int, close, end int, ranges []csConditional) int {
	for _, span := range ranges {
		if span.start <= tokens[close].line || span.start > tokens[end].line || span.end <= tokens[end].line {
			continue
		}
		for i := end + 1; i < len(tokens) && tokens[i].line < span.end; i++ {
			if tokens[i].text == ";" {
				end = i
			} else if (tokens[i].text == "(" || tokens[i].text == "[" || tokens[i].text == "{") && pairs[i] > i {
				i = pairs[i]
			}
		}
	}
	return end
}

func (f *Finder) findCSharpFunctions(lines []string, startLine int, filename string) (*FindResult, error) {
	facts := f.sanitizer.CleanLinesFacts(lines)
	tokens, pairs := csTokens(facts)
	conditionals := csConditionals(facts)
	result := &FindResult{Filename: filename}
	type classScope struct {
		name string
		end  int
	}
	var classes []classScope
	braceBoundaries := make(map[int]int)
	blockBodies := make(map[int]bool)
	boundary := 0
	for i := 0; i < len(tokens); i++ {
		for len(classes) > 0 && classes[len(classes)-1].end < i {
			classes = classes[:len(classes)-1]
		}
		switch tokens[i].text {
		case "{":
			braceBoundaries[i] = boundary
			if csPropertyBody(tokens, pairs, boundary, i) {
				blockBodies[i] = true
			}
			if name := csClassName(tokens, pairs, boundary, i); name != "" && pairs[i] >= i {
				classes = append(classes, classScope{name, pairs[i]})
				result.Classes = append(result.Classes, ClassBounds{Name: name, Start: tokens[boundary].line + startLine, End: tokens[pairs[i]].line + startLine})
				blockBodies[i] = true
			}
			if boundary == i {
				blockBodies[i] = true
			} else if boundary < i {
				switch tokens[boundary].text {
				case "if", "else", "for", "foreach", "while", "do", "switch", "try", "catch", "finally", "lock", "using", "fixed", "get", "set", "init", "add", "remove":
					blockBodies[i] = true
				}
			}
			boundary = i + 1
		case "}":
			open := pairs[i]
			if open >= 0 && !blockBodies[open] {
				boundary = braceBoundaries[open]
			} else {
				boundary = i + 1
			}
		case ";":
			boundary = i + 1
		case "(":
			if pairs[i] < i {
				continue
			}
			className := ""
			if len(classes) > 0 {
				className = classes[len(classes)-1].name
			}
			name, start := csFunctionName(tokens, pairs, boundary, i, className)
			if name == "" {
				continue
			}
			end, body := csFunctionEnd(tokens, pairs, pairs[i])
			if end < i {
				continue
			}
			if body < 0 {
				end = csExpressionAlternatives(tokens, pairs, pairs[i], end, conditionals)
			}
			fn := FunctionBounds{Name: name, Start: tokens[start].line + startLine, End: tokens[end].line + startLine, ClassName: className, Scope: className, SignatureLine: tokens[i-1].line + startLine, SignatureName: strings.TrimPrefix(tokens[i-1].text, "@")}
			if f.extractMode {
				fn.Lines = append([]string(nil), lines[fn.Start-startLine:fn.End-startLine+1]...)
			}
			if f.mapMode || f.funcNames[name] {
				result.Functions = append(result.Functions, fn)
			}
			if body >= 0 {
				blockBodies[body] = true
				i = body - 1
			} else {
				i = pairs[i]
				boundary = i + 1
			}
		}
	}
	return result, nil
}

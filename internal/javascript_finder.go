package internal

import (
	"sort"
	"strings"
	"unicode"
)

type jsToken struct {
	text      string
	line, end int
	literal   bool
}

func jsWord(r rune) bool {
	return r == '_' || r == '$' || unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsMark(r) || r == '\u200c' || r == '\u200d'
}

func jsIdentifier(token jsToken) bool {
	if token.literal || token.text == "" {
		return false
	}
	for i, r := range token.text {
		if !jsWord(r) || i == 0 && unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

// LineFacts remain the lexical authority. Recover masked literals as opaque
// tokens for method names and expression endpoints; comments remain absent.
func jsTokens(lines []string, facts []LineFacts) ([]jsToken, []int) {
	raw := []rune(strings.Join(lines, "\n"))
	clean := []rune(strings.Join(cleanedTexts(facts), "\n"))
	var tokens []jsToken
	var pairs []int
	var stack []int
	line := 0
	for i := 0; i < len(clean); {
		start, firstLine, literal := i, line, false
		if unicode.IsSpace(clean[i]) {
			switch {
			case raw[i] == '/' && i+1 < len(raw) && raw[i+1] == '/':
				for i < len(raw) && raw[i] != '\n' {
					i++
				}
			case raw[i] == '/' && i+1 < len(raw) && raw[i+1] == '*':
				i += 2
				for i+1 < len(raw) && !(raw[i] == '*' && raw[i+1] == '/') {
					i++
				}
				i = min(len(raw), i+2)
			case raw[i] == '\'' || raw[i] == '"':
				i, literal = quotedEnd(raw, i), true
			case raw[i] == '`':
				i = templateEnd(raw, i+1)
				if i < 0 {
					i = len(raw)
				}
				literal = true
			case raw[i] == '/':
				// The shared sanitizer classified this slash as a regex literal.
				i++
				inClass := false
				for i < len(raw) && raw[i] != '\n' {
					if raw[i] == '\\' {
						i += 2
						continue
					}
					if raw[i] == '/' && !inClass {
						i++
						for i < len(raw) && isRegexFlag(raw[i]) {
							i++
						}
						break
					}
					if raw[i] == '[' {
						inClass = true
					} else if raw[i] == ']' {
						inClass = false
					}
					i++
				}
				i, literal = min(i, len(raw)), true
			default:
				i++
			}
		} else {
			i++
			if jsWord(clean[start]) || clean[start] == '#' {
				for i < len(clean) && jsWord(clean[i]) {
					i++
				}
			}
		}
		for _, r := range raw[start:i] {
			if r == '\n' {
				line++
			}
		}
		if unicode.IsSpace(clean[start]) && !literal {
			continue
		}
		text := string(clean[start:i])
		if literal {
			text = string(raw[start:i])
		}
		index := len(tokens)
		tokens = append(tokens, jsToken{text, firstLine, line, literal})
		pairs = append(pairs, -1)
		if literal {
			continue
		}
		switch text {
		case "(", "[", "{":
			stack = append(stack, index)
		case ")", "]", "}":
			if len(stack) > 0 {
				open := stack[len(stack)-1]
				if tokens[open].text == map[string]string{")": "(", "]": "[", "}": "{"}[text] {
					pairs[open], pairs[index] = index, open
					stack = stack[:len(stack)-1]
				}
			}
		}
	}
	return tokens, pairs
}

func jsArrowAt(tokens []jsToken, i int) bool {
	return i+1 < len(tokens) && tokens[i].text == "=" && tokens[i+1].text == ">"
}

func jsBindings(tokens []jsToken, pairs []int) map[int]int {
	bindings := map[int]int{}
	for i, token := range tokens {
		if token.text != "const" && token.text != "let" && token.text != "var" {
			continue
		}
		for j := i + 1; j+1 < len(tokens) && jsIdentifier(tokens[j]); {
			if tokens[j+1].text == "," {
				j += 2 // a preceding declarator need not have an initializer
				continue
			}
			if j+2 >= len(tokens) || tokens[j+1].text != "=" {
				break
			}
			bindings[j+2] = j
			end := jsExpressionEnd(tokens, pairs, j+2)
			if end+1 >= len(tokens) || tokens[end+1].text != "," {
				break
			}
			j = end + 2
		}
	}
	return bindings
}

func jsExpressionEnd(tokens []jsToken, pairs []int, first int) int {
	last := first
	for i := first; i < len(tokens); i++ {
		text := tokens[i].text
		if text == ";" || text == "," || text == ")" || text == "]" || text == "}" {
			break
		}
		if i > first && tokens[i].line > tokens[last].end && !jsExpressionContinues(tokens[last], tokens[i]) {
			break
		}
		if text == "function" && !tokens[i].literal {
			open := i + 1
			if open < len(tokens) && tokens[open].text == "*" {
				open++
			}
			if open < len(tokens) && jsIdentifier(tokens[open]) {
				open++
			}
			if open < len(tokens) && tokens[open].text == "(" && pairs[open] > open {
				body := pairs[open] + 1
				if body < len(tokens) && tokens[body].text == "{" && pairs[body] > body {
					i, last = pairs[body], pairs[body]
					continue // a function body may open on the line below its parameters
				}
			}
		}
		if (text == "(" || text == "[" || text == "{") && pairs[i] > i {
			i = pairs[i]
		}
		last = i
	}
	return last
}

func jsExpressionContinues(previous, next jsToken) bool {
	if !previous.literal {
		switch previous.text {
		case "=", ">", "+", "-", "*", "/", "%", "?", ":", ".", ",", "!", "&", "|", "^", "~", "<", "await", "yield", "new", "typeof", "void", "delete", "in", "instanceof":
			return true
		}
	}
	if next.literal {
		return strings.HasPrefix(next.text, "`")
	}
	switch next.text {
	case "(", "[", ".", "?", "+", "-", "*", "/", "%", "&", "|", "^", "<", ">", ":", "=":
		return true
	}
	return false
}

func jsMethod(tokens []jsToken, pairs []int, open int) (string, int, int) {
	if open == 0 || pairs[open] < open {
		return "", 0, 0
	}
	body := pairs[open] + 1
	if body >= len(tokens) || tokens[body].text != "{" || pairs[body] < body {
		return "", 0, 0
	}
	first := open - 1
	name := tokens[first]
	if !jsIdentifier(name) && !name.literal && !strings.HasPrefix(name.text, "#") && (name.text == "" || !unicode.IsDigit([]rune(name.text)[0])) {
		return "", 0, 0
	}
	for first > 0 {
		previous := tokens[first-1].text
		if previous != "async" && previous != "get" && previous != "set" && previous != "static" && previous != "*" {
			break
		}
		first--
	}
	if first == 0 || !strings.Contains("{},;", tokens[first-1].text) {
		return "", 0, 0
	}
	return name.text, first, body
}

func (f *Finder) findJavaScriptFunctions(lines []string, startLine int, filename string) (*FindResult, error) {
	facts := f.sanitizer.CleanLinesFacts(lines)
	tokens, pairs := jsTokens(lines, facts)
	result := &FindResult{Filename: filename, Classes: f.findClassesWithOffset(facts, startLine-1)}
	bindings := jsBindings(tokens, pairs)
	bodies := map[int]bool{}
	type located struct {
		at int
		fn FunctionBounds
	}
	var functions []located
	add := func(name string, first, end, signature int) {
		fn := FunctionBounds{Name: name, Start: tokens[first].line + startLine, End: tokens[end].end + startLine}
		if signature >= 0 {
			fn.SignatureLine, fn.SignatureName = tokens[signature].line+startLine, name
		}
		fn.ClassName = f.findClassForLine(result.Classes, fn.Start)
		fn.Scope = fn.ClassName
		if f.extractMode {
			fn.Lines = append([]string(nil), lines[fn.Start-startLine:fn.End-startLine+1]...)
		}
		if f.mapMode || f.funcNames[name] {
			functions = append(functions, located{first, fn})
		}
	}
	for i, token := range tokens {
		if token.literal || token.text != "function" || i > 0 && tokens[i-1].text == "." {
			continue
		}
		open, name := i+1, ""
		if open < len(tokens) && tokens[open].text == "*" {
			open++
		}
		if open < len(tokens) && jsIdentifier(tokens[open]) {
			name = tokens[open].text
			open++
		}
		if open >= len(tokens) || tokens[open].text != "(" || pairs[open] < open {
			continue
		}
		body := pairs[open] + 1
		if body >= len(tokens) || tokens[body].text != "{" || pairs[body] < body {
			continue
		}
		bodies[body] = true
		first := i
		if i > 0 && tokens[i-1].text == "async" && tokens[i-1].line == token.line {
			first--
		}
		signature := open - 1
		if name == "" {
			signature = -1 // an anonymous variable value has no name( declaration token
			if binding, ok := bindings[first]; ok && jsExpressionEnd(tokens, pairs, first) == pairs[body] {
				name, first = tokens[binding].text, binding
			}
		}
		if name != "" {
			add(name, first, pairs[body], signature)
		}
	}
	for value, binding := range bindings {
		open := value
		if tokens[open].text == "async" && !jsArrowAt(tokens, open+1) {
			open++
		}
		if open >= len(tokens) {
			continue
		}
		arrow := open + 1
		if tokens[open].text == "(" && pairs[open] > open {
			arrow = pairs[open] + 1
		} else if !jsIdentifier(tokens[open]) {
			continue
		}
		if !jsArrowAt(tokens, arrow) || arrow+2 >= len(tokens) {
			continue
		}
		body, end := arrow+2, -1
		if tokens[body].text == "{" {
			bodies[body], end = true, pairs[body]
		} else {
			end = jsExpressionEnd(tokens, pairs, body)
		}
		if end >= body {
			add(tokens[binding].text, binding, end, -1)
		}
	}
	for i := range tokens {
		if jsArrowAt(tokens, i) && i+2 < len(tokens) && tokens[i+2].text == "{" {
			bodies[i+2] = true // anonymous callback bodies are executable scopes too
		}
	}
	type scope struct{ kind string }
	var scopes []scope
	classBodies := map[int]bool{}
	for i, token := range tokens {
		if token.literal || token.text != "class" || i > 0 && tokens[i-1].text == "." {
			continue
		}
		if i+1 >= len(tokens) || !jsIdentifier(tokens[i+1]) && tokens[i+1].text != "{" {
			continue // an object method named class() is not a class declaration
		}
		for j := i + 1; j < len(tokens); j++ {
			if tokens[j].text == "{" {
				classBodies[j] = true
				break
			}
			if tokens[j].text == ";" {
				break
			}
			if tokens[j].text == "(" && pairs[j] > j {
				j = pairs[j]
			}
		}
	}
	for i, token := range tokens {
		switch token.text {
		case "{":
			kind := "block"
			if classBodies[i] {
				kind = "class"
			} else if !bodies[i] && i > 0 {
				switch tokens[i-1].text {
				case "=", "(", "[", ",", ":", "?", "return":
					kind = "object"
				}
			}
			scopes = append(scopes, scope{kind})
		case "}", ")", "]":
			if len(scopes) > 0 {
				scopes = scopes[:len(scopes)-1]
			}
		case "[":
			scopes = append(scopes, scope{"block"})
		case "(":
			if len(scopes) > 0 && scopes[len(scopes)-1].kind != "block" {
				if name, first, body := jsMethod(tokens, pairs, i); name != "" {
					bodies[body] = true
					add(name, first, pairs[body], i-1)
				}
			}
			scopes = append(scopes, scope{"block"})
		}
	}
	sort.SliceStable(functions, func(i, j int) bool { return functions[i].at < functions[j].at })
	for _, function := range functions {
		result.Functions = append(result.Functions, function.fn)
	}
	return result, nil
}

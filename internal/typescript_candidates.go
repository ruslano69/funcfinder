package internal

import (
	"slices"
	"strings"
	"unicode/utf8"
)

// The line scanner remains the normal TS path. Recover headers it cannot
// represent (nested generics, split values, keyword/quoted members) from
// balanced tokens, only in files with an unresolved header-shaped line.
type tsCandidateKey struct {
	name string
	line int
}

var tsRecoveryModifiers = append(append([]string(nil), tsMethodModifiers...), "export", "default", "declare")

func tsNameEnd(s string, i int) int {
	for i < len(s) {
		r, n := utf8.DecodeRuneInString(s[i:])
		if !jsWord(r) {
			break
		}
		i += n
	}
	return i
}
func tsCandidateFile(lines []string, facts []LineFacts, known map[tsCandidateKey]int, offset int) bool {
	for i, fact := range facts {
		if fact.StartState != StateNormal {
			continue
		}
		text := strings.TrimSpace(fact.Cleaned)
		kind := ""
		for {
			end := tsNameEnd(text, 0)
			word := text[:end]
			if word == "function" || word == "const" || word == "let" || word == "var" {
				kind = word
				text = strings.TrimSpace(text[end:])
				if strings.HasPrefix(text, "*") {
					text = strings.TrimSpace(text[1:])
				}
				break
			}
			modifier := false
			for _, m := range tsRecoveryModifiers {
				if word == m {
					modifier = true
					break
				}
			}
			if !modifier || end == len(text) || (text[end] != ' ' && text[end] != '\t') {
				break
			}
			text = strings.TrimSpace(text[end:])
		}
		end := tsNameEnd(text, 0)
		if end == 0 && strings.HasPrefix(text, "(") && tsQuotedNameFirst(lines[i]) {
			return true
		}
		if end == 0 || end > len(text) {
			continue
		}
		name, rest := text[:end], strings.TrimSpace(text[end:])
		if known[tsCandidateKey{name, i + 1 + offset}] > 0 {
			continue
		}
		if kind == "function" && (strings.HasPrefix(rest, "(") || strings.HasPrefix(rest, "<")) {
			return true
		}
		if kind == "const" || kind == "let" || kind == "var" {
			if strings.HasSuffix(rest, "=") {
				for j := i + 1; j < len(facts); j++ {
					next := strings.TrimSpace(facts[j].Cleaned)
					if next == "" {
						continue
					}
					if strings.HasPrefix(next, "(") {
						if tsParenValue(tsJoinedText(facts, j, next)) {
							return true
						}
						break
					}
					if strings.HasPrefix(next, "<") || strings.HasPrefix(next, "async") || strings.HasPrefix(next, "function") {
						return true
					}
					break
				}
			}
			if strings.Contains(name, "$") && strings.Contains(fact.Cleaned, "=>") {
				return true
			}
			if strings.HasPrefix(rest, "=") {
				rhs := strings.TrimSpace(rest[1:])
				if strings.HasPrefix(rhs, "async ") {
					rhs = strings.TrimSpace(rhs[6:])
				}
				n := tsNameEnd(rhs, 0)
				if n > 0 && strings.HasPrefix(strings.TrimSpace(rhs[n:]), "=>") {
					return true
				}
			}
			continue
		}
		if (strings.HasPrefix(rest, "<") && tsCandidateGeneric(tsJoinedText(facts, i, rest))) || (name == "catch" && strings.HasPrefix(rest, "(")) {
			return true
		}
		if strings.HasPrefix(rest, "(") && strings.HasSuffix(strings.TrimSpace(fact.Cleaned), "(") {
			for j := i + 1; j < len(facts); j++ {
				next := strings.TrimSpace(facts[j].Cleaned)
				if next == "" {
					continue
				}
				n := tsNameEnd(next, 0)
				if n > 0 && strings.HasPrefix(strings.TrimSpace(next[n:]), ":") {
					return true
				}
				break
			}
		}
	}
	return false
}

// tsQuotedNameFirst reports whether a raw line, after its modifier words and any leading block comments, starts with a
// quote: the shape of a quoted method name ("name"(...)), which cleaning blanks so that the cleaned line starts with
// "(". A line that starts with "(" for any other reason (a call's continued arguments) is not a candidate.
func tsQuotedNameFirst(raw string) bool {
	text := strings.TrimSpace(raw)
	for {
		if strings.HasPrefix(text, "/*") {
			end := strings.Index(text[2:], "*/")
			if end < 0 {
				return false
			}
			text = strings.TrimSpace(text[end+4:])
			continue
		}
		end := tsNameEnd(text, 0)
		if end == 0 || end == len(text) || (text[end] != ' ' && text[end] != '\t') || !slices.Contains(tsRecoveryModifiers, text[:end]) {
			break
		}
		text = strings.TrimSpace(text[end:])
	}
	return text != "" && (text[0] == '"' || text[0] == '\'')
}

// tsJoinLines is how many following lines the gate reads to see how a header that continues over lines goes on.
const tsJoinLines = 64

// tsJoinedText returns first (the rest of cleaned line i) followed by the cleaned text of the next tsJoinLines lines.
// The gate reads a parenthesis or a type argument list that does not close on its line through it: a call whose
// arguments continue over lines is not a header, which the line alone could not tell.
func tsJoinedText(facts []LineFacts, i int, first string) string {
	var b strings.Builder
	b.WriteString(first)
	for j := i + 1; j < len(facts) && j <= i+tsJoinLines; j++ {
		b.WriteByte(' ')
		b.WriteString(strings.TrimSpace(facts[j].Cleaned))
	}
	return b.String()
}

// tsParenValue reports whether s, which starts with "(", can start an arrow function value: the parenthesis closes and
// "=>" or a return type (":") follows it. A parenthesis that does not close within s is still a candidate.
func tsParenValue(s string) bool {
	depth := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				tail := strings.TrimSpace(s[i+1:])
				return strings.HasPrefix(tail, "=>") || strings.HasPrefix(tail, ":")
			}
		}
	}
	return true
}

func tsCandidateGeneric(s string) bool {
	depth := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '<' {
			depth++
		}
		if s[i] != '>' || (i > 0 && s[i-1] == '=') {
			continue
		}
		depth--
		if depth > 0 {
			continue
		}
		j := skipBlanks(s, i+1)
		if j >= len(s) || s[j] != '(' {
			return false
		}
		parens := 0
		for ; j < len(s); j++ {
			if s[j] == '(' {
				parens++
			}
			if s[j] == ')' {
				parens--
				if parens == 0 {
					tail := strings.TrimSpace(s[j+1:])
					return strings.HasPrefix(tail, ":") || strings.HasPrefix(tail, "{")
				}
			}
		}
		return true
	}
	return true
}

func tsAngleEnd(t []jsToken, pairs []int, i int) int {
	if i >= len(t) || t[i].text != "<" {
		return i
	}
	depth := 0
	for ; i < len(t); i++ {
		switch t[i].text {
		case "<":
			depth++
		case ">":
			if i > 0 && t[i-1].text == "=" {
				continue
			}
			depth--
			if depth == 0 {
				return i + 1
			}
		case "(", "[", "{":
			if pairs[i] > i {
				i = pairs[i]
			}
		case ";":
			return -1
		}
	}
	return -1
}

// The first object literal after ':' is a return type; a later brace is the
// body. Brackets, function types and generic arguments are skipped whole.
func tsAfterParameters(t []jsToken, pairs []int, i int) int {
	if i >= len(t) {
		return -1
	}
	if t[i].text != ":" {
		if t[i].text == "{" || t[i].text == ";" {
			return i
		}
		return -1
	}
	atom := true
	for i++; i < len(t); i++ {
		switch t[i].text {
		case "{":
			if atom && pairs[i] > i {
				i = pairs[i]
				atom = false
			} else {
				return i
			}
		case "(", "[":
			if pairs[i] <= i {
				return -1
			}
			i = pairs[i]
			atom = false
		case "<":
			end := tsAngleEnd(t, pairs, i)
			if end < 0 {
				return -1
			}
			i = end - 1
			atom = false
		case ";":
			return i
		case "}", "=":
			return -1
		case "|", "&", "?", ":":
			atom = true
		default:
			atom = false
		}
	}
	return -1
}

func tsFunctionShape(t []jsToken, pairs []int, i int) (name, body int) {
	name = -1
	i++
	if i < len(t) && t[i].text == "*" {
		i++
	}
	if i < len(t) && jsIdentifier(t[i]) {
		name = i
		i++
	}
	i = tsAngleEnd(t, pairs, i)
	if i < 0 || i >= len(t) || t[i].text != "(" || pairs[i] <= i {
		return name, -1
	}
	return name, tsAfterParameters(t, pairs, pairs[i]+1)
}
func tsArrowBody(t []jsToken, pairs []int, i int) int {
	if i >= len(t) {
		return -1
	}
	if t[i].text == "async" && !jsArrowAt(t, i+1) {
		i++
	}
	i = tsAngleEnd(t, pairs, i)
	if i < 0 || i >= len(t) {
		return -1
	}
	if t[i].text == "(" && pairs[i] > i {
		i = pairs[i] + 1
	} else if jsIdentifier(t[i]) {
		i++
	} else {
		return -1
	}
	if i < len(t) && t[i].text == ":" {
		for i++; i < len(t); i++ {
			if jsArrowAt(t, i) {
				break
			}
			if t[i].text == "(" || t[i].text == "[" || t[i].text == "{" {
				if pairs[i] <= i {
					return -1
				}
				i = pairs[i]
			}
			if t[i].text == "<" {
				end := tsAngleEnd(t, pairs, i)
				if end < 0 {
					return -1
				}
				i = end - 1
			}
			if t[i].text == ";" || t[i].text == "}" {
				return -1
			}
		}
	}
	if jsArrowAt(t, i) && i+2 < len(t) {
		return i + 2
	}
	return -1
}
func tsArrowEnd(t []jsToken, pairs []int, body int) int {
	for {
		if t[body].text == "{" {
			return pairs[body]
		}
		next := tsArrowBody(t, pairs, body)
		if next < 0 {
			return jsExpressionEnd(t, pairs, body)
		}
		body = next
	}
}

func (f *Finder) recoverTSFunctions(lines []string, facts []LineFacts, offset int, classes []ClassBounds, functions []FunctionBounds) []FunctionBounds {
	known := map[tsCandidateKey]int{}
	for _, fn := range functions {
		known[tsCandidateKey{fn.Name, fn.Start}]++
	}
	if !tsCandidateFile(lines, facts, known, offset) {
		return functions
	}
	t, pairs := jsTokens(lines, facts)
	bodies := map[int]bool{}
	classBodies := map[int]bool{}
	add := func(name string, first, end int) {
		if end < first || end >= len(t) || (!f.mapMode && !f.funcNames[name]) {
			return
		}
		key := tsCandidateKey{name, t[first].line + 1 + offset}
		if known[key] > 0 {
			known[key]--
			return
		}
		class := f.findClassForLine(classes, key.line-1)
		fn := FunctionBounds{Name: name, Start: key.line, End: t[end].end + 1 + offset, ClassName: class, Scope: class}
		if f.extractMode {
			fn.Lines = append([]string(nil), lines[fn.Start-offset-1:fn.End-offset]...)
		}
		if strings.HasPrefix(name, "\"") || strings.HasPrefix(name, "'") {
			for j := 0; j < len(functions); j++ {
				if functions[j].Start == fn.Start && functions[j].Name == "async" {
					functions = append(functions[:j], functions[j+1:]...)
					j--
				}
			}
		}
		functions = append(functions, fn)
	}
	for i, token := range t {
		if token.literal {
			continue
		}
		if token.text == "function" {
			name, body := tsFunctionShape(t, pairs, i)
			if body < 0 {
				continue
			}
			bodies[body] = true
			prev := i - 1
			if prev >= 0 && t[prev].text == "async" {
				prev--
			}
			if name >= 0 && (prev < 0 || strings.Contains("{};", t[prev].text) || t[prev].text == "export" || t[prev].text == "default") {
				add(t[name].text, i, func() int {
					if t[body].text == ";" {
						return body
					}
					return pairs[body]
				}())
			}
		}
		if token.text == "class" || token.text == "interface" {
			for j := i + 1; j < len(t); j++ {
				if t[j].text == "<" {
					end := tsAngleEnd(t, pairs, j)
					if end < 0 {
						break
					}
					j = end - 1
					continue
				}
				if t[j].text == "(" && pairs[j] > j {
					j = pairs[j]
					continue
				}
				if t[j].text == "{" {
					classBodies[j] = true
					break
				}
				if t[j].text == ";" {
					break
				}
			}
		}
		if token.text != "const" && token.text != "let" && token.text != "var" {
			continue
		}
		name := i + 1
		if name >= len(t) || !jsIdentifier(t[name]) {
			continue
		}
		for j := name + 1; j < len(t); j++ {
			if t[j].text == "=" && !jsArrowAt(t, j) {
				value := j + 1
				body := tsArrowBody(t, pairs, value)
				if body >= 0 {
					bodies[body] = true
					end := tsArrowEnd(t, pairs, body)
					if tsDirectValue(t, end) {
						add(t[name].text, name, end)
					}
				} else if value < len(t) && t[value].text == "function" {
					_, b := tsFunctionShape(t, pairs, value)
					if b >= 0 && t[b].text == "{" {
						bodies[b] = true
						if tsDirectValue(t, pairs[b]) {
							add(t[name].text, name, pairs[b])
						}
					}
				}
				break
			}
			if t[j].text == "(" || t[j].text == "[" || t[j].text == "{" {
				if pairs[j] <= j {
					break
				}
				j = pairs[j]
				continue
			}
			if t[j].text == "<" {
				end := tsAngleEnd(t, pairs, j)
				if end < 0 {
					break
				}
				j = end - 1
				continue
			}
			if t[j].text == ";" || t[j].text == "," || t[j].text == ")" || t[j].text == "}" {
				break
			}
		}
	}
	scopes := []string{}
	for i, token := range t {
		if len(scopes) > 0 && (scopes[len(scopes)-1] == "class" || scopes[len(scopes)-1] == "object") && (jsIdentifier(token) || (token.literal && (strings.HasPrefix(token.text, "\"") || strings.HasPrefix(token.text, "'")))) {
			first := i
			for first > 0 {
				modifier := t[first-1].text
				found := modifier == "*"
				for _, m := range tsMethodModifiers {
					found = found || m == modifier
				}
				if !found {
					break
				}
				first--
			}
			if first > 0 && strings.Contains("{};,", t[first-1].text) {
				open := i + 1
				if open < len(t) && t[open].text == "?" {
					open++
				}
				open = tsAngleEnd(t, pairs, open)
				if open >= 0 && open < len(t) && t[open].text == "(" && pairs[open] > open {
					body := tsAfterParameters(t, pairs, pairs[open]+1)
					if body >= 0 {
						if t[body].text == "{" && pairs[body] > body {
							bodies[body] = true
							add(token.text, first, pairs[body])
						} else if t[body].text == ";" && scopes[len(scopes)-1] == "class" && token.text != "new" {
							add(token.text, first, body)
						}
					}
				}
			}
		}
		if token.literal {
			continue
		}
		switch token.text {
		case "{":
			kind := "block"
			if classBodies[i] {
				kind = "class"
			} else if !bodies[i] && i > 0 {
				switch t[i-1].text {
				case "=", "(", "[", ",", ":", "?", "return":
					kind = "object"
				}
			}
			if kind == "object" && i > 0 && t[i-1].text == ":" {
				for j := i - 2; j >= 0; j-- {
					if t[j].text == "case" || t[j].text == "default" {
						kind = "block"
						break
					}
					if strings.Contains("{};:", t[j].text) {
						break
					}
				}
			}
			scopes = append(scopes, kind)
		case "(", "[":
			scopes = append(scopes, "block")
		case "}", ")", "]":
			if len(scopes) > 0 {
				scopes = scopes[:len(scopes)-1]
			}
		}
	}
	return functions
}

func tsDirectValue(t []jsToken, end int) bool {
	if end < 0 {
		return false
	}
	if end+1 >= len(t) {
		return true
	}
	switch t[end+1].text {
	case "as", "satisfies", "(", ".", "[", "!":
		return false
	}
	return true
}

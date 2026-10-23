package internal

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"
)

func scriptObjectTokens(src []byte, language string) ([]rustObjectToken, error) {
	if !utf8.Valid(src) {
		return nil, fmt.Errorf("source must be UTF-8")
	}
	raw := []rune(string(src))
	var positions []int
	for offset := range string(src) {
		positions = append(positions, offset)
	}
	positions = append(positions, len(src))
	var tokens []rustObjectToken
	var stack []int
	control := map[int]bool{}
	blocks := map[int]bool{}
	operand := true
	for i := 0; i < len(raw); {
		if raw[i] == '\ufeff' && i == 0 || unicode.IsSpace(raw[i]) {
			i++
			continue
		}
		if i == 0 && raw[i] == '#' && i+1 < len(raw) && raw[i+1] == '!' {
			for i < len(raw) && raw[i] != '\n' {
				i++
			}
			continue
		}
		if raw[i] == '/' && i+1 < len(raw) && raw[i+1] == '/' {
			for i < len(raw) && raw[i] != '\n' {
				i++
			}
			continue
		}
		if raw[i] == '/' && i+1 < len(raw) && raw[i+1] == '*' {
			i += 2
			for i+1 < len(raw) && !(raw[i] == '*' && raw[i+1] == '/') {
				i++
			}
			if i+1 >= len(raw) {
				return nil, fmt.Errorf("unclosed comment")
			}
			i += 2
			continue
		}
		start, literal := i, false
		switch raw[i] {
		case '\'', '"':
			i = quotedEnd(raw, i)
			if i <= start+1 || raw[i-1] != raw[start] {
				return nil, fmt.Errorf("unclosed quoted literal")
			}
			escapes := 0
			for j := i - 2; j >= start && raw[j] == '\\'; j-- {
				escapes++
			}
			if escapes%2 != 0 {
				return nil, fmt.Errorf("unclosed escaped literal")
			}
			literal = true
		case '`':
			i = templateEnd(raw, i+1)
			if i < 0 {
				return nil, fmt.Errorf("unclosed template literal")
			}
			literal = true
		case '/':
			if operand {
				i = regexLiteralEnd(raw, i, true)
				if i < 0 {
					return nil, fmt.Errorf("unclosed regular expression")
				}
				literal = true
			} else {
				i++
			}
		case '<':
			if operand {
				if end := scriptJSXLiteral(raw, i); end > i {
					i, literal = end, true
				}
			}
			if !literal {
				i++
			}
		default:
			i++
			if jsWord(raw[start]) || raw[start] == '#' {
				for i < len(raw) && jsWord(raw[i]) {
					i++
				}
			}
		}
		text := string(raw[start:i])
		index := len(tokens)
		tokens = append(tokens, rustObjectToken{text: text, start: positions[start], end: positions[i], pair: -1, literal: literal})
		if literal {
			operand = false
			continue
		}
		switch text {
		case "(", "[", "{":
			if len(tokens) > 1 {
				previous := tokens[index-1].text
				if text == "(" {
					control[index] = previous == "if" || previous == "while" || previous == "for" || previous == "with" || previous == "switch" || previous == "catch"
				}
				if text == "{" {
					blocks[index] = previous == ")" || previous == ">" || previous == "else" || previous == "try" || previous == "finally" || previous == "do"
				}
			}
			stack = append(stack, index)
			operand = true
		case ")", "]", "}":
			if len(stack) == 0 || tokens[stack[len(stack)-1]].text != map[string]string{")": "(", "]": "[", "}": "{"}[text] {
				return nil, fmt.Errorf("mismatched delimiter at byte %d", positions[start])
			}
			open := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			tokens[open].pair, tokens[index].pair = index, open
			operand = text == ")" && control[open] || text == "}" && blocks[open]
		default:
			operand = strings.Contains("=,:;!?~+-*/%&|^<>", text) || text == "return" || text == "throw" || text == "case" || text == "yield" || text == "await" || text == "typeof" || text == "void" || text == "delete" || text == "in" || text == "instanceof"
			if text == "." {
				operand = false
			}
		}
	}
	if len(stack) > 0 {
		return nil, fmt.Errorf("unclosed delimiter at byte %d", tokens[stack[0]].start)
	}
	return tokens, nil
}
func scriptExpressionEnd(tokens []jsToken, pairs []int, first, hi int, jsx bool) int {
	for {
		body := tsArrowBody(tokens, pairs, first)
		if body <= first || body >= hi {
			break
		}
		first = body
	}
	typePosition := false
	last := first
	for i := first; i < hi; i++ {
		text := tokens[i].text
		if text == ";" || text == "," || text == ")" || text == "]" || text == "}" {
			break
		}
		if i > first && tokens[i].line > tokens[last].end && !jsExpressionContinues(tokens[last], tokens[i]) {
			break
		}
		if text == "<" && jsx {
			end := scriptJSXEnd(tokens, pairs, i, hi)
			if end >= i {
				i, last = end, end
				continue
			}
		}
		if text == "as" || text == "satisfies" {
			typePosition = true
		}
		if text == "<" {
			end := tsAngleEnd(tokens, pairs, i)
			if end > i && end < hi && (typePosition || tokens[end].text == "(" || tokens[end].text == ".") {
				i, last = end-1, end-1
				continue
			}
		}
		if text == "function" && !tokens[i].literal {
			_, body := scriptFunctionShape(tokens, pairs, i)
			if body >= 0 && tokens[body].text == "{" {
				i, last = pairs[body], pairs[body]
				continue
			}
		}
		if (text == "(" || text == "[" || text == "{") && pairs[i] > i {
			i = pairs[i]
		}
		last = i
	}
	return last
}
func scriptJSXEnd(tokens []jsToken, pairs []int, first, hi int) int {
	depth := 0
	for i := first; i < hi; i++ {
		if tokens[i].text == "{" && pairs[i] > i {
			i = pairs[i]
			continue
		}
		if tokens[i].text != "<" {
			continue
		}
		closing := i+1 < hi && tokens[i+1].text == "/"
		angle, j := 0, i+1
		for ; j < hi; j++ {
			if tokens[j].literal {
				continue
			}
			if tokens[j].text == "{" && pairs[j] > j {
				j = pairs[j]
				continue
			}
			if tokens[j].text == "<" {
				angle++
			}
			if tokens[j].text == ">" {
				if angle > 0 {
					angle--
				} else {
					break
				}
			}
		}
		if j >= hi {
			return -1
		}
		if closing {
			depth--
		} else if j == i+1 || tokens[j-1].text != "/" {
			depth++
		}
		if depth == 0 {
			return j
		}
		i = j
	}
	return -1
}

func scriptJSXLiteral(raw []rune, first int) int {return scriptJSXLiteralDepth(raw,first,0)}

func scriptJSXLiteralDepth(raw []rune, first, depth int) int {
	if depth > 64 {
		return -1
	}
	if first+1 >= len(raw) || raw[first] != '<' {
		return -1
	}
	j := first + 1
	if raw[j] != '>' && !unicode.IsLetter(raw[j]) && raw[j] != '_' && raw[j] != '$' {
		return -1
	}
	nameStart := j
	for j < len(raw) && (jsWord(raw[j]) || raw[j] == '.' || raw[j] == '-' || raw[j] == ':') {
		j++
	}
	name := string(raw[nameStart:j])
	nesting := 0
	for i := first; i < len(raw); {
		if raw[i] == '{' {
			end := scriptJSXExprEnd(raw, i+1, depth+1)
			if end < 0 {
				return -1
			}
			i = end
			continue
		}
		if raw[i] != '<' {
			i++
			continue
		}
		closing := i+1 < len(raw) && raw[i+1] == '/'
		j, angle := i+1, 0
		for j < len(raw) {
			if raw[j] == '\'' || raw[j] == '"' {
				j = quotedEnd(raw, j)
				continue
			}
			if raw[j] == '{' {
				end := scriptJSXExprEnd(raw, j+1, depth+1)
				if end < 0 {
					return -1
				}
				j = end
				continue
			}
			if raw[j] == '<' {
				angle++
			}
			if raw[j] == '>' {
				if angle > 0 {
					angle--
				} else {
					break
				}
			}
			j++
		}
		if j >= len(raw) {
			return -1
		}
		last := j - 1
		for last > i && unicode.IsSpace(raw[last]) {
			last--
		}
		if i == first && name != "" && raw[last] != '/' && !strings.Contains(string(raw[j+1:]), "</"+name) {
			return -1
		}
		if closing {
			nesting--
		} else if raw[last] != '/' {
			nesting++
		}
		if nesting == 0 {
			return j + 1
		}
		i = j + 1
	}
	return -1
}

func scriptJSXExprEnd(raw []rune, first, depth int) int {
	if depth > 64 {
		return -1
	}
	braces, operand := 0, true
	for i := first; i < len(raw); {
		if unicode.IsSpace(raw[i]) {
			i++
			continue
		}
		if raw[i] == '/' && i+1 < len(raw) && raw[i+1] == '/' {
			for i < len(raw) && raw[i] != '\n' {
				i++
			}
			continue
		}
		if raw[i] == '/' && i+1 < len(raw) && raw[i+1] == '*' {
			i += 2
			for i+1 < len(raw) && !(raw[i] == '*' && raw[i+1] == '/') {
				i++
			}
			if i+1 >= len(raw) {
				return -1
			}
			i += 2
			continue
		}
		if raw[i] == '\'' || raw[i] == '"' {
			i = quotedEnd(raw, i)
			operand = false
			continue
		}
		if raw[i] == '`' {
			i = templateEnd(raw, i+1)
			if i < 0 {
				return -1
			}
			operand = false
			continue
		}
		if raw[i] == '<' && operand {
			if end := scriptJSXLiteralDepth(raw, i, depth+1); end > i {
				i = end
				operand = false
				continue
			}
		}
		if raw[i] == '/' && operand {
			end := regexLiteralEnd(raw, i, true)
			if end < 0 {
				return -1
			}
			i = end
			operand = false
			continue
		}
		if raw[i] == '{' {
			braces++
			operand = true
			i++
			continue
		}
		if raw[i] == '}' {
			if braces == 0 {
				return i + 1
			}
			braces--
			operand = false
			i++
			continue
		}
		if jsWord(raw[i]) {
			start := i
			for i < len(raw) && jsWord(raw[i]) {
				i++
			}
			word := string(raw[start:i])
			operand = word == "return" || word == "throw" || word == "typeof" || word == "void" || word == "yield" || word == "await" || word == "case"
			continue
		}
		operand = strings.ContainsRune("(=,:;!?~+-*/%&|^<>", raw[i])
		i++
	}
	return -1
}

func scriptAfterParameters(t []jsToken, pairs []int, i int) int {
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
		case "=":
			if jsArrowAt(t, i) {
				i++
				atom = true
				continue
			}
			return -1
		case "}":
			return -1
		case "|", "&", "?", ":":
			atom = true
		default:
			atom = false
		}
	}
	return -1
}

func scriptFunctionShape(t []jsToken, pairs []int, i int) (name, body int) {
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
	return name, scriptAfterParameters(t, pairs, pairs[i]+1)
}
func scriptCodeItems(path string, src []byte, scope, context, language string) ([]rustObjectItem, error) {
	lexed, err := scriptObjectTokens(src, language)
	if err != nil {
		return nil, objectError("invalid_source", err.Error())
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, objectError("invalid_path", err.Error())
	}
	abs = filepath.Clean(abs)
	starts := []int{0}
	for i, b := range src {
		if b == '\n' {
			starts = append(starts, i+1)
		}
	}
	t, pairs := make([]jsToken, len(lexed)), make([]int, len(lexed))
	for i, token := range lexed {
		line, _ := rustBytePosition(starts, token.start)
		end, _ := rustBytePosition(starts, token.end-1)
		t[i] = jsToken{token.text, line - 1, end - 1, token.literal}
		pairs[i] = token.pair
	}
	jsx := strings.HasSuffix(strings.ToLower(path), ".tsx") || strings.HasSuffix(strings.ToLower(path), ".jsx")
	revision := codeRevision(src)
	var items []rustObjectItem
	add := func(first, last int, kind, name, key, attrs, scope, context string, depth int) {
		item := rustObjectItem{start: lexed[first].start, end: lexed[last].end, scope: scope, context: context, key: key, attributes: attrs, depth: depth}
		if kind != "" {
			line, column := rustBytePosition(starts, item.start)
			endLine, _ := rustBytePosition(starts, item.end-1)
			text := strings.ReplaceAll(string(src[item.start:item.end]), "\r\n", "\n")
			if strings.HasPrefix(context, "binding:") {
				text = strings.TrimPrefix(context, "binding:") + " " + text + ";"
			}
			item.object = CodeObject{Protocol: "funcfinder.code/v1", Path: abs, Language: language, Kind: kind, Name: name, Line: line, Column: column, EndLine: endLine, Revision: revision, Source: text}
			item.object.ID = codeObjectID(abs, language+":"+kind, name, line, column)
		}
		items = append(items, item)
	}
	isValue := func(first, last int) bool {
		at := first
		if at < len(t) && t[at].text == "async" && at+1 < len(t) && t[at+1].text == "function" {
			at++
		}
		if at < len(t) && t[at].text == "function" {
			_, body := scriptFunctionShape(t, pairs, at)
			return body >= 0 && t[body].text == "{" && pairs[body] == last
		}
		return tsArrowBody(t, pairs, first) >= 0
	}
	decorated := func(first, hi int) int {
		for first < hi && t[first].text == "@" {
			first++
			if first < hi && jsIdentifier(t[first]) {
				first++
			}
			for first+1 < hi && t[first].text == "." && jsIdentifier(t[first+1]) {
				first += 2
			}
			if first < hi && t[first].text == "(" {
				first = pairs[first] + 1
			}
		}
		return first
	}
	var walk func(int, int, string, string, int) error
	walk = func(lo, hi int, scope, context string, depth int) error {
		if depth > 128 {
			return objectError("invalid_source", "JS/TS scope nesting exceeds 128")
		}
		blockNumber := 0
		for first := lo; first < hi; {
			if t[first].text == ";" || context == "object" && t[first].text == "," {
				first++
				continue
			}
			if t[first].text == "import" && first+1 < hi && t[first+1].text != "(" && t[first+1].text != "." {
				last := first
				for j := first + 1; j < hi; j++ {
					if pairs[j] > j {
						j = pairs[j]
					}
					last = j
					if t[j].literal || t[j].text == ";" {
						break
					}
				}
				if last+1 < hi && t[last+1].text == ";" {
					last++
				}
				add(first, last, "", "", scope+"/import:"+rustTokenKey(lexed, first, last+1), "", scope, context, depth)
				first = last + 1
				continue
			}
			// Re-exports own their braces; names inside are not declarations.
			if t[first].text == "export" && first+1 < hi && (t[first+1].text == "{" || t[first+1].text == "*" || t[first+1].text == "type" && first+2 < hi && (t[first+2].text == "{" || t[first+2].text == "*")) {
				last := scriptExpressionEnd(t, pairs, first, hi, jsx)
				if last+1 < hi && t[last+1].text == ";" {
					last++
				}
				add(first, last, "", "", scope+"/export:"+rustTokenKey(lexed, first, last+1), "", scope, context, depth)
				first = last + 1
				continue
			}
			head := decorated(first, hi)
			for head < hi && (t[head].text == "export" || t[head].text == "default" || t[head].text == "declare" || t[head].text == "abstract") {
				head = decorated(head+1, hi)
			}
			if head >= hi {
				return objectError("invalid_source", "incomplete declaration prefix")
			}
			word := t[head].text
			functionAt := head
			if word == "async" && head+1 < hi && t[head+1].text == "function" {
				functionAt++
			}
			if t[functionAt].text == "function" && (context == "module" || strings.HasPrefix(context, "binding:")) {
				nameAt, body := scriptFunctionShape(t, pairs, functionAt)
				if nameAt < 0 || body < 0 || body >= hi {
					return objectError("invalid_source", "anonymous/incomplete function declarations are not objects")
				}
				last := body
				category := "signature"
				if t[body].text == "{" {
					last = pairs[body]
					category = "body"
				}
				if last < first || last >= hi {
					return objectError("invalid_source", "function crosses its scope")
				}
				name := t[nameAt].text
				key := scope + "/function:" + name + ":" + category
				if language == "ts" {
					key += rustTokenKey(lexed, nameAt+1, body)
				}
				add(first, last, "function", name, key, rustTokenKey(lexed, first, nameAt), scope, "module", depth)
				first = last + 1
				continue
			}
			typeAt := head
			if word == "const" && head+1 < hi && t[head+1].text == "enum" {
				typeAt++
			}
			typeWord := t[typeAt].text
			if typeWord == "class" || typeWord == "interface" || typeWord == "enum" || typeWord == "namespace" || typeWord == "module" {
				nameAt := typeAt + 1
				if nameAt >= hi || !jsIdentifier(t[nameAt]) && !t[nameAt].literal {
					return objectError("invalid_source", "anonymous/computed type containers are unsupported")
				}
				body := -1
				for j := nameAt + 1; j < hi; j++ {
					if t[j].text == "<" {
						end := tsAngleEnd(t, pairs, j)
						if end < 0 {
							break
						}
						j = end - 1
						continue
					}
					if t[j].text == "(" || t[j].text == "[" {
						j = pairs[j]
						continue
					}
					if t[j].text == "{" {
						body = j
						break
					}
					if t[j].text == ";" {
						break
					}
				}
				if body < 0 || pairs[body] >= hi {
					return objectError("invalid_source", "incomplete type/namespace declaration")
				}
				last, name, kind, next := pairs[body], t[nameAt].text, "type", "class"
				if typeWord == "interface" {
					next = "interface"
				}
				if typeWord == "namespace" || typeWord == "module" {
					kind, next = "", "module"
				}
				key := scope + "/" + typeWord + ":" + name
				add(first, last, kind, name, key, rustTokenKey(lexed, first, nameAt), scope, context, depth)
				if typeWord != "enum" {
					if err := walk(body+1, last, key, next, depth+1); err != nil {
						return err
					}
				}
				first = last + 1
				continue
			}
			if word == "type" && language == "ts" && head+1 < hi && jsIdentifier(t[head+1]) {
				equal := head + 2
				if equal < hi && t[equal].text == "<" {
					equal = tsAngleEnd(t, pairs, equal)
				}
				if equal < 0 || equal >= hi || t[equal].text != "=" {
					return objectError("invalid_source", "unsupported type alias")
				}
				last := equal + 1
				for j := equal + 1; j < hi; j++ {
					if t[j].text == ";" {
						last = j
						break
					}
					if j > equal+1 && t[j].line > t[last].end && !t[j].literal && (t[j].text == "export" || t[j].text == "const" || t[j].text == "let" || t[j].text == "var" || t[j].text == "function" || t[j].text == "class" || t[j].text == "interface" || t[j].text == "type" || t[j].text == "namespace" || t[j].text == "import") {
						break
					}
					if t[j].text == "<" {
						end := tsAngleEnd(t, pairs, j)
						if end > j {
							j = end - 1
							last = j
							continue
						}
					}
					if pairs[j] > j {
						j = pairs[j]
					}
					last = j
				}
				name := t[head+1].text
				add(first, last, "type", name, scope+"/alias:"+name, rustTokenKey(lexed, first, head+1), scope, context, depth)
				first = last + 1
				continue
			}
			if word == "const" || word == "let" || word == "var" {
				binding := head + 1
				if binding >= hi {
					return objectError("invalid_source", "empty variable declaration")
				}
				for binding < hi {
					patternEnd := binding
					name := ""
					if jsIdentifier(t[binding]) {
						name = t[binding].text
					} else if t[binding].text == "{" || t[binding].text == "[" {
						patternEnd = pairs[binding]
					} else {
						return objectError("invalid_source", "unsupported variable binding")
					}
					equal := patternEnd + 1
					for equal < hi && t[equal].text != ";" && t[equal].text != "," {
						if t[equal].text == "=" && (equal+1 >= hi || t[equal+1].text != ">") {
							break
						}
						if t[equal].text == "<" {
							end := tsAngleEnd(t, pairs, equal)
							if end > equal {
								equal = end
								continue
							}
						}
						if pairs[equal] > equal {
							equal = pairs[equal]
						}
						if equal+1 < hi && t[equal+1].line > t[equal].end && (t[equal+1].text == "export" || t[equal+1].text == "const" || t[equal+1].text == "function") {
							equal++
							break
						}
						equal++
					}
					value, last, kind := equal+1, equal-1, ""
					objectBody := -1
					if equal < hi && t[equal].text == "=" {
						if value >= hi {
							return objectError("invalid_source", "empty variable initializer")
						}
						last = scriptExpressionEnd(t, pairs, value, hi, jsx)
						if name != "" && isValue(value, last) {
							kind = "function"
						}
						if name != "" && t[value].text == "{" && pairs[value] == last {
							objectBody = value
						}
					}
					if last < binding {
						return objectError("invalid_source", "incomplete variable declaration")
					}
					key := scope + "/binding:" + name
					if name == "" || kind == "" && objectBody < 0 {
						key = scope + "/opaque-binding:" + rustTokenKey(lexed, binding, last+1)
					}
					add(binding, last, kind, name, key, "binding:"+word, scope, "binding:"+word, depth)
					if objectBody >= 0 {
						if err := walk(objectBody+1, last, key, "object", depth+1); err != nil {
							return err
						}
					}
					if last+1 < hi && t[last+1].text == "," {
						binding = last + 2
						continue
					}
					first = last + 1
					if first < hi && t[first].text == ";" {
						first++
					}
					break
				}
				continue
			}
			if context == "class" || context == "interface" || context == "object" {
				nameAt := decorated(first, hi)
				for nameAt+1 < hi {
					modifier := t[nameAt].text
					if modifier != "public" && modifier != "private" && modifier != "protected" && modifier != "static" && modifier != "readonly" && modifier != "abstract" && modifier != "declare" && modifier != "override" && modifier != "async" && modifier != "get" && modifier != "set" && modifier != "*" {
						break
					}
					if t[nameAt+1].text == "(" || t[nameAt+1].text == "=" || t[nameAt+1].text == ":" || t[nameAt+1].text == ";" {
						break
					}
					nameAt++
				}
				if nameAt < hi && (jsIdentifier(t[nameAt]) || t[nameAt].literal || strings.HasPrefix(t[nameAt].text, "#")) {
					name, open := t[nameAt].text, nameAt+1
					if open < hi && (t[open].text == "?" || t[open].text == "!") {
						open++
					}
					if open < hi && t[open].text == "<" {
						open = tsAngleEnd(t, pairs, open)
					}
					if open >= 0 && open < hi && t[open].text == "(" {
						body := scriptAfterParameters(t, pairs, pairs[open]+1)
						if body >= 0 && body < hi {
							last, category := body, "signature"
							if t[body].text == "{" {
								last, category = pairs[body], "body"
							}
							attrs := rustTokenKey(lexed, first, nameAt)
							key := scope + "/method:" + name + ":" + attrs + ":" + category
							if language == "ts" {
								key += rustTokenKey(lexed, open, body)
							}
							add(first, last, "function", name, key, attrs, scope, context, depth)
							first = last + 1
							continue
						}
					}
					equal := nameAt + 1
					if context == "object" && equal < hi && t[equal].text == ":" {
						equal++
					} else {
						for equal < hi && t[equal].text != ";" && t[equal].text != "," {
							if t[equal].text == "=" && (equal+1 >= hi || t[equal+1].text != ">") {
								break
							}
							if t[equal].text == "<" {
								end := tsAngleEnd(t, pairs, equal)
								if end > equal {
									equal = end
									continue
								}
							}
							if pairs[equal] > equal {
								equal = pairs[equal]
							}
							if equal+1 < hi && t[equal+1].line > t[equal].end && !jsExpressionContinues(t[equal], t[equal+1]) {
								equal++
								break
							}
							equal++
						}
						if equal < hi && t[equal].text == "=" {
							equal++
						} else {
							equal = -1
						}
					}
					if equal >= 0 && equal < hi {
						last := scriptExpressionEnd(t, pairs, equal, hi, jsx)
						kind := ""
						if isValue(equal, last) {
							kind = "function"
						}
						objectBody := -1
						if t[equal].text == "{" && pairs[equal] == last {
							objectBody = equal
						}
						if context != "object" && last+1 < hi && t[last+1].text == ";" {
							last++
						}
						key := scope + "/field:" + name
						if kind == "" && objectBody < 0 {
							key = scope + "/opaque:" + rustTokenKey(lexed, first, last+1)
						}
						add(first, last, kind, name, key, rustTokenKey(lexed, first, nameAt), scope, context, depth)
						if objectBody >= 0 {
							if err := walk(objectBody+1, pairs[objectBody], key, "object", depth+1); err != nil {
								return err
							}
						}
						first = last + 1
						continue
					}
				}
			}
			if context == "module" && (word == "global" || word == "{" || word == "if" || word == "else" || word == "try" || word == "catch" || word == "finally" || word == "for" || word == "while" || word == "do" || word == "switch") {
				body := head
				for body < hi && t[body].text != "{" && t[body].text != ";" {
					if pairs[body] > body {
						body = pairs[body]
					}
					body++
				}
				if body < hi && t[body].text == "{" {
					blockNumber++
					key := scope + fmt.Sprintf("/block:%d:", blockNumber) + rustTokenKey(lexed, first, body)
					add(first, pairs[body], "", "", key, "", scope, context, depth)
					if err := walk(body+1, pairs[body], key, "module", depth+1); err != nil {
						return err
					}
					first = pairs[body] + 1
					continue
				}
			}
			// Direct assignment-owned object literals are containers, not call callbacks.
			if context == "module" {
				equal := first
				for equal < hi && t[equal].text != "=" && t[equal].text != ";" {
					if pairs[equal] > equal {
						equal = pairs[equal]
					}
					if equal+1 < hi && t[equal+1].line > t[equal].end && !jsExpressionContinues(t[equal], t[equal+1]) {
						break
					}
					equal++
				}
				if equal+1 < hi && t[equal].text == "=" && t[equal+1].text == "{" {
					body := equal + 1
					last := scriptExpressionEnd(t, pairs, first, hi, jsx)
					if pairs[body] == last {
						key := scope + "/assignment:" + rustTokenKey(lexed, first, body)
						add(first, last, "", "", key, "", scope, context, depth)
						if err := walk(body+1, last, key, "object", depth+1); err != nil {
							return err
						}
						first = last + 1
						continue
					}
				}
			}
			last := scriptExpressionEnd(t, pairs, first, hi, jsx)
			if last < first {
				return objectError("invalid_source", "unsupported declaration boundary")
			}
			if last+1 < hi && t[last+1].text == ";" {
				last++
			}
			add(first, last, "", "", scope+"/opaque:"+rustTokenKey(lexed, first, last+1), "", scope, context, depth)
			first = last + 1
		}
		return nil
	}
	if err := walk(0, len(t), scope, context, 0); err != nil {
		return nil, err
	}
	return items, nil
}
func readScriptCodeObject(path, language, kind, name string, line, column int, src []byte) (*CodeObject, error) {
	if (kind != "function" && kind != "type") || name == "" || line < 0 || column < 0 {
		return nil, objectError("invalid_selector", "select a function/type name and nonnegative coordinates")
	}
	items, err := scriptCodeItems(path, src, "", "module", language)
	if err != nil {
		return nil, err
	}
	var matches []CodeObject
	for _, s := range items {
		o := s.object
		if o.Kind == kind && o.Name == name && (line == 0 || o.Line == line) && (column == 0 || o.Column == column) {
			matches = append(matches, o)
		}
	}
	if len(matches) == 0 {
		return nil, objectError("not_found", "no JS/TS declaration matches the selector")
	}
	if len(matches) > 1 {
		return nil, &ObjectError{Code: "ambiguous", Message: "select a candidate by coordinates or edit its object directly", Candidates: matches}
	}
	return &matches[0], nil
}
func scriptObjectPatch(item rustObjectItem, payload string, parsed []rustObjectItem, operation, language string, src []byte) (int, int, string, error) {
	start, end := item.start, item.end
	binding := strings.HasPrefix(item.context, "binding:")
	object := item.context == "object"
	if operation == "remove" && (binding || object) {
		tokens, err := scriptObjectTokens(src, language)
		if err != nil {
			return 0, 0, "", err
		}
		first, last := -1, -1
		for i, token := range tokens {
			if token.start == start {
				first = i
			}
			if token.end == end {
				last = i
			}
		}
		if first < 0 || last < first {
			return 0, 0, "", objectError("invalid_identity", "object has no token boundaries")
		}
		if last+1 < len(tokens) && tokens[last+1].text == "," {
			return start, tokens[last+1].end, string(src[end:tokens[last+1].start]), nil
		}
		if first > 0 && tokens[first-1].text == "," {
			return tokens[first-1].start, end, string(src[tokens[first-1].end:start]), nil
		}
		if object {
			return start, end, "", nil
		}
		if first == 0 || tokens[first-1].text != strings.TrimPrefix(item.context, "binding:") {
			return 0, 0, "", objectError("invalid_source", "unsupported declaration-list boundary")
		}
		keyword := first - 1
		start = tokens[keyword].start
		for keyword > 0 && (tokens[keyword-1].text == "export" || tokens[keyword-1].text == "declare") {
			keyword--
			start = tokens[keyword].start
		}
		text := string(src[tokens[first-1].end:item.start])
		if last+1 < len(tokens) && tokens[last+1].text == ";" {
			text += string(src[end:tokens[last+1].start])
			end = tokens[last+1].end
		}
		return start, end, text, nil
	}
	if operation == "remove" {
		return start, end, "", nil
	}
	if binding || object {
		var roots []rustObjectItem
		for _, node := range parsed {
			if node.depth == 0 {
				roots = append(roots, node)
			}
		}
		if len(roots) == 0 {
			return 0, 0, "", objectError("invalid_replacement", "no binding/member supplied")
		}
		first, last := roots[0].start, roots[len(roots)-1].end
		prefixEnd := 0
		if binding {
			kind := strings.TrimPrefix(item.context, "binding:")
			tokens, err := scriptObjectTokens([]byte(payload), language)
			if err != nil || len(tokens) == 0 || tokens[0].text != kind {
				return 0, 0, "", objectError("contract_changed", "preserve the original const/let/var keyword")
			}
			prefixEnd = tokens[0].end
			for _, node := range roots {
				if node.context != item.context {
					return 0, 0, "", objectError("invalid_replacement", "a declaration list accepts compatible bindings only")
				}
			}
		}
		if strings.TrimSpace(payload[prefixEnd:first]) != "" {
			return 0, 0, "", objectError("invalid_replacement", "cannot discard trivia outside a binding/member")
		}
		suffix := strings.TrimSpace(payload[last:])
		if suffix != "" && !(binding && suffix == ";") && !(object && suffix == ",") {
			return 0, 0, "", objectError("invalid_replacement", "unexpected trailing binding/member text")
		}
		payload = payload[first:last]
		if operation == "insert_after" {
			return end, end, ",\n" + payload, nil
		}
		return start, end, payload, nil
	}
	if operation == "insert_after" {
		return end, end, "\n" + payload + "\n", nil
	}
	return start, end, payload, nil
}

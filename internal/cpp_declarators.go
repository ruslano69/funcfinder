package internal

import (
	"strings"
	"unicode/utf8"
)

// The sanitizer blanks literals and comments. Recover the original gap for
// an apparently empty parameter list, so a string initializer is not f().
func cppEmptyParameters(lines []string, facts []LineFacts, tokens []csToken, open, close int) bool {
	first, last := tokens[open].line, tokens[close].line
	hasQuote := false
	for _, line := range lines[first : last+1] {
		if strings.ContainsAny(line, "'\"") {
			hasQuote = true
			break
		}
	}
	if !hasQuote {
		return true
	}
	column := func(index int) int {
		line := tokens[index].line
		start := index
		for start > 0 && tokens[start-1].line == line {
			start--
		}
		clean := facts[line].Cleaned
		offset := 0
		for j := start; j <= index; j++ {
			at := strings.Index(clean[offset:], tokens[j].text)
			if at < 0 {
				return -1
			}
			offset += at + len(tokens[j].text)
		}
		return utf8.RuneCountInString(clean[:offset]) - 1
	}
	from, to := column(open), column(close)
	if from < 0 || to < 0 {
		return false
	}
	comment := false
	for line := first; line <= last; line++ {
		raw := []rune(lines[line])
		lo, hi := 0, len(raw)
		if line == first {
			lo = from + 1
		}
		if line == last {
			hi = to
		}
		if lo > hi || hi > len(raw) {
			return false
		}
		for i := lo; i < hi; i++ {
			if i+1 < hi {
				if comment && raw[i] == '*' && raw[i+1] == '/' {
					comment = false
					i++
					continue
				}
				if !comment && raw[i] == '/' {
					switch raw[i+1] {
					case '/':
						i = hi
					case '*':
						comment = true
						i++
					}
				}
			}
			if i < hi && !comment && (raw[i] == '\'' || raw[i] == '"') {
				return false
			}
		}
	}
	return true
}

// cppAngleGroup is used only in a declaration prefix or a template-id. The
// shared brace lexer deliberately leaves angle brackets unpaired.
func cppAngleGroup(tokens []csToken, pairs []int, open, limit int) int {
	depth := 0
	for i := open; i < limit; i++ {
		switch tokens[i].text {
		case "<":
			depth++
		case ">":
			depth--
			if depth == 0 {
				return i + 1
			}
		case "(", "[":
			if pairs[i] > i && pairs[i] < limit {
				i = pairs[i]
			}
		case ";", "{", "}":
			return -1
		}
	}
	return -1
}

func cppQualifiedPart(tokens []csToken, lower, end int) int {
	i := end - 1
	if i >= lower && tokens[i].text == ">" {
		depth := 1
		for i--; i >= lower; i-- {
			switch tokens[i].text {
			case ">":
				depth++
			case "<":
				depth--
			case ";", "{", "}":
				return -1
			}
			if depth == 0 {
				i--
				break
			}
		}
	}
	if i < lower || !csIdentifier(tokens[i].text) {
		return -1
	}
	return i
}

type cppDeclarator struct {
	name, qualifier   string
	nameAt, prefixEnd int
}

func cppNameBeforeParams(tokens []csToken, begin, open int) (cppDeclarator, bool) {
	d := cppDeclarator{}
	for i := open - 1; i >= begin; i-- {
		if tokens[i].text != "operator" {
			continue
		}
		var spelling strings.Builder
		for j := i + 1; j < open; j++ {
			spelling.WriteString(tokens[j].text)
		}
		switch op := spelling.String(); op {
		case "=", "+", "-", "*", "/", "%", "^", "&", "|", "~", "!", "<", ">",
			"+=", "-=", "*=", "/=", "%=", "^=", "&=", "|=", "<<", ">>", "<<=", ">>=",
			"==", "!=", "<=", ">=", "<=>", "&&", "||", "++", "--", ",", "->", "->*", "()", "[]":
			d.name, d.nameAt = "operator"+op, i
		default:
			return d, false
		}
		break
	}
	if d.name == "" {
		d.nameAt = cppQualifiedPart(tokens, begin, open)
		if d.nameAt < 0 {
			return d, false
		}
		d.name = tokens[d.nameAt].text
		if d.nameAt > begin && tokens[d.nameAt-1].text == "~" {
			d.name = "~" + d.name
			d.nameAt--
		}
	}
	d.prefixEnd = d.nameAt
	for d.prefixEnd >= begin+2 && tokens[d.prefixEnd-1].text == ":" && tokens[d.prefixEnd-2].text == ":" {
		owner := cppQualifiedPart(tokens, begin, d.prefixEnd-2)
		if owner < 0 {
			break
		}
		if d.qualifier == "" {
			d.qualifier = tokens[owner].text
		}
		d.prefixEnd = owner
	}
	return d, true
}

// Distinguish ptr/ref operators in the return declarator from operators in
// template arguments and decltype expressions.
func cppReturnPrefix(tokens []csToken, pairs []int, begin, end int) (hasType, indirect, valid bool) {
	for i := begin; i < end; {
		switch word := tokens[i].text; word {
		case "const", "volatile", "static", "inline", "virtual", "explicit", "constexpr", "consteval",
			"extern", "friend", "typename", "class", "struct":
			i++
		case "*", "&":
			if !hasType {
				return false, false, false
			}
			indirect = true
			i++
		case ":":
			if i+1 >= end || tokens[i+1].text != ":" {
				return false, false, false
			}
			i += 2
		case "<":
			i = cppAngleGroup(tokens, pairs, i, end)
			if i < 0 {
				return false, false, false
			}
		case "decltype", "typeof", "__typeof__":
			if i+1 >= end || tokens[i+1].text != "(" || pairs[i+1] <= i+1 || pairs[i+1] >= end {
				return false, false, false
			}
			hasType = true
			i = pairs[i+1] + 1
		case "[":
			if i+1 >= end || tokens[i+1].text != "[" || pairs[i] <= i || pairs[i] >= end {
				return false, false, false
			}
			i = pairs[i] + 1
		default:
			if !csIdentifier(word) || word == "return" || word == "using" || word == "typedef" ||
				word == "throw" || word == "new" || word == "delete" || word == "requires" {
				return false, false, false
			}
			hasType = true
			i++
		}
	}
	return hasType, indirect, true
}

func cppDeclaratorEnd(tokens []csToken, pairs []int, end int, ctor bool) (last, body int) {
	for ; end < len(tokens); end++ {
		switch tokens[end].text {
		case "const", "volatile", "&", "override", "final", "constexpr":
		case "noexcept", "throw":
			if end+1 < len(tokens) && tokens[end+1].text == "(" && pairs[end+1] > end+1 {
				end = pairs[end+1]
			}
		case "=":
			if end+2 < len(tokens) && tokens[end+2].text == ";" {
				switch tokens[end+1].text {
				case "default", "delete", "0":
					return end + 2, -1
				}
			}
			return -1, -1
		case ":":
			if !ctor {
				return -1, -1
			}
			// A mem-initializer ends at a paired argument list or braced
			// initializer. A comma starts another initializer.
			for end++; end < len(tokens); end++ {
				switch tokens[end].text {
				case "(", "{":
					if pairs[end] <= end {
						return -1, -1
					}
					end = pairs[end] + 1
					if end < len(tokens) && tokens[end].text == "," {
						continue
					}
					if end < len(tokens) && tokens[end].text == "{" && pairs[end] > end {
						return pairs[end], end
					}
					return -1, -1
				case "<":
					next := cppAngleGroup(tokens, pairs, end, len(tokens))
					if next < 0 {
						return -1, -1
					}
					end = next - 1
				case ";", "}", "=":
					return -1, -1
				}
			}
			return -1, -1
		case "{":
			if pairs[end] > end {
				return pairs[end], end
			}
			return -1, -1
		case ";":
			return end, -1
		default:
			return -1, -1
		}
	}
	return -1, -1
}

func cppPlausibleParameters(lines []string, facts []LineFacts, tokens []csToken, pairs []int, open, close int) bool {
	start := open + 1
	if start == close {
		return cppEmptyParameters(lines, facts, tokens, open, close)
	}
	// Validate each parameter independently. A call in a direct initializer
	// can be qualified just like a type, but its argument list is not a type.
	typed := false
	words := 0
	for i := start; i < close; i++ {
		word := tokens[i].text
		switch word {
		case "void", "int", "char", "bool", "float", "double", "long", "short", "signed", "unsigned",
			"auto", "const", "volatile", "struct", "class", "typename", "wchar_t", "char8_t", "char16_t", "char32_t":
			typed = true
		case "*", "&":
			if !typed && words == 0 {
				return false
			}
			typed = true
		case ".":
			if i+2 >= close || tokens[i+1].text != "." || tokens[i+2].text != "." {
				return false
			}
			typed = true
			i += 2
		case ":":
			if i+1 >= close || tokens[i+1].text != ":" {
				return false
			}
			// Count a qualified name as one word.
			if words > 0 {
				words--
			}
			i++
		case "<":
			next := cppAngleGroup(tokens, pairs, i, close)
			if next < 0 {
				return false
			}
			i = next - 1
		case "(", "[":
			if pairs[i] <= i || pairs[i] >= close {
				return false
			}
			switch {
			case word == "[", i > start && tokens[i-1].text == "decltype",
				i+1 < close && (tokens[i+1].text == "*" || tokens[i+1].text == "&"),
				i > start && tokens[i-1].text == ")" && typed:
				typed = true
				i = pairs[i]
			default:
				return false
			}
		case "=":
			if !typed && words < 2 {
				return false
			}
			for i++; i < close && tokens[i].text != ","; i++ {
				if pairs[i] > i && pairs[i] < close {
					i = pairs[i]
				}
				if tokens[i].text == "<" {
					if next := cppAngleGroup(tokens, pairs, i, close); next > i {
						i = next - 1
					}
				}
			}
			i--
		case ",":
			if !typed && words < 2 {
				return false
			}
			typed, words = false, 0
		default:
			if !csIdentifier(word) || word == "true" || word == "false" || word == "nullptr" {
				return false
			}
			words++
		}
	}
	return typed || words >= 2
}

// Only treat a class-key at the start of a declaration as a scope opener.
// For instance, an elaborated return type in "struct X *make() {" is not one.
func cppClassScope(tokens []csToken, begin, brace int) string {
	if begin+1 >= brace {
		return ""
	}
	switch tokens[begin].text {
	case "class", "struct", "union":
		if !csIdentifier(tokens[begin+1].text) {
			return ""
		}
	default:
		return ""
	}
	for i := begin + 2; i < brace; i++ {
		switch tokens[i].text {
		case "(", ")", "*", "&", "=", ";":
			return ""
		case ":":
			return tokens[begin+1].text
		}
	}
	// No base list. A declarator after the type name is not a class body.
	if brace == begin+2 || brace == begin+3 && tokens[begin+2].text == "final" {
		return tokens[begin+1].text
	}
	return ""
}

// Merge only supported token recoveries into the existing line-based map.
// The brace stack allows local class methods while ignoring declarations and
// call/initialization expressions inside ordinary statement blocks.
func (f *Finder) recoverCppDeclarators(lines []string, facts []LineFacts, tokens []csToken, pairs []int, startLine int, result *FindResult) {
	type scope struct {
		class string
		body  bool
	}
	type span struct {
		name      string
		line, end int
	}
	existing := make(map[span]int, len(result.Functions))
	for _, fn := range result.Functions {
		line := fn.SignatureLine
		if line == 0 {
			line = fn.Start
		}
		existing[span{fn.Name, line, fn.End}]++
	}
	scopes := []scope{{}}
	boundary, templateEnd := 0, -1
	for i := 0; i < len(tokens); i++ {
		current := scopes[len(scopes)-1]
		switch tokens[i].text {
		case "template":
			if i == boundary && i+1 < len(tokens) && tokens[i+1].text == "<" {
				if next := cppAngleGroup(tokens, pairs, i+1, len(tokens)); next > i {
					i, boundary, templateEnd = next-1, next, next
				}
			}
		case ":":
			if i == boundary+1 && current.class != "" {
				switch tokens[boundary].text {
				case "public", "protected", "private":
					boundary = i + 1
				}
			}
		case "{":
			name := cppClassScope(tokens, boundary, i)
			next := scope{class: name, body: name == ""}
			if boundary < i && (tokens[boundary].text == "namespace" ||
				tokens[boundary].text == "inline" && boundary+1 < i && tokens[boundary+1].text == "namespace" ||
				tokens[boundary].text == "extern" && boundary+1 == i) {
				next.body = false
			}
			scopes = append(scopes, next)
			boundary = i + 1
		case "}":
			if len(scopes) > 1 {
				scopes = scopes[:len(scopes)-1]
			}
			boundary = i + 1
		case ";":
			boundary = i + 1
		case "[":
			if pairs[i] > i {
				i = pairs[i]
			}
		case "(":
			close := pairs[i]
			if close <= i {
				continue
			}
			if current.body {
				i = close
				continue
			}
			// Preserve a possible operator() prefix before its actual parameters.
			d, ok := cppNameBeforeParams(tokens, boundary, i)
			if !ok || startsWithReservedWord(d.name, f.config.StatementWords) {
				i = close
				continue
			}
			hasType, indirect, valid := cppReturnPrefix(tokens, pairs, boundary, d.prefixEnd)
			owner := current.class
			if d.qualifier != "" {
				owner = d.qualifier
			}
			ctor := owner != "" && !hasType && strings.TrimPrefix(d.name, "~") == owner
			last, body := cppDeclaratorEnd(tokens, pairs, close+1, ctor)
			if !valid || !hasType && !ctor || last < 0 {
				i = close
				continue
			}
			key := span{d.name, tokens[d.nameAt].line + startLine, tokens[last].line + startLine}
			covered := existing[key] > 0
			if covered {
				existing[key]--
			}
			// A multiline mem-initializer can make the line scanner end at
			// its first initializer. Repair that occurrence before merging.
			if !covered && ctor && body >= 0 && tokens[close+1].text == ":" {
				for j := range result.Functions {
					fn := &result.Functions[j]
					if fn.Name != key.name || fn.Start != key.line || fn.End >= tokens[body].line+startLine {
						continue
					}
					fn.End = key.end
					if f.extractMode {
						fn.Lines = append([]string(nil), lines[fn.Start-startLine:fn.End-startLine+1]...)
					}
					covered = true
					break
				}
			}
			// Local classes are often inside a legacy function's skip range.
			// Only admit ordinary signatures here when inside such a class.
			localClass := current.class != "" && len(scopes) > 2 && scopes[len(scopes)-2].body
			splitReturn := hasType && tokens[boundary].line < tokens[d.nameAt].line
			if !covered && (indirect || ctor || localClass || body >= 0 && (boundary == templateEnd || splitReturn)) &&
				(body >= 0 || cppPlausibleParameters(lines, facts, tokens, pairs, i, close)) {
				fn := FunctionBounds{
					Name: d.name, Start: tokens[boundary].line + startLine, End: key.end,
					ClassName: owner, Scope: owner, SignatureLine: key.line, SignatureName: d.name,
				}
				if f.extractMode {
					fn.Lines = append([]string(nil), lines[fn.Start-startLine:fn.End-startLine+1]...)
				}
				result.Functions = append(result.Functions, fn)
			}
			if body >= 0 {
				scopes = append(scopes, scope{body: true})
				i, boundary = body, body+1
			} else {
				i, boundary = last, last+1
			}
		}
	}
}

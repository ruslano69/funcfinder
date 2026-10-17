package internal

import "strings"

var javaModifiers = map[string]bool{
	"public": true, "private": true, "protected": true, "static": true,
	"final": true, "abstract": true, "synchronized": true, "native": true,
	"strictfp": true, "default": true, "sealed": true,
}

var javaStatements = map[string]bool{
	"return": true, "throw": true, "new": true, "if": true, "else": true,
	"for": true, "while": true, "do": true, "switch": true, "case": true,
	"catch": true, "try": true, "finally": true, "assert": true,
	"break": true, "continue": true, "class": true, "interface": true,
	"enum": true, "record": true, "package": true, "import": true,
	"this": true, "super": true, "instanceof": true,
}

func javaIdentifier(text string) bool {
	return csIdentifier(strings.ReplaceAll(text, "$", "_")) && !strings.HasPrefix(text, "@")
}

// Skip declaration modifiers and annotations, including qualified annotation
// names and array-valued arguments. @interface starts a type declaration.
func javaPrefix(tokens []csToken, pairs []int, start, end int) int {
	for start < end {
		word := tokens[start].text
		if strings.HasPrefix(word, "@") && word != "@interface" {
			start++
			for start+1 < end && tokens[start].text == "." {
				start += 2
			}
			if start < end && tokens[start].text == "(" && pairs[start] > start {
				start = pairs[start] + 1
			}
		} else if javaModifiers[word] {
			start++
		} else if start+2 < end && word == "non" && tokens[start+1].text == "-" && tokens[start+2].text == "sealed" {
			start += 3
		} else {
			break
		}
	}
	return start
}

// Recognize a return type rather than accepting any text before a call.
// Generic arguments are balanced; annotations can occur on qualified segments
// and array dimensions as well as before the type.
func javaTypeEnd(tokens []csToken, pairs []int, start, end int) int {
	i := javaPrefix(tokens, pairs, start, end)
	if i >= end || !javaIdentifier(tokens[i].text) || javaStatements[tokens[i].text] {
		return start
	}
	i++
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
		case ".":
			i = javaPrefix(tokens, pairs, i+1, end)
			if i >= end || !javaIdentifier(tokens[i].text) {
				return start
			}
			i++
		case "[":
			if pairs[i] != i+1 || pairs[i] >= end {
				return start
			}
			i += 2
		default:
			next := javaPrefix(tokens, pairs, i, end)
			if next == i {
				return i
			}
			i = next
		}
	}
	return i
}

func javaFunctionName(tokens []csToken, pairs []int, start, open int, className string) string {
	start = javaPrefix(tokens, pairs, start, open)
	if start < open && tokens[start].text == "<" {
		depth := 1
		start++
		for start < open && depth > 0 {
			if tokens[start].text == "<" {
				depth++
			} else if tokens[start].text == ">" {
				depth--
			}
			start++
		}
		if depth != 0 {
			return ""
		}
	}
	start = javaPrefix(tokens, pairs, start, open)
	nameAt := open - 1
	if start > nameAt || !javaIdentifier(tokens[nameAt].text) {
		return ""
	}
	name := tokens[nameAt].text
	if javaStatements[name] {
		return ""
	}
	if start == nameAt && name == className || start < nameAt && javaTypeEnd(tokens, pairs, start, nameAt) == nameAt {
		return name
	}
	return ""
}

// Annotation element defaults may contain arrays/annotations. Their braces
// belong to a value; the declaration ends at its semicolon, not the first }.
func javaFunctionEnd(tokens []csToken, pairs []int, close, limit int) (int, int) {
	value := false
	for i := close + 1; i < limit; i++ {
		switch tokens[i].text {
		case "default":
			value = true
		case "(", "[":
			if pairs[i] < i || pairs[i] >= limit {
				return -1, -1
			}
			i = pairs[i]
		case "{":
			if pairs[i] < i || pairs[i] >= limit {
				return -1, -1
			}
			if !value {
				return pairs[i], i
			}
			i = pairs[i]
		case ";":
			return i, -1
		case "}":
			return -1, -1
		}
	}
	return -1, -1
}

// A single token walk uses an explicit brace stack. Method bodies disable
// member declarations, while named/local/anonymous type bodies enable them.
func (f *Finder) findJavaFunctions(lines []string, startLine int, filename string) (*FindResult, error) {
	tokens, pairs := braceTokens(f.sanitizer.CleanLinesFacts(lines), true)
	result := &FindResult{Filename: filename}
	type scope struct {
		end                   int
		className             string
		members, record, enum bool
	}
	scopes := []scope{{end: len(tokens)}}
	boundary, functionBody := 0, -1
	appendFunction := func(name string, start, nameAt, end int, className string) {
		fn := FunctionBounds{Name: name, Start: tokens[start].line + startLine, End: tokens[end].line + startLine, ClassName: className, Scope: className}
		if nameAt >= 0 {
			fn.SignatureLine, fn.SignatureName = tokens[nameAt].line+startLine, name
		}
		if f.extractMode {
			fn.Lines = append([]string(nil), lines[fn.Start-startLine:fn.End-startLine+1]...)
		}
		if f.mapMode || f.funcNames[name] {
			result.Functions = append(result.Functions, fn)
		}
	}
	for i := 0; i < len(tokens); i++ {
		parent := scopes[len(scopes)-1]
		switch tokens[i].text {
		case "{":
			if pairs[i] <= i {
				continue
			}
			child := scope{end: pairs[i], className: parent.className}
			start := javaPrefix(tokens, pairs, boundary, i)
			if i != functionBody && start < i {
				switch tokens[start].text {
				case "class", "interface", "enum", "record", "@interface":
					if start+1 < i && javaIdentifier(tokens[start+1].text) {
						child.className, child.members = tokens[start+1].text, true
						child.record, child.enum = tokens[start].text == "record", tokens[start].text == "enum"
						// Use the type keyword's line, so the directory's structural
						// pass deduplicates this entry even with leading annotations.
						result.Classes = append(result.Classes, ClassBounds{Name: child.className, Start: tokens[start].line + startLine, End: tokens[pairs[i]].line + startLine})
					}
				default:
					if parent.members && parent.record && start+1 == i && tokens[start].text == parent.className {
						appendFunction(parent.className, boundary, -1, pairs[i], parent.className)
					} else {
						// new Type(...) { ... }, including inside call arguments;
						// enum constants may also have an anonymous class body.
						for j := boundary; j < i; j++ {
							if tokens[j].text == "new" && tokens[i-1].text == ")" {
								child.members = true
								break
							}
						}
						if parent.members && parent.enum && (tokens[i-1].text == ")" || start+1 == i && javaIdentifier(tokens[start].text)) {
							child.members = true
						}
					}
				}
			}
			scopes = append(scopes, child)
			boundary = i + 1
		case "}":
			if len(scopes) > 1 && scopes[len(scopes)-1].end == i {
				scopes = scopes[:len(scopes)-1]
			}
			boundary = i + 1
		case ";":
			if parent.members {
				scopes[len(scopes)-1].enum = false // enum constants end at this semicolon
			}
			boundary = i + 1
		case ",":
			if parent.members && parent.enum {
				boundary = i + 1
			}
		case "(":
			if pairs[i] < i {
				continue
			}
			if boundary < i && strings.HasPrefix(tokens[boundary].text, "@") && javaPrefix(tokens, pairs, boundary, i) >= i {
				i = pairs[i]
				continue
			}
			if !parent.members {
				continue
			}
			name := javaFunctionName(tokens, pairs, boundary, i, parent.className)
			if name == "" {
				continue
			}
			end, body := javaFunctionEnd(tokens, pairs, pairs[i], parent.end)
			if end < i {
				continue
			}
			appendFunction(name, boundary, i-1, end, parent.className)
			if body >= 0 {
				functionBody = body
				i = body - 1
			} else {
				i, boundary = end, end+1
			}
		}
	}
	return result, nil
}

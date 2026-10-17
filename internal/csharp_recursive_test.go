package internal

import "strings"

// Experimental scope traversal. Signature recognition, tokenization and endpoint
// rules are shared with production; only explicit scope stacks/maps are replaced.
// Kept in a test file so the default tools do not ship a second implementation.
func (f *Finder) findCSharpFunctionsRecursive(lines []string, startLine int, filename string) (*FindResult, error) {
	facts := f.sanitizer.CleanLinesFacts(lines)
	tokens, pairs := csTokens(facts)
	conditionals := csConditionals(facts)
	result := &FindResult{Filename: filename}
	var walk func(int, int, string)
	walk = func(first, limit int, className string) {
		boundary := first
		for i := first; i < limit; i++ {
			switch tokens[i].text {
			case "{":
				end := pairs[i]
				if end <= i || end >= limit {
					continue
				}
				childClass := className
				block := boundary == i || csPropertyBody(tokens, pairs, boundary, i)
				if name := csClassName(tokens, pairs, boundary, i); name != "" {
					childClass, block = name, true
					result.Classes = append(result.Classes, ClassBounds{Name: name, Start: tokens[boundary].line + startLine, End: tokens[end].line + startLine})
				}
				if boundary < i {
					switch tokens[boundary].text {
					case "if", "else", "for", "foreach", "while", "do", "switch", "try", "catch", "finally", "lock", "using", "fixed", "get", "set", "init", "add", "remove":
						block = true
					}
				}
				walk(i+1, end, childClass)
				if block {
					boundary = end + 1
				}
				i = end
			case "}", ";":
				boundary = i + 1
			case "(":
				if pairs[i] < i {
					continue
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
					walk(body+1, end, className)
					i = end
				} else {
					i = pairs[i]
				}
				boundary = i + 1
			}
		}
	}
	walk(0, len(tokens), "")
	return result, nil
}

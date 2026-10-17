package internal

import (
	"slices"
	"strings"
)

// Token and endpoint helpers are shared with the measured C# scanner. The core
// walker consumes only events, so neither those helpers nor keywords leak into it.
type braceTokenAdapter struct {
	tokens       []csToken
	pairs        []int
	conditionals []csConditional
	headerEnds   map[int]int
	config       *LanguageConfig
	startLine    int
}

func (a *braceTokenAdapter) Len() int { return len(a.tokens) }

func (f *Finder) findBraceFunctionsRecursive(lines []string, startLine int, filename string) (*FindResult, error) {
	a := &braceTokenAdapter{config: f.config, startLine: startLine}
	var facts []LineFacts
	if f.config.LangKey == "d" {
		prepared, ends := prepareDLines(lines, f.config)
		a.headerEnds = ends
		facts = lexerFacts(prepared, lines)
	} else {
		facts = f.sanitizer.CleanLinesFacts(lines)
	}
	a.tokens, a.pairs = csTokens(facts)
	a.conditionals = csConditionals(facts)
	return f.findBraceRecursive(lines, startLine, filename, a)
}

func (a *braceTokenAdapter) Event(i, boundary int, context braceContext) braceEvent {
	tokens, pairs := a.tokens, a.pairs
	switch tokens[i].text {
	case ":":
		if a.config.LangKey == "d" && boundary == i {
			return braceEvent{boundary: true}
		}
		if a.config.LangKey == "d" && boundary < i {
			word := tokens[boundary].text
			if dAttributeLine.MatchString(a.header(boundary, i)) || word == "case" || word == "default" || word == "package" || word == "extern" || word == "align" || word == "static" || word == "final" || word == "abstract" || word == "override" || word == "synchronized" {
				return braceEvent{boundary: true}
			}
		}
	case "}", ";":
		return braceEvent{boundary: true}
	case "{":
		end := pairs[i]
		if end <= i {
			return braceEvent{}
		}
		if a.config.LangKey == "java" && !context.inFunction && context.className != "" {
			start := a.javaPrefix(boundary, i)
			if start+1 == i && tokens[start].text == context.className {
				fn := &FunctionBounds{Name: context.className, Start: tokens[start].line + a.startLine, End: tokens[end].line + a.startLine, ClassName: context.className, Scope: context.className}
				return braceEvent{function: fn, end: end, body: i, next: end}
			}
		}
		name := a.className(boundary, i)
		event := braceEvent{end: end, reset: boundary == i || a.block(boundary, i)}
		if name != "" {
			event.class = &ClassBounds{Name: name, Start: tokens[boundary].line + a.startLine, End: tokens[end].line + a.startLine}
			event.reset = true
		}
		return event
	case "(":
		if pairs[i] < i {
			return braceEvent{}
		}
		if a.config.LangKey == "java" && a.javaPrefix(boundary, i) >= i {
			return braceEvent{next: pairs[i]}
		}
		name, start := a.functionName(boundary, i, context)
		if name == "" {
			return braceEvent{}
		}
		end, body := csFunctionEnd(tokens, pairs, pairs[i])
		if end < i || a.config.LangKey == "d" && name == "this" && body < 0 && context.inFunction {
			return braceEvent{}
		}
		if body < 0 && a.config.LangKey == "cs" {
			end = csExpressionAlternatives(tokens, pairs, pairs[i], end, a.conditionals)
		}
		fn := &FunctionBounds{Name: name, Start: tokens[start].line + a.startLine, End: tokens[end].line + a.startLine, ClassName: context.className, Scope: context.className}
		if a.config.LangKey == "cs" {
			fn.SignatureLine = tokens[i-1].line + a.startLine
			fn.SignatureName = strings.TrimPrefix(tokens[i-1].text, "@")
		}
		if last, ok := a.headerEnds[fn.Start-a.startLine]; ok && fn.End < last+a.startLine {
			fn.End = last + a.startLine
		}
		return braceEvent{function: fn, end: end, body: body, next: pairs[i]}
	}
	return braceEvent{}
}

func (a *braceTokenAdapter) block(start, end int) bool {
	if a.config.LangKey == "cs" && csPropertyBody(a.tokens, a.pairs, start, end) {
		return true
	}
	if start >= end {
		return false
	}
	switch a.tokens[start].text {
	case "if", "else", "for", "foreach", "while", "do", "switch", "try", "catch", "finally", "lock", "using", "fixed", "get", "set", "init", "add", "remove":
		return true
	}
	if a.config.LangKey == "d" || a.config.LangKey == "java" {
		return true
	}
	return false
}

// Rebuild a header with word separation while retaining punctuation adjacency.
// This lets existing D patterns remain the recognition contract in the pilot.
func (a *braceTokenAdapter) header(start, end int) string {
	var text strings.Builder
	for i := start; i < end; i++ {
		if i > start && (a.tokens[i].text == "~" && csIdentifier(a.tokens[i-1].text) || a.tokens[i].text == "." && strings.HasPrefix(a.tokens[i-1].text, "@")) {
			text.WriteByte(' ')
		}
		if i > start && csIdentifier(a.tokens[i].text) {
			previous := a.tokens[i-1].text
			if csIdentifier(previous) || previous == ")" || previous == "]" || previous == ">" || previous == "*" {
				text.WriteByte(' ')
			}
		}
		text.WriteString(a.tokens[i].text)
	}
	return text.String()
}

func (a *braceTokenAdapter) className(start, end int) string {
	if a.config.LangKey == "cs" {
		return csClassName(a.tokens, a.pairs, start, end)
	}
	if a.config.LangKey == "java" {
		start = a.javaPrefix(start, end)
		if start < end {
			switch a.tokens[start].text {
			case "class", "interface", "enum", "record":
				if start+1 < end {
					return a.tokens[start+1].text
				}
			}
		}
		return ""
	}
	return ExtractFuncName(a.config.ClassRegex().FindStringSubmatch(a.header(start, end)))
}

var braceJavaModifiers = map[string]bool{
	"public": true, "private": true, "protected": true, "static": true,
	"final": true, "abstract": true, "synchronized": true, "native": true,
	"strictfp": true, "default": true, "sealed": true,
}

func (a *braceTokenAdapter) javaPrefix(start, end int) int {
	for start < end {
		word := a.tokens[start].text
		if strings.HasPrefix(word, "@") {
			start++
			for start+1 < end && a.tokens[start].text == "." {
				start += 2
			}
			if start < end && a.tokens[start].text == "(" && a.pairs[start] > start {
				start = a.pairs[start] + 1
			}
		} else if braceJavaModifiers[word] {
			start++
		} else {
			break
		}
	}
	return start
}

func (a *braceTokenAdapter) functionName(start, open int, context braceContext) (string, int) {
	switch a.config.LangKey {
	case "cs":
		return csFunctionName(a.tokens, a.pairs, start, open, context.className)
	case "d":
		if start < open && a.tokens[start].text == "else" {
			start++
		}
		if start < open && csStatements[a.tokens[start].text] && a.tokens[start].text != "typeof" && a.tokens[start].text != "this" && a.tokens[start].text != "new" {
			return "", start
		}
		text := a.header(start, open+1)
		text = braceDStatements.Replace(text)
		if strings.HasPrefix(text, "else") {
			text = "    " + text[4:]
		}
		text = normalizeDPrefix(text)
		originalStart := start
		for start < open {
			prefixEnd := len(a.header(originalStart, start+1))
			if strings.TrimSpace(text[:prefixEnd]) != "" {
				break
			}
			start++
		}
		if start < open && csStatements[a.tokens[start].text] && a.tokens[start].text != "typeof" && a.tokens[start].text != "this" && a.tokens[start].text != "new" {
			return "", start
		}
		matches := a.config.FuncRegex().FindStringSubmatch(text)
		if matches == nil || len(matches[0]) != len(text) {
			return "", start
		}
		name := ExtractFuncName(matches)
		if slices.Contains(a.config.ExcludeWords, name) {
			return "", start
		}
		return name, start
	case "java":
		start = a.javaPrefix(start, open)
		if start < open && a.tokens[start].text == "<" {
			depth := 1
			start++
			for start < open && depth > 0 {
				if a.tokens[start].text == "<" {
					depth++
				} else if a.tokens[start].text == ">" {
					depth--
				}
				start++
			}
		}
		start = a.javaPrefix(start, open)
		if start >= open {
			return "", start
		}
		switch a.tokens[start].text {
		case "return", "throw", "new", "if", "else", "for", "while", "do", "switch", "case", "catch", "try", "assert", "class", "interface", "enum", "record":
			return "", start
		}
		nameAt := open - 1
		name := a.tokens[nameAt].text
		if !csIdentifier(name) {
			return "", start
		}
		if start == nameAt && name == context.className || start < nameAt && csSkipType(a.tokens, a.pairs, start, nameAt) == nameAt {
			return name, start
		}
		return "", start
	}
	return "", start
}

var braceDStatements = strings.NewReplacer("?ersion(", "version(", "?ebug(", "debug(", "?ragma(", "pragma(", "?tatic if(", "static if(", "?lse", "else")

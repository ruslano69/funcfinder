package internal

import (
	"sort"
	"strings"
)

// cppNestedClasses adds inner type scopes omitted by the legacy outer-class walk.
func cppNestedClasses(facts []LineFacts, startLine int, classes []ClassBounds) []ClassBounds {
	t, pairs := braceTokens(facts, false)
	for i := 0; i+1 < len(t); i++ {
		if (t[i].text != "class" && t[i].text != "struct") || !csIdentifier(t[i+1].text) {
			continue
		}
		line := t[i].line + startLine
		nested, exists := false, false
		for _, c := range classes {
			if c.Start < line && line <= c.End {
				nested = true
			}
			if c.Start == line && c.Name == t[i+1].text {
				exists = true
			}
		}
		if !nested || exists {
			continue
		}
		for j := i + 2; j < len(t); j++ {
			if t[j].text == ";" || t[j].text == ">" || t[j].text == "=" || t[j].text == "(" {
				break
			}
			if t[j].text == "{" {
				if pairs[j] > j {
					classes = append(classes, ClassBounds{Name: t[i+1].text, Start: line, End: t[pairs[j]].line + startLine})
				}
				break
			}
		}
	}
	return classes
}

// recoverCppMembers incrementally moves the headers the line recognizer cannot
// read to a token scan. Existing results supply function-body exclusion ranges.
func (f *Finder) recoverCppMembers(lines []string, facts []LineFacts, startLine int, result *FindResult) {
	// Mask continued directives too; their contents are not C++ declarations.
	clean := append([]LineFacts(nil), facts...)
	directive := false
	for i, fact := range clean {
		text := strings.TrimSpace(fact.Cleaned)
		if directive || strings.HasPrefix(text, "#") {
			directive = strings.HasSuffix(text, "\\")
			clean[i].Cleaned = ""
		}
	}
	t, pairs := braceTokens(clean, false)
	at := func(i int, text string) bool { return i >= 0 && i < len(t) && t[i].text == text }
	boundary := 0
	for i := 0; i < len(t); i++ {
		if at(i, "{") || at(i, "}") || at(i, ";") {
			boundary = i + 1
			continue
		}
		if at(i, ":") && i == boundary+1 && (at(boundary, "public") || at(boundary, "private") || at(boundary, "protected")) {
			boundary = i + 1
			continue
		}
		if !at(i, "(") || pairs[i] <= i || i == boundary {
			continue
		}
		line := t[i-1].line + startLine
		class := f.findClassForLine(result.Classes, line-1)
		if class == "" {
			continue
		}
		covered := false
		for _, fn := range result.Functions {
			if fn.Start <= line && line <= fn.End {
				covered = true
				break
			}
		}
		if covered {
			continue
		}
		name := t[i-1].text
		if !csIdentifier(name) || startsWithReservedWord(name, f.config.StatementWords) {
			continue
		}
		conversion := false
		for j := boundary; j < i; j++ {
			if at(j, "operator") {
				parts := make([]string, 0, i-j-1)
				for k := j + 1; k < i; k++ {
					if !strings.Contains(" void bool char wchar_t char8_t char16_t char32_t int short long float double signed unsigned const volatile * & ", " "+t[k].text+" ") {
						parts = nil
						break
					}
					parts = append(parts, t[k].text)
				}
				if len(parts) > 0 {
					name = "operator " + strings.Join(parts, " ")
					conversion = true
				}
				break
			}
		}
		// Ordinary members need a return type. Reject expressions and assignments.
		valid := conversion || i-1 > boundary
		for j := boundary; j < i-1; j++ {
			if at(j, "=") || at(j, ".") || at(j, "(") || at(j, ")") || at(j, "return") || at(j, "operator") && !conversion {
				valid = false
			}
		}
		if !valid {
			continue
		}
		end, special := pairs[i]+1, conversion
		for end < len(t) {
			switch t[end].text {
			case "const", "volatile", "override", "final":
				end++
			case "&":
				special = true
				end++
			case "noexcept":
				end++
				if at(end, "(") && pairs[end] > end {
					special = true
					end = pairs[end] + 1
				}
			default:
				goto suffixDone
			}
		}
	suffixDone:
		if !special || end >= len(t) {
			continue
		}
		if at(end, "{") && pairs[end] > end {
			end = pairs[end]
		} else if !at(end, ";") {
			continue
		}
		if f.mapMode || f.funcNames[name] {
			fn := FunctionBounds{Name: name, Start: t[boundary].line + startLine, End: t[end].line + startLine, ClassName: class, Scope: class, SignatureLine: line, SignatureName: t[i-1].text}
			if f.extractMode {
				fn.Lines = append([]string(nil), lines[fn.Start-startLine:fn.End-startLine+1]...)
			}
			result.Functions = append(result.Functions, fn)
		}
		i, boundary = end, end+1
	}
	sort.SliceStable(result.Functions, func(i, j int) bool { return result.Functions[i].Start < result.Functions[j].Start })
}

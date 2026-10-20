// docs_python.go - Python docstrings for --doc: the string literal that opens the body of a def or class.
package internal

import "strings"

// headerSearchLines bounds how far a signature may run before its colon.
const headerSearchLines = 20

// declarationDoc returns the doc lines of the declaration that starts at line start (1-based): the docstring of a Python
// declaration when it has one, otherwise the comment lines above it (DocLines).
func declarationDoc(lines []string, start int, lang *LanguageConfig) []string {
	if lang != nil && lang.LangKey == "py" {
		if doc := pythonDocstring(lines, start); doc != nil {
			return doc
		}
	}
	return DocLines(lines, start, lang)
}

// pythonDocstring returns the docstring of the def or class that starts at line start, without its quotes, or nil when
// it has none. The docstring is the first statement of the body: blank and comment lines may come before it, nothing
// else. A signature may run over several lines; the header ends at the line whose code ends with a colon, outside any
// bracket. A one-line declaration (`def f(): pass`) has no docstring here.
func pythonDocstring(lines []string, start int) []string {
	depth := 0
	end := -1
	for j := start - 1; j < len(lines) && j < start-1+headerSearchLines; j++ {
		code := stripPyComment(lines[j])
		for _, r := range code {
			switch r {
			case '(', '[', '{':
				depth++
			case ')', ']', '}':
				depth--
			}
		}
		if depth <= 0 {
			if strings.HasSuffix(strings.TrimSpace(code), ":") {
				end = j
			}
			break
		}
	}
	if end < 0 {
		return nil
	}

	body := end + 1
	for ; body < len(lines); body++ {
		t := strings.TrimSpace(lines[body])
		if t != "" && !strings.HasPrefix(t, "#") {
			break
		}
	}
	if body >= len(lines) {
		return nil
	}
	return docstringAt(lines, body)
}

// docstringAt reads the string literal that starts line b, without its quotes. It returns nil when the line does not
// start with a plain or raw string: f-strings and byte strings are not docstrings.
func docstringAt(lines []string, b int) []string {
	open := lines[b]
	raw := strings.TrimSpace(open)
	for len(raw) > 0 && strings.ContainsRune("rRuU", rune(raw[0])) {
		raw = raw[1:]
	}

	if strings.HasPrefix(raw, `"""`) || strings.HasPrefix(raw, `'''`) {
		q := raw[:3]
		indent := open[:len(open)-len(strings.TrimLeft(open, " \t"))]
		rest := raw[3:]
		if k := strings.Index(rest, q); k >= 0 {
			return []string{strings.TrimSpace(rest[:k])}
		}
		doc := []string{}
		if first := strings.TrimSpace(rest); first != "" {
			doc = append(doc, first)
		}
		for j := b + 1; j < len(lines); j++ {
			if k := strings.Index(lines[j], q); k >= 0 {
				if tail := strings.TrimSpace(lines[j][:k]); tail != "" {
					doc = append(doc, dedentDoc(lines[j][:k], indent))
				}
				return doc
			}
			doc = append(doc, dedentDoc(lines[j], indent))
		}
		return doc // a docstring that never closes: what there is
	}

	if len(raw) > 0 && (raw[0] == '"' || raw[0] == '\'') {
		q := raw[:1]
		rest := raw[1:]
		if k := strings.Index(rest, q); k >= 0 {
			return []string{rest[:k]}
		}
	}
	return nil
}

// dedentDoc removes the indentation of the opening line from a docstring line, and the trailing blanks.
func dedentDoc(line, indent string) string {
	if strings.HasPrefix(line, indent) {
		line = line[len(indent):]
	} else {
		line = strings.TrimLeft(line, " \t")
	}
	return strings.TrimRight(line, " \t")
}

// stripPyComment returns the line without a trailing # comment. A # inside a string is not told apart: a signature or a
// bracket line seldom holds one.
func stripPyComment(line string) string {
	if k := strings.Index(line, "#"); k >= 0 {
		return line[:k]
	}
	return line
}

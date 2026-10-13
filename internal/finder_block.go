package internal

import (
	"regexp"
	"strings"
)

// rubyHeredoc matches the start of a heredoc, <<~MD, <<-SQL or <<'TEXT'. Its body is
// text, up to a line that holds only the same word, and it is not code.
var rubyHeredoc = regexp.MustCompile(`<<[~-]?['"]?([A-Z_][A-Z0-9_]*)['"]?`)

// rubyEndless matches an endless method, def name(args) = expr, whose body is on the
// header line. The space before = keeps a setter, def name=(v), out of it.
var rubyEndless = regexp.MustCompile(`^\s*def\s+(?:self\.)?[\w?!]+(?:\([^)]*\))?\s+=\s`)

// rubyEndSuffix matches a def header that also ends with end on the same line.
var rubyEndSuffix = regexp.MustCompile(`^def\b.*\bend$`)

// rubyOneLine reports whether a def header also closes on its own line: an endless
// method, or a body closed with end on the same line (def name; end).
func rubyOneLine(cleaned string) bool {
	if rubyEndless.MatchString(cleaned) {
		return true
	}
	return rubyEndSuffix.MatchString(strings.TrimSpace(cleaned))
}

// findFunctionsByBlockEnd finds functions in a language whose blocks end with a
// keyword, not a brace (Ruby: def ... end). A function ends at the keyword at the
// indentation of its header, as the struct finder ends a class. A keyword at a lower
// indentation closes the function on the line before it, so a missing end cannot
// swallow the rest of the file.
func (f *Finder) findFunctionsByBlockEnd(lines []string, lineOffset int, classes []ClassBounds, result *FindResult) (*FindResult, error) {
	funcRegex := f.config.FuncRegex()
	keyword := f.config.BlockEndKeyword
	state := StateNormal
	var current *FunctionBounds
	indent := 0

	heredocEnd := "" // the word that closes the heredoc being read, if any
	for lineNum, line := range lines {
		lineNo := lineNum + 1 + lineOffset
		if heredocEnd != "" {
			// The body is not cleaned, so its terminator is compared with the raw line.
			if strings.TrimSpace(line) == heredocEnd {
				heredocEnd = ""
			}
			if current != nil && f.extractMode {
				current.Lines = append(current.Lines, line)
			}
			continue
		}
		fact, next := f.sanitizer.lineFacts(line, state)
		state = next
		cleaned := fact.Cleaned
		if m := rubyHeredoc.FindStringSubmatch(cleaned); m != nil {
			heredocEnd = m[1]
		}

		if current != nil {
			if f.extractMode {
				current.Lines = append(current.Lines, line)
			}
			if rubyBlockEndAt(cleaned, fact.Indent, keyword, indent) {
				current.End = lineNo
				result.Functions = append(result.Functions, *current)
				current = nil
			} else if strings.TrimSpace(cleaned) == keyword && fact.Indent < indent {
				current.End = lineNo - 1
				result.Functions = append(result.Functions, *current)
				current = nil
			}
			continue
		}

		matches := funcRegex.FindStringSubmatch(cleaned)
		if matches == nil {
			continue
		}
		funcName := ExtractFuncName(matches)
		if !(f.mapMode || f.funcNames[funcName]) || f.isKeywordName(funcName) {
			continue
		}
		className := ""
		if f.config.HasClasses() {
			className = f.findClassForLine(classes, lineNo)
		}
		current = &FunctionBounds{
			Name:      funcName,
			Start:     lineNo,
			Lines:     []string{},
			ClassName: className,
			Scope:     className,
		}
		if f.extractMode {
			current.Lines = append(current.Lines, line)
		}
		indent = fact.Indent
		if rubyOneLine(cleaned) {
			current.End = lineNo
			result.Functions = append(result.Functions, *current)
			current = nil
		}
	}

	return result, nil
}

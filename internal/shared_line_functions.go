package internal

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"unicode/utf8"
)

// sharedLineFunctions exposes structural boundaries to the legacy line parsers.
// It changes only lines with a declaration after a boundary, and maps every
// result back to the original physical lines. Literal/comment text is never
// inspected for boundaries. Native token finders do not use this adapter.
func (f *Finder) sharedLineFunctions(lines []string, startLine int, filename string) (*FindResult, bool, error) {
	switch f.config.LangKey {
	case "go", "c", "cpp", "d", "rust", "swift", "kotlin", "php", "ruby":
	default:
		return nil, false, nil
	}
	// Cheap raw gate only; the lexical pass below makes the final decision.
	potential := false
	for _, line := range lines {
		for at := 0; at < len(line); at++ {
			if !strings.ContainsRune("{};", rune(line[at])) {
				continue
			}
			if at+1 < len(line) && f.config.FuncSubmatch(strings.TrimSpace(line[at+1:])) != nil {
				potential = true
				break
			}
		}
		if potential {
			break
		}
	}
	if !potential {
		return nil, false, nil
	}
	clean := make([]string, len(lines))
	switch f.config.LangKey {
	case "d":
		lexer := dLineLexer{}
		for i, line := range lines {
			clean[i] = lexer.cleanLine(line)
		}
	case "php":
		facts := NewSanitizer(f.config, false).CleanLinesFacts(maskPHPHeredocs(lines, f.config))
		for i := range facts {
			clean[i] = facts[i].Cleaned
		}
	default:
		facts := NewSanitizer(f.config, false).CleanLinesFacts(lines)
		for i := range facts {
			clean[i] = facts[i].Cleaned
		}
	}
	var expanded []string
	var physical []int
	changed := false
	for i, line := range lines {
		cuts := sharedLineCuts(clean[i])
		eligible := false
		for _, at := range cuts {
			if at < len(clean[i]) && f.config.FuncSubmatch(strings.TrimSpace(clean[i][at:])) != nil {
				eligible = true
				break
			}
		}
		if eligible && len(clean[i]) != len(line) {
			// D's lexer masks one rune with one space, so a non-ASCII literal
			// changes byte lengths. Translate token boundaries back to raw bytes.
			if utf8.RuneCountInString(clean[i]) != utf8.RuneCountInString(line) {
				eligible = false
			} else {
				ci, ri := 0, 0
				for j, cut := range cuts {
					for ci < cut {
						_, cw := utf8.DecodeRuneInString(clean[i][ci:])
						_, rw := utf8.DecodeRuneInString(line[ri:])
						ci += cw
						ri += rw
					}
					cuts[j] = ri
				}
			}
		}
		if eligible {
			changed = true
			from := 0
			for _, at := range cuts {
				if at >= len(line) {
					continue
				}
				expanded = append(expanded, f.sharedLinePart(line[from:at], line))
				physical = append(physical, i+startLine)
				from = at
			}
			expanded = append(expanded, f.sharedLinePart(line[from:], line))
		} else {
			expanded = append(expanded, line)
		}
		physical = append(physical, i+startLine)
	}
	if !changed {
		return nil, false, nil
	}
	if f.config.LangKey == "go" {
		if result, ok := f.goSharedLineFunctions(lines, startLine, filename); ok {
			return result, true, nil
		}
	}
	// Each resulting line has at most one declaration start. Re-entering the
	// normal finder retains language-specific signature and scope decisions.
	result, err := f.FindFunctionsInLines(expanded, 1, filename)
	if err != nil {
		return nil, true, err
	}
	mapLine := func(n int) int {
		if n < 1 {
			return startLine
		}
		if n > len(physical) {
			return startLine + len(lines) - 1
		}
		return physical[n-1]
	}
	for i := range result.Functions {
		fn := &result.Functions[i]
		fn.Start, fn.End = mapLine(fn.Start), mapLine(fn.End)
		if fn.SignatureLine > 0 {
			fn.SignatureLine = mapLine(fn.SignatureLine)
		}
		if f.extractMode {
			fn.Lines = append([]string(nil), lines[fn.Start-startLine:fn.End-startLine+1]...)
		}
	}
	for i := range result.Classes {
		c := &result.Classes[i]
		c.Start, c.End = mapLine(c.Start), mapLine(c.End)
	}
	return result, true, nil
}

// On complete Go files the grammar also distinguishes result type braces from
// body braces. Fragments/custom patterns retain the configured finder path.
func (f *Finder) goSharedLineFunctions(lines []string, startLine int, filename string) (*FindResult, bool) {
	const pattern = `^\s*func\s+(\([^)]*\)\s+)?({IDENT}+)(?:\[[^\]]*\])?\s*\(`
	if f.config.FuncPattern != pattern {
		return nil, false
	}
	fs := token.NewFileSet()
	file, err := parser.ParseFile(fs, filename, strings.Join(lines, "\n"), parser.SkipObjectResolution)
	if err != nil {
		return nil, false
	}
	result := &FindResult{Filename: filename, Functions: []FunctionBounds{}, Classes: []ClassBounds{}}
	for _, d := range file.Decls {
		fn, ok := d.(*ast.FuncDecl)
		if !ok || (!f.mapMode && !f.funcNames[fn.Name.Name]) {
			continue
		}
		a, b := fs.Position(fn.Pos()).Line, fs.Position(fn.End()).Line
		bound := FunctionBounds{Name: fn.Name.Name, Start: a + startLine - 1, End: b + startLine - 1}
		if f.extractMode {
			bound.Lines = append([]string(nil), lines[a-1:b]...)
		}
		result.Functions = append(result.Functions, bound)
	}
	return result, true
}

func (f *Finder) sharedLinePart(part, original string) string {
	switch f.config.LangKey {
	case "c":
		return strings.TrimLeft(part, " \t")
	case "ruby":
		indent := original[:len(original)-len(strings.TrimLeft(original, " \t"))]
		return indent + strings.TrimSuffix(strings.TrimSpace(part), ";")
	case "php":
		return strings.Replace(part, "<?php", "     ", 1)
	}
	return part
}

// sharedLineCuts walks cleaned code. Semicolons in for headers, attribute
// arguments and array expressions are protected by their enclosing brackets.
func sharedLineCuts(code string) []int {
	if !strings.ContainsAny(code, "{};") {
		return nil
	}
	var cuts []int
	paren, square := 0, 0
	for i := 0; i < len(code); i++ {
		switch code[i] {
		case '(':
			paren++
		case ')':
			if paren > 0 {
				paren--
			}
		case '[':
			square++
		case ']':
			if square > 0 {
				square--
			}
		case '{', '}', ';':
			if paren == 0 && square == 0 {
				cuts = append(cuts, i+1)
			}
		}
	}
	return cuts
}

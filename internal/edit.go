package internal

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FuncTarget selects one function to replace. Line is the 1-based line of its start as the finder reports it;
// zero selects by name, which must then match exactly one function.
type FuncTarget struct {
	Name string
	Line int
}

// ReplaceResult is a validated replacement. Output is the complete new file, with the original line endings.
type ReplaceResult struct {
	Start  int // first replaced line (1-based)
	End    int // last replaced line (1-based, inclusive)
	Output []byte
}

// ReplaceFunction replaces one function of any configured language. The span comes from the production finder.
// Nothing is returned unless every check passes:
//   - the target is found exactly once (trailing blank lines are not part of its span);
//   - the current text equals expectOld when expectOld is non-nil (line endings are ignored);
//   - the replacement is not empty and its brackets balance (languages with brackets; the check ignores strings
//     and comments through the language's cleaner);
//   - the finder finds the same functions and classes, in the same order, in the new file as in the old one, so a
//     replacement cannot add, remove or rename a declaration;
//   - for Go, the span also agrees with go/ast and the new file parses (see edit_go.go).
//
// Lines outside the span are kept byte for byte. Mixed line endings are refused, because the file is rewritten
// with one line ending.
func ReplaceFunction(cfg *LanguageConfig, src []byte, target FuncTarget, expectOld []byte, replacement []byte) (*ReplaceResult, error) {
	if target.Name == "" {
		return nil, fmt.Errorf("function name is empty")
	}
	crlf := bytes.Count(src, []byte("\r\n"))
	if crlf > 0 && crlf != bytes.Count(src, []byte("\n")) {
		return nil, fmt.Errorf("mixed line endings: refusing to rewrite the file")
	}
	eol := "\n"
	if crlf > 0 {
		eol = "\r\n"
	}
	text := strings.ReplaceAll(string(src), "\r\n", "\n")
	if text == "" {
		return nil, fmt.Errorf("source is empty")
	}
	finalNewline := strings.HasSuffix(text, "\n")
	lines := strings.Split(strings.TrimSuffix(text, "\n"), "\n")

	dir, err := os.MkdirTemp("", "funcfinder-edit-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	ext := ""
	if len(cfg.Extensions) > 0 {
		ext = cfg.Extensions[0]
	}
	origPath := filepath.Join(dir, "original"+ext)
	if err := os.WriteFile(origPath, []byte(text), 0o600); err != nil {
		return nil, err
	}

	finder := CreateFinder(cfg, "", "map", false, false)
	before, err := finder.FindFunctions(origPath)
	if err != nil {
		return nil, fmt.Errorf("finding functions: %v", err)
	}
	fn, err := selectFunction(before.Functions, target)
	if err != nil {
		return nil, err
	}
	// Some finders (Python) end a function after its trailing blank lines; those are not part of the function.
	for fn.End > fn.Start && strings.TrimSpace(lines[fn.End-1]) == "" {
		fn.End--
	}
	if cfg.LangKey == "go" {
		if err := checkGoSpanWithAST(text, fn, target.Name); err != nil {
			return nil, err
		}
	}

	oldText := strings.Join(lines[fn.Start-1:fn.End], "\n")
	if expectOld != nil && oldText != strings.ReplaceAll(string(expectOld), "\r\n", "\n") {
		return nil, fmt.Errorf("function %s at lines %d-%d is not the expected text (it changed since it was read)",
			target.Name, fn.Start, fn.End)
	}

	newText := strings.TrimSuffix(strings.ReplaceAll(string(replacement), "\r\n", "\n"), "\n")
	if strings.TrimSpace(newText) == "" {
		return nil, fmt.Errorf("replacement is empty")
	}
	if !cfg.IndentBased {
		if err := checkBracketsBalance(cfg, newText); err != nil {
			return nil, err
		}
	}
	newLines := strings.Split(newText, "\n")
	if err := sameDecorators(lines[fn.Start-1:fn.End], newLines); err != nil {
		return nil, err
	}

	out := make([]string, 0, len(lines)-(fn.End-fn.Start+1)+len(newLines))
	out = append(out, lines[:fn.Start-1]...)
	out = append(out, newLines...)
	out = append(out, lines[fn.End:]...)
	outText := strings.Join(out, "\n")
	if finalNewline {
		outText += "\n"
	}

	newPath := filepath.Join(dir, "replaced"+ext)
	if err := os.WriteFile(newPath, []byte(outText), 0o600); err != nil {
		return nil, err
	}
	after, err := finder.FindFunctions(newPath)
	if err != nil {
		return nil, fmt.Errorf("replacement does not give a file the finder can read: %v", err)
	}
	if err := sameDeclarations(before, after); err != nil {
		return nil, err
	}
	if cfg.LangKey == "go" {
		if err := checkGoDeclarationsKept(text, outText, target.Name); err != nil {
			return nil, err
		}
	}

	return &ReplaceResult{
		Start:  fn.Start,
		End:    fn.End,
		Output: []byte(strings.ReplaceAll(outText, "\n", eol)),
	}, nil
}

// selectFunction returns the one function the target selects.
func selectFunction(functions []FunctionBounds, target FuncTarget) (FunctionBounds, error) {
	var matches []FunctionBounds
	for _, fn := range functions {
		if fn.Name == target.Name && (target.Line == 0 || fn.Start == target.Line) {
			matches = append(matches, fn)
		}
	}
	switch len(matches) {
	case 0:
		if target.Line != 0 {
			return FunctionBounds{}, fmt.Errorf("no function %s starts at line %d", target.Name, target.Line)
		}
		return FunctionBounds{}, fmt.Errorf("no function named %s", target.Name)
	case 1:
		return matches[0], nil
	}
	starts := make([]string, len(matches))
	for i, fn := range matches {
		starts[i] = fmt.Sprint(fn.Start)
	}
	return FunctionBounds{}, fmt.Errorf("%d functions named %s start at lines %s: select one with the start line",
		len(matches), target.Name, strings.Join(starts, ", "))
}

// sameDecorators requires the replacement to restate the decorators and annotations above the function. The
// finder's span starts at the first of them, so a replacement that dropped them would silently change behaviour.
func sameDecorators(old, replacement []string) error {
	want := leadingDecorators(old)
	got := leadingDecorators(replacement)
	if strings.Join(want, "\n") != strings.Join(got, "\n") {
		return fmt.Errorf("replacement changes the decorators above the function: was [%s], now [%s]; restate them in the replacement",
			strings.Join(want, " "), strings.Join(got, " "))
	}
	return nil
}

// leadingDecorators returns the lines before the declaration that start with a decorator or annotation (@x) or a
// Rust attribute (#[x]). Other languages' attribute syntaxes are not recognised.
func leadingDecorators(lines []string) []string {
	var out []string
	for _, line := range lines {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "@") || strings.HasPrefix(t, "#[") {
			out = append(out, t)
			continue
		}
		if t == "" {
			continue
		}
		break
	}
	return out
}

// sameDeclarations requires the finder to report the same function names and class names, in the same order.
func sameDeclarations(before, after *FindResult) error {
	if want, got := funcNames(before), funcNames(after); strings.Join(want, ",") != strings.Join(got, ",") {
		return fmt.Errorf("replacement changes the functions in the file: was [%s], now [%s]",
			strings.Join(want, ", "), strings.Join(got, ", "))
	}
	if want, got := classNames(before), classNames(after); strings.Join(want, ",") != strings.Join(got, ",") {
		return fmt.Errorf("replacement changes the types in the file: was [%s], now [%s]",
			strings.Join(want, ", "), strings.Join(got, ", "))
	}
	return nil
}

func funcNames(r *FindResult) []string {
	names := make([]string, len(r.Functions))
	for i, fn := range r.Functions {
		names[i] = fn.Name
	}
	return names
}

func classNames(r *FindResult) []string {
	names := make([]string, len(r.Classes))
	for i, c := range r.Classes {
		names[i] = c.Name
	}
	return names
}

// checkBracketsBalance refuses a replacement whose code (strings and comments removed) does not close what it opens.
func checkBracketsBalance(cfg *LanguageConfig, text string) error {
	code := NewSanitizer(cfg, false).CleanCode(text)
	for _, pair := range [][2]string{{"(", ")"}, {"[", "]"}, {"{", "}"}} {
		open := strings.Count(code, pair[0])
		closed := strings.Count(code, pair[1])
		if open != closed {
			return fmt.Errorf("replacement has unbalanced %s%s: %d opened, %d closed", pair[0], pair[1], open, closed)
		}
	}
	return nil
}

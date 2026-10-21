package internal

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FuncTarget selects one function to edit. Line is the 1-based line of its start as the finder reports it;
// zero selects by name, which must then match exactly one function.
type FuncTarget struct {
	Name string
	Line int
}

// ReplaceResult is a validated edit. Output is the complete new file, with the original line endings. Start and
// End are the replaced lines (1-based, inclusive) for a replacement; for an insertion they are both the line the
// new text follows.
type ReplaceResult struct {
	Start  int
	End    int
	Output []byte
}

// ReplaceFunction replaces one function of any configured language. The span comes from the production finder.
// Nothing is returned unless every check passes:
//   - the target is found exactly once (trailing blank lines are not part of its span);
//   - the current text equals expectOld when expectOld is non-nil (line endings and one trailing newline ignored);
//   - the replacement is not empty and its brackets balance (languages with brackets; the check ignores strings
//     and comments through the language's cleaner);
//   - decorators above the function are kept: a replacement without them keeps the old ones, a different set is
//     refused;
//   - the finder finds the same functions and types in the same order in the new file, so a replacement cannot
//     add, remove or rename a declaration;
//   - for Go, the span also agrees with go/ast and the new file parses (see edit_go.go).
//
// Lines outside the span are kept byte for byte. Mixed line endings are refused, because the file is rewritten
// with one line ending.
func ReplaceFunction(cfg *LanguageConfig, src []byte, target FuncTarget, expectOld []byte, replacement []byte) (*ReplaceResult, error) {
	e, err := openEdit(cfg, src, target)
	if err != nil {
		return nil, err
	}
	defer e.close()

	if expectOld != nil && e.oldText() != strings.TrimSuffix(strings.ReplaceAll(string(expectOld), "\r\n", "\n"), "\n") {
		return nil, fmt.Errorf("function %s at lines %d-%d is not the expected text (it changed since it was read)",
			target.Name, e.fn.Start, e.fn.End)
	}
	newLines, err := e.newLines(replacement)
	if err != nil {
		return nil, err
	}
	// A replacement without decorators keeps the old ones, so omitting them never loses them.
	if len(leadingDecorators(newLines)) == 0 {
		newLines = append(rawLeadingDecorators(e.lines[e.fn.Start-1:e.fn.End]), newLines...)
	}
	if err := sameDecorators(e.lines[e.fn.Start-1:e.fn.End], newLines); err != nil {
		return nil, err
	}
	if err := e.checkIndent(newLines); err != nil {
		return nil, err
	}

	out := make([]string, 0, len(e.lines)-(e.fn.End-e.fn.Start+1)+len(newLines))
	out = append(out, e.lines[:e.fn.Start-1]...)
	out = append(out, newLines...)
	out = append(out, e.lines[e.fn.End:]...)
	outText := e.join(out)

	after, err := e.findIn("replaced", outText)
	if err != nil {
		return nil, err
	}
	if err := sameDeclarations(e.before, after); err != nil {
		return nil, err
	}
	if e.cfg.LangKey == "go" {
		if err := checkGoDeclarationsKept(e.text, outText, target.Name); err != nil {
			return nil, err
		}
	}
	return &ReplaceResult{Start: e.fn.Start, End: e.fn.End, Output: e.output(outText)}, nil
}

// InsertAfterFunction adds the functions in replacement after the target, with one blank line in between. The
// finder must report the new functions directly after the target, and the types must not change. Decorators in
// the replacement belong to the new function and are not compared with the target's.
func InsertAfterFunction(cfg *LanguageConfig, src []byte, target FuncTarget, replacement []byte) (*ReplaceResult, error) {
	e, err := openEdit(cfg, src, target)
	if err != nil {
		return nil, err
	}
	defer e.close()

	newLines, err := e.newLines(replacement)
	if err != nil {
		return nil, err
	}
	if err := e.checkIndent(newLines); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(e.lines)+len(newLines)+1)
	out = append(out, e.lines[:e.fn.End]...)
	out = append(out, "")
	out = append(out, newLines...)
	out = append(out, e.lines[e.fn.End:]...)
	outText := e.join(out)

	after, err := e.findIn("inserted", outText)
	if err != nil {
		return nil, err
	}
	if err := sameTypes(e.before, after); err != nil {
		return nil, err
	}
	added := len(after.Functions) - len(e.before.Functions)
	if added < 1 {
		return nil, fmt.Errorf("replacement defines no function")
	}
	next := e.fnIndex + 1
	if !sameNames(funcNames(e.before)[:next], funcNames(after)[:next]) ||
		!sameNames(funcNames(e.before)[next:], funcNames(after)[next+added:]) {
		return nil, fmt.Errorf("the new functions are not directly after %s", target.Name)
	}
	if e.cfg.LangKey == "go" {
		if _, err := goDeclSignatures(outText); err != nil {
			return nil, fmt.Errorf("insertion does not give a valid file: %v", err)
		}
	}
	return &ReplaceResult{Start: e.fn.End, End: e.fn.End, Output: e.output(outText)}, nil
}

// editFile holds the source of one edit and the finder's view of it.
type editFile struct {
	cfg          *LanguageConfig
	text         string // LF line endings, without the final newline in lines
	lines        []string
	eol          string
	finalNewline bool
	finder       LanguageFinder
	dir          string
	ext          string
	before       *FindResult
	fn           FunctionBounds
	fnIndex      int // index of fn in before.Functions
}

// openEdit validates the source and locates the target function.
func openEdit(cfg *LanguageConfig, src []byte, target FuncTarget) (*editFile, error) {
	if target.Name == "" {
		return nil, fmt.Errorf("function name is empty")
	}
	crlf := bytes.Count(src, []byte("\r\n"))
	if crlf > 0 && crlf != bytes.Count(src, []byte("\n")) {
		return nil, fmt.Errorf("mixed line endings: refusing to rewrite the file")
	}
	e := &editFile{cfg: cfg, eol: "\n"}
	if crlf > 0 {
		e.eol = "\r\n"
	}
	e.text = strings.ReplaceAll(string(src), "\r\n", "\n")
	if e.text == "" {
		return nil, fmt.Errorf("source is empty")
	}
	e.finalNewline = strings.HasSuffix(e.text, "\n")
	e.lines = strings.Split(strings.TrimSuffix(e.text, "\n"), "\n")

	dir, err := os.MkdirTemp("", "funcfinder-edit-")
	if err != nil {
		return nil, err
	}
	e.dir = dir
	if len(cfg.Extensions) > 0 {
		e.ext = cfg.Extensions[0]
	}
	e.finder = CreateFinder(cfg, "", "map", false, false)
	before, err := e.findIn("original", e.text)
	if err != nil {
		e.close()
		return nil, err
	}
	e.before = before
	fn, idx, err := selectFunction(before.Functions, target)
	if err != nil {
		e.close()
		return nil, err
	}
	// A function that ends the file can be reported one line past the end; clamp it. Some finders (Python) end a
	// function after its trailing blank lines; those are not part of the function.
	if fn.End > len(e.lines) {
		fn.End = len(e.lines)
	}
	for fn.End > fn.Start && strings.TrimSpace(e.lines[fn.End-1]) == "" {
		fn.End--
	}
	e.fn = fn
	e.fnIndex = idx
	if err := e.checkOwnLines(); err != nil {
		e.close()
		return nil, err
	}
	if cfg.LangKey == "go" {
		if err := checkGoSpanWithAST(e.text, fn, target.Name); err != nil {
			e.close()
			return nil, err
		}
	}
	return e, nil
}

// checkOwnLines refuses a function whose first or last line also holds other code. Spans are whole lines, so an
// edit would delete that code too. The check reads the cleaned lines (strings and comments removed) and is
// conservative: a one-line function with a ";" or two "}" is refused as well.
func (e *editFile) checkOwnLines() error {
	cleaned := NewSanitizer(e.cfg, false).CleanLines(e.lines)
	if len(cleaned) != len(e.lines) {
		return nil
	}
	first := cleaned[e.fn.Start-1]
	last := cleaned[e.fn.End-1]
	oneLine := e.fn.Start == e.fn.End
	shared := fmt.Sprintf("%s: lines %d-%d hold other code too; edit those lines by hand", e.fn.Name, e.fn.Start, e.fn.End)

	if e.cfg.IndentBased {
		if strings.Contains(first, ";") || (!oneLine && strings.Contains(last, ";")) {
			return fmt.Errorf("%s", shared)
		}
		return nil
	}
	if oneLine {
		if strings.Contains(first, ";") || strings.Count(first, "}") > 1 {
			return fmt.Errorf("%s", shared)
		}
		return nil
	}
	if strings.Contains(first, ";") || strings.Contains(first, "}") {
		return fmt.Errorf("%s", shared)
	}
	// The first "}" on the last line closes the function; anything after it is other code.
	if i := strings.Index(last, "}"); i >= 0 {
		if strings.TrimRight(strings.TrimSpace(last[i+1:]), ";,)") != "" {
			return fmt.Errorf("%s", shared)
		}
	}
	return nil
}

func (e *editFile) close() {
	os.RemoveAll(e.dir)
}

func (e *editFile) oldText() string {
	return strings.Join(e.lines[e.fn.Start-1:e.fn.End], "\n")
}

// newLines normalises the replacement text and checks its brackets.
func (e *editFile) newLines(replacement []byte) ([]string, error) {
	text := strings.TrimSuffix(strings.ReplaceAll(string(replacement), "\r\n", "\n"), "\n")
	if strings.TrimSpace(text) == "" {
		return nil, fmt.Errorf("replacement is empty")
	}
	if !e.cfg.IndentBased {
		if err := checkBracketsBalance(e.cfg, text); err != nil {
			return nil, err
		}
	}
	return strings.Split(text, "\n"), nil
}

// checkIndent keeps indentation-based code (Python) in its block. The new text must start at the target's indentation
// and no line of it may be less indented than that, or the code moves out of its class or body. Tabs and spaces
// may not be mixed in one indentation. Lines inside a triple-quoted string are exempt: their indentation is content.
func (e *editFile) checkIndent(lines []string) error {
	if !e.cfg.IndentBased {
		return nil
	}
	base := indentOf(e.lines[e.fn.Start-1])
	first := -1
	for i, line := range lines {
		if strings.TrimSpace(line) != "" {
			first = i
			break
		}
	}
	if first < 0 {
		return nil
	}
	if got := indentOf(lines[first]); got != base {
		return fmt.Errorf("replacement starts at indentation %q, the function is at %q", got, base)
	}
	inString := false
	header := -1      // index of the def/class line
	needBody := false // the header ends with ":", so the next code line must be indented deeper
	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if !inString {
			ind := indentOf(line)
			t := strings.TrimSpace(line)
			if !strings.HasPrefix(ind, base) {
				return fmt.Errorf("replacement line %d is less indented than the function (%q): it would leave its block", i+1, ind)
			}
			if strings.Contains(ind, "\t") && strings.Contains(ind, " ") {
				return fmt.Errorf("replacement line %d mixes tabs and spaces in its indentation", i+1)
			}
			switch {
			case header < 0 && !isDecorator(t):
				header = i
				needBody = strings.HasSuffix(t, ":")
			case header >= 0:
				if needBody {
					if len(ind) <= len(base) {
						return fmt.Errorf("replacement line %d: the def has no indented body", header+1)
					}
					needBody = false
				}
				if ind == base && !startsBlock(t) {
					return fmt.Errorf("replacement line %d is at the function's indentation but is not a declaration", i+1)
				}
			}
		}
		if strings.Count(line, `"""`)%2 == 1 || strings.Count(line, `'''`)%2 == 1 {
			inString = !inString
		}
	}
	if needBody {
		return fmt.Errorf("replacement line %d: the def has no indented body", header+1)
	}
	return nil
}

// isDecorator reports whether a trimmed line is a decorator or annotation above a declaration.
func isDecorator(t string) bool {
	return strings.HasPrefix(t, "@") || strings.HasPrefix(t, "#[")
}

// startsBlock reports whether a trimmed line can sit at the indentation of a function: a declaration, a decorator
// of the next one, or a comment.
func startsBlock(t string) bool {
	for _, p := range []string{"def ", "async def ", "class ", "@", "#"} {
		if strings.HasPrefix(t, p) {
			return true
		}
	}
	return false
}

// indentOf returns the leading spaces and tabs of a line.
func indentOf(line string) string {
	return line[:len(line)-len(strings.TrimLeft(line, " \t"))]
}

// join returns the lines as the file text, with the original final newline.
func (e *editFile) join(lines []string) string {
	text := strings.Join(lines, "\n")
	if e.finalNewline {
		text += "\n"
	}
	return text
}

// output converts LF text back to the file's line endings.
func (e *editFile) output(text string) []byte {
	return []byte(strings.ReplaceAll(text, "\n", e.eol))
}

// findIn writes text to a scratch file and runs the finder on it.
func (e *editFile) findIn(name, text string) (*FindResult, error) {
	path := filepath.Join(e.dir, name+e.ext)
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		return nil, err
	}
	result, err := e.finder.FindFunctions(path)
	if err != nil {
		return nil, fmt.Errorf("the finder cannot read the %s file: %v", name, err)
	}
	return result, nil
}

// selectFunction returns the one function the target selects, and its index in functions.
func selectFunction(functions []FunctionBounds, target FuncTarget) (FunctionBounds, int, error) {
	var matches []int
	for i, fn := range functions {
		if fn.Name == target.Name && (target.Line == 0 || fn.Start == target.Line) {
			matches = append(matches, i)
		}
	}
	switch len(matches) {
	case 0:
		if target.Line != 0 {
			return FunctionBounds{}, 0, fmt.Errorf("no function %s starts at line %d", target.Name, target.Line)
		}
		return FunctionBounds{}, 0, fmt.Errorf("no function named %s", target.Name)
	case 1:
		return functions[matches[0]], matches[0], nil
	}
	starts := make([]string, len(matches))
	for i, idx := range matches {
		starts[i] = fmt.Sprint(functions[idx].Start)
	}
	return FunctionBounds{}, 0, fmt.Errorf("%d functions named %s start at lines %s: select one with the start line",
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

// leadingDecorators returns the trimmed decorator lines of rawLeadingDecorators.
func leadingDecorators(lines []string) []string {
	raw := rawLeadingDecorators(lines)
	out := make([]string, len(raw))
	for i, line := range raw {
		out[i] = strings.TrimSpace(line)
	}
	return out
}

// rawLeadingDecorators returns the lines before the declaration that start with a decorator or annotation (@x) or a
// Rust attribute (#[x]), as written. Other languages' attribute syntaxes are not recognised.
func rawLeadingDecorators(lines []string) []string {
	var out []string
	for _, line := range lines {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "@") || strings.HasPrefix(t, "#[") {
			out = append(out, line)
			continue
		}
		if t == "" {
			continue
		}
		break
	}
	return out
}

// sameDeclarations requires the finder to report the same function names and type names, in the same order.
func sameDeclarations(before, after *FindResult) error {
	if !sameNames(funcNames(before), funcNames(after)) {
		return fmt.Errorf("replacement changes the functions in the file: was [%s], now [%s]",
			strings.Join(funcNames(before), ", "), strings.Join(funcNames(after), ", "))
	}
	return sameTypes(before, after)
}

// sameTypes requires the finder to report the same type names, in the same order.
func sameTypes(before, after *FindResult) error {
	if !sameNames(classNames(before), classNames(after)) {
		return fmt.Errorf("replacement changes the types in the file: was [%s], now [%s]",
			strings.Join(classNames(before), ", "), strings.Join(classNames(after), ", "))
	}
	return nil
}

func sameNames(a, b []string) bool {
	return strings.Join(a, "\x00") == strings.Join(b, "\x00")
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

// checkBracketsBalance refuses text whose code (strings and comments removed) does not close what it opens.
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

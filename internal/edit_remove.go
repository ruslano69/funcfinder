package internal

import (
	"fmt"
	"strings"
)

// RemoveFunction deletes one function of any configured language, with its decorators and any functions nested in
// it. The blank lines that follow it go too, so the separation before the next declaration stays. The finder must
// then report the remaining functions and types in the same order. For a Python block, removing the last statement
// of a def or class is refused, because that leaves the block without a body. Go also re-parses the file.
func RemoveFunction(cfg *LanguageConfig, src []byte, target FuncTarget) (*ReplaceResult, error) {
	e, err := openEdit(cfg, src, target)
	if err != nil {
		return nil, err
	}
	defer e.close()

	lo := e.fn.Start - 1
	hi := e.fn.End
	for hi < len(e.lines) && strings.TrimSpace(e.lines[hi]) == "" {
		hi++
	}
	out := make([]string, 0, len(e.lines))
	out = append(out, e.lines[:lo]...)
	out = append(out, e.lines[hi:]...)
	if strings.TrimSpace(strings.Join(out, "")) == "" {
		return nil, fmt.Errorf("removing %s would leave the file empty", target.Name)
	}
	if e.cfg.IndentBased {
		if err := checkBlockKeepsBody(out, lo); err != nil {
			return nil, err
		}
	}
	outText := e.join(out)

	// The removed span holds the target and any functions nested in it; they are contiguous in the finder's order.
	removed := 0
	for _, fn := range e.before.Functions {
		if fn.Start >= e.fn.Start && fn.Start <= e.fn.End {
			removed++
		}
	}
	after, err := e.findIn("removed", outText)
	if err != nil {
		return nil, err
	}
	names := funcNames(e.before)
	want := append(append([]string{}, names[:e.fnIndex]...), names[e.fnIndex+removed:]...)
	if !sameNames(want, funcNames(after)) {
		return nil, fmt.Errorf("removal changes the other functions in the file: want [%s], got [%s]",
			strings.Join(want, ", "), strings.Join(funcNames(after), ", "))
	}
	if err := sameTypes(e.before, after); err != nil {
		return nil, err
	}
	if e.cfg.LangKey == "go" {
		before, err := goDeclSignatures(e.text)
		if err != nil {
			return nil, err
		}
		left, err := goDeclSignatures(outText)
		if err != nil {
			return nil, fmt.Errorf("removal does not give a valid file: %v", err)
		}
		if len(left) != len(before)-1 {
			return nil, fmt.Errorf("removal changes the top-level declarations from %d to %d", len(before), len(left))
		}
	}
	return &ReplaceResult{Start: e.fn.Start, End: e.fn.End, Output: e.output(outText)}, nil
}

// checkBlockKeepsBody refuses a removal that leaves the header before lo (a def, class or other ":" block) with no
// line indented deeper than itself.
func checkBlockKeepsBody(lines []string, lo int) error {
	prev := -1
	for i := lo - 1; i >= 0; i-- {
		if strings.TrimSpace(lines[i]) != "" {
			prev = i
			break
		}
	}
	if prev < 0 || !strings.HasSuffix(strings.TrimSpace(lines[prev]), ":") {
		return nil
	}
	headerIndent := indentOf(lines[prev])
	for i := lo; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "" {
			continue
		}
		if len(indentOf(lines[i])) > len(headerIndent) {
			return nil
		}
		break
	}
	return fmt.Errorf("removing the function would leave %q without a body", strings.TrimSpace(lines[prev]))
}

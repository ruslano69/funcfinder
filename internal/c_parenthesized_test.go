package internal

import (
	"strings"
	"testing"
)

func TestCFinder_ParenthesizedNamesAndFunctionPointerTypes(t *testing.T) {
	config, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	lines := []string{
		"API int (wrapped)(",
		"    int value);",
		`API const char *(text)(void) { return "{ }"; }`,
		"typedef void (*Callback)(int);",
		"typedef Result (*Producer)(void);",
		"extern Result (*callback)(int);",
		"API void (*another)(int);",
		"static int ordinary(void) { return 1; }",
		"API void (split) (",
		"    int n,",
		"    void (*callback)(int))",
		"{",
		"    callback(n);",
		"}",
		"typedef void ( /* comment */ *Spaced)(int);",
		"API void (next)(void);",
	}
	want := map[string][2]int{"wrapped": {1, 2}, "text": {3, 3}, "ordinary": {8, 8}, "split": {9, 14}, "next": {16, 16}}
	for _, offset := range []int{0, 30} {
		result, err := NewFinder(config["c"], nil, true, true, false).FindFunctionsInLines(lines, offset+1, "test.c")
		if err != nil {
			t.Fatal(err)
		}
		for i := range result.Functions {
			result.Functions[i].Start -= offset
			result.Functions[i].End -= offset
		}
		checkLanguageBounds(t, strings.Join(lines, "\n"), result, want)
		selected, err := NewFinder(config["c"], []string{"split"}, false, true, false).FindFunctionsInLines(lines, offset+1, "test.c")
		if err != nil || len(selected.Functions) != 1 || selected.Functions[0].Start != 9+offset || selected.Functions[0].End != 14+offset {
			t.Errorf("selected function = %v, error %v", selected, err)
		}
	}
}

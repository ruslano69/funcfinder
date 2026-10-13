package internal

import (
	"reflect"
	"strconv"
	"testing"
)

// The functions after a docstring that holds quotes are found, and a function after a raw triple-quoted string is found
// too: the Python logical lines are read through the sanitizer, and a string must not end early in either case.
// Only the names and starts are compared here: the end of a function before a blank line is a separate question.
func TestPythonFinder_FunctionsAfterQuotedStrings(t *testing.T) {
	src := `def first():
    """Doc with "quotes" inside."""
    return 1

def second():
    text = r"""a "b" c"""
    return text

def third():
    return 3
`
	path := createTempFile(t, src, "quoted_*.py")
	res, err := NewPythonFinder(getPyConfig(t), "", "map", false).FindFunctions(path)
	if err != nil {
		t.Fatalf("FindFunctions: %v", err)
	}
	var got []string
	for _, fn := range res.Functions {
		got = append(got, fn.Name+":"+strconv.Itoa(fn.Start))
	}
	want := []string{"first:1", "second:5", "third:9"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("functions = %v, want %v", got, want)
	}
}

package internal

import (
	"reflect"
	"strconv"
	"strings"
	"testing"
)

// A class header is followed by its body on the next code line: a comment between them is not code. The next line is
// read cleaned, so the class does not end on its header. Before, the raw next line was compared, the comment was
// taken for a line that is not a body, and the class ended on its header.
func TestClassBodyAfterCommentLine(t *testing.T) {
	cfg := langConfigOf(t, "cpp")
	src := "class Widget\n// the body follows\n{\n  int x;\n};\n"
	res, err := NewFinder(cfg, nil, true, false, false).FindFunctionsInLines(strings.Split(src, "\n"), 1, "test.cpp")
	if err != nil {
		t.Fatalf("FindFunctionsInLines: %v", err)
	}
	var got []string
	for _, c := range res.Classes {
		got = append(got, c.Name+":"+strconv.Itoa(c.Start)+"-"+strconv.Itoa(c.End))
	}
	want := []string{"Widget:1-5"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("classes = %v, want %v", got, want)
	}
}

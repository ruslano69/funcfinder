package internal

import (
	"reflect"
	"testing"
)

// A Ruby method ends at its `end`, and a trailing comment after the `end` does not hide it: the end is read from the
// cleaned line. Before, the raw line was compared, so `end # the first method` never closed the method and the
// second method was not found at all.
func TestFinderBlock_RubyEndWithTrailingComment(t *testing.T) {
	src := `def first
  1
end # the first method

def second
  2
end
`
	got := spans(mapFunctions(t, "ruby", src))
	want := []string{"first:1-3", "second:5-7"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("functions = %v, want %v", got, want)
	}
}

// A heredoc body is text, not code: the words in it do not change the state of the lines after it. The body is read
// up to its terminator, which is compared with the raw line because the body is not cleaned.
func TestFinderBlock_HeredocBodyIsNotCode(t *testing.T) {
	src := `def report
  text = <<~MD
    it's "not" code
  MD
  text
end

def after
  2
end
`
	got := spans(mapFunctions(t, "ruby", src))
	want := []string{"after:8-10", "report:1-6"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("functions = %v, want %v", got, want)
	}
}

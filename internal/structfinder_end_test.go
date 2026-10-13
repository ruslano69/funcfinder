package internal

import (
	"reflect"
	"strconv"
	"strings"
	"testing"
)

// A Ruby block ends at its `end`, and a trailing comment after the `end` does not hide it: the end is read from the
// cleaned line, where the comment is blank. Before, the raw line was compared, so `end # the class` never closed
// the class and the class swallowed the next one.
func TestStructFinder_RubyEndWithTrailingComment(t *testing.T) {
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	lc, err := cfg.GetLanguageConfig("ruby")
	if err != nil {
		t.Fatal(err)
	}
	src := `class Foo
  x = 1
end # the class

class Bar
  y = 2
end
`
	res, err := NewStructFinderFactory().CreateStructFinder(lc, "", true, false).FindStructuresInLines(strings.Split(src, "\n"), 1, "t.rb")
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, ty := range res.Types {
		got = append(got, ty.Name+":"+strconv.Itoa(ty.Start)+"-"+strconv.Itoa(ty.End))
	}
	want := []string{"Foo:1-3", "Bar:5-7"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("types = %v, want %v", got, want)
	}
}

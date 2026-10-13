package internal

import (
	"reflect"
	"strconv"
	"strings"
	"testing"
)

// The text of a docstring is not code: a line in it that looks like a class header is not a class, and an unindented
// line of it does not end the class. Before, the raw lines were matched, so both were taken as code.
func TestPythonStructFinder_DocstringIsNotCode(t *testing.T) {
	src := "class Real:\n    \"\"\"Example:\nclass Fake(Base):\n    pass\n\"\"\"\n    value: int = 1\n\nclass Next:\n    other: str = ''"
	res, err := NewPythonStructFinder(getPyConfig(t), "", true, false).FindStructuresInLines(strings.Split(src, "\n"), 1, "t.py")
	if err != nil {
		t.Fatalf("FindStructuresInLines: %v", err)
	}
	var got []string
	for _, ty := range res.Types {
		got = append(got, ty.Name+":"+strconv.Itoa(ty.Start)+"-"+strconv.Itoa(ty.End))
	}
	want := []string{"Real:1-7", "Next:8-9"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("types = %v, want %v", got, want)
	}
}

// A line of a docstring that looks like a field is not a field.
func TestPythonStructFinder_DocstringLineIsNotAField(t *testing.T) {
	src := "class Point:\n    \"\"\"\n    x: int\n    \"\"\"\n    y: int = 0\n"
	res, err := NewPythonStructFinder(getPyConfig(t), "", true, false).FindStructuresInLines(strings.Split(src, "\n"), 1, "t.py")
	if err != nil {
		t.Fatalf("FindStructuresInLines: %v", err)
	}
	if len(res.Types) != 1 {
		t.Fatalf("types = %d, want 1", len(res.Types))
	}
	var got []string
	for _, f := range res.Types[0].Fields {
		got = append(got, f.Name)
	}
	if want := []string{"y"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("fields = %v, want %v", got, want)
	}
}

// A decorator on the line above a class names its kind: @dataclass and @attr.s. A decorator above a def does not reach
// the class that comes after the def.
func TestPythonStructFinder_DecoratorNamesKind(t *testing.T) {
	src := "from dataclasses import dataclass\n\n@dataclass(frozen=True)\nclass Point:\n    x: int\n    y: int = 0\n\n@attr.s\nclass Box:\n    w: int = 1\n\nclass Plain:\n    v: int = 2\n\n@dataclass\ndef make():\n    pass\n\nclass Late:\n    z: int = 3\n"
	res, err := NewPythonStructFinder(getPyConfig(t), "", true, false).FindStructuresInLines(strings.Split(src, "\n"), 1, "t.py")
	if err != nil {
		t.Fatalf("FindStructuresInLines: %v", err)
	}
	var got []string
	for _, ty := range res.Types {
		got = append(got, ty.Name+":"+ty.Kind+":"+strconv.Itoa(ty.Start))
	}
	want := []string{"Point:dataclass:4", "Box:attrs:9", "Plain:class:12", "Late:class:19"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("types = %v, want %v", got, want)
	}
}

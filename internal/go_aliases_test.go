package internal

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// The aliases of a Go file are the single-line `type A = S` and the `A = S` lines of a `type ( ... )` group.
// An assignment outside a type group is not an alias.
func TestFindGoAliases(t *testing.T) {
	src := `package alias

// S is the one struct; A, B and C are names for it.
type S struct{ X int }

type A = S

type (
	B = S
	C = S
	D = int
)

x = 1
type E int
`
	got := findGoAliases(strings.Split(src, "\n"))
	want := []AliasBounds{
		{Name: "A", Target: "S", Start: 6},
		{Name: "B", Target: "S", Start: 9},
		{Name: "C", Target: "S", Start: 10},
		{Name: "D", Target: "int", Start: 11},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("aliases = %+v, want %+v", got, want)
	}
}

// The JSON map lists a file's aliases apart from its types, and omits the key for a file without any.
func TestFormatDirResultsJSON_GoAliases(t *testing.T) {
	results := []DirResult{
		{
			Path:    "a.go",
			Classes: []ClassBounds{{Name: "S", Start: 4}},
			Aliases: []AliasBounds{{Name: "A", Target: "S", Start: 6}},
		},
		{Path: "b.go", Functions: []FunctionBounds{{Name: "F", Start: 1}}},
	}
	var got struct {
		Files []struct {
			Path    string       `json:"path"`
			Classes []jsonSymbol `json:"classes"`
			Aliases []jsonAlias  `json:"aliases"`
		} `json:"files"`
	}
	if err := json.Unmarshal([]byte(formatDirResultsJSON(results)), &got); err != nil {
		t.Fatalf("formatDirResultsJSON() is not valid JSON: %v", err)
	}
	if len(got.Files) != 2 {
		t.Fatalf("files = %d, want 2", len(got.Files))
	}
	if want := []jsonSymbol{{Name: "S", Line: 4}}; !reflect.DeepEqual(got.Files[0].Classes, want) {
		t.Errorf("classes = %+v, want %+v (an alias is not a class)", got.Files[0].Classes, want)
	}
	if want := []jsonAlias{{Name: "A", Line: 6, Target: "S"}}; !reflect.DeepEqual(got.Files[0].Aliases, want) {
		t.Errorf("aliases = %+v, want %+v", got.Files[0].Aliases, want)
	}
	if len(got.Files[1].Aliases) != 0 {
		t.Errorf("file without aliases lists %+v", got.Files[1].Aliases)
	}
}

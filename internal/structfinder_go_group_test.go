package internal

import (
	"strings"
	"testing"
)

// Go `type ( ... )` group members carry no `type` keyword of their own, so the
// struct patterns never saw them: a whole group was invisible (gin's
// binding/form.go `formBinding`/`formPostBinding`/`formMultipartBinding`).
// Members are read with a virtual `type ` prefix; alias members (`A = S`) match
// no struct pattern and stay aliases.
func TestGoStructFinder_TypeGroupMembers(t *testing.T) {
	goCfg := getGoConfig(t)
	code := `package binding

type (
	formBinding          struct{}
	formPostBinding      struct{}
	formMultipartBinding struct{}
)

type (
	GroupedStruct struct {
		A int
	}
	GroupedNamed string
	GroupedIface interface {
		M()
	}
)

type (
	AliasA = GroupedStruct
	AliasB = int
)

type After string
`
	factory := NewStructFinderFactory()
	sf := factory.CreateStructFinder(goCfg, "", true, false)
	res, err := sf.FindStructuresInLines(strings.Split(code, "\n"), 1, "t.go")
	if err != nil {
		t.Fatalf("FindStructuresInLines() error = %v", err)
	}

	got := map[string][2]int{} // name -> [start, end]
	for _, ty := range res.Types {
		got[ty.Name] = [2]int{ty.Start, ty.End}
	}

	for name, want := range map[string][2]int{
		"formBinding": {4, 4}, "formPostBinding": {5, 5}, "formMultipartBinding": {6, 6},
		"GroupedStruct": {10, 12}, "GroupedNamed": {13, 13}, "GroupedIface": {14, 16},
		"After": {24, 24},
	} {
		if got[name] != want {
			t.Errorf("type %q span = %v, want %v (all: %v)", name, got[name], want, got)
		}
	}
	// Alias members are names for existing types, not types of their own.
	for _, name := range []string{"AliasA", "AliasB"} {
		if _, ok := got[name]; ok {
			t.Errorf("alias %q listed as a type (all: %v)", name, got)
		}
	}
}

// A `named` type whose expression opens a brace (`type testRequests []struct {`)
// spans lines: the relaxed `named` pattern matches it, and the unmatched `{`
// keeps the type open until the braces balance instead of closing it inline.
func TestGoStructFinder_NamedCompositeMultiline(t *testing.T) {
	goCfg := getGoConfig(t)
	code := `package main

type testRequests []struct {
	path  string
	route string
}

type Counts map[string]struct {
	N int
}

type After int
`
	factory := NewStructFinderFactory()
	sf := factory.CreateStructFinder(goCfg, "", true, false)
	res, err := sf.FindStructuresInLines(strings.Split(code, "\n"), 1, "t.go")
	if err != nil {
		t.Fatalf("FindStructuresInLines() error = %v", err)
	}

	got := map[string][2]int{}
	for _, ty := range res.Types {
		got[ty.Name] = [2]int{ty.Start, ty.End}
	}

	for name, want := range map[string][2]int{
		"testRequests": {3, 6}, "Counts": {8, 10}, "After": {12, 12},
	} {
		if got[name] != want {
			t.Errorf("type %q span = %v, want %v (all: %v)", name, got[name], want, got)
		}
	}
}

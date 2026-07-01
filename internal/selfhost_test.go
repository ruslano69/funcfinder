package internal

import (
	"strings"
	"testing"
)

// Regressions found by running funcfinder on its own source ("the compiler must
// compile itself"): Go generics and composite defined types were missed.

// Generic functions and methods carry a `[type params]` list between the name
// and the argument list. The func pattern must skip it (non-capturing) and still
// report the bare name.
func TestGoFinder_GenericFunctions(t *testing.T) {
	cfg := getGoConfig(t)
	code := `package p

func GenericFunction[T any](value T) T {
	return value
}

func ComplexGenericFunction[K comparable, V any](m map[K]V, key K) (V, bool) {
	v, ok := m[key]
	return v, ok
}

func (r *Repo) Load[T any](id int) T {
	var z T
	return z
}

func Plain(x int) int {
	return x
}
`
	f := NewFinder(cfg, nil, true, false, false)
	res, err := f.FindFunctionsInLines(strings.Split(code, "\n"), 1, "t.go")
	if err != nil {
		t.Fatalf("FindFunctionsInLines() error = %v", err)
	}
	got := map[string]bool{}
	for _, fn := range res.Functions {
		got[fn.Name] = true
	}
	for _, name := range []string{"GenericFunction", "ComplexGenericFunction", "Load", "Plain"} {
		if !got[name] {
			t.Errorf("generic function %q not found (funcs: %v)", name, got)
		}
	}
}

// Defined types whose underlying type is a composite that *contains* braces
// (`map[...]struct{}`, `map[...]interface{}`) are single-line `named` types. The
// `named` pattern must allow interior balanced braces while still refusing a
// trailing unmatched `{` (which means a multi-line struct/interface body).
func TestGoStructFinder_CompositeDefinedTypes(t *testing.T) {
	goCfg := getGoConfig(t)
	code := `package main

type ShardGraph map[string]map[string]struct{}

type JSONOutput map[string]map[string]interface{}

type Config struct {
	Port int
}
`
	factory := NewStructFinderFactory()
	sf := factory.CreateStructFinder(goCfg, "", true, false)
	res, err := sf.FindStructuresInLines(strings.Split(code, "\n"), 1, "t.go")
	if err != nil {
		t.Fatalf("FindStructuresInLines() error = %v", err)
	}
	got := map[string]int{}
	for _, ty := range res.Types {
		got[ty.Name] = ty.Start
	}
	for name, wantStart := range map[string]int{
		"ShardGraph": 3, "JSONOutput": 5, "Config": 7,
	} {
		if got[name] != wantStart {
			t.Errorf("type %q start = %d, want %d (all: %v)", name, got[name], wantStart, got)
		}
	}
	// The composite types must not swallow the real struct after them.
	for _, ty := range res.Types {
		if ty.Name == "Config" && ty.End != 9 {
			t.Errorf("Config span = %d-%d, want 7-9", ty.Start, ty.End)
		}
	}
}

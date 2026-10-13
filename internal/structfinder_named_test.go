package internal

import (
	"reflect"
	"strings"
	"testing"
)

// Go "defined types" — `type X string`, `type X func(...)`, `type X []byte`,
// `type X map[...]` — are not struct/interface/alias, so they were previously
// missed. The "named" struct pattern plus single-line closing fixes both
// detection and the swallow that a brace-less type otherwise causes.
func TestGoStructFinder_DefinedTypes(t *testing.T) {
	goCfg := getGoConfig(t)
	code := `package main

type SyncMode string
type RetryableFunc func(ctx int) error
type MultiStringFlag []string
type Mapping map[string]int

type Config struct {
	Port int
}

type Handler = Config
`
	factory := NewStructFinderFactory()
	sf := factory.CreateStructFinder(goCfg, "", true, false)
	res, err := sf.FindStructuresInLines(strings.Split(code, "\n"), 1, "t.go")
	if err != nil {
		t.Fatalf("FindStructuresInLines() error = %v", err)
	}

	got := map[string]int{} // name -> start line
	for _, ty := range res.Types {
		got[ty.Name] = ty.Start
	}

	// All five must be present — crucially Config (a real struct) must NOT be
	// swallowed by the brace-less defined types that precede it.
	for name, wantStart := range map[string]int{
		"SyncMode": 3, "RetryableFunc": 4, "MultiStringFlag": 5,
		"Mapping": 6, "Config": 8,
	} {
		if got[name] != wantStart {
			t.Errorf("type %q start = %d, want %d (all types: %v)", name, got[name], wantStart, got)
		}
	}
	// Handler is an alias for Config: a name for an existing type, not a type of its own.
	if _, ok := got["Handler"]; ok {
		t.Errorf("alias Handler listed as a type (all types: %v)", got)
	}

	// Config must still span its braces (8-10), proving the preceding defined
	// types closed immediately rather than leaving the parser "open".
	for _, ty := range res.Types {
		if ty.Name == "Config" && ty.End != 10 {
			t.Errorf("Config span = %d-%d, want 8-10", ty.Start, ty.End)
		}
	}
}

// A Go alias (`type A = S`, or a line of a `type (...)` group) is a name for an existing type, not a new one:
// one struct with several names is one struct. Only S and the defined type E are types here.
func TestGoStructFinder_AliasIsNotAType(t *testing.T) {
	goCfg := getGoConfig(t)
	code := `package alias

type S struct{ X int }

type A = S

type (
	B = S
	C = S
	D = int
)

type E int
`
	factory := NewStructFinderFactory()
	sf := factory.CreateStructFinder(goCfg, "", true, false)
	res, err := sf.FindStructuresInLines(strings.Split(code, "\n"), 1, "alias.go")
	if err != nil {
		t.Fatalf("FindStructuresInLines() error = %v", err)
	}

	var names []string
	for _, ty := range res.Types {
		names = append(names, ty.Name)
	}
	want := []string{"S", "E"}
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("types = %v, want %v", names, want)
	}
}

// An inline struct/interface with balanced braces on one line (`type X struct{}`,
// `struct{ io.Writer }`) must close on its own line. Otherwise it stays "open"
// (braceCount nets to 0, looking like a multi-line signature) and swallows the
// next type declaration — the residual category the ruler surfaced on tdtp.
func TestGoStructFinder_InlineBraces_NoSwallow(t *testing.T) {
	goCfg := getGoConfig(t)
	code := `package main

type MetadataService struct{}

type ColumnInfo struct {
	Name string
}

type nopWriteCloser struct{ io.Writer }

type nopReadCloser struct{ io.Reader }

type Empty interface{}

type Stringer interface{ String() string }
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

	// The ones after an inline struct are the canaries: if the inline struct
	// stayed open, ColumnInfo / nopReadCloser / Stringer would be swallowed.
	for name, wantStart := range map[string]int{
		"MetadataService": 3, "ColumnInfo": 5, "nopWriteCloser": 9,
		"nopReadCloser": 11, "Empty": 13, "Stringer": 15,
	} {
		if got[name] != wantStart {
			t.Errorf("type %q start = %d, want %d (all: %v)", name, got[name], wantStart, got)
		}
	}
}

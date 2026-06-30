package internal

import (
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

	// All six must be present — crucially Config (a real struct) must NOT be
	// swallowed by the brace-less defined types that precede it.
	for name, wantStart := range map[string]int{
		"SyncMode": 3, "RetryableFunc": 4, "MultiStringFlag": 5,
		"Mapping": 6, "Config": 8, "Handler": 12,
	} {
		if got[name] != wantStart {
			t.Errorf("type %q start = %d, want %d (all types: %v)", name, got[name], wantStart, got)
		}
	}

	// Config must still span its braces (8-10), proving the preceding defined
	// types closed immediately rather than leaving the parser "open".
	for _, ty := range res.Types {
		if ty.Name == "Config" && ty.End != 10 {
			t.Errorf("Config span = %d-%d, want 8-10", ty.Start, ty.End)
		}
	}
}

package internal

import (
	"strings"
	"testing"
)

// Experiment: Go func_pattern/class_pattern/struct_type_patterns now use
// Unicode-aware name classes ([\p{L}\p{Nd}_]) instead of ASCII-only \w, so
// non-ASCII identifiers (which Go permits) are detected.

func TestGoFinder_UnicodeIdentifiers(t *testing.T) {
	goCfg := getGoConfig(t)

	code := `package main

type Конфиг struct {
	Порт int
}

type Café interface {
	Open()
}

func Привет() string {
	return "привет"
}

func (c *Конфиг) Старт() {
	Привет()
}

func ASCIIName() {}
`
	lines := strings.Split(code, "\n")

	t.Run("functions", func(t *testing.T) {
		finder := NewFinder(goCfg, nil, true, false, false)
		res, err := finder.FindFunctionsInLines(lines, 1, "u.go")
		if err != nil {
			t.Fatalf("FindFunctionsInLines() error = %v", err)
		}
		found := map[string]bool{}
		for _, fn := range res.Functions {
			found[fn.Name] = true
		}
		for _, want := range []string{"Привет", "Старт", "ASCIIName"} {
			if !found[want] {
				t.Errorf("function %q not detected; got %v", want, found)
			}
		}
	})

	t.Run("types", func(t *testing.T) {
		factory := NewStructFinderFactory()
		sf := factory.CreateStructFinder(goCfg, "", true, false)
		res, err := sf.FindStructuresInLines(lines, 1, "u.go")
		if err != nil {
			t.Fatalf("FindStructuresInLines() error = %v", err)
		}
		found := map[string]bool{}
		for _, ty := range res.Types {
			found[ty.Name] = true
		}
		for _, want := range []string{"Конфиг", "Café"} {
			if !found[want] {
				t.Errorf("type %q not detected; got %v", want, found)
			}
		}
	})
}

// callgraph must now resolve call edges between Unicode-named functions, using
// the shared identifier classes (its old hardcoded ASCII regex + \b missed them).
func TestCallGraph_UnicodeIdentifiers(t *testing.T) {
	goCfg := getGoConfig(t)
	code := "package main\n\nfunc Старт() {}\n\nfunc Привет() {\n\tСтарт()\n}\n"
	tmpfile := createTempFile(t, code, "test_callgraph_unicode_*.go")

	fcg, err := BuildFileCallGraph(tmpfile, goCfg, nil, nil)
	if err != nil {
		t.Fatalf("BuildFileCallGraph() error = %v", err)
	}
	if !hasEdge(fcg, "Привет", "Старт") {
		t.Errorf("expected edge Привет -> Старт, got %+v", fcg.Calls)
	}
}

// Guard against regressions: a purely-ASCII file must produce exactly the same
// functions as before the Unicode change.
func TestGoFinder_ASCIIUnchanged(t *testing.T) {
	goCfg := getGoConfig(t)
	code := `package main

func Foo() {}

func (s *Server) Bar(x int) error {
	return nil
}

func baz_123() {}
`
	lines := strings.Split(code, "\n")
	finder := NewFinder(goCfg, nil, true, false, false)
	res, err := finder.FindFunctionsInLines(lines, 1, "a.go")
	if err != nil {
		t.Fatalf("FindFunctionsInLines() error = %v", err)
	}
	got := map[string]bool{}
	for _, fn := range res.Functions {
		got[fn.Name] = true
	}
	want := []string{"Foo", "Bar", "baz_123"}
	if len(got) != len(want) {
		t.Errorf("got %d functions %v, want exactly %v", len(got), got, want)
	}
	for _, w := range want {
		if !got[w] {
			t.Errorf("ASCII function %q missing; got %v", w, got)
		}
	}
}

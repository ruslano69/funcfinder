package internal

import (
	"strings"
	"testing"
)

func TestSharedLineFunctionDiscovery(t *testing.T) {
	cases := []struct{ lang, code string }{
		{"go", "package demo; func Alpha() int { return 1 }; func Beta() int { return 2 }"},
		{"c", "int Alpha(void) { return 1; } int Beta(void) { return 2; }"},
		{"cpp", "int Alpha() { return 1; } int Beta() { return 2; }"},
		{"d", "int Alpha() { return 1; } int Beta() { return 2; }"},
		{"rust", "fn Alpha() -> i32 { 1 } fn Beta() -> i32 { 2 }"},
		{"swift", "func Alpha() -> Int { return 1 }; func Beta() -> Int { return 2 }"},
		{"kotlin", "fun Alpha(): Int { return 1 }; fun Beta(): Int { return 2 }"},
		{"php", "<?php function Alpha() { return 1; } function Beta() { return 2; }"},
		{"ruby", "def Alpha; 1; end; def Beta; 2; end"},
		{"cpp", "class Demo { int Alpha() { return 1; } int Beta() { return 2; } };"},
		{"d", "class Demo { int Alpha() { return 1; } int Beta() { return 2; } }"},
		{"go", "package demo; func Alpha() string { return \"(\" }; func Beta() string { return \"} func Hidden(){}\" }"},
		{"cpp", "int Alpha() { /* ( ; fake() { } */ return 1; } int Beta() { return 2; }"},
		{"ruby", "  def Alpha; 1; end; def Beta; 2; end"},
		{"d", "string Alpha() { return \"тест (; }\"; } int Beta() { return 2; }"},
		{"kotlin", "fun Alpha(): Int = 1; fun Beta(): Int = 2"},
		{"ruby", "def Alpha = 1; def Beta = 2"},
	}
	config, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range cases {
		t.Run(tc.lang+tc.code[:4], func(t *testing.T) {
			cfg, err := config.GetLanguageConfig(tc.lang)
			if err != nil {
				t.Fatal(err)
			}
			for _, selected := range []string{"", "Beta"} {
				f := NewFinder(cfg, ParseFuncNames(selected), selected == "", true, false)
				r, err := f.FindFunctionsInLines([]string{tc.code}, 17, "probe")
				if err != nil {
					t.Fatal(err)
				}
				want := []string{"Alpha", "Beta"}
				if selected != "" {
					want = []string{"Beta"}
				}
				if len(r.Functions) != len(want) {
					t.Fatalf("selected=%q: %+v", selected, r.Functions)
				}
				for i, fn := range r.Functions {
					if fn.Name != want[i] || fn.Start != 17 || fn.End != 17 || strings.Join(fn.Lines, "\n") != tc.code {
						t.Fatalf("wrong source bounds: %+v", fn)
					}
					if strings.HasPrefix(tc.code, "class Demo") && fn.ClassName != "Demo" {
						t.Fatalf("class scope lost: %+v", fn)
					}
				}
			}
		})
	}
}

func TestSharedGoResultTypeAndMethods(t *testing.T) {
	cfg := goEditConfig(t)
	code := []string{"package demo", "type T int; type U int", "func Alpha() interface{ Value() int } {", " return nil", "}; func (T) Beta() {}; func (U) Beta() {}"}
	r, err := NewFinder(cfg, nil, true, true, false).FindFunctionsInLines(code, 8, "probe.go")
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Functions) != 3 {
		t.Fatalf("wrong functions: %+v", r.Functions)
	}
	if r.Functions[0].Start != 10 || r.Functions[0].End != 12 {
		t.Fatalf("result type became body: %+v", r.Functions[0])
	}
	for _, fn := range r.Functions[1:] {
		if fn.Name != "Beta" || fn.Start != 12 || fn.End != 12 {
			t.Fatalf("method lost: %+v", fn)
		}
	}
}

func TestSharedLineMultilineAndLiterals(t *testing.T) {
	cfg := goEditConfig(t)
	code := []string{"package demo", "func Alpha() {", " _ = `}; func Fake() {};`", "}; func Beta() {", " for i:=0;i<2;i++ { _ = func() int { return i }() }", "}; func Gamma() {} // func Hidden() {}"}
	f := NewFinder(cfg, nil, true, true, false)
	r, err := f.FindFunctionsInLines(code, 1, "probe.go")
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Functions) != 3 {
		t.Fatalf("wrong functions: %+v", r.Functions)
	}
	for i, want := range []struct {
		name       string
		start, end int
	}{{"Alpha", 2, 4}, {"Beta", 4, 6}, {"Gamma", 6, 6}} {
		fn := r.Functions[i]
		if fn.Name != want.name || fn.Start != want.start || fn.End != want.end {
			t.Fatalf("wrong bounds: %+v", fn)
		}
	}
}

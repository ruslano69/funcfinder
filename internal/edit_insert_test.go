package internal

import (
	"strings"
	"testing"
)

// The new function in each fixture has the fixture's shape under a new name.
func insertFixtureFunc(fx editFixture) string {
	return strings.Replace(fx.repl, fx.name, fx.name+"Next", 1)
}

func TestInsertAfterFunction_EveryLanguage(t *testing.T) {
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	for _, fx := range editFixtures {
		t.Run(fx.lang, func(t *testing.T) {
			lc, err := cfg.GetLanguageConfig(fx.lang)
			if err != nil {
				t.Fatalf("config: %v", err)
			}
			added := insertFixtureFunc(fx)
			res, err := InsertAfterFunction(lc, []byte(fx.src), FuncTarget{Name: fx.name}, []byte(added))
			if err != nil {
				t.Fatalf("InsertAfterFunction: %v", err)
			}
			want := strings.Replace(fx.src, fx.old, fx.old+"\n\n"+added, 1)
			if string(res.Output) != want {
				t.Fatalf("output mismatch:\n%s\nwant:\n%s", res.Output, want)
			}
		})
	}
}

func TestInsertAfterFunction_RefusesWhatIsNotAFunctionOrBreaksBrackets(t *testing.T) {
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	for _, fx := range editFixtures {
		t.Run(fx.lang, func(t *testing.T) {
			lc, err := cfg.GetLanguageConfig(fx.lang)
			if err != nil {
				t.Fatalf("config: %v", err)
			}
			if _, err := InsertAfterFunction(lc, []byte(fx.src), FuncTarget{Name: fx.name}, []byte("// only a comment\n")); err == nil {
				t.Errorf("text with no function accepted")
			}
			if fx.braces {
				broken := strings.Replace(insertFixtureFunc(fx), "}", "", 1)
				if _, err := InsertAfterFunction(lc, []byte(fx.src), FuncTarget{Name: fx.name}, []byte(broken)); err == nil {
					t.Errorf("missing closing brace accepted")
				}
			}
		})
	}
}

func TestInsertAfterFunction_GoKeepsOtherDeclarations(t *testing.T) {
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	lc, err := cfg.GetLanguageConfig("go")
	if err != nil {
		t.Fatalf("config: %v", err)
	}
	src := "package demo\n\nfunc Add(a, b int) int {\n\treturn a + b\n}\n\nfunc keep() {}\n"
	res, err := InsertAfterFunction(lc, []byte(src), FuncTarget{Name: "Add"}, []byte("func Sub(a, b int) int {\n\treturn a - b\n}\n"))
	if err != nil {
		t.Fatalf("InsertAfterFunction: %v", err)
	}
	want := "package demo\n\nfunc Add(a, b int) int {\n\treturn a + b\n}\n\nfunc Sub(a, b int) int {\n\treturn a - b\n}\n\nfunc keep() {}\n"
	if string(res.Output) != want {
		t.Fatalf("output mismatch:\n%s\nwant:\n%s", res.Output, want)
	}
	// A new import or type is a change to the file's declarations, not an insertion after the function.
	if _, err := InsertAfterFunction(lc, []byte(src), FuncTarget{Name: "Add"}, []byte("type Pair struct{ A, B int }\n")); err == nil {
		t.Errorf("type inserted without refusal")
	}
}

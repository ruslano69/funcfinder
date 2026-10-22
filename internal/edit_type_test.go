package internal

import (
	"strings"
	"testing"
)

const goTypesSource = "package demo\n\ntype Point struct {\n\tX, Y int\n}\n\ntype Reader interface {\n\tRead() int\n}\n\ntype Pair struct{ A, B int }\n\nfunc keep() {}\n"

func goTypeConfig(t *testing.T) *LanguageConfig {
	t.Helper()
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	lc, err := cfg.GetLanguageConfig("go")
	if err != nil {
		t.Fatalf("config: %v", err)
	}
	return lc
}

func TestReplaceType_Go(t *testing.T) {
	lc := goTypeConfig(t)
	old := "type Point struct {\n\tX, Y int\n}"
	repl := "type Point struct {\n\tX, Y, Z int\n}"
	res, err := ReplaceFunction(lc, []byte(goTypesSource), FuncTarget{Name: "Point", Kind: "type"}, nil, []byte(repl))
	if err != nil {
		t.Fatalf("replace type: %v", err)
	}
	if want := strings.Replace(goTypesSource, old, repl, 1); string(res.Output) != want {
		t.Fatalf("output mismatch:\n%s", res.Output)
	}
}

func TestInsertAfterType_Go(t *testing.T) {
	lc := goTypeConfig(t)
	old := "type Point struct {\n\tX, Y int\n}"
	add := "type Size struct{ W, H int }"
	res, err := InsertAfterFunction(lc, []byte(goTypesSource), FuncTarget{Name: "Point", Kind: "type"}, []byte(add))
	if err != nil {
		t.Fatalf("insert after type: %v", err)
	}
	if want := strings.Replace(goTypesSource, old, old+"\n\n"+add, 1); string(res.Output) != want {
		t.Fatalf("output mismatch:\n%s", res.Output)
	}
}

func TestRemoveType_Go(t *testing.T) {
	lc := goTypeConfig(t)
	res, err := RemoveFunction(lc, []byte(goTypesSource), FuncTarget{Name: "Pair", Kind: "type"})
	if err != nil {
		t.Fatalf("remove type: %v", err)
	}
	if want := strings.Replace(goTypesSource, "type Pair struct{ A, B int }\n\n", "", 1); string(res.Output) != want {
		t.Fatalf("output mismatch:\n%s", res.Output)
	}
}

func TestTypeEdits_Refusals_Go(t *testing.T) {
	lc := goTypeConfig(t)
	// A rename changes the types in the file.
	if _, err := ReplaceFunction(lc, []byte(goTypesSource), FuncTarget{Name: "Point", Kind: "type"}, nil,
		[]byte("type Dot struct {\n\tX, Y int\n}")); err == nil {
		t.Errorf("renamed type accepted")
	}
	// A function in the replacement changes the functions in the file.
	if _, err := ReplaceFunction(lc, []byte(goTypesSource), FuncTarget{Name: "Point", Kind: "type"}, nil,
		[]byte("type Point struct {\n\tX, Y int\n}\n\nfunc New() Point { return Point{} }")); err == nil {
		t.Errorf("function added by a type edit accepted")
	}
	// A grouped member is removed without deleting the wrapper or its neighbor.
	grouped := "package demo\n\ntype (\n\tA int\n\tB string\n)\n"
	if res, err := RemoveFunction(lc, []byte(grouped), FuncTarget{Name: "A", Kind: "type"}); err != nil || !strings.Contains(string(res.Output), "B string") {
		t.Errorf("grouped type removal failed: %v", err)
	}
	// A shared-line type owns only its declaration and explicit separator.
	shared := "package demo\n\ntype Pair struct{ A, B int }; type Q int\n"
	if res, err := RemoveFunction(lc, []byte(shared), FuncTarget{Name: "Pair", Kind: "type"}); err != nil || string(res.Output) != "package demo\n\n type Q int\n" {
		t.Errorf("shared type removal failed: %v", err)
	}
	// The go/ast check still applies: an unbalanced brace is refused.
	if _, err := ReplaceFunction(lc, []byte(goTypesSource), FuncTarget{Name: "Point", Kind: "type"}, nil,
		[]byte("type Point struct {\n\tX, Y int\n")); err == nil {
		t.Errorf("unbalanced type accepted")
	}
}

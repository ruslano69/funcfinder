package internal

import (
	"strings"
	"testing"
)

func TestGoTypeASTEdits(t *testing.T) {
	cfg := goTypeConfig(t)
	for _, decl := range []string{"type Box[T any] struct { Value T }", "type Box = int", "type Box interface { Read() int }"} {
		for _, grouped := range []bool{false, true} {
			old := decl
			replacement := "type Box struct { Ready bool }"
			prefix, suffix := "package demo\n", "; type Keep int\nfunc keep() {}\n"
			if grouped {
				old = strings.TrimPrefix(decl, "type ")
				prefix += "type (\n "
				suffix = "; Keep int\n)\nfunc keep() {}\n"
			}
			src := prefix + old + suffix
			want := prefix + replacement + suffix
			if grouped {
				want = prefix + strings.TrimPrefix(replacement, "type ") + suffix
			}
			r, err := ReplaceFunction(cfg, []byte(src), FuncTarget{Name: "Box", Kind: "type"}, []byte(old), []byte(replacement))
			if err != nil {
				t.Fatal(err)
			}
			if string(r.Output) != want {
				t.Fatalf("neighbor changed: %s", r.Output)
			}
			if _, err := ReplaceFunction(cfg, []byte(src), FuncTarget{Name: "Box", Kind: "type"}, []byte("stale"), []byte(replacement)); err == nil {
				t.Fatal("stale guard bypassed")
			}
			if _, err := RemoveFunction(cfg, []byte(src), FuncTarget{Name: "Box", Kind: "type", Line: 999}); err == nil {
				t.Fatal("line ignored")
			}
			r, err = InsertAfterFunction(cfg, []byte(src), FuncTarget{Name: "Box", Kind: "type"}, []byte("type Added int"))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(r.Output), "Added int") || !strings.Contains(string(r.Output), "Keep int") {
				t.Fatal("insertion lost declarations")
			}
			for _, bad := range []string{"type Extra int\nvar injected = 1", "type Extra int\nfunc injected() {}", "type Keep int"} {
				if _, err := InsertAfterFunction(cfg, []byte(src), FuncTarget{Name: "Box", Kind: "type"}, []byte(bad)); err == nil {
					t.Fatalf("accepted %q", bad)
				}
			}
		}
	}
}

func TestGoTypeASTPreservesTrivia(t *testing.T) {
	cfg := goTypeConfig(t)
	for _, eol := range []string{"\n", "\r\n"} {
		src := "\xef\xbb\xbfpackage demo" + eol + "type ( A int /* ; keep */; B string )" + eol
		r, err := RemoveFunction(cfg, []byte(src), FuncTarget{Name: "A", Kind: "type"})
		if err != nil {
			t.Fatal(err)
		}
		if string(r.Output) != strings.Replace(src, "A int /* ; keep */;", " /* ; keep */", 1) {
			t.Fatalf("trivia changed: %q", r.Output)
		}
	}
	all, _ := LoadConfig()
	d, _ := all.GetLanguageConfig("d")
	if _, err := ReplaceFunction(d, []byte("struct A {}"), FuncTarget{Name: "A", Kind: "type"}, nil, []byte("struct A { int x; }")); err == nil {
		t.Fatal("non-Go type edit accepted")
	}
}

func TestGoTypeASTGroupInsertion(t *testing.T) {
	cfg := goTypeConfig(t)
	src := []byte("package demo\ntype ( A int; Keep string )\n")
	r, err := InsertAfterFunction(cfg, src, FuncTarget{Name: "A", Kind: "type"}, []byte("type X int\ntype Y string"))
	if err != nil {
		t.Fatal(err)
	}
	if string(r.Output) != "package demo\ntype ( A int; X int; Y string; Keep string )\n" {
		t.Fatalf("group wrapper changed: %s", r.Output)
	}
	if _, err := ReplaceFunction(cfg, src, FuncTarget{Name: "A", Kind: "type"}, nil, []byte("// documentation\ntype A string")); err == nil {
		t.Fatal("declaration comment silently dropped")
	}
	if _, err := InsertAfterFunction(cfg, src, FuncTarget{Name: "A", Kind: "type"}, []byte("type X int\ntype X string")); err == nil {
		t.Fatal("duplicate payload names accepted")
	}
}

func TestGoTypeASTPhysicalLine(t *testing.T) {
	src := []byte("package demo\n//line generated.go:900\ntype A int\n")
	r, err := ReplaceFunction(goTypeConfig(t), src, FuncTarget{Name: "A", Kind: "type", Line: 3}, []byte("type A int"), []byte("type A int64"))
	if err != nil {
		t.Fatal(err)
	}
	if r.Start != 3 || r.End != 3 || string(r.Output) != "package demo\n//line generated.go:900\ntype A int64\n" {
		t.Fatalf("wrong physical span: %+v", r)
	}
}

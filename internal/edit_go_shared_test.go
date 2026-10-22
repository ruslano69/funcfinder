package internal

import (
	"strings"
	"testing"
)

func TestGoSharedEdit(t *testing.T) {
	cfg := goEditConfig(t)
	for _, eol := range []string{"\n", "\r\n"} {
		for _, prefix := range []string{"", "\xef\xbb\xbf"} {
			src := prefix + "package demo" + eol + `func A() string { return ";{}" }; /* keep ; } */ func B() int { for i:=0;i<1;i++ { _=func()int{return 1}() }; return 2 }; func C() int { return 3 }`
			old := `func B() int { for i:=0;i<1;i++ { _=func()int{return 1}() }; return 2 }`
			res, err := ReplaceFunction(cfg, []byte(src), FuncTarget{Name: "B"}, []byte(old), []byte(`func B() int { return 4 }`))
			if err != nil {
				t.Fatal(err)
			}
			if string(res.Output) != strings.Replace(src, old, `func B() int { return 4 }`, 1) {
				t.Fatalf("neighbor changed: %s", res.Output)
			}
			if _, err := ReplaceFunction(cfg, []byte(src), FuncTarget{Name: "B"}, []byte(old+" stale"), []byte(`func B() int { return 4 }`)); err == nil {
				t.Fatal("accepted stale text")
			}
			res, err = RemoveFunction(cfg, []byte(src), FuncTarget{Name: "B"})
			if err != nil {
				t.Fatal(err)
			}
			if string(res.Output) != strings.Replace(src, old+";", "", 1) {
				t.Fatalf("remove changed neighbor: %s", res.Output)
			}
			res, err = InsertAfterFunction(cfg, []byte(src), FuncTarget{Name: "B"}, []byte(`func Added() int { return 5 }`))
			if err != nil {
				t.Fatal(err)
			}
			if string(res.Output) != strings.Replace(src, old+";", old+"; func Added() int { return 5 };", 1) {
				t.Fatalf("insert changed neighbor: %s", res.Output)
			}
		}
	}
}

func TestGoSharedEditBoundaries(t *testing.T) {
	cfg := goEditConfig(t)
	for _, name := range []string{"A", "B", "C"} {
		src := []byte("package demo; func A() {}; func B() {}; func C() {}\n")
		if _, err := RemoveFunction(cfg, src, FuncTarget{Name: name}); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
	src := []byte("package demo\ntype T int; func (t T) A() {}; func B() {}\n")
	if _, err := ReplaceFunction(cfg, src, FuncTarget{Name: "A"}, nil, []byte("func A() {}")); err == nil {
		t.Fatal("receiver changed")
	}
	src = []byte("package demo\nfunc A() {} /* separator ; } */; func B() {}\n")
	r, err := RemoveFunction(cfg, src, FuncTarget{Name: "A"})
	if err != nil {
		t.Fatal(err)
	}
	if string(r.Output) != "package demo\n /* separator ; } */ func B() {}\n" {
		t.Fatalf("comment lost: %s", r.Output)
	}
}

func TestGoSharedEditGuards(t *testing.T) {
	cfg := goEditConfig(t)
	src := []byte("package demo\nfunc A() {\n _ = `; }`\n}; func B() {}\n")
	if _, err := ReplaceFunction(cfg, src, FuncTarget{Name: "A"}, src[len("package demo\n"):], []byte("func A() {}")); err != nil {
		t.Fatal(err)
	}
	if _, err := InsertAfterFunction(cfg, src, FuncTarget{Name: "A"}, []byte("func B() {}")); err == nil {
		t.Fatal("duplicate accepted")
	}
	src = []byte("package demo; func A() {}\nfunc B() {}\n")
	r, err := RemoveFunction(cfg, src, FuncTarget{Name: "A"})
	if err != nil {
		t.Fatal(err)
	}
	if string(r.Output) != "package demo; \nfunc B() {}\n" {
		t.Fatalf("implicit newline removed: %q", r.Output)
	}
	src = []byte("package demo\ntype T int; type U int; func (T) A() {}; func (U) A() {}\n")
	if _, err := RemoveFunction(cfg, src, FuncTarget{Name: "A", Line: 2}); err == nil {
		t.Fatal("ambiguous method accepted")
	}
}

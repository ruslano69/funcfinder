package internal

import (
	"bytes"
	"strings"
	"testing"
)

func TestCodeObjectRoundTrip(t *testing.T) {
	for _, eol := range []string{"\n", "\r\n"} {
		src := []byte("\xef\xbb\xbfpackage demo" + eol + "func A() int { return 1 }; func B() int { return 2 }; var keep=3" + eol)
		o, err := ReadCodeObject("probe.go", "go", "function", "B", 0, 0, src)
		if err != nil {
			t.Fatal(err)
		}
		if o.Source != "func B() int { return 2 }" {
			t.Fatalf("neighbor leaked: %s", o.Source)
		}
		o.Source = "func B() int {\n return 20\n}"
		out, updated, err := ReplaceCodeObject(*o, src)
		if err != nil {
			t.Fatal(err)
		}
		want := bytes.Replace(src, []byte("func B() int { return 2 }"), []byte(strings.ReplaceAll(o.Source, "\n", eol)), 1)
		if !bytes.Equal(out, want) || updated.Revision == o.Revision || updated.EndLine != 4 {
			t.Fatalf("bad result: %q %+v", out, updated)
		}
		if _, _, err = ReplaceCodeObject(*o, out); err == nil {
			t.Fatal("stale object accepted")
		}
		unchanged, current, err := ReplaceCodeObject(*updated, out)
		if err != nil || !bytes.Equal(unchanged, out) || current.Revision != updated.Revision {
			t.Fatalf("read/save not idempotent: %v", err)
		}
	}
}

func TestCodeObjectIdentityAndAmbiguity(t *testing.T) {
	src := []byte("package demo\ntype T int; type U int; func (T) Save() {}; func (U) Save() {}\n")
	_, err := ReadCodeObject("probe.go", "go", "function", "Save", 0, 0, src)
	e, ok := err.(*ObjectError)
	if !ok || e.Code != "ambiguous" || len(e.Candidates) != 2 {
		t.Fatalf("no candidates: %v", err)
	}
	o := e.Candidates[1]
	o.Source = "func (U) Save() { println(1) }"
	if _, _, err = ReplaceCodeObject(o, src); err != nil {
		t.Fatal(err)
	}
	o.Source = "func (T) Save() {}"
	if _, _, err = ReplaceCodeObject(o, src); err == nil {
		t.Fatal("receiver changed")
	}
	o = e.Candidates[1]
	o.ID = "wrong"
	if _, _, err = ReplaceCodeObject(o, src); err == nil {
		t.Fatal("wrong identity accepted")
	}
	o = e.Candidates[1]
	o.Line++
	if _, _, err = ReplaceCodeObject(o, src); err == nil {
		t.Fatal("coordinates ignored")
	}
}

func TestCodeObjectTypesAndContext(t *testing.T) {
	src := []byte("package demo\ntype ( Box[T any] struct { Value T }; ID = int ); var keep=1\n")
	for _, name := range []string{"Box", "ID"} {
		o, err := ReadCodeObject("probe.go", "go", "type", name, 0, 0, src)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(o.Source, "type ") {
			t.Fatal("not a complete type declaration")
		}
		o.Source = "type " + name + " string"
		out, _, err := ReplaceCodeObject(*o, src)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Contains(out, []byte("type ( ")) || !bytes.HasSuffix(out, []byte(" ); var keep=1\n")) {
			t.Fatalf("group/context lost: %s", out)
		}
	}
	src = []byte("package demo\nfunc A() {}; var keep=1\n")
	o, err := ReadCodeObject("probe.go", "go", "function", "A", 0, 0, src)
	if err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"func A() {} //", "func A() {}; var injected=1", "func Renamed() {}", "not Go"} {
		o.Source = bad
		if _, _, err := ReplaceCodeObject(*o, src); err == nil {
			t.Fatalf("accepted %q", bad)
		}
	}
}

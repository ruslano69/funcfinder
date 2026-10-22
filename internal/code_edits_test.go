package internal

import (
	"bytes"
	"strings"
	"testing"
)

func editObject(t *testing.T, src []byte, kind, name string) CodeObject {
	t.Helper()
	o, err := ReadCodeObject("batch.go", "go", kind, name, 0, 0, src)
	if err != nil {
		t.Fatal(err)
	}
	return *o
}

func TestCodeEditTransaction(t *testing.T) {
	for _, eol := range []string{"\n", "\r\n"} {
		src := []byte("\xef\xbb\xbfpackage demo" + eol + "type ( A int; B string ); func Alpha() int { return 1 }; func Beta() {}; var Keep=3" + eol)
		a := editObject(t, src, "function", "Alpha")
		beta := editObject(t, src, "function", "Beta")
		typ := editObject(t, src, "type", "A")
		replaced := a
		replaced.Source = "func Alpha() int {\n return Helper()\n}"
		edits := []CodeEdit{{Operation: "remove", Object: beta}, {Operation: "insert_after", Object: typ, Source: "type Added int"}, {Operation: "replace", Object: replaced}, {Operation: "insert_after", Object: a, Source: "func Helper() int { return 42 }"}}
		out, results, err := PlanCodeEdits(edits, src)
		if err != nil {
			t.Fatal(err)
		}
		want := "\xef\xbb\xbfpackage demo" + eol + "type ( A int; Added int; B string ); func Alpha() int {" + eol + " return Helper()" + eol + "}; func Helper() int { return 42 };  var Keep=3" + eol
		if string(out) != want {
			t.Fatalf("wrong splice: %q", out)
		}
		if len(results) != 4 || len(results[0].Objects) != 0 || results[1].Objects[0].Name != "Added" || results[2].Objects[0].Name != "Alpha" || results[3].Objects[0].Name != "Helper" {
			t.Fatalf("wrong results: %+v", results)
		}
		for _, r := range results {
			for _, o := range r.Objects {
				if o.Revision != codeRevision(out) {
					t.Fatal("not the final batch revision")
				}
			}
		}
		if _, _, err := PlanCodeEdits(edits, out); err == nil {
			t.Fatal("stale transaction accepted")
		}
	}
}

func TestCodeEditRefusals(t *testing.T) {
	src := []byte("package demo\nfunc A() {}; func B() {}; var Keep=1\n")
	a := editObject(t, src, "function", "A")
	b := editObject(t, src, "function", "B")
	bad := b
	bad.Source = "func Renamed() {}"
	cases := [][]CodeEdit{
		{{Operation: "remove", Object: a}, {Operation: "replace", Object: bad}},
		{{Operation: "remove", Object: a}, {Operation: "replace", Object: a}},
		{{Operation: "replace", Object: a}, {Operation: "replace", Object: a}},
		{{Operation: "insert_after", Object: a, Source: "func Keep() {}"}},
		{{Operation: "insert_after", Object: a, Source: "func A() {}"}},
		{{Operation: "insert_after", Object: a, Source: "func New() {}; var injected=1"}},
		{{Operation: "insert_after", Object: a, Source: "func New() {} //"}},
		{{Operation: "remove", Object: a, Source: "ignored?"}},
		{{Operation: "unknown", Object: a}},
	}
	for _, edits := range cases {
		copy := bytes.Clone(src)
		out, _, err := PlanCodeEdits(edits, src)
		if err == nil || out != nil {
			t.Fatalf("accepted %+v", edits)
		}
		if !bytes.Equal(src, copy) {
			t.Fatal("input mutated")
		}
	}
	other := b
	other.Path = "another.go"
	if _, _, err := PlanCodeEdits([]CodeEdit{{Operation: "remove", Object: a}, {Operation: "remove", Object: other}}, src); err == nil {
		t.Fatal("multiple files accepted")
	}
}

func TestCodeEditRemovalCommentsAndEOF(t *testing.T) {
	src := []byte("package demo; func A() {} /* keep ; */; func B() {}")
	a := editObject(t, src, "function", "A")
	out, _, err := PlanCodeEdits([]CodeEdit{{Operation: "remove", Object: a}}, src)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "package demo;  /* keep ; */ func B() {}" {
		t.Fatalf("comment removed: %s", out)
	}
	b := editObject(t, out, "function", "B")
	out, results, err := PlanCodeEdits([]CodeEdit{{Operation: "insert_after", Object: b, Source: "func C() {}"}}, out)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(string(out), "func B() {}; func C() {};") || results[0].Objects[0].Name != "C" {
		t.Fatal("EOF insertion failed")
	}
}

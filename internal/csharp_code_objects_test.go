package internal

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func TestCSharpCodeObjectsRead(t *testing.T) {
	src := []byte("using System; namespace N; class C { [Obsolete(\"}\")] public int A() => 1; int B() { return 2; } int B(int x) => x; C() {} ~C() {} class Nested { int M() => 3; } }")
	path := filepath.Join(t.TempDir(), "sample.cs")
	items, err := csharpCodeItems(path, src, "", "module")
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, item := range items {
		if item.object.Kind != "" {
			names = append(names, item.object.Name)
		}
	}
	if strings.Join(names, ",") != "C,A,B,B,C,~C,Nested,M" {
		t.Fatal(names)
	}
	a, err := ReadCodeObject(path, "cs", "function", "A", 0, 0, src)
	if err != nil {
		t.Fatal(err)
	}
	if a.Source != "[Obsolete(\"}\")] public int A() => 1;" {
		t.Fatal(a.Source)
	}
	if a.Column != bytes.Index(src, []byte("[Obsolete"))+1 {
		t.Fatal(a.Column)
	}
	_, err = ReadCodeObject(path, "cs", "function", "B", 0, 0, src)
	ambiguous, ok := err.(*ObjectError)
	if !ok || ambiguous.Code != "ambiguous" || len(ambiguous.Candidates) != 2 {
		t.Fatalf("ambiguity: %#v", err)
	}
	for _, candidate := range ambiguous.Candidates {
		selected, err := ReadCodeObject(path, "cs", "function", "B", candidate.Line, candidate.Column, src)
		if err != nil || selected.Source != candidate.Source {
			t.Fatalf("selection: %#v %v", selected, err)
		}
	}
	for _, code := range []string{
		"class C { string A() => @\"a\"\"}b\"; int B() => 2; }",
		"class C { string A() => \"\"\"raw { } \\\" text\"\"\"; int B() => 2; }",
		"class C { string A() => $\"{string.Join(\"}\", new[]{\"x\"})}\"; int B() => 2; }",
		"class C { string A() => $$\"\"\"raw {{string.Join(\"}\", new[]{\"x\"})}} end\"\"\"; int B() => 2; }",
		"class C { char A() => '}'; int B() => 2; }",
		"class C { string A() => $@\"{{escaped}} {1}\"; int B() => 2; }",
	} {
		data := []byte(code)
		obj, err := ReadCodeObject(path, "cs", "function", "B", 0, 0, data)
		if err != nil || obj.Source != "int B() => 2;" {
			t.Fatalf("literal case %s: %#v %v", code, obj, err)
		}
	}
	data := []byte(utf8BOM + "// кириллица\r\nclass C { string note = \"😀\"; int A() => 1; int B() => 2; }\r\n")
	obj, err := ReadCodeObject(path, "cs", "function", "A", 0, 0, data)
	if err != nil {
		t.Fatal(err)
	}
	if obj.Column != bytes.Index(bytes.Split(data, []byte("\n"))[1], []byte("int A"))+1 {
		t.Fatal(obj.Column)
	}
	obj.Source = "int A() => 10;"
	out, _, err := ReplaceCodeObject(*obj, data)
	if err != nil || !bytes.Equal(out, bytes.Replace(data, []byte("int A() => 1;"), []byte("int A() => 10;"), 1)) {
		t.Fatalf("BOM/Unicode/CRLF: %s %v", out, err)
	}
}
func TestCSharpCodeObjectsTransactions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sample.cs")
	src := []byte("namespace N { class C { int field = 7; int P { get; set; } int A() => 1; int B() => 2; } }")
	read := func(kind, name string) CodeObject {
		t.Helper()
		obj, err := ReadCodeObject(path, "cs", kind, name, 0, 0, src)
		if err != nil {
			t.Fatal(err)
		}
		return *obj
	}
	a, b := read("function", "A"), read("function", "B")
	changed := a
	changed.Source = "int A() => Added();"
	edits := []CodeEdit{{Operation: "replace", Object: changed}, {Operation: "insert_after", Object: a, Source: "int Added() => 20;"}, {Operation: "remove", Object: b}}
	out, results, err := PlanCodeEdits(edits, src)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "namespace N { class C { int field = 7; int P { get; set; } int A() => Added();\nint Added() => 20;\n  } }" {
		t.Fatalf("neighbors: %s", out)
	}
	if len(results) != 3 || len(results[0].Objects) != 1 || len(results[1].Objects) != 1 || len(results[2].Objects) != 0 {
		t.Fatalf("results: %#v", results)
	}
	for _, result := range results {
		if result.Revision != codeRevision(out) {
			t.Fatal("old revision returned")
		}
	}
	for _, result := range results[:2] {
		if result.Objects[0].Revision != codeRevision(out) {
			t.Fatal("stale object returned")
		}
	}
	_, _, err = PlanCodeEdits([]CodeEdit{{Operation: "remove", Object: b}}, out)
	if object, ok := err.(*ObjectError); !ok || object.Code != "stale_revision" {
		t.Fatalf("stale: %v", err)
	}
	container := read("type", "C")
	_, _, err = PlanCodeEdits([]CodeEdit{{Operation: "remove", Object: container}, {Operation: "replace", Object: changed}}, src)
	if object, ok := err.(*ObjectError); !ok || object.Code != "overlapping_edits" {
		t.Fatalf("overlap: %v", err)
	}
	container.Source = "class C { public int Value => 42; }"
	typed, _, err := ReplaceCodeObject(container, src)
	if err != nil || !strings.Contains(string(typed), "namespace N { class C { public int Value => 42; } }") {
		t.Fatalf("type edit: %s %v", typed, err)
	}
	for _, declaration := range []string{"interface I { int M(); }", "record R(int X);", "struct S { public int X; }", "enum E { A, B }"} {
		data := []byte(declaration)
		items, err := csharpCodeItems(path, data, "", "module")
		if err != nil || len(items) == 0 || items[0].object.Kind != "type" || items[0].object.Source != declaration {
			t.Fatalf("type %s: %#v %v", declaration, items, err)
		}
	}
}
func TestCSharpCodeObjectsGuards(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sample.cs")
	src := []byte("class C { [Attr(\"x\")] int M(int x) => x; int M(string x) => 2; }")
	_, err := ReadCodeObject(path, "cs", "function", "M", 0, 0, src)
	object, ok := err.(*ObjectError)
	if !ok || len(object.Candidates) != 2 {
		t.Fatalf("overloads: %v", err)
	}
	m := object.Candidates[0]
	for _, source := range []string{"int M(int x) => x;", "[Attr(\"x\")] int Other(int x) => x;", "[Attr(\"x\")] int M(int x) {", "[Attr(\"x\")] int M(int x) => x; int Extra() => 1;", "[Attr(\"x\")] int M(int x) => x; // swallowed"} {
		edited := m
		edited.Source = source
		out, _, err := ReplaceCodeObject(edited, src)
		if err == nil || out != nil {
			t.Fatalf("accepted %q: %s", source, out)
		}
	}
	_, _, err = PlanCodeEdits([]CodeEdit{{Operation: "insert_after", Object: m, Source: "int M(int y) => y;"}}, src)
	if object, ok := err.(*ObjectError); !ok || object.Code != "duplicate_declaration" {
		t.Fatalf("duplicate overload: %v", err)
	}
	out, _, err := PlanCodeEdits([]CodeEdit{{Operation: "insert_after", Object: m, Source: "int M(double value) => 3;"}}, src)
	if err != nil || !strings.Contains(string(out), "M(double value)") {
		t.Fatalf("new overload: %s %v", out, err)
	}
	bad := m
	bad.Column++
	if _, _, err := ReplaceCodeObject(bad, src); err == nil {
		t.Fatal("forged coordinate accepted")
	}
	bad = m
	bad.Source = "[Attr(\"x\")] int M(int x) {"
	if out, _, err := PlanCodeEdits([]CodeEdit{{Operation: "remove", Object: object.Candidates[1]}, {Operation: "replace", Object: bad}}, src); err == nil || out != nil {
		t.Fatalf("partial transaction: %s %v", out, err)
	}
	for _, source := range []string{"#if DEBUG\nclass C {}\n#endif", "class C { string s = \"unterminated; }", "class C { (] }", "class C { /* unclosed }"} {
		if _, err := csharpCodeItems(path, []byte(source), "", "module"); err == nil {
			t.Fatalf("invalid source accepted: %s", source)
		}
	}
	if _, err := csharpCodeItems(path, []byte("#nullable enable\nclass C {}"), "", "module"); err != nil {
		t.Fatal(err)
	}
	// Balanced expression grammar is deliberately left to Roslyn/project checks.
	m.Source = "[Attr(\"x\")] int M(int x) => x + ;"
	if _, _, err := ReplaceCodeObject(m, src); err != nil {
		t.Fatalf("unexpected full-grammar claim: %v", err)
	}
}

func TestCSharpCodeObjectsScopeAndTypeOperations(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sample.cs")
	src := []byte("#nullable enable\nnamespace N { class C<T> { int M() => 1; } class Keep {} }")
	c, err := ReadCodeObject(path, "cs", "type", "C", 0, 0, src)
	if err != nil {
		t.Fatal(err)
	}
	keep, err := ReadCodeObject(path, "cs", "type", "Keep", 0, 0, src)
	if err != nil {
		t.Fatal(err)
	}
	out, _, err := PlanCodeEdits([]CodeEdit{{Operation: "insert_after", Object: *c, Source: "class C<T,U> { int M() => 2; }"}, {Operation: "remove", Object: *keep}}, src)
	if err != nil || !bytes.HasPrefix(out, []byte("#nullable enable\nnamespace N {")) || !strings.Contains(string(out), "class C<T,U>") || strings.Contains(string(out), "Keep") {
		t.Fatalf("type operations: %s %v", out, err)
	}
	_, _, err = PlanCodeEdits([]CodeEdit{{Operation: "insert_after", Object: *c, Source: "#nullable disable\nclass Added {}"}}, src)
	if err == nil {
		t.Fatal("directive outside insertion object accepted")
	}
	for _, data := range [][]byte{[]byte("class C {}\r\n\n"), []byte{'c', 0xff}} {
		if bytes.Contains(data, []byte("\r\n")) {
			obj, err := ReadCodeObject(path, "cs", "type", "C", 0, 0, data)
			if err != nil {
				t.Fatal(err)
			}
			if _, _, err := ReplaceCodeObject(*obj, data); err == nil {
				t.Fatal("mixed endings accepted")
			}
		} else if _, err := csharpCodeItems(path, data, "", "module"); err == nil {
			t.Fatal("malformed UTF-8 accepted")
		}
	}
	src = []byte("class C { int IFoo.M() => 1; int A() { int Local() => 2; return Local(); } }")
	m, err := ReadCodeObject(path, "cs", "function", "M", 0, 0, src)
	if err != nil {
		t.Fatal(err)
	}
	m.Source = "int IBar.M() => 1;"
	if _, _, err := ReplaceCodeObject(*m, src); err == nil {
		t.Fatal("explicit interface identity changed")
	}
	if _, err := ReadCodeObject(path, "cs", "function", "Local", 0, 0, src); err == nil {
		t.Fatal("opaque local function exposed")
	}
}

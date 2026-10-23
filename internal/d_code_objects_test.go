package internal

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func TestDCodeObjectsRead(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sample.d")
	src := []byte("module sample; import std.stdio; /+ outer /+ nested +/ comment +/ struct Box { @safe int add(int x) { return x; } int add(string x) { return 2; } this(int x) {} ~this() {} struct Nested { int value() { return 3; } } } alias Number = int; enum Mode { a, b }")
	items, err := dCodeItems(path, src, "", "module")
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, item := range items {
		if item.object.Kind != "" {
			names = append(names, item.object.Name)
		}
	}
	if strings.Join(names, ",") != "Box,add,add,this,~this,Nested,value,Number,Mode" {
		t.Fatal(names)
	}
	_, err = ReadCodeObject(path, "d", "function", "add", 0, 0, src)
	ambiguous, ok := err.(*ObjectError)
	if !ok || ambiguous.Code != "ambiguous" || len(ambiguous.Candidates) != 2 {
		t.Fatalf("overloads: %#v", err)
	}
	for _, candidate := range ambiguous.Candidates {
		obj, err := ReadCodeObject(path, "d", "function", "add", candidate.Line, candidate.Column, src)
		if err != nil || obj.Source != candidate.Source {
			t.Fatalf("selector: %#v %v", obj, err)
		}
	}
	for _, code := range []string{
		"string f() { return `} fake() {`; } int keep() { return 2; }",
		"string f() { return r\"} fake() {\"; } int keep() { return 2; }",
		"string f() { return q{ int fake() { /+ } /+ { +/ +/ return 1; } }; } int keep() { return 2; }",
		"string f() { return q\"[ nested [braces] } { ]\"; } int keep() { return 2; }",
		"string f() { return q\"TAG\n} fake() {\nTAG\"; } int keep() { return 2; }",
		"string f() { return q\"/ } fake() { /\"; } int keep() { return 2; }",
		"string f() { return x\"7D 7B\"; } int keep() { return 2; }",
		"char f() { return '}'; } int keep() { return 2; }",
	} {
		obj, err := ReadCodeObject(path, "d", "function", "keep", 0, 0, []byte(code))
		if err != nil || obj.Source != "int keep() { return 2; }" {
			t.Fatalf("literal case %s: %#v %v", code, obj, err)
		}
	}
	for _, code := range []string{
		"int checked(int x) in { assert(x >= 0); } out(r) { assert(r >= 0); } body { return x; } int keep() { return 2; }",
		"int checked(int x) in(x >= 0) out(r; r >= 0) do { return x; } int keep() { return 2; }",
		"T checked(T)(T x) if(is(T == int)) { return x; } int keep() { return 2; }",
	} {
		data := []byte(code)
		obj, err := ReadCodeObject(path, "d", "function", "checked", 0, 0, data)
		if err != nil || obj.Source != strings.Split(code, " int keep")[0] {
			t.Fatalf("contracts/templates %s: %#v %v", code, obj, err)
		}
	}
	data := []byte(utf8BOM + "// пример\r\nstring note = \"😀\"; int first() { return 1; } int keep() { return 2; }\r\n")
	obj, err := ReadCodeObject(path, "d", "function", "first", 0, 0, data)
	if err != nil {
		t.Fatal(err)
	}
	if obj.Column != bytes.Index(bytes.Split(data, []byte("\n"))[1], []byte("int first"))+1 {
		t.Fatal(obj.Column)
	}
	obj.Source = "int first() { return 10; }"
	out, _, err := ReplaceCodeObject(*obj, data)
	if err != nil || !bytes.Equal(out, bytes.Replace(data, []byte("return 1;"), []byte("return 10;"), 1)) {
		t.Fatalf("byte preservation: %s %v", out, err)
	}
}
func TestDCodeObjectsTransactions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sample.d")
	src := []byte("module sample; struct Box { int field = 7; int first() { return 1; } int keep() { return 2; } } alias Count = int;")
	read := func(kind, name string) CodeObject {
		t.Helper()
		obj, err := ReadCodeObject(path, "d", kind, name, 0, 0, src)
		if err != nil {
			t.Fatal(err)
		}
		return *obj
	}
	a, b := read("function", "first"), read("function", "keep")
	edited := a
	edited.Source = "int first() { return added(); }"
	out, results, err := PlanCodeEdits([]CodeEdit{{Operation: "replace", Object: edited}, {Operation: "insert_after", Object: a, Source: "int added() { return 20; }"}, {Operation: "remove", Object: b}}, src)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(out), "module sample; struct Box { int field = 7; ") || !strings.HasSuffix(string(out), " } alias Count = int;") {
		t.Fatalf("neighbor changed: %s", out)
	}
	if len(results) != 3 || len(results[0].Objects) != 1 || len(results[1].Objects) != 1 || len(results[2].Objects) != 0 {
		t.Fatalf("results: %#v", results)
	}
	for _, result := range results {
		if result.Revision != codeRevision(out) {
			t.Fatal("old revision")
		}
	}
	for _, result := range results[:2] {
		if result.Objects[0].Revision != codeRevision(out) {
			t.Fatal("stale result object")
		}
	}
	if _, _, err := PlanCodeEdits([]CodeEdit{{Operation: "remove", Object: b}}, out); err == nil {
		t.Fatal("stale object accepted")
	}
	container := read("type", "Box")
	if _, _, err := PlanCodeEdits([]CodeEdit{{Operation: "remove", Object: container}, {Operation: "replace", Object: edited}}, src); err == nil {
		t.Fatal("container overlap accepted")
	}
	container.Source = "struct Box { int value; }"
	if out, _, err := ReplaceCodeObject(container, src); err != nil || !strings.Contains(string(out), "struct Box { int value; }") {
		t.Fatalf("type replacement: %s %v", out, err)
	}
	alias := read("type", "Count")
	originalAlias := alias
	alias.Source = "alias Count = long;"
	if _, _, err := ReplaceCodeObject(alias, src); err != nil {
		t.Fatal(err)
	}
	out, _, err = PlanCodeEdits([]CodeEdit{{Operation: "insert_after", Object: originalAlias, Source: "union Extra { int x; double y; }"}, {Operation: "remove", Object: read("type", "Box")}}, src)
	if err != nil || strings.Contains(string(out), "struct Box") || !strings.Contains(string(out), "union Extra") {
		t.Fatalf("type CRUD: %s %v", out, err)
	}
	for _, source := range []string{"public: int f() { return 1; } int keep() { return 2; }", "extern(C) { int f(); int keep(); }", "@safe { int f() { return 1; } int keep() { return 2; } }", "template Group(T) { T f(T x) { return x; } }"} {
		obj, err := ReadCodeObject(path, "d", "function", "f", 0, 0, []byte(source))
		if err != nil {
			t.Fatal(source, err)
		}
		if _, _, err := ReplaceCodeObject(*obj, []byte(source)); err != nil {
			t.Fatal(source, err)
		}
	}
}
func TestDCodeObjectsGuards(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sample.d")
	src := []byte("@safe int f(int x) { return x; } int keep() { return 2; }")
	obj, err := ReadCodeObject(path, "d", "function", "f", 0, 0, src)
	if err != nil {
		t.Fatal(err)
	}
	for _, source := range []string{"int f(int x) { return x; }", "@safe int other(int x) { return x; }", "@safe int f(int x) {", "@safe int f(int x) { return x; } int extra() { return 3; }", "@safe int f(int x) { return x; } // swallow neighbor"} {
		edit := *obj
		edit.Source = source
		if out, _, err := ReplaceCodeObject(edit, src); err == nil || out != nil {
			t.Fatalf("accepted %q: %s", source, out)
		}
	}
	_, _, err = PlanCodeEdits([]CodeEdit{{Operation: "insert_after", Object: *obj, Source: "int f(int y) { return y; }"}}, src)
	if object, ok := err.(*ObjectError); !ok || object.Code != "duplicate_declaration" {
		t.Fatalf("duplicate: %v", err)
	}
	if _, _, err := PlanCodeEdits([]CodeEdit{{Operation: "insert_after", Object: *obj, Source: "int f(string x) { return 3; }"}}, src); err != nil {
		t.Fatal("overload", err)
	}
	bad := *obj
	bad.Source = "@safe int f(int x) {"
	keep, _ := ReadCodeObject(path, "d", "function", "keep", 0, 0, src)
	if out, _, err := PlanCodeEdits([]CodeEdit{{Operation: "remove", Object: *keep}, {Operation: "replace", Object: bad}}, src); err == nil || out != nil {
		t.Fatalf("partial transaction: %s %v", out, err)
	}
	bad = *obj
	bad.Column++
	if _, _, err := ReplaceCodeObject(bad, src); err == nil {
		t.Fatal("forged identity accepted")
	}
	for _, source := range []string{"version(Windows) { int f() { return 1; } }", "static if(true) { int f() { return 1; } }", "mixin(\"int f() {}\");", "int f() { /+ unclosed }", "int f() { return q{ unclosed; }", "int f() { return q\"[unclosed\"; }", "#line 1 \"test.d\"\nint f() {}"} {
		if _, err := dCodeItems(path, []byte(source), "", "module"); err == nil {
			t.Fatal("unsupported source accepted:", source)
		}
	}
	// Delimiter validation is not D expression grammar or semantic checking.
	bad = *obj
	bad.Source = "@safe int f(int x) { return x + ; }"
	if _, _, err := ReplaceCodeObject(bad, src); err != nil {
		t.Fatalf("unexpected grammar claim: %v", err)
	}
}

func TestDCodeObjectsScopeAndEdges(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sample.d")
	for _, source := range []string{
		"int f() => 3; int keep() { return 2; }",
		"extern(C++) int f(int x); int keep() { return 2; }",
		"@property int f() const { return 3; } int keep() { return 2; }",
		"@(q{ } not code { }) int f() { return 3; } int keep() { return 2; }",
	} {
		obj, err := ReadCodeObject(path, "d", "function", "f", 0, 0, []byte(source))
		if err != nil {
			t.Fatal(source, err)
		}
		out, _, err := ReplaceCodeObject(*obj, []byte(source))
		if err != nil || string(out) != source {
			t.Fatalf("identity %s: %s %v", source, out, err)
		}
	}
	for _, data := range [][]byte{[]byte("int f() {}\r\n\n"), []byte{'i', 0xff}} {
		if bytes.Contains(data, []byte("\r\n")) {
			obj, err := ReadCodeObject(path, "d", "function", "f", 0, 0, data)
			if err != nil {
				t.Fatal(err)
			}
			if _, _, err := ReplaceCodeObject(*obj, data); err == nil {
				t.Fatal("mixed endings accepted")
			}
		} else if _, err := dCodeItems(path, data, "", "module"); err == nil {
			t.Fatal("invalid UTF-8 accepted")
		}
	}
	src := []byte("unittest { void helper() {} } int outer() { int inner() { return 1; } return inner(); }")
	for _, name := range []string{"helper", "inner"} {
		if _, err := ReadCodeObject(path, "d", "function", name, 0, 0, src); err == nil {
			t.Fatal("opaque body exposed", name)
		}
	}
	obj, err := ReadCodeObject(path, "d", "function", "outer", 0, 0, src)
	if err != nil {
		t.Fatal(err)
	}
	other := *obj
	other.Path += ".other"
	if out, _, err := PlanCodeEdits([]CodeEdit{{Operation: "remove", Object: *obj}, {Operation: "remove", Object: other}}, src); err == nil || out != nil {
		t.Fatal("cross-file batch accepted")
	}
	if _, _, err := PlanCodeEdits([]CodeEdit{{Operation: "insert_after", Object: *obj, Source: "public: int leaked() { return 1; }"}}, src); err == nil {
		t.Fatal("attribute section outside inserted object accepted")
	}
	if _, err := ReadCodeObject(path, "d", "type", "literal", 0, 0, []byte("enum literal = 42;")); err == nil {
		t.Fatal("manifest constant called a type")
	}
}

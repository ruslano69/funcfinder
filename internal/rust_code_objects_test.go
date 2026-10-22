package internal

import (
	"bytes"
	"strings"
	"testing"
)

const rustObjectFixture = `#![allow(dead_code)]
macro_rules! fake { () => { fn hidden() {} }; }
pub struct Data<T> { pub value: T }
pub enum Choice { A, B(u32) }
type Number = u32;
trait Read {
    fn read(&self) -> u32;
    fn defaulted(&self) -> u32 { 1 }
}
impl Data<u32> {
    #[inline]
    pub fn read(&self) -> u32 { self.value }
}
mod inner { pub fn same() {} }
mod other { pub fn same() {} }
unsafe extern "C" { safe fn external(value: u32) -> u32; }
pub fn text<'a>(value: &'a str) -> &'a str {
    /* outer /* nested fn ghost() {} */ comment */
    let raw = r###"}; fn not_a_function() {} /*"###;
    let byte = b'}'; let unicode = 'я'; let escaped = '\u{7b}';
    let _ = (raw, byte, unicode, escaped);
    value
}
fn r#match() {}
fn factory<F: Fn() -> u32>() -> impl Fn() -> u32 { || 1 }
`

func rustRead(t *testing.T, src []byte, kind, name string) CodeObject {
	t.Helper()
	o, err := ReadCodeObject("objects.rs", "rust", kind, name, 0, 0, src)
	if err != nil {
		t.Fatal(err)
	}
	return *o
}

func TestRustCodeObjectLexicalScopes(t *testing.T) {
	src := []byte(rustObjectFixture)
	for _, name := range []string{"text", "external", "r#match", "defaulted", "factory"} {
		o := rustRead(t, src, "function", name)
		if !strings.Contains(o.Source, "fn "+name) {
			t.Fatal(o.Source)
		}
	}
	for _, name := range []string{"hidden", "ghost", "not_a_function"} {
		if _, err := ReadCodeObject("objects.rs", "rust", "function", name, 0, 0, src); err == nil {
			t.Fatalf("false item %s", name)
		}
	}
	_, err := ReadCodeObject("objects.rs", "rust", "function", "read", 0, 0, src)
	e, ok := err.(*ObjectError)
	if !ok || e.Code != "ambiguous" || len(e.Candidates) != 2 {
		t.Fatalf("missing scoped candidates: %v", err)
	}
	o := e.Candidates[1]
	if !strings.HasPrefix(o.Source, "#[inline]") {
		t.Fatalf("attribute missing: %q", o.Source)
	}
	o.Source = strings.Replace(o.Source, "self.value", "self.value + 1", 1)
	out, updated, err := ReplaceCodeObject(o, src)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out, bytes.Replace(src, []byte("self.value }"), []byte("self.value + 1 }"), 1)) || updated.Revision == o.Revision {
		t.Fatal("wrong scoped replacement")
	}
	o.Source = "pub fn read(&self) -> u32 { 1 }"
	if _, _, err := ReplaceCodeObject(o, src); err == nil {
		t.Fatal("attribute silently removed")
	}
}

func TestRustCodeObjectTransaction(t *testing.T) {
	src := []byte("pub struct Data { pub value: u32 }\nfn old() {}\nfn value() -> u32 { 1 }\n")
	v := rustRead(t, src, "function", "value")
	old := rustRead(t, src, "function", "old")
	typ := rustRead(t, src, "type", "Data")
	changed := v
	changed.Source = "fn value() -> u32 { helper() }"
	updatedType := typ
	updatedType.Source = "pub struct Data { pub value: u32, pub ready: bool }"
	edits := []CodeEdit{{Operation: "replace", Object: changed}, {Operation: "insert_after", Object: v, Source: "fn helper() -> u32 { 42 }"}, {Operation: "remove", Object: old}, {Operation: "replace", Object: updatedType}}
	out, results, err := PlanCodeEdits(edits, src)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 4 || results[1].Objects[0].Name != "helper" || len(results[2].Objects) != 0 || !bytes.Contains(out, []byte("pub ready: bool")) {
		t.Fatalf("wrong batch: %s %+v", out, results)
	}
	for _, r := range results {
		if r.Revision != codeRevision(out) {
			t.Fatal("intermediate revision returned")
		}
	}
	if _, _, err := PlanCodeEdits(edits, out); err == nil {
		t.Fatal("stale Rust batch accepted")
	}
	if _, _, err := PlanCodeEdits([]CodeEdit{{Operation: "replace", Object: v}, {Operation: "remove", Object: v}}, src); err == nil {
		t.Fatal("overlap accepted")
	}
}

func TestRustCodeObjectTriviaAndRefusals(t *testing.T) {
	for _, eol := range []string{"\n", "\r\n"} {
		src := []byte("\xef\xbb\xbf// docs" + eol + "#[inline] fn first() {} fn second() {}" + eol)
		o := rustRead(t, src, "function", "second")
		o.Source = "fn second() {\n let имя = r#\"привет ; }\"#;\n let _ = имя;\n}"
		out, _, err := ReplaceCodeObject(o, src)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.HasPrefix(out, []byte("\xef\xbb\xbf// docs"+eol+"#[inline] fn first() {} ")) {
			t.Fatal("neighbor/BOM changed")
		}
		if eol == "\r\n" && bytes.Contains(bytes.ReplaceAll(out, []byte("\r\n"), nil), []byte("\n")) {
			t.Fatal("CRLF lost")
		}
	}
	src := []byte("fn a() {} fn b() {}")
	a := rustRead(t, src, "function", "a")
	for _, bad := range []string{"fn renamed() {}", "fn a() {} fn extra() {}", "fn a() {", "#![allow(dead_code)] fn a() {}"} {
		o := a
		o.Source = bad
		if _, _, err := ReplaceCodeObject(o, src); err == nil {
			t.Fatalf("accepted %q", bad)
		}
	}
	if _, _, err := PlanCodeEdits([]CodeEdit{{Operation: "insert_after", Object: a, Source: "fn b() {}"}}, src); err == nil {
		t.Fatal("duplicate accepted")
	}
}

func TestRustCodeObjectContainers(t *testing.T) {
	src := []byte("trait Read { fn read(&self) -> u32; }\nstruct Data;\n")
	trait := rustRead(t, src, "type", "Read")
	method := rustRead(t, src, "function", "read")
	newTrait := trait
	newTrait.Source = "trait Read { fn read(&self) -> u32 { 42 } }"
	out, updated, err := ReplaceCodeObject(newTrait, src)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Kind != "type" || updated.Name != "Read" || !bytes.HasSuffix(out, []byte("\nstruct Data;\n")) {
		t.Fatal("trait span incorrect")
	}
	if _, _, err := PlanCodeEdits([]CodeEdit{{Operation: "replace", Object: newTrait}, {Operation: "remove", Object: method}}, src); err == nil {
		t.Fatal("nested edits overlap")
	}
	bad := method
	bad.Source = "fn read(&self) -> u32; #![allow(dead_code)]"
	if _, _, err := ReplaceCodeObject(bad, src); err == nil {
		t.Fatal("inner attribute escaped item")
	}
}

func TestRustCodeObjectTypeForms(t *testing.T) {
	src := []byte("struct Tuple<T>(T); struct Unit; union Bits { number: u32, byte: u8 } enum Choice { A, B(u32) } type Callback = fn(u32) -> u32;")
	for _, name := range []string{"Tuple", "Unit", "Bits", "Choice", "Callback"} {
		o := rustRead(t, src, "type", name)
		out, updated, err := ReplaceCodeObject(o, src)
		if err != nil || !bytes.Equal(src, out) || updated.Name != name {
			t.Fatalf("%s roundtrip: %v", name, err)
		}
	}
	if _, err := ReadCodeObject("bad.rs", "rust", "function", "a", 0, 0, []byte("/* nested /* unclosed */ fn a() {}")); err == nil {
		t.Fatal("unclosed nested comment accepted")
	}
}

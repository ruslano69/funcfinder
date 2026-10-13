package internal

import (
	"reflect"
	"testing"
)

// rustFunctionNames returns the names of the functions the Rust finder reports for src, in file order.
func rustFunctionNames(t *testing.T, src string) []string {
	t.Helper()
	path := createTempFile(t, src, "rust_*.rs")
	res, err := CreateFinder(langConfigOf(t, "rust"), "", "map", false, false).FindFunctions(path)
	if err != nil {
		t.Fatalf("FindFunctions: %v", err)
	}
	var names []string
	for _, fn := range res.Functions {
		names = append(names, fn.Name)
	}
	return names
}

// A char literal whose content is a double quote, an escaped quote, a backslash or a multi-byte character
// must close on its own. Before, the quote opened a string that never closed: every function after it was lost.
func TestRustCharLiteralsDoNotOpenStrings(t *testing.T) {
	src := "pub(crate) fn q(c: char) -> char {\n" +
		"    match c {\n" +
		"        '\"' => '\\'',\n" +
		"        '\\\\' => '\\u{a0}',\n" +
		"        'À' => 'A',\n" +
		"        _ => c,\n" +
		"    }\n" +
		"}\n\n" +
		"pub fn after(c: char) -> char {\n" +
		"    c\n" +
		"}\n"
	got := rustFunctionNames(t, src)
	if want := []string{"q", "after"}; !reflect.DeepEqual(got, want) {
		t.Errorf("functions = %v, want %v", got, want)
	}
}

// A lifetime ('a) is code, not a char literal: a quote that does not close on its line starts nothing.
func TestRustLifetimesAreCode(t *testing.T) {
	src := "pub fn longest<'a>(x: &'a str, y: &'a str) -> &'a str {\n" +
		"    if x.len() > y.len() { x } else { y }\n" +
		"}\n\n" +
		"fn tail(c: char) -> char {\n" +
		"    c\n" +
		"}\n"
	got := rustFunctionNames(t, src)
	if want := []string{"longest", "tail"}; !reflect.DeepEqual(got, want) {
		t.Errorf("functions = %v, want %v", got, want)
	}
}

// Every visibility form of a Rust item is recognised: pub(crate), pub(super), pub(self), pub(in path), and plain pub.
func TestRustVisibilityForms(t *testing.T) {
	src := "pub(super) fn a() {}\n" +
		"pub(self) fn b() {}\n" +
		"pub(in crate::m) fn c() {}\n" +
		"pub(crate) fn d() {}\n" +
		"pub fn e() {}\n" +
		"fn f() {}\n"
	got := rustFunctionNames(t, src)
	if want := []string{"a", "b", "c", "d", "e", "f"}; !reflect.DeepEqual(got, want) {
		t.Errorf("functions = %v, want %v", got, want)
	}
}

// A raw string with more hashes (r##"..."##) can span lines and hold quotes and a double slash. Before, r## was not a
// raw opener at all: its quote opened an ordinary string, and the next quote on a later line closed that string, so the
// rest of the file was read as text. A plain r"..." is a raw string too, where a backslash does not escape the quote.
func TestRustRawStringsWithMoreHashes(t *testing.T) {
	src := "fn a() {\n" +
		"    let s = r##\"<svg\n" +
		"      <image href=\"http://x.y/z\"/>\n" +
		"    </svg>\"##;\n" +
		"}\n\n" +
		"pub fn b() {}\n\n" +
		"fn c() {\n" +
		"    let p = r\"\\\";\n" +
		"}\n" +
		"fn d() {}\n"
	got := rustFunctionNames(t, src)
	if want := []string{"a", "b", "c", "d"}; !reflect.DeepEqual(got, want) {
		t.Errorf("functions = %v, want %v", got, want)
	}
}

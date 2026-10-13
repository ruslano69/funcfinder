package internal

import (
	"reflect"
	"testing"
)

// A later declarator of a const list whose value is an arrow (`const id = ..., notification = (...) => ...`) is a
// function: the keyword that opened the list applies to it, as on its own line.
func TestDeclarations_TSSecondDeclaratorArrow(t *testing.T) {
	src := `const id = randomUUID(), notification = (actionUrl: string, meta: object) => ({ actionUrl, meta } as Note);
const after = 1;
`
	got := spans(mapFunctions(t, "ts", src))
	want := []string{"notification:1-1"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("functions = %v, want %v", got, want)
	}
}

// A later declarator with a block body runs over lines and ends at its closing brace; what follows it on that line
// (`}, other = 2;`) is another declarator, and the next declarator is not a function.
func TestDeclarations_TSSecondDeclaratorBlockBodyOverLines(t *testing.T) {
	src := `let a = 1, handler = async (x: number) => {
  return x + a;
}, other = 2;
`
	got := spans(mapFunctions(t, "ts", src))
	want := []string{"handler:1-3"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("functions = %v, want %v", got, want)
	}
}

// A declarator whose value is a value, or a comma inside a call's arguments, is not a function.
func TestDeclarations_TSSecondDeclaratorValueIsNotFunction(t *testing.T) {
	src := `const total = 1, sum = (1 + 2) * 3;
f(a, b = (x) => { return x; });
const pair = { a: 1, b: (x) => x };
`
	got := spans(mapFunctions(t, "ts", src))
	if len(got) != 0 {
		t.Fatalf("functions = %v, want none", got)
	}
}

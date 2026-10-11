package internal

import (
	"reflect"
	"testing"
)

// An arrow function bound to a const is a function whether or not async is followed by a
// space: async() and async () are the same declaration. Minified code writes it without.
func TestDeclarations_TSAsyncArrowWithoutSpace(t *testing.T) {
	src := `const load=async()=>{
  return 1;
};
const save = async (x: number) => {
  return x;
};
`
	got := spans(mapFunctions(t, "ts", src))
	want := []string{"load:1-3", "save:4-6"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("functions = %v, want %v", got, want)
	}
}

// A const annotated with a function type holds an arrow function: the => of the type is not
// the end of the annotation.
func TestDeclarations_TSTypedConstArrowFunction(t *testing.T) {
	src := `export const parse: (e: Error) => string = (e) => {
  return e.message;
};
`
	got := spans(mapFunctions(t, "ts", src))
	want := []string{"parse:1-3"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("functions = %v, want %v", got, want)
	}
}

// A method may take a parameter that is itself a function type, with its own parentheses.
func TestDeclarations_TSMethodWithFunctionTypedParameter(t *testing.T) {
	src := `class Bus {
  add(name: string, fn: (val: number) => any): this {
    return this;
  }
}
`
	got := spans(mapFunctions(t, "ts", src))
	want := []string{"add:2-4"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("functions = %v, want %v", got, want)
	}
}

// A method whose name is a string literal is a method; its name keeps the quotes, as the
// syntax tree has it.
func TestDeclarations_TSMethodNamedByStringLiteral(t *testing.T) {
	src := `class Bench {
  "=== INVALID"() {
    return 1;
  }
  'arktype (instanceof)'(x) {
    return x;
  }
}
`
	got := spans(mapFunctions(t, "ts", src))
	want := []string{`"=== INVALID":2-4`, "'arktype (instanceof)':5-7"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("functions = %v, want %v", got, want)
	}
}

package internal

import (
	"reflect"
	"testing"
)

// An abstract method has no body: its declaration ends in ';' and is listed on its own lines, as
// the C++ declarations are. A concrete method after them is unaffected.
func TestDeclarations_TSAbstractMethodsWithoutBody(t *testing.T) {
	src := `abstract class Base {
  abstract run(): void;
  abstract size(a: number, b?: string): number;
  abstract fetch(
    url: string
  ): Promise<void>;
  public abstract name(): string;
  concrete(): void {
  }
}
`
	got := spans(mapFunctions(t, "ts", src))
	want := []string{"concrete:8-9", "fetch:4-6", "name:7-7", "run:2-2", "size:3-3"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("functions = %v, want %v", got, want)
	}
}

// A property declared abstract, and an abstract getter, are not method declarations for this rule.
func TestDeclarations_TSAbstractPropertyIsNotAMethod(t *testing.T) {
	src := `abstract class Base {
  abstract name: string;
  abstract readonly id: number;
  run(): void {
  }
}
`
	got := spans(mapFunctions(t, "ts", src))
	want := []string{"run:4-5"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("functions = %v, want %v", got, want)
	}
}

// A signature in an interface may span lines: its first line ends at the parenthesis, and the
// signature ends with ';'. Its class is the interface.
func TestDeclarations_TSInterfaceSignatureSpansLines(t *testing.T) {
	src := `interface Ports {
  dismiss(
    id: string
  ): Promise<void>;
  count(): number;
}
`
	fns := mapFunctions(t, "ts", src)
	got := spans(fns)
	want := []string{"count:5-5", "dismiss:2-4"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("functions = %v, want %v", got, want)
	}
	for _, f := range fns {
		if f.ClassName != "Ports" {
			t.Errorf("%s: class = %q, want %q", f.Name, f.ClassName, "Ports")
		}
	}
}

// `const x = (...)` is a value when its parenthesis closes on the line and no arrow or return type
// follows it; an arrow inside the value (`.map((r) => r.id)`) does not make it a function.
func TestDeclarations_TSConstParenthesizedValueIsNotFunction(t *testing.T) {
	src := `const cos = (await db.query("x")).rows.map((r) => r.id);
const total = (1 + 2) * 3;
const add = (a, b) => a + b;
const multi = (
  a: number
) => a;
`
	got := spans(mapFunctions(t, "ts", src))
	want := []string{"add:3-3", "multi:4-6"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("functions = %v, want %v", got, want)
	}
}

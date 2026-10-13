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

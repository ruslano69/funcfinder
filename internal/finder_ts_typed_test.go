package internal

import (
	"reflect"
	"testing"
)

// A component whose type argument runs over lines (`const X: React.FC<{` ... `}> = (...) => (`) is a function.
// A declaration with the same type and a value (`= [`) is not.
func TestDeclarations_TSTypedDeclarationOverLines(t *testing.T) {
	src := `const Card: React.FC<{
  title: string;
  open?: boolean;
}> = ({ title }) => (
  <div>{title}</div>
);
const items: Item<{
  id: number;
}> = [
  { id: 1 },
];
const after = 1;
`
	got := spans(mapFunctions(t, "ts", src))
	want := []string{"Card:1-6"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("functions = %v, want %v", got, want)
	}
}

// A method whose return type runs over lines (`listUsers(): Promise<` ... `> {`) is a method of its class.
func TestDeclarations_TSMethodReturnTypeOverLines(t *testing.T) {
	src := `class Adapter {
  async listUsers(): Promise<
    Array<{ id: string }>
  > {
    return [];
  }
}
`
	fns := mapFunctions(t, "ts", src)
	got := spans(fns)
	want := []string{"listUsers:2-6"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("functions = %v, want %v", got, want)
	}
	if len(fns) == 1 && fns[0].ClassName != "Adapter" {
		t.Errorf("listUsers: class = %q, want %q", fns[0].ClassName, "Adapter")
	}
}

// A generic arrow (`const one = async <T = any>(...) => ...`): the '=' of the default is not the declaration's.
func TestDeclarations_TSGenericArrowWithDefaultTypeArgument(t *testing.T) {
	src := `const one = async <T = any>(sql: string, params: unknown[] = []) => (await db.query(sql, params)).rows;
const two = <T = any>(x: T) => x;
`
	got := spans(mapFunctions(t, "ts", src))
	want := []string{"one:1-1", "two:2-2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("functions = %v, want %v", got, want)
	}
}

// A typed component with a block body: the braces of its type argument are not the body, so the component ends
// with its own closing brace, not on the line where the body opens.
func TestDeclarations_TSTypedComponentWithBlockBody(t *testing.T) {
	src := `const Box: React.FC<{
  a: string;
}> = ({ a }) => {
  return a;
};
const after = 1;
`
	got := spans(mapFunctions(t, "ts", src))
	want := []string{"Box:1-5"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("functions = %v, want %v", got, want)
	}
}

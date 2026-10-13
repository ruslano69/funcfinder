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

// A regular expression />/ after '(' is not a JSX tag end, and the '/>' that closes a self-closing tag after an
// expression attribute is not a regular expression: both functions end on their own lines.
func TestDeclarations_TSSlashGreaterThan(t *testing.T) {
	src := `function esc(s: string) {
  return s.replace(/>/g, '&gt;');
}
function view(x: X) {
  return <input value={x}/>;
}
`
	got := spans(mapFunctions(t, "ts", src))
	want := []string{"esc:1-3", "view:4-6"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("functions = %v, want %v", got, want)
	}
}

// A generic call whose type argument is a literal runs over lines (`query<{` ... `}>(`): its braces are the
// type's, and the arrow's expression ends at its own `);`.
func TestDeclarations_TSGenericCallTypeLiteralOverLines(t *testing.T) {
	src := `export const loadRows = (companyId: string, year: number) => query<{
  id: string;
}>(
  "SELECT 1",
  [year, companyId]
);
const after = 1;
`
	got := spans(mapFunctions(t, "ts", src))
	want := []string{"loadRows:1-6"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("functions = %v, want %v", got, want)
	}
}

// A one-line method whose body closes on its line, followed by the closing brackets of an object literal
// (`}});`), is a function that ends on that line.
func TestDeclarations_TSOneLineMethodBeforeClosingBrackets(t *testing.T) {
	src := `const relay = new Server({
  onData(stream){stream.resume();}});
const other = 1;
`
	got := spans(mapFunctions(t, "ts", src))
	want := []string{"onData:2-2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("functions = %v, want %v", got, want)
	}
}

// A return type with braces (`): { a: number } {`) is not the body: the function ends with its own brace.
func TestDeclarations_TSReturnTypeBracesAreNotTheBody(t *testing.T) {
	src := `function pair(x: number): { a: number } {
  return { a: x };
}
`
	got := spans(mapFunctions(t, "ts", src))
	want := []string{"pair:1-3"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("functions = %v, want %v", got, want)
	}
}

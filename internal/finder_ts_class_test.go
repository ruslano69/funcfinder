package internal

import (
	"reflect"
	"testing"
)

// An abstract class is a class: its members belong to it, as they do for a plain class. The header
// variants below differ only in their modifiers; a multi-line method is found only inside a class body.
func TestDeclarations_TSClassHeaderModifiers(t *testing.T) {
	headers := []string{
		"class A {",
		"export class A {",
		"export default class A {",
		"abstract class A {",
		"export abstract class A {",
		"export default abstract class A {",
	}
	for _, header := range headers {
		src := header + `
  run(
    a: number
  ): void {
    return;
  }
}
`
		fns := mapFunctions(t, "ts", src)
		var got string
		for _, f := range fns {
			if f.Name == "run" {
				got = f.ClassName
			}
		}
		if got != "A" {
			t.Errorf("header %q: run's class = %q, want %q", header, got, "A")
		}
	}
}

// A class header may span lines before its body opens: the class must not end on a header line.
func TestDeclarations_TSClassHeaderSpansLines(t *testing.T) {
	src := `export class Box<
  T
> extends Base<T> {
  extend(
    a: number
  ): Box<T> {
    return this;
  }
}
`
	fns := mapFunctions(t, "ts", src)
	var got string
	for _, f := range fns {
		if f.Name == "extend" {
			got = f.ClassName
		}
	}
	if got != "Box" {
		t.Errorf("extend's class = %q, want %q", got, "Box")
	}
}

// A closing JSX tag (</h4>) is not a regular expression: the slash after '<' must not blank the code that
// follows it, so the component ends on its own closing brace.
func TestDeclarations_TSXClosingTagIsNotRegex(t *testing.T) {
	src := `export function Panel({ items }: { items: Item[] }) {
  return <div>
    <h4>{title}</h4>{items.map((i) => <tr key={i.id}><td>{i.name}</td></tr>)}
  </div>;
}
function after() {
  return 1;
}
`
	got := spans(mapFunctions(t, "ts", src))
	want := []string{"Panel:1-5", "after:6-8"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("functions = %v, want %v", got, want)
	}
}

// A component typed with a braced type argument (`React.FC<{ children: T }>`) keeps its body: the braces
// of the type are not the body, and the arrow's parenthesized JSX body ends at its own `);`.
func TestDeclarations_TSXTypedArrowComponentWithBracedTypeArgument(t *testing.T) {
	src := `const Shell: React.FC<{ children: React.ReactNode }> = ({ children }) => (
  <div className="a">
    <p>{children}</p>
  </div>
);
const after = 1;
`
	got := spans(mapFunctions(t, "ts", src))
	want := []string{"Shell:1-5"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("functions = %v, want %v", got, want)
	}
}

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

// A template literal may hold an expression, and the expression may hold another template
// literal. The inner backticks belong to the expression, not to the outer literal.
func TestDeclarations_TSTemplateExpressionHoldsNestedTemplate(t *testing.T) {
	src := `export function renderItems(items: string[]): string {
  return ` + "`<ul>${items.map((i) => `<li>${i}</li>`).join('')}</ul>`" + `;
}

export function printText(text: string): void {
  console.log(text);
}
`
	got := spans(mapFunctions(t, "ts", src))
	want := []string{"printText:5-7", "renderItems:1-3"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("functions = %v, want %v", got, want)
	}
}

// Strings inside a template expression do not end the template, and the braces around the
// expression are not braces of the function.
func TestDeclarations_TSTemplateExpressionWithStrings(t *testing.T) {
	src := `export function label(n: number): string {
  return ` + "`n=${n > 0 ? 'positive' : \"none\"}`" + `;
}

export function printText(text: string): void {
  console.log(text);
}
`
	got := spans(mapFunctions(t, "ts", src))
	want := []string{"label:1-3", "printText:5-7"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("functions = %v, want %v", got, want)
	}
}

// An arrow function whose header ends in => has its expression body on the next line: the
// header is not complete, even when its braces balance.
func TestDeclarations_TSArrowHeaderEndingInArrow(t *testing.T) {
	src := `const reservationEnded = (m?: { isReserved?: boolean } | null) =>
  !!m?.isReserved;
export function after(): number {
  return 1;
}
`
	got := spans(mapFunctions(t, "ts", src))
	want := []string{"after:3-5", "reservationEnded:1-2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("functions = %v, want %v", got, want)
	}
}

// A component whose parameters are destructured over several lines and whose body is a
// parenthesized expression: the body runs to the closing ); after the parameters close.
func TestDeclarations_TSArrowParenthesizedBodyAfterParameters(t *testing.T) {
	src := `const Section = ({
  title,
}) => (
  <section>{title}</section>
);

export function after() {
  return 1;
}
`
	got := spans(mapFunctions(t, "ts", src))
	want := []string{"Section:1-5", "after:7-9"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("functions = %v, want %v", got, want)
	}
}

// A brace inside a type argument is not the body: the component's arrow still opens a
// parenthesized body, which runs to its closing ); and the function is one declaration.
func TestDeclarations_TSBraceInTypeArgumentBeforeArrowBody(t *testing.T) {
	src := `const Shell: React.FC<{ children: string }> = (x) => (
  x
);

export function after() {
  return 1;
}
`
	got := spans(mapFunctions(t, "ts", src))
	want := []string{"Shell:1-3", "after:5-7"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("functions = %v, want %v", got, want)
	}
}

// A declaration can follow another statement on the same line: after a semicolon, the next
// const is a function of its own, and the line after it is not part of its body.
func TestDeclarations_TSFunctionAfterSemicolonOnSameLine(t *testing.T) {
	src := `let checks=0;const check=(ok:unknown,label:string)=>{assert.ok(ok,label);checks++;};
export function after(): void {
  check(true, 'x');
}
`
	got := spans(mapFunctions(t, "ts", src))
	want := []string{"after:2-4", "check:1-1"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("functions = %v, want %v", got, want)
	}
}

// A class member can follow a field on the same line, after a semicolon: an empty field, then a constructor.
func TestDeclarations_TSConstructorAfterFieldOnSameLine(t *testing.T) {
	src := `class Session {
  cookie='';constructor(public a: string){}
  get size(): number {
    return 1;
  }
}
`
	got := spans(mapFunctions(t, "ts", src))
	want := []string{"constructor:2-2", "size:3-5"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("functions = %v, want %v", got, want)
	}
}

// An object literal method can follow a property after a comma on the same line.
func TestDeclarations_TSMethodAfterPropertyOnSameLine(t *testing.T) {
	src := `const handlers = {name: 'x', async run(ctx) {
  return ctx;
}};
`
	got := spans(mapFunctions(t, "ts", src))
	want := []string{"run:1-3"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("functions = %v, want %v", got, want)
	}
}

// A body that closes early on a line is closed there, even when two sibling functions open
// after it on the same line: the line's net brace count is positive, its lowest depth is not.
func TestDeclarations_TSBodyClosesBeforeSiblingsOnSameLine(t *testing.T) {
	src := `export function outer(n: number) {
  return n;
}; export function first(n: number) { return n; }; export function second(n: number) {
  return n + 1;
}
`
	got := spans(mapFunctions(t, "ts", src))
	want := []string{"first:3-3", "outer:1-3", "second:3-5"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("functions = %v, want %v", got, want)
	}
}

// Minified code puts several functions on one line: the close of one body is followed by the
// headers of two more, and the first body still ends on this line.
func TestDeclarations_TSMinifiedBodyClosesBeforeTwoHeaders(t *testing.T) {
	src := `export function outer(n: number) {
  return n;}function a(n: number) {return n;}function b(n: number) {
  return n + 1;
}
`
	got := spans(mapFunctions(t, "ts", src))
	want := []string{"a:2-2", "b:2-4", "outer:1-2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("functions = %v, want %v", got, want)
	}
}

package internal

import (
	"reflect"
	"testing"
)

// Signatures without a body in an interface or a class: a parameter list that spans lines (its first line ends
// inside an object type), an optional method (`name?(...)`), and a return type that holds ';' in its braces.
func TestDeclarations_TSMemberSignatureShapes(t *testing.T) {
	src := `interface Ports {
  process(input: {
    id: string;
    amount: number;
  }): Promise<void>;
  permits?(client: Client, p: Principal): Promise<boolean>;
  cancel?(orderId: string): Promise<{ cancelled: boolean; timesheets: Row[] }>;
  count(): number;
}
`
	fns := mapFunctions(t, "ts", src)
	got := spans(fns)
	want := []string{"cancel:7-7", "count:8-8", "permits:6-6", "process:2-5"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("functions = %v, want %v", got, want)
	}
	for _, f := range fns {
		if f.ClassName != "Ports" {
			t.Errorf("%s: class = %q, want %q", f.Name, f.ClassName, "Ports")
		}
	}
}

// A value assigned inside a one-line block is not a function; an arrow assigned there is.
func TestDeclarations_TSInlineBlockValueIsNotFunction(t *testing.T) {
	src := `async function run(rows: Row[]) {
  try{const repo=(await resolve()).orders;if(!repo)throw new Error('x');}catch(e){}
  for(const p of rows){const payroll=(await query(p.id)).rows;}
  if(rows.length){const add=(a: number, b: number) => a + b;}
}
`
	got := spans(mapFunctions(t, "ts", src))
	want := []string{"add:4-4", "run:1-5"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("functions = %v, want %v", got, want)
	}
}

// A declaration whose parenthesis runs over lines is a value unless an arrow follows it; an immediately invoked
// function is a value too. A parameter list over lines followed by an arrow is a function.
func TestDeclarations_TSMultilineParenthesizedValueIsNotFunction(t *testing.T) {
	src := `const staff = (
  rows.map((r) => r.id)
).filter(Boolean);
const cp1251Table = (() => {
  return 1;
})();
const multi = (
  a: number
) => a;
`
	got := spans(mapFunctions(t, "ts", src))
	want := []string{"multi:7-9"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("functions = %v, want %v", got, want)
	}
}

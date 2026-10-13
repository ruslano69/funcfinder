package internal

import "testing"

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

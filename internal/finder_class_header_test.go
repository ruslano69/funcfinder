package internal

import "testing"

// A class's body may open on the line after its header (Allman style): the header is still open.
func TestDeclarations_ClassBodyOnNextLine(t *testing.T) {
	src := `class A
{
  run(
    a: number
  ): void {
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
		t.Errorf("run's class = %q, want %q", got, "A")
	}
}

// A complete header with no body (a Scala case class on one line) is a class of that one line: the
// object that follows is a class of its own, and the function in it belongs to the object.
func TestDeclarations_CompleteHeaderWithoutBodyIsOneLine(t *testing.T) {
	src := `case class Success(value: Int) extends Result
object Logger {
  def log(message: String): Unit = println(message)
}
`
	fns := mapFunctions(t, "scala", src)
	var got string
	for _, f := range fns {
		if f.Name == "log" {
			got = f.ClassName
		}
	}
	if got != "Logger" {
		t.Errorf("log's class = %q, want %q", got, "Logger")
	}
}

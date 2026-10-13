package internal

import (
	"reflect"
	"testing"
)

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

// A base list that goes on the next line (`class X<T> : public A,`) keeps the class open: its constructors
// belong to it, and the class ends with its body.
func TestCpp_ClassBaseListContinuesOnNextLine(t *testing.T) {
	src := `template <typename T>
class Holder : public Base,
               public Other<T> {
 public:
  Holder(T v) : value_(v) {}
};
`
	got := spans(mapFunctions(t, "cpp", src))
	want := []string{"Holder:5-5"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("functions = %v, want %v", got, want)
	}
}

// A class whose header ends at its template arguments, with the base list on the next line
// (`class P<F, enable_if_t<X<F>::value>>` / `: public buffer<char> {`), keeps its members.
func TestCpp_ClassHeaderWithBaseListOnNextLine(t *testing.T) {
	src := `class P<F, enable_if_t<has<F>::value>>
    : public buffer<char> {
 public:
  P(F*) : b(1) {}
  ~P() {
    f();
  }
};
`
	got := spans(mapFunctions(t, "cpp", src))
	want := []string{"P:4-4", "~P:5-7"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("functions = %v, want %v", got, want)
	}
}

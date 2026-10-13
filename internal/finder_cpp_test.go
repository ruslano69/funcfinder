package internal

import (
	"reflect"
	"testing"
)

// A signature may carry a nested parenthesis in its parameters and end its line before the
// return type: the header runs to the body's closing brace.
func TestCpp_SignatureWithNestedParenthesesAndTrailingReturn(t *testing.T) {
	src := `template <size_t N>
inline auto fill(char (&out)[N], int n)
    -> int {
  return n;
}
`
	got := spans(mapFunctions(t, "cpp", src))
	want := []string{"fill:2-5"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("functions = %v, want %v", got, want)
	}
}

// A member whose signature spans lines is a function; so is one whose body sits on its header line.
func TestCpp_MultilineAndOneLineMembers(t *testing.T) {
	src := `int multi(int a,
          int b) {
  return a + b;
}

struct A {
  int get() const { return m; }
  void body_line() { run(); }
  int wide(int a,
           int b) const {
    return a;
  }
};
`
	got := spans(mapFunctions(t, "cpp", src))
	want := []string{"body_line:8-8", "get:7-7", "multi:1-4", "wide:9-12"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("functions = %v, want %v", got, want)
	}
}

// Constructors and destructors have no return type: a class's own name is a constructor, a
// bodyless one is a declaration, and a destructor is ~ the name.
func TestCpp_ConstructorsAndDeclarations(t *testing.T) {
	src := `class A {
 public:
  A(int x) : m(x) {}
  explicit A(int x, int y);
  A(const A&) = delete;
  ~A() {}
};
`
	got := spans(mapFunctions(t, "cpp", src))
	want := []string{"A:3-3", "A:4-4", "A:5-5", "~A:6-6"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("functions = %v, want %v", got, want)
	}
}

// An out-of-class constructor is qualified by its class and carries an initializer list.
func TestCpp_OutOfClassConstructorWithInitializerList(t *testing.T) {
	src := `A::A(int x) : m(x) {
  f();
}
`
	got := spans(mapFunctions(t, "cpp", src))
	want := []string{"A:1-3"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("functions = %v, want %v", got, want)
	}
}

// In a member-initializer list, `member{...}` is a braced initialization: the body is the brace
// after it, and the function ends with that body.
func TestCpp_BracedInitializerInListIsNotTheBody(t *testing.T) {
	src := `struct S {
  S(int n, int... v)
      : args{{n}, v...} {
    n = 0;
  }
};
`
	got := spans(mapFunctions(t, "cpp", src))
	want := []string{"S:2-5"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("functions = %v, want %v", got, want)
	}
}

// A default argument `= {}` inside the parentheses is not a body: the constructor ends with its own
// body, which opens on the next line.
func TestCpp_BraceInDefaultArgumentDoesNotEndHeader(t *testing.T) {
	src := `class C {
 public:
  C(int x, int y = {})
      : v(x) {}
};
`
	got := spans(mapFunctions(t, "cpp", src))
	want := []string{"C:3-4"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("functions = %v, want %v", got, want)
	}
}

// A class whose body opens and closes on one line must not hold the class open over the code that
// follows: the constructor of the next class is its member.
func TestCpp_OneLineClassDoesNotSwallowNextClass(t *testing.T) {
	src := `struct has_append : std::false_type {};
class B {
 public:
  B(int x) : m(x) {}
};
`
	got := spans(mapFunctions(t, "cpp", src))
	want := []string{"B:4-4"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("functions = %v, want %v", got, want)
	}
}

// A class with an export macro between `class` and its name is named by that name, so its
// members are its own.
func TestCpp_ClassNameAfterExportMacro(t *testing.T) {
	src := `class FMT_API file {
 public:
  file(const char* path, int flags);
};
`
	got := spans(mapFunctions(t, "cpp", src))
	want := []string{"file:3-3"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("functions = %v, want %v", got, want)
	}
}

// A class template's header may carry its template prefix on the same line as `class`.
func TestCpp_TemplatePrefixedClassHeader(t *testing.T) {
	src := `template <typename T> class box {
 public:
  box(T v) : v_(v) {}
};
`
	got := spans(mapFunctions(t, "cpp", src))
	want := []string{"box:3-3"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("functions = %v, want %v", got, want)
	}
}

// A template prefix does not stop a one-line declaration, and a declaration line is not a call.
func TestCpp_TemplateDeclarationAndStatementsAreNotFunctions(t *testing.T) {
	src := `template <typename T> void tf(T);
template <typename T>
void tg(T t) {
  use(t);
}
void body() {
  if constexpr (N == 0)
    return;
  return helper(x);
  foo(bar);
}
`
	got := spans(mapFunctions(t, "cpp", src))
	want := []string{"body:6-11", "tf:1-1", "tg:3-5"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("functions = %v, want %v", got, want)
	}
}

// Operator overloads are named by their operator symbol. A conversion operator names a type and
// is not a function here.
func TestCpp_OperatorsAndConversions(t *testing.T) {
	src := `struct V {
  friend bool operator>(const V& a, const V& b) { return true; }
  bool operator==(const V& o) const {
    return true;
  }
  operator bool() const { return true; }
  int get() const { return 1; }
};
`
	got := spans(mapFunctions(t, "cpp", src))
	want := []string{"get:7-7", "operator==:3-5", "operator>:2-2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("functions = %v, want %v", got, want)
	}
}

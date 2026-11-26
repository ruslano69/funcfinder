package internal

import (
	"reflect"
	"strings"
	"testing"
)

func TestCppRecoverPointerReferenceDeclarators(t *testing.T) {
	src := `namespace scope {
struct Value {
 Value *clone() const { return nullptr; }
 Value &operator=(const Value &) { return *this; }
 Value &&move() && noexcept(false);
 Value(Value &&) noexcept;
};
Value::Value(Value &&) noexcept = default;
template<class F> bool query(F f,
                            int value) { return f(value); }
Value *first() { return nullptr; } Value *second() { return nullptr; }
Value **choose(Value (*make)(int), int n, const char *name = "test");
Value *commented(/* "comment" */);
const char *initialized("value");
Value *temporary(scope::first());
Value *literal(nullptr);
Value *value = first();
Value *(*callback)(int);
typedef Value *Function(int);
void outer() {
 struct Guard {
  explicit Guard(int &count) : count{count} { count++; }
  ~Guard() { count--; }
  Value *get() { return nullptr; }
  int &count;
 };
 Value *local(first());
 auto expression = (value != nullptr) & (first() != nullptr);
}
}`
	want := []string{
		"Guard:22-22", "Value:6-6", "Value:8-8", "choose:12-12", "clone:3-3",
		"commented:13-13", "first:11-11", "get:24-24", "move:5-5", "operator=:4-4",
		"outer:20-29", "query:9-10", "second:11-11", "~Guard:23-23",
	}
	if got := spans(mapFunctions(t, "cpp", src)); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}

	config, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	cpp, err := config.GetLanguageConfig("cpp")
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(src, "\n")
	finder := NewFinder(cpp, []string{"get", "second"}, false, true, false)
	result, err := finder.FindFunctionsInLines(lines, 101, "objects.cpp")
	if err != nil {
		t.Fatal(err)
	}
	if got := spans(result.Functions); !reflect.DeepEqual(got, []string{"get:124-124", "second:111-111"}) {
		t.Fatalf("selection with offset returned %v", got)
	}
	for _, fn := range result.Functions {
		if !reflect.DeepEqual(fn.Lines, lines[fn.Start-101:fn.End-100]) {
			t.Errorf("%s did not preserve original extraction: %q", fn.Name, fn.Lines)
		}
		if fn.Name == "get" && fn.ClassName != "Guard" {
			t.Errorf("local class owner = %q, want Guard", fn.ClassName)
		}
	}
}

func TestCppRecoverSplitReturnAndConstructorBounds(t *testing.T) {
	src := `template<class T> struct Base { explicit Base(T) {} };
template<class T> struct Other { explicit Other(T) {} };
template<class T> struct Composite : Base<T>, Other<T> {
 Composite(T value) :
   Base<T>(value),
   Other<T>(value)
 {
 }
};
static int
checked(int x,
        int y)
{
 return x + y;
}`
	want := []string{"Base:1-1", "Composite:4-8", "Other:2-2", "checked:10-15"}
	if got := spans(mapFunctions(t, "cpp", src)); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

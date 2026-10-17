package internal

import (
	"reflect"
	"strings"
	"testing"
)

func TestBracePilot_JavaAndD(t *testing.T) {
	config, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	for _, sample := range []struct {
		language, text string
		want           map[string][2]int
	}{
		{"java", `interface Work {
    void run();
}
class Outer {
    Outer() { }
    @Deprecated
    public <T> T
    convert(T value)
        throws Exception {
        return value;
    }
    void method() {
        Object value = new Object() {
            public String toString() { return "}"; }
        };
        class Local { Local() { } }
    }
    int[] array() { return new int[] {1, 2}; }
    @SuppressWarnings({"one", "two"})
    public <T> @Nullable T delegate(T item) { return item; }
    record R(int value) { R { } }
}`, map[string][2]int{"run": {2, 2}, "Outer": {5, 5}, "convert": {7, 11}, "method": {12, 17}, "toString": {14, 14}, "Local": {16, 16}, "array": {18, 18}, "delegate": {20, 20}, "R": {21, 21}}},
		{"d", `module pilot;
/+ a nested /+ comment +/
void fake() { }
+/
struct Owner {
    this(int n) {
        this((int x) { return x; }, n);
        struct Local {
            @disable this(this);
        }
        int nested() { return n; }
        auto literal = q{ void fake2() { } };
    }
    ~this() { }
    int checked(int n)
        in(n >= 0)
        out(result) { assert(result >= 0); }
        do {
            return n;
        }
}
int
splitHeader();`, map[string][2]int{"this": {6, 13}, "~this": {14, 14}, "nested": {11, 11}, "checked": {15, 20}, "splitHeader": {22, 23}}},
		{"d", `module prefixes;
version (First)
version (Second)
int conditional() { return 1; }
else
int alternate() { return 2; }
static nothrow @nogc:
static ~this() { }
void outer() {
    switch (1) {
        case 1: int local() { return 1; }
        default: break;
    }
    static if (enabled) return target(); else return target();
}`, map[string][2]int{"conditional": {4, 4}, "alternate": {6, 6}, "~this": {8, 8}, "outer": {9, 15}, "local": {11, 11}}},
	} {
		t.Run(sample.language, func(t *testing.T) {
			lines := strings.Split(sample.text, "\n")
			for _, offset := range []int{1, 71} {
				finder := NewFinder(config[sample.language], nil, true, true, false)
				result, err := finder.findBraceFunctionsRecursive(lines, offset, "pilot."+sample.language)
				if err != nil {
					t.Fatal(err)
				}
				got := make(map[string][2]int)
				for _, fn := range result.Functions {
					start, end := fn.Start-offset+1, fn.End-offset+1
					if sample.language == "d" && start == 9 && fn.Name == "this" {
						if fn.Name != "this" || fn.ClassName != "Local" || end != 9 {
							t.Fatalf("local postblit: %+v", fn)
						}
						continue
					}
					got[fn.Name] = [2]int{start, end}
					if !reflect.DeepEqual(fn.Lines, lines[start-1:end]) {
						t.Errorf("original source lost: %+v", fn)
					}
				}
				if !reflect.DeepEqual(got, sample.want) {
					t.Fatalf("bounds = %v, want %v", got, sample.want)
				}
			}
		})
	}
}

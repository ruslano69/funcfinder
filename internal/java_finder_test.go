package internal

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestJavaFinder_DeclarationContextsAndBounds(t *testing.T) {
	config, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(`interface Work {
    void run();
}
class Outer {
    Outer() { this(1); }
    Outer(int n) { }
    @SuppressWarnings({"one", "two"})
    public <T> @Nullable T
    convert(T value)
        throws Exception { return value; }
    void method() {
        Object value = new Object() {
            public String toString() { return "}"; }
        };
        class Local { Local() { } int get() { return 1; } }
        if (ready()) { call(); }
        returnCall();
    }
    int[] array() { return new int[] {1, 2}; }
    record R(int value) { R { check(value); } }
    @interface Tag {
        String[] names() default {"a", "b"};
        Class<?> type() default String.class;
    }
    native void nativeMethod() throws Exception;
    int one() { return 1; } int two() { return 2; }
}`, "\n")
	want := map[string]int{"run:2": 2, "Outer:5": 5, "Outer:6": 6, "convert:7": 10,
		"method:11": 18, "toString:13": 13, "Local:15": 15, "get:15": 15,
		"array:19": 19, "R:20": 20, "names:22": 22, "type:23": 23,
		"nativeMethod:25": 25, "one:26": 26, "two:26": 26}
	for _, offset := range []int{0, 70} {
		result, err := NewFinder(config["java"], nil, true, true, false).FindFunctionsInLines(lines, offset+1, "sample.java")
		if err != nil {
			t.Fatal(err)
		}
		got := map[string]int{}
		for _, fn := range result.Functions {
			got[fmt.Sprintf("%s:%d", fn.Name, fn.Start-offset)] = fn.End - offset
			if !reflect.DeepEqual(fn.Lines, lines[fn.Start-offset-1:fn.End-offset]) {
				t.Errorf("extraction lost original source: %+v", fn)
			}
			if fn.Name == "Local" && fn.ClassName != "Local" || fn.Name == "names" && fn.ClassName != "Tag" {
				t.Errorf("wrong enclosing type: %+v", fn)
			}
		}
		if !reflect.DeepEqual(got, want) || len(result.Functions) != len(want) {
			t.Fatalf("functions = %v, want %v", got, want)
		}
		selected, err := NewFinder(config["java"], []string{"convert", "Local", "names"}, false, true, false).FindFunctionsInLines(lines, offset+1, "sample.java")
		if err != nil || len(selected.Functions) != 3 {
			t.Fatalf("selected functions = %+v, err %v", selected, err)
		}
	}
}

func TestJavaFinder_AnonymousArgumentsEnumsAndIdentifiers(t *testing.T) {
	config, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(`non-sealed class Dollar$Type {
    Dollar$Type() { }
    Dollar$Type value$() { return this; }
    void outer() {
        register(new Runnable() {
            public void run() { target(); }
        });
        Runnable lambda = () -> { target(); };
    }
    enum E {
        A { void special() { } }, B;
        E() { }
        abstract void special();
        <T, U> T generic(T value, U ignored) { return value; }
    }
    java.util.@Marker List<String> @Marker [] annotated() { return null; }
    int delegate() { return 1; }
    void target() { }
}`, "\n")
	result, err := NewFinder(config["java"], nil, true, false, false).FindFunctionsInLines(lines, 1, "extra.java")
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, fn := range result.Functions {
		got = append(got, fn.Name)
	}
	want := []string{"Dollar$Type", "value$", "outer", "run", "special", "E", "special", "generic", "annotated", "delegate", "target"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("names = %v, want %v", got, want)
	}
}

func TestJavaCallGraph_NestedMethodsAndSplitSignatures(t *testing.T) {
	config, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(`class Owner {
    void outer() {
        class Local {
            void nested() {
                target();
            }
        }
    }
    int
    recurse(int n) {
        return n > 0 ? recurse(n - 1) : 0;
    }
    void inline() { inline(); }
    void target() { }
}`, "\n")
	result, err := NewFinder(config["java"], nil, true, false, false).FindFunctionsInLines(lines, 1, "calls.java")
	if err != nil {
		t.Fatal(err)
	}
	graph := buildFileCallGraph("calls.java", lines, result.Functions, config["java"], nil, nil)
	want := map[string]bool{"nested->target": true, "recurse->recurse": true, "inline->inline": true}
	if len(graph.Calls) != len(want) {
		t.Fatalf("calls = %v", graph.Calls)
	}
	for _, edge := range graph.Calls {
		if !want[edge.Caller+"->"+edge.Callee] || edge.Caller == "recurse" && edge.Line != 11 {
			t.Errorf("unexpected edge: %+v", edge)
		}
	}
}

func TestJavaFinder_AnnotatedTypeCoordinates(t *testing.T) {
	config, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	lines := []string{"@Deprecated", "public class Owner {", "    void run() { }", "}"}
	result, err := NewFinder(config["java"], nil, true, false, false).FindFunctionsInLines(lines, 1, "Owner.java")
	if err != nil || len(result.Classes) != 1 || result.Classes[0].Start != 2 || result.Classes[0].End != 4 {
		t.Fatalf("type coordinates = %+v, error %v", result, err)
	}
}

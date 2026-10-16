package internal

import (
	"fmt"
	"strings"
	"testing"
)

func TestCSharpFinder_DeclarationsExpressionsAndStatementContext(t *testing.T) {
	config, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	lines := []string{
		"using System;", "interface IContract {", "    void Stop();",
		"    Task<T> Convert<T>(", "        T value) where T : class;", "}",
		"class Owner<T> {", "    public Owner(int n)", "        : this()", "    { }",
		"    private Owner() { }", "    ~Owner() { }", "    [Marker]", "    public object?",
		"    IContract.Get()", "        => new object {", `            Value = Invoke(() => { return ";"; })`, "        };",
		"    public static implicit operator Owner<T>(int value) => new Owner<T>(value);",
		"    public static bool operator ==(Owner<T> left, Owner<T> right) => false;",
		"    public void Outer() {", "        var value = new Owner<T>();", "        lock(this) { Invoke(); }",
		"        try { Invoke(); } catch(Exception) { throw new Exception(); }",
		"        int Local(int n) => n > 0 ? Local(n - 1) : 0;",
		"        var item = value is {} provider", "            ? provider(1) : 0;",
		"        object Next() => new {", `            Name = "x"`, "        };", "    }",
		"    public string Branch() =>", "#if ONE", `        "one";`, "#else", `        "two";`, "#endif",
		"    public void Recurse() {", "        Recurse();", "    }", "    public void Inline() => Inline();",
		"    public void Call() {", "        (value as ICallback)?.OnCompleted();", "        (await Invoke()).ShouldBe(1);",
		"        var list = new[] { new Owner<T>(), new Owner<T>() };", "    }",
		"    class Nested {", "        public Nested() { }", "    }", "}",
		"record Primary(int Value) {", "    public int Double() => Value * 2;", "}",
	}
	want := map[string]int{
		"Stop:3": 3, "Convert:4": 5, "Owner:8": 10, "Owner:11": 11, "~Owner:12": 12,
		"Get:14": 18, "implicit operator Owner<T>:19": 19, "operator ==:20": 20,
		"Outer:21": 31, "Local:25": 25, "Next:28": 30, "Branch:32": 36, "Recurse:38": 40,
		"Inline:41": 41, "Call:42": 46, "Nested:48": 48, "Double:52": 52,
	}
	for _, offset := range []int{0, 70} {
		result, err := NewFinder(config["cs"], nil, true, true, false).FindFunctionsInLines(lines, offset+1, "sample.cs")
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Functions) != len(want) {
			t.Fatalf("functions = %v, want %d", result.Functions, len(want))
		}
		for _, fn := range result.Functions {
			key := fmt.Sprintf("%s:%d", fn.Name, fn.Start-offset)
			if end, ok := want[key]; !ok || fn.End != end+offset {
				t.Errorf("unexpected function: %+v", fn)
			}
			if strings.Join(fn.Lines, "\n") != strings.Join(lines[fn.Start-offset-1:fn.End-offset], "\n") {
				t.Errorf("extraction differs for %s", key)
			}
			if fn.Name == "Nested" && fn.ClassName != "Nested" {
				t.Errorf("constructor has wrong scope: %+v", fn)
			}
			if fn.Name == "Get" && fn.SignatureLine != 15+offset {
				t.Errorf("split header signature line: %+v", fn)
			}
		}
		selected, err := NewFinder(config["cs"], []string{"Local", "Get"}, false, true, false).FindFunctionsInLines(lines, offset+1, "sample.cs")
		if err != nil || len(selected.Functions) != 2 {
			t.Errorf("selected functions = %v, err %v", selected, err)
		}
	}
}

func TestCSharpCallGraph_SignaturesRecursionAndLocalCallers(t *testing.T) {
	config, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(`class C {
    C() { }
    void Outer() {
        int Local() {
            Target();
            return 1;
        }
        Local();
    }
    void Target() { }
    int Recurse(int n) {
        return n > 0 ? Recurse(n - 1) : 0;
    }
    void Inline() => Inline();
    ~C() { }
    public static implicit operator C(int n) => new C();
}`, "\n")
	result, err := NewFinder(config["cs"], nil, true, false, false).FindFunctionsInLines(lines, 1, "sample.cs")
	if err != nil {
		t.Fatal(err)
	}
	graph := buildFileCallGraph("sample.cs", lines, result.Functions, config["cs"], nil, nil)
	want := map[string]bool{"Outer->Local": true, "Local->Target": true, "Recurse->Recurse": true, "Inline->Inline": true, "implicit operator C->C": true}
	if len(graph.Calls) != len(want) {
		t.Errorf("calls = %v", graph.Calls)
	}
	for _, edge := range graph.Calls {
		if !want[edge.Caller+"->"+edge.Callee] {
			t.Errorf("unexpected call: %+v", edge)
		}
	}
}

func TestCSharpFinder_PropertiesDoNotHideFollowingMethods(t *testing.T) {
	config, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	text := `class C {
    int Property { get; set; }
    void First() { var v = value is {} provider ? provider(1) : 0; }
    void Second() { }
    event Action Changed { add { } remove { } }
    void Third() { }
    int this[int i] { get { return i; } }
    void Fourth() { }
}`
	result, err := NewFinder(config["cs"], nil, true, true, false).FindFunctionsInLines(strings.Split(text, "\n"), 1, "sample.cs")
	if err != nil {
		t.Fatal(err)
	}
	checkLanguageBounds(t, text, result, map[string][2]int{"First": {3, 3}, "Second": {4, 4}, "Third": {6, 6}, "Fourth": {8, 8}})
	text = `class C {
    int ICount.Count { get { return 1; } }
    void AfterCount() { }
    object IMap<string, object>.this[string key] { get { return null; } set { Work(); } }
    void AfterIndexer() { }
}`
	result, err = NewFinder(config["cs"], nil, true, true, false).FindFunctionsInLines(strings.Split(text, "\n"), 1, "sample.cs")
	if err != nil {
		t.Fatal(err)
	}
	checkLanguageBounds(t, text, result, map[string][2]int{"AfterCount": {3, 3}, "AfterIndexer": {5, 5}})
}

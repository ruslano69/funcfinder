package internal

import (
	"os"
	"strings"
	"testing"
)

func TestScalaFinder_ExpressionAndDeclarationBounds(t *testing.T) {
	config, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	content := strings.Join([]string{
		"object A {",
		"  override def size: Int = 1",
		"  private[A] def action: (Int, Int) => Int = (a, b) => a + b",
		"  @marker def ++(x: Int): Int = x",
		"  def abstractValue: Int",
		"  def text =", `    "first " +`, `      "last"`,
		"  def chained = make {", "    1", "  }", "    .toString",
		"  def generate(f: Int => Int)", "    : Int = {", "    f(1)", "  }",
		"  def make(x: => Int): Int = x", "  class Inner {}", "}",
	}, "\n")
	path := createTempFile(t, content, "scala_bounds_*.scala")
	t.Cleanup(func() { os.Remove(path) })
	result, err := NewFinder(config["scala"], nil, true, true, false).FindFunctions(path)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string][2]int{"size": {2, 2}, "action": {3, 3}, "++": {4, 4}, "abstractValue": {5, 5}, "text": {6, 8}, "chained": {9, 12}, "generate": {13, 16}, "make": {17, 17}}
	checkLanguageBounds(t, content, result, want)
}

func TestScalaFinder_TripleStringsSymbolsAndInlineMembers(t *testing.T) {
	config, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	content := strings.Join([]string{
		"object A {",
		`  def text = """`, `def fake() = "text"`, `""""`,
		"  def symbol = 'name", "  def brace = '{'",
		"  def runnable = new Runnable { def run(): Unit = () }",
		"  def next = 2", "}",
	}, "\n")
	path := createTempFile(t, content, "scala_literals_*.scala")
	t.Cleanup(func() { os.Remove(path) })
	result, err := NewFinder(config["scala"], nil, true, true, false).FindFunctions(path)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string][2]int{"text": {2, 4}, "symbol": {5, 5}, "brace": {6, 6}, "runnable": {7, 7}, "run": {7, 7}, "next": {8, 8}}
	checkLanguageBounds(t, content, result, want)
}

func TestDFinder_CommentsStringsContractsAndNestedDefinitions(t *testing.T) {
	config, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	content := strings.Join([]string{
		"module test;", "/+ docs /+ nested +/", "void fake() {}", "+/",
		"struct Box(T) {}", "enum flag(T) = true;",
		"auto ref value() @safe { return data; }",
		"int contract(int n) @safe", `in(n >= 0, "not negative")`, "out(r) { assert(r >= 0); }", "do {", "    return n;", "}",
		"void outer()", "    @safe", "{", "    uint local() { return 1; }", `    auto r = r"{ } '";`, `    auto tokens = q{ auto nested = q{ void fake2() { auto s = "}"; } }; };`, "}",
		"Event delegate() @safe popState() @safe { return callback; }",
		"struct Owner {", "    this(int n) {}", "    @disable this(this);", "}",
	}, "\n")
	path := createTempFile(t, content, "d_scopes_*.d")
	t.Cleanup(func() { os.Remove(path) })
	result, err := NewFinder(config["d"], nil, true, true, false).FindFunctions(path)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string][2]int{"value": {7, 7}, "contract": {8, 13}, "outer": {14, 20}, "local": {17, 17}, "popState": {21, 21}}
	if len(result.Functions) != len(want)+2 {
		t.Fatalf("functions = %v, want five functions and two constructors", result.Functions)
	}
	var ordinary []FunctionBounds
	constructors := 0
	for _, fn := range result.Functions {
		if fn.Name == "this" {
			constructors++
			if fn.Start != fn.End || (fn.Start != 23 && fn.Start != 24) {
				t.Errorf("constructor bounds: %+v", fn)
			}
		} else {
			ordinary = append(ordinary, fn)
		}
	}
	if constructors != 2 {
		t.Errorf("constructors = %d, want 2", constructors)
	}
	result.Functions = ordinary
	checkLanguageBounds(t, content, result, want)
	if len(result.Classes) < 2 {
		t.Errorf("class discovery lost type declarations: %v", result.Classes)
	}
}

func TestDFinder_PrefixedMultilineHeadersAndConstructorCalls(t *testing.T) {
	config, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	lines := []string{
		"module test;",
		"extern(Windows) int external(int n);",
		"@(tag(1)) @system shared static this() {}",
		"static if (enabled) static int conditional() { return 1; }",
		"version(feature) public int selected() { return 1; }",
		"pragma(inline, true) ref inout(ubyte) component() inout { return data; }",
		`deprecated("old") int legacy() { return 1; }`,
		"typeof(this) identity() { return this; }",
		"private void*[2] pair() { return null; }",
		"char *pointer() { return null; }",
		"Item*[] pointers() { return null; }",
		"@property const (char*) qualified() { return null; }",
		"@property empty() const { return true; }",
		"public @property opDispatch(string name)() { return 1; }",
		"static int",
		"multiline(int n)",
		"{",
		"    return n;",
		"}",
		"Item *",
		"bodyless();",
		"nothrow @safe",
		"int attributed() { return 1; }",
		"class Owner {",
		"    this(int n)",
		"    {",
		"        this((int n) { work(); }, n);",
		"        try",
		"            work();",
		"        catch(Exception) {}",
		"        struct Local {",
		"            @disable this(this);",
		"        }",
		"    }",
		"    this();",
		"    @disable new();",
		"}",
	}
	want := map[string][2]int{
		"external": {2, 2}, "conditional": {4, 4}, "selected": {5, 5}, "component": {6, 6},
		"legacy": {7, 7}, "identity": {8, 8}, "pair": {9, 9}, "pointer": {10, 10}, "pointers": {11, 11},
		"qualified": {12, 12}, "empty": {13, 13}, "opDispatch": {14, 14}, "multiline": {15, 19},
		"bodyless": {20, 21}, "attributed": {23, 23}, "new": {36, 36},
	}
	for _, offset := range []int{0, 40} {
		finder := NewFinder(config["d"], nil, true, true, false)
		result, err := finder.FindFunctionsInLines(lines, offset+1, "test.d")
		if err != nil {
			t.Fatal(err)
		}
		var ordinary []FunctionBounds
		constructors := make(map[int]int)
		for _, fn := range result.Functions {
			if fn.Name == "this" {
				constructors[fn.Start-offset] = fn.End - offset
				if fn.Start-offset == 32 && fn.ClassName != "Local" {
					t.Errorf("local constructor scope: %+v", fn)
				}
			} else {
				fn.Start -= offset
				fn.End -= offset
				ordinary = append(ordinary, fn)
			}
		}
		if len(constructors) != 4 || constructors[3] != 3 || constructors[25] != 34 || constructors[32] != 32 || constructors[35] != 35 {
			t.Errorf("constructor declarations = %v", constructors)
		}
		result.Functions = ordinary
		checkLanguageBounds(t, strings.Join(lines, "\n"), result, want)
		selected, err := NewFinder(config["d"], []string{"this"}, false, true, false).FindFunctionsInLines(lines, offset+1, "test.d")
		if err != nil || len(selected.Functions) != 4 {
			t.Errorf("selected constructors = %v, error %v", selected, err)
		}
	}
}

func checkLanguageBounds(t *testing.T, content string, result *FindResult, want map[string][2]int) {
	t.Helper()
	if len(result.Functions) != len(want) {
		t.Fatalf("functions = %v, want %d declarations", result.Functions, len(want))
	}
	lines := strings.Split(content, "\n")
	for _, fn := range result.Functions {
		bounds, ok := want[fn.Name]
		if !ok || fn.Start != bounds[0] || fn.End != bounds[1] {
			t.Errorf("unexpected bounds: %+v", fn)
		}
		if strings.Join(fn.Lines, "\n") != strings.Join(lines[fn.Start-1:fn.End], "\n") {
			t.Errorf("%s extraction changed raw source", fn.Name)
		}
	}
}

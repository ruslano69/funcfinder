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

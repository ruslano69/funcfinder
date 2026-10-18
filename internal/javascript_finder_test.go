package internal

import (
	"reflect"
	"strings"
	"testing"
)

func TestJavaScriptFinder_ExpressionsMinifiedBodiesAndDollarNames(t *testing.T) {
	lines := strings.Split(`function $(){}function $name(a){return a}const anonymous=function(n){return n}, alias=function named(){return 1};
(function wrapped(){function inner(){return /[{}]/.test("}")}})();
function first(){return 1}function second(){
 return first()
}
const one = value => value + 1, two = async (value) => {
 return value
};
const text = () =>
 "function fake(){}";
const value = (call());
`, "\n")
	want := []string{"$:1-1", "$name:1-1", "anonymous:1-1", "first:3-3", "inner:2-2", "named:1-1", "one:6-6", "second:3-5", "text:9-10", "two:6-8", "wrapped:2-2"}
	for _, offset := range []int{1, 71} {
		finder := NewFinder(langConfigOf(t, "js"), nil, true, true, false)
		result, err := finder.FindFunctionsInLines(lines, offset, "sample.js")
		if err != nil {
			t.Fatal(err)
		}
		for i := range result.Functions {
			fn := &result.Functions[i]
			if !reflect.DeepEqual(fn.Lines, lines[fn.Start-offset:fn.End-offset+1]) {
				t.Errorf("source extraction changed: %+v", fn)
			}
			fn.Start -= offset - 1
			fn.End -= offset - 1
		}
		if got := spans(result.Functions); !reflect.DeepEqual(got, want) {
			t.Fatalf("functions = %v, want %v", got, want)
		}
		selected, err := NewFinder(langConfigOf(t, "js"), []string{"$name", "anonymous", "two"}, false, true, false).FindFunctionsInLines(lines, offset, "sample.js")
		if err != nil || len(selected.Functions) != 3 {
			t.Fatalf("selected = %+v, error %v", selected, err)
		}
	}
}

func TestJavaScriptFinder_MethodsAndLiteralGuards(t *testing.T) {
	source := `class Owner$ {
 constructor() { }
 static async load(
   value = { nested: true }
 ) { return value }
 get size() { return 1 }
 #private() { }
 "quoted"() { }
}
const object = { async() {}, catch() {}, *iterate() {}, 1() {}, 'key'() {}, function() {}, class() {} };
const notAFunction = "function fake(){}";
const regex = /function fake\(\)\{\}/;
// function comment(){}
/* function blockComment(){} */
function after() { }
`
	want := []string{"\"quoted\":8-8", "#private:7-7", "'key':10-10", "1:10-10", "after:15-15", "async:10-10", "catch:10-10", "class:10-10", "constructor:2-2", "function:10-10", "iterate:10-10", "load:3-5", "size:6-6"}
	if got := spans(mapFunctions(t, "js", source)); !reflect.DeepEqual(got, want) {
		t.Fatalf("methods = %v, want %v", got, want)
	}
	for _, fn := range mapFunctions(t, "js", source) {
		if fn.Name == "constructor" && fn.ClassName != "Owner$" {
			t.Errorf("dollar class scope lost: %+v", fn)
		}
	}
}

func TestJavaScriptFinder_UninitializedDeclaratorsAndInvokedValues(t *testing.T) {
	source := `var count,
    first = function() { },
    second = function() { };
var immediatelyInvoked = function() { return 1; }();
var namedValue = function realName() { return 2; }();
var cached = (function() { return 3; })();
var direct = async function()
{ return 4; };
`
	want := []string{"direct:7-8", "first:2-2", "realName:5-5", "second:3-3"}
	if got := spans(mapFunctions(t, "js", source)); !reflect.DeepEqual(got, want) {
		t.Fatalf("functions = %v, want %v", got, want)
	}
}

func TestJavaScriptFinder_CallbacksInObjectValuesAreNotMethods(t *testing.T) {
	source := `const object = {
 value: combine(1, function() { return 1 }),
 nested: [function() { return 2 }],
 method() { return 3 }
};`
	if got := spans(mapFunctions(t, "js", source)); !reflect.DeepEqual(got, []string{"method:4-4"}) {
		t.Fatalf("functions = %v", got)
	}
}

func TestJavaScriptCallGraph_SplitSignaturesAndNestedCallers(t *testing.T) {
	lines := strings.Split(`function outer() {
 const alias = function inner() {
  target();
 };
}
function $cash(n) { return n ? $cash(n-1) : 0; }
const arrow = n => arrow(n);
function
split() { return 1; }
function target() { }
`, "\n")
	config := langConfigOf(t, "js")
	result, err := NewFinder(config, nil, true, false, false).FindFunctionsInLines(lines, 1, "sample.cjs")
	if err != nil {
		t.Fatal(err)
	}
	graph := buildFileCallGraph("sample.cjs", lines, result.Functions, config, nil, nil)
	want := map[string]bool{"inner->target": true, "$cash->$cash": true, "arrow->arrow": true}
	if len(graph.Calls) != len(want) {
		t.Fatalf("calls = %v", graph.Calls)
	}
	for _, edge := range graph.Calls {
		if !want[edge.Caller+"->"+edge.Callee] {
			t.Errorf("unexpected call: %+v", edge)
		}
	}
	configMap, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if got := configMap.GetLanguageByExtension("sample.cjs"); got == nil || got.LangKey != "js" {
		t.Errorf(".cjs language = %v", got)
	}
}

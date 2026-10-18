package internal

import (
	"reflect"
	"strings"
	"testing"
)

func TestJSTSMissesLexicalBoundaries(t *testing.T) {
	cases := []struct {
		name, source string
		want         []string
	}{
		{"regex in template expression", "function quote(value) {\n return `\"${value.replace(/([\"\\\\])/g, \"\\\\$1\")}\"`;\n}\nfunction after() {}", []string{"after:4-4", "quote:1-3"}},
		{"escaped template delimiter", "function banner() {\n const text = `art \\` brace }\n still text`;\n}\nfunction after() {}", []string{"after:5-5", "banner:1-4"}},
		{"comment opener in comment", "/** paths named docs/* live here */\nfunction after() {}", []string{"after:2-2"}},
		{"JSX bracketed slash text", "function view() {\n return (<Text>[q]uit [/]filter</Text>);\n}\nfunction after() {}", []string{"after:4-4", "view:1-3"}},
		{"comments in interpolation", "function text() {\n return `${ /* } ` */ 1 // } `\n }`;\n}\nfunction after() {}", []string{"after:5-5", "text:1-4"}},
	}
	for _, lang := range []string{"js", "ts"} {
		for _, tc := range cases {
			t.Run(lang+"/"+tc.name, func(t *testing.T) {
				if got := spans(mapFunctions(t, lang, tc.source)); !reflect.DeepEqual(got, tc.want) {
					t.Fatalf("functions = %v, want %v", got, tc.want)
				}
			})
		}
	}
}

func TestTSMissingHeaderShapes(t *testing.T) {
	cases := []struct {
		name, source string
		want         []string
	}{
		{"nested and multiline type parameters", `class Box {
 static create<
 T extends Array<Array<number>>
 >(
 value:T
 ):T {return value;}
 returns<T extends Array<Array<number>>>(value:T):T {return value;}
}`, []string{"create:2-6", "returns:7-7"}},
		{"keyword members and catch statement", `interface P {
 catch(x:Value):Value;
}
const p={
 catch(){return this;},
};
function normal(){try{call();}catch(error){}}`, []string{"catch:2-2", "catch:5-5", "normal:7-7"}},
		{"quoted and multiline object methods", `const p={
 async "quoted"(){return 1;},
 async run(
 x:number
 ){return x;}
};`, []string{"\"quoted\":2-2", "run:3-5"}},
		{"split and curried arrows", `const record =
 (level:string) =>
 (tag:unknown):number =>
 entries.push({level,tag});
const disk: typeof import("./api").reader =
 async()=>({
 value:1
 });`, []string{"disk:5-8", "record:1-4"}},
		{"dollar name after comment", `export /*marker*/ function $constructor<T>(
 x:T
 ):T {return x;}`, []string{"$constructor:1-3"}},
		{"case blocks are executable scopes", `const record =
 (x:number)=>
 (y:number)=>x+y;
function control(value:string) {
 switch(value) {
 case "a": {
 if(value){call();}
 for(const v of values){call(v);}
 }
 }
}`, []string{"control:4-11", "record:1-3"}},
		{"asserted function values keep profile exclusions", `const record =
 (x:number)=>
 (y:number)=>x+y;
const Ctor=function(this:unknown){return this;} as unknown as new()=>Value;`, []string{"record:1-3"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := spans(mapFunctions(t, "ts", tc.source)); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("functions = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestTSRecoveredSelectionOffsetsAndExtraction(t *testing.T) {
	lines := strings.Split(`class Box {
 async run(
 x:number
 ){return x;}
}
const curry =
 (x:number)=>
 (y:number)=>x+y;`, "\n")
	for _, offset := range []int{1, 71} {
		result, err := NewFinder(langConfigOf(t, "ts"), nil, true, true, false).FindFunctionsInLines(lines, offset, "recovery.ts")
		if err != nil {
			t.Fatal(err)
		}
		for i := range result.Functions {
			fn := &result.Functions[i]
			if !reflect.DeepEqual(fn.Lines, lines[fn.Start-offset:fn.End-offset+1]) {
				t.Fatalf("extraction changed: %+v", fn)
			}
			if fn.Name == "run" && fn.ClassName != "Box" {
				t.Fatalf("method scope: %+v", fn)
			}
			fn.Start -= offset - 1
			fn.End -= offset - 1
		}
		if got := spans(result.Functions); !reflect.DeepEqual(got, []string{"curry:6-8", "run:2-4"}) {
			t.Fatal(got)
		}
		selected, err := NewFinder(langConfigOf(t, "ts"), []string{"curry"}, false, true, false).FindFunctionsInLines(lines, offset, "recovery.ts")
		if err != nil || len(selected.Functions) != 1 || selected.Functions[0].Name != "curry" {
			t.Fatalf("selected: %+v, %v", selected, err)
		}
	}
}

package internal

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func TestScriptCodeObjectsRead(t *testing.T) {
	for _, language := range []string{"js", "ts"} {
		path := filepath.Join(t.TempDir(), "sample."+language)
		src := []byte("// function fake() {}\nexport function plain() { return /[{}]/.test(`a${`${1}`}`); } const a = x => x+1, b = x => x*2, c = x => x+3; class Box { #value() { return 1; } get value() { return 2; } set value(x) {} handler = async (x) => x; }")
		items, err := scriptCodeItems(path, src, "", "module", language)
		if err != nil {
			t.Fatal(language, err)
		}
		var names []string
		for _, item := range items {
			if item.object.Kind != "" {
				names = append(names, item.object.Name)
			}
		}
		if strings.Join(names, ",") != "plain,a,b,c,Box,#value,value,value,handler" {
			t.Fatal(language, names)
		}
		obj, err := ReadCodeObject(path, language, "function", "b", 0, 0, src)
		if err != nil || obj.Source != "const b = x => x*2;" {
			t.Fatalf("binding: %#v %v", obj, err)
		}
		if obj.Column != bytes.Index(bytes.Split(src, []byte("\n"))[1], []byte("b ="))+1 {
			t.Fatal("byte column", obj.Column)
		}
		_, err = ReadCodeObject(path, language, "function", "value", 0, 0, src)
		if ambiguity, ok := err.(*ObjectError); !ok || ambiguity.Code != "ambiguous" || len(ambiguity.Candidates) != 2 {
			t.Fatalf("accessors: %v", err)
		}
	}
	for _, test := range []struct{ path, source, name, want string }{
		{"sample.ts", "export const f = async <T>(x: T): Promise<{v:T}> => ({v:x});", "f", "const f = async <T>(x: T): Promise<{v:T}> => ({v:x});"},
		{"sample.ts", "const f = (x: number) => make<A,B>(x); const keep = () => 2;", "f", "const f = (x: number) => make<A,B>(x);"},
		{"sample.tsx", "const View = () => <div><span>{`hi${1}`}</span></div>; const keep = () => 2;", "View", "const View = () => <div><span>{`hi${1}`}</span></div>;"},
		{"sample.tsx", "const View = () => (\n <div>{[1,2].map(x => <span>{x}</span>)}</div>\n);", "View", "const View = () => (\n <div>{[1,2].map(x => <span>{x}</span>)}</div>\n);"},
		{"sample.ts", "namespace N { export function f<T>(x:T):T {return x;} }", "f", "export function f<T>(x:T):T {return x;}"},
		{"sample.ts", "class C { @tag static async f<T>(x:T):Promise<T> { return x; } }", "f", "@tag static async f<T>(x:T):Promise<T> { return x; }"},
		{"sample.ts", "type Mapping<T> = Record<string, T> | { [K in keyof T]: T[K] };", "Mapping", "type Mapping<T> = Record<string, T> | { [K in keyof T]: T[K] };"},
	} {
		kind := "function"
		if test.name == "Mapping" {
			kind = "type"
		}
		obj, err := ReadCodeObject(test.path, "ts", kind, test.name, 0, 0, []byte(test.source))
		if err != nil || obj.Source != test.want {
			t.Fatalf("case %s: %#v %v", test.source, obj, err)
		}
	}
}
func TestScriptCodeObjectsTransactions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sample.ts")
	src := []byte("const a = x => x+1, b = x => x*2, c = x => x+3;\n")
	read := func(name string) CodeObject {
		t.Helper()
		obj, err := ReadCodeObject(path, "ts", "function", name, 0, 0, src)
		if err != nil {
			t.Fatal(err)
		}
		return *obj
	}
	a, b, c := read("a"), read("b"), read("c")
	changed := b
	changed.Source = "const b = x => helper(x);"
	out, results, err := PlanCodeEdits([]CodeEdit{{Operation: "replace", Object: changed}, {Operation: "insert_after", Object: b, Source: "const helper = x => x*20;"}, {Operation: "remove", Object: a}}, src)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "const  b = x => helper(x),\nhelper = x => x*20, c = x => x+3;") {
		t.Fatalf("bindings: %s", out)
	}
	if len(results) != 3 || results[0].Objects[0].Source != "const b = x => helper(x);" || results[1].Objects[0].Name != "helper" || len(results[2].Objects) != 0 {
		t.Fatalf("results: %#v", results)
	}
	for _, result := range results {
		if result.Revision != codeRevision(out) {
			t.Fatal("old revision")
		}
	}
	if _, _, err := PlanCodeEdits([]CodeEdit{{Operation: "remove", Object: c}}, out); err == nil {
		t.Fatal("stale accepted")
	}
	for _, name := range []string{"a", "b", "c"} {
		obj := read(name)
		removed, _, err := PlanCodeEdits([]CodeEdit{{Operation: "remove", Object: obj}}, src)
		if err != nil {
			t.Fatal(name, err)
		}
		for _, other := range []string{"a", "b", "c"} {
			if name != other {
				if _, err := ReadCodeObject(path, "ts", "function", other, 0, 0, removed); err != nil {
					t.Fatal(other, err)
				}
			}
		}
	}
	for _, source := range []string{"export const only = () => 1;", "let only = () => 1\n", "const obj = { only() {return 1;}, keep: () => 2 };", "const obj = { keep: () => 2, only() {return 1;} };", "const obj = { only() {return 1;} };"} {
		data := []byte(source)
		obj, err := ReadCodeObject(path, "ts", "function", "only", 0, 0, data)
		if err != nil {
			t.Fatal(source, err)
		}
		if _, _, err := PlanCodeEdits([]CodeEdit{{Operation: "remove", Object: *obj}}, data); err != nil {
			t.Fatal(source, err)
		}
	}
	data := []byte("const obj = { a(){return 1;}, b:()=>2 }; class C { m(){return 3;} }")
	obj, err := ReadCodeObject(path, "ts", "function", "a", 0, 0, data)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := PlanCodeEdits([]CodeEdit{{Operation: "insert_after", Object: *obj, Source: "added(){return 4;}"}}, data); err != nil {
		t.Fatal(err)
	}
	typeObj, err := ReadCodeObject(path, "ts", "type", "C", 0, 0, data)
	if err != nil {
		t.Fatal(err)
	}
	typeObj.Source = "class C { next(){return 10;} }"
	if _, _, err := ReplaceCodeObject(*typeObj, data); err != nil {
		t.Fatal(err)
	}
}
func TestScriptCodeObjectsGuards(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sample.ts")
	src := []byte("const f = (x:number) => x; const keep = () => 2;")
	obj, err := ReadCodeObject(path, "ts", "function", "f", 0, 0, src)
	if err != nil {
		t.Fatal(err)
	}
	for _, source := range []string{"let f = (x:number) => x;", "const renamed = () => 1;", "const f = () => {", "const f = () => 1, leaked = () => 2;", "const /*discarded*/ f = () => 1;", "const f = () => 1; // outside"} {
		edited := *obj
		edited.Source = source
		if out, _, err := ReplaceCodeObject(edited, src); err == nil || out != nil {
			t.Fatalf("accepted %q: %s", source, out)
		}
	}
	bad := *obj
	bad.Column++
	if _, _, err := ReplaceCodeObject(bad, src); err == nil {
		t.Fatal("forged metadata accepted")
	}
	keep, _ := ReadCodeObject(path, "ts", "function", "keep", 0, 0, src)
	bad = *obj
	bad.Source = "const f = () => {"
	if out, _, err := PlanCodeEdits([]CodeEdit{{Operation: "remove", Object: *keep}, {Operation: "replace", Object: bad}}, src); err == nil || out != nil {
		t.Fatal("partial write plan")
	}
	for _, source := range []string{"function f(){ /* unclosed", "const f = () => `missing${1};", "const f=()=>({]);", "const f=()=>\"unterminated"} {
		if _, err := scriptCodeItems(path, []byte(source), "", "module", "ts"); err == nil {
			t.Fatal("bad lexical structure accepted", source)
		}
	}
	data := []byte(utf8BOM + "// функция\r\nconst note=\"😀\", f = () => 1, keep=()=>2;\r\n")
	unicodeObj, err := ReadCodeObject(path, "ts", "function", "f", 0, 0, data)
	if err != nil {
		t.Fatal(err)
	}
	unicodeObj.Source = "const f = () => 20;"
	out, _, err := ReplaceCodeObject(*unicodeObj, data)
	if err != nil || !bytes.Equal(out, bytes.Replace(data, []byte("f = () => 1"), []byte("f = () => 20"), 1)) {
		t.Fatalf("UTF-8/BOM/EOL: %s %v", out, err)
	}
}

func TestScriptCodeObjectsRealShapes(t *testing.T) {
	for _, tc := range []struct{ source, name, kind, want string }{
		{"import value, { type Thing } from 'lib'; export { type Other } from 'lib'; function f(){return 1;}", "f", "function", "function f(){return 1;}"},
		{"declare global { namespace Express { interface Request { principal?: string; } } }", "Request", "type", "interface Request { principal?: string; }"},
		{"(globalThis as any).storage = { getItem:()=>null, setItem:()=>{} };", "getItem", "function", "getItem:()=>null"},
		{"try { function f(): () => number { return () => 1; } } catch(e) {}", "f", "function", "function f(): () => number { return () => 1; }"},
		{"const f = (x:unknown): x is Record<string,unknown> => !!x, keep=()=>2;", "f", "function", "const f = (x:unknown): x is Record<string,unknown> => !!x;"},
		{"const f = () => value as Record<string,unknown>, keep=()=>2;", "f", "function", "const f = () => value as Record<string,unknown>;"},
		{"const View = () => <div>It's {'ok'}{[1].map(x=><span>{x}</span>)}</div>;", "View", "function", "const View = () => <div>It's {'ok'}{[1].map(x=><span>{x}</span>)}</div>;"},
	} {
		path := filepath.Join(t.TempDir(), "real.tsx")
		src := []byte(tc.source)
		obj, err := ReadCodeObject(path, "ts", tc.kind, tc.name, 0, 0, src)
		if err != nil || obj.Source != tc.want {
			t.Fatalf("%s: %#v %v", tc.source, obj, err)
		}
		out, _, err := ReplaceCodeObject(*obj, src)
		if err != nil || !bytes.Equal(out, src) {
			t.Fatalf("identity %s: %s %v", tc.source, out, err)
		}
	}
	path := filepath.Join(t.TempDir(), "assignment.ts")
	src := []byte("(globalThis as any).storage = { getItem:()=>null, setItem:()=>{} };")
	obj, err := ReadCodeObject(path, "ts", "function", "getItem", 0, 0, src)
	if err != nil {
		t.Fatal(err)
	}
	edited := *obj
	edited.Source = "getItem:()=>42"
	out, _, err := PlanCodeEdits([]CodeEdit{{Operation: "replace", Object: edited}, {Operation: "insert_after", Object: *obj, Source: "removeItem:()=>{}"}}, src)
	if err != nil || string(out) != "(globalThis as any).storage = { getItem:()=>42,\nremoveItem:()=>{}, setItem:()=>{} };" {
		t.Fatalf("assignment: %s %v", out, err)
	}
	mixed := []byte("function f(){return 1;}\r\nfunction keep(){}\n")
	mixedObj, err := ReadCodeObject(path, "ts", "function", "f", 0, 0, mixed)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := ReplaceCodeObject(*mixedObj, mixed); err == nil {
		t.Fatal("mixed endings accepted")
	}
}

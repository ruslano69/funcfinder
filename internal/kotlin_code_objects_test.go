package internal

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func TestKotlinCodeObjectsRead(t *testing.T) {
	cases := []struct{ src, name, kind, want string }{
		{"fun a()=1; fun b()=2; fun c()=3", "b", "function", "fun b()=2"},
		{"/** doc */\n@Deprecated(\"old\") public suspend fun <T> List<T>.choose(x: T): T = x\nfun keep(){}", "choose", "function", "@Deprecated(\"old\") public suspend fun <T> List<T>.choose(x: T): T = x"},
		{"interface I { fun f(): () -> Unit\nfun keep() }", "f", "function", "fun f(): () -> Unit"},
		{"fun f() = if (true)\n  1\nelse\n  2\nfun keep()=3", "f", "function", "fun f() = if (true)\n  1\nelse\n  2"},
		{"fun f() = try { 1 } catch(e: Exception) { 2 } finally { println() }\nfun keep(){}", "f", "function", "fun f() = try { 1 } catch(e: Exception) { 2 } finally { println() }"},
		{"fun f() = listOf(1)\n .map { it+1 }\nfun keep(){}", "f", "function", "fun f() = listOf(1)\n .map { it+1 }"},
		{"fun f() = 1 to\n  2\nfun keep()=3", "f", "function", "fun f() = 1 to\n  2"},
		{"class C<T>(val cb: () -> T)\nfun keep(){}", "C", "type", "class C<T>(val cb: () -> T)"},
		{"fun interface F { fun invoke():Int }", "F", "type", "fun interface F { fun invoke():Int }"},
		{"class C { companion object { fun f()=1 } }", "f", "function", "fun f()=1"},
		{"typealias F<T> = (T) -> List<T>\nfun keep(){}", "F", "type", "typealias F<T> = (T) -> List<T>"},
		{"/* outer /* nested */ */\nfun `a test`() = \"${run { \"}\" }}\"\nfun keep(){}", "`a test`", "function", "fun `a test`() = \"${run { \"}\" }}\""},
		{"fun raw() = \"\"\"} fun fake(){} ${\"x\"}\n\"\"\"\"\nfun keep(){}", "raw", "function", "fun raw() = \"\"\"} fun fake(){} ${\"x\"}\n\"\"\"\""},
		{"@file:JvmName(\"Sample\")\nfun f()=1", "f", "function", "fun f()=1"},
	}
	for _, tc := range cases {
		path := filepath.Join(t.TempDir(), "sample.kt")
		src := []byte(tc.src)
		obj, err := ReadCodeObject(path, "kotlin", tc.kind, tc.name, 0, 0, src)
		if err != nil || obj.Source != tc.want {
			t.Fatalf("%s: %#v %v", tc.src, obj, err)
		}
		out, _, err := ReplaceCodeObject(*obj, src)
		if err != nil || !bytes.Equal(out, src) {
			t.Fatalf("identity: %s %v", out, err)
		}
	}
	src := []byte("fun f(x:Int)=x\nfun f(x:String)=x\nclass C { fun f()=1; val p get()=1; init { fun local(){} } }\nfun outer(){fun nested(){}}")
	_, err := ReadCodeObject("overloads.kt", "kotlin", "function", "f", 0, 0, src)
	amb, ok := err.(*ObjectError)
	if !ok || amb.Code != "ambiguous" || len(amb.Candidates) != 3 {
		t.Fatalf("overloads %v", err)
	}
	for _, name := range []string{"local", "nested", "get"} {
		if _, err := ReadCodeObject("overloads.kt", "kotlin", "function", name, 0, 0, src); err == nil {
			t.Fatal("opaque body discovered", name)
		}
	}
}
func TestKotlinCodeObjectsTransactions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sample.kt")
	src := []byte("fun a()=1; fun b()=2; fun c()=3\nclass C { fun m()=4 }")
	read := func(kind, name string) CodeObject {
		t.Helper()
		obj, err := ReadCodeObject(path, "kotlin", kind, name, 0, 0, src)
		if err != nil {
			t.Fatal(err)
		}
		return *obj
	}
	a, b, c := read("function", "a"), read("function", "b"), read("function", "c")
	changed := b
	changed.Source = "fun b() = helper()"
	out, results, err := PlanCodeEdits([]CodeEdit{{Operation: "replace", Object: changed}, {Operation: "insert_after", Object: b, Source: "fun helper() = 20"}, {Operation: "remove", Object: a}}, src)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "fun b() = helper()\nfun helper() = 20\n; fun c()=3") || len(results) != 3 || results[1].Objects[0].Name != "helper" {
		t.Fatalf("transaction: %s %#v", out, results)
	}
	for _, result := range results {
		if result.Revision != codeRevision(out) {
			t.Fatal("old revision")
		}
	}
	if _, _, err := PlanCodeEdits([]CodeEdit{{Operation: "remove", Object: c}}, out); err == nil {
		t.Fatal("stale accepted")
	}
	typ := read("type", "C")
	typ.Source = "class C { fun next()=5 }"
	if _, _, err := ReplaceCodeObject(typ, src); err != nil {
		t.Fatal(err)
	}
	method := read("function", "m")
	if _, _, err := PlanCodeEdits([]CodeEdit{{Operation: "insert_after", Object: method, Source: "fun next()=5"}}, src); err != nil {
		t.Fatal(err)
	}
	if _, _, err := PlanCodeEdits([]CodeEdit{{Operation: "remove", Object: read("type", "C")}, {Operation: "remove", Object: method}}, src); err == nil {
		t.Fatal("overlap accepted")
	}
	if _, _, err := PlanCodeEdits([]CodeEdit{{Operation: "insert_after", Object: b, Source: b.Source}}, src); err == nil {
		t.Fatal("duplicate accepted")
	}
}
func TestKotlinCodeObjectsGuards(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sample.kt")
	src := []byte("@Deprecated(\"old\") suspend fun String.f()=length\nfun keep()=2")
	obj, err := ReadCodeObject(path, "kotlin", "function", "f", 0, 0, src)
	if err != nil {
		t.Fatal(err)
	}
	for _, source := range []string{"suspend fun String.f()=length", "@Deprecated(\"old\") suspend fun Int.f()=this", "@Deprecated(\"old\") suspend fun String.renamed()=length", "@Deprecated(\"old\") suspend fun String.f(){", "@Deprecated(\"old\") suspend fun String.f()=length\nfun leak()=1"} {
		edited := *obj
		edited.Source = source
		if out, _, err := ReplaceCodeObject(edited, src); err == nil || out != nil {
			t.Fatalf("accepted %s: %s", source, out)
		}
	}
	bad := *obj
	bad.Column++
	if _, _, err := ReplaceCodeObject(bad, src); err == nil {
		t.Fatal("forged identity accepted")
	}
	for _, source := range []string{"fun f()=\"unclosed", "fun f(){ /* outer /* inner */ }", "fun f() = ([)]", "fun f()=\"${run { 1 }\"", "fun `unclosed() {}"} {
		if _, err := kotlinCodeItems(path, []byte(source), "", "module"); err == nil {
			t.Fatal("bad lexical source accepted", source)
		}
	}
	data := []byte(utf8BOM + "// 😀\r\nfun f()=1; fun keep()=2\r\n")
	unicodeObj, err := ReadCodeObject(path, "kotlin", "function", "f", 0, 0, data)
	if err != nil {
		t.Fatal(err)
	}
	unicodeObj.Source = "fun f()=20"
	out, _, err := ReplaceCodeObject(*unicodeObj, data)
	if err != nil || !bytes.Equal(out, bytes.Replace(data, []byte("fun f()=1"), []byte("fun f()=20"), 1)) {
		t.Fatalf("BOM/EOL: %s %v", out, err)
	}
	mixed := []byte("fun f()=1\r\nfun keep()=2\n")
	o, _ := ReadCodeObject(path, "kotlin", "function", "f", 0, 0, mixed)
	if _, _, err := ReplaceCodeObject(*o, mixed); err == nil {
		t.Fatal("mixed endings accepted")
	}
	raw := []byte("object Raw {\r\nval text = \"\"\"line\r\ninside\r\n\"\"\"\r\nfun f()=1\r\n}\r\n")
	rawObj, err := ReadCodeObject(path, "kotlin", "type", "Raw", 0, 0, raw)
	if err != nil {
		t.Fatal(err)
	}
	rawOut, _, err := ReplaceCodeObject(*rawObj, raw)
	if err != nil || !bytes.Equal(rawOut, raw) {
		t.Fatalf("raw CRLF type identity: %s %v", rawOut, err)
	}

}

func TestKotlinPropertyObjects(t *testing.T) {
	for _, tc := range []struct{ src, kind, name, want string }{
		{"class C { var p:Int=1\n get()=field\n private set(value){field=value}\n fun keep()=3 }", "property", "p", "var p:Int=1\n get()=field\n private set(value){field=value}"},
		{"class C { var p:Int=1\n get()=field\n private set(value){field=value} }", "accessor", "get", "get()=field"},
		{"class C { var p:Int=1\n get()=field\n private set(value){field=value} }", "accessor", "set", "private set(value){field=value}"},
		{"var p=1\n private set\nfun keep()=3", "property", "p", "var p=1\n private set"},
		{"val String.size:Int get()=length\nfun keep(){}", "property", "size", "val String.size:Int get()=length"},
		{"val <T> List<T>.first:T get()=this[0]\nfun keep(){}", "property", "first", "val <T> List<T>.first:T get()=this[0]"},
		{"val p by lazy { object { fun local()=1 } }\nfun keep()=2", "property", "p", "val p by lazy { object { fun local()=1 } }"},
		{"val get = lookup.get() + get()\nfun keep(){}", "property", "get", "val get = lookup.get() + get()"},
		{"val p=1\n @Deprecated(\"old\") get()=field\nfun keep(){}", "accessor", "get", "@Deprecated(\"old\") get()=field"},
		{"@get:Deprecated(\"old\") val p=1\nfun keep(){}", "property", "p", "@get:Deprecated(\"old\") val p=1"},
	} {
		path := filepath.Join(t.TempDir(), "property.kt")
		src := []byte(tc.src)
		obj, err := ReadCodeObject(path, "kotlin", tc.kind, tc.name, 0, 0, src)
		if err != nil || obj.Source != tc.want {
			t.Fatalf("%s: %#v %v", tc.src, obj, err)
		}
		out, _, err := ReplaceCodeObject(*obj, src)
		if err != nil || !bytes.Equal(out, src) {
			t.Fatalf("identity: %s %v", out, err)
		}
	}
	src := []byte("class C { var p:Int=1\n get()=field\n set(v){field=v}\n var q=2\n get()=field\n fun keep()=3 }")
	path := filepath.Join(t.TempDir(), "property.kt")
	read := func(kind, name string, line int) CodeObject {
		t.Helper()
		obj, err := ReadCodeObject(path, "kotlin", kind, name, line, 0, src)
		if err != nil {
			t.Fatal(err)
		}
		return *obj
	}
	property := read("property", "p", 0)
	getter := read("accessor", "get", 2)
	setter := read("accessor", "set", 3)
	_, err := ReadCodeObject(path, "kotlin", "accessor", "get", 0, 0, src)
	amb, ok := err.(*ObjectError)
	if !ok || amb.Code != "ambiguous" || len(amb.Candidates) != 2 {
		t.Fatalf("getter ambiguity: %v", err)
	}
	changed := getter
	changed.Source = "get()=field*10"
	out, _, err := PlanCodeEdits([]CodeEdit{{Operation: "replace", Object: changed}, {Operation: "remove", Object: setter}}, src)
	if err != nil || !strings.Contains(string(out), "var p:Int=1\n get()=field*10\n \n var q=2") {
		t.Fatalf("accessor transaction: %s %v", out, err)
	}
	inserted, _, err := PlanCodeEdits([]CodeEdit{{Operation: "insert_after", Object: getter, Source: "val added=4"}}, src)
	if err == nil || inserted != nil {
		t.Fatal("property scope accepted a property")
	}
	if _, _, err := PlanCodeEdits([]CodeEdit{{Operation: "insert_after", Object: getter, Source: "get()=1"}}, src); err == nil {
		t.Fatal("duplicate getter accepted")
	}
	if _, _, err := PlanCodeEdits([]CodeEdit{{Operation: "replace", Object: property}, {Operation: "remove", Object: setter}}, src); err == nil {
		t.Fatal("property/accessor overlap accepted")
	}
	changed = property
	changed.Source = "val p:Int=1"
	if _, _, err := ReplaceCodeObject(changed, src); err == nil {
		t.Fatal("var changed to val")
	}
	changed = getter
	changed.Source = "set(v){field=v}"
	if _, _, err := ReplaceCodeObject(changed, src); err == nil {
		t.Fatal("getter changed to setter")
	}
	removed, _, err := PlanCodeEdits([]CodeEdit{{Operation: "remove", Object: property}}, src)
	if err != nil || strings.Contains(string(removed), "set(v)") || !strings.Contains(string(removed), "var q=2") {
		t.Fatalf("property removal: %s %v", removed, err)
	}
	changed = property
	changed.Source = "var p:Int=20\n get()=field*2\n set(v){field=v/2}"
	if _, _, err := ReplaceCodeObject(changed, src); err != nil {
		t.Fatal(err)
	}
	for _, edit := range []CodeEdit{{Operation: "insert_after", Object: property, Source: "val added=4"}, {Operation: "remove", Object: getter}, {Operation: "remove", Object: setter}} {
		if _, _, err := PlanCodeEdits([]CodeEdit{edit}, src); err != nil {
			t.Fatal(edit.Operation, err)
		}
	}
	readonly := []byte("val p:Int get()=1")
	g, err := ReadCodeObject(path, "kotlin", "accessor", "get", 0, 0, readonly)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := PlanCodeEdits([]CodeEdit{{Operation: "insert_after", Object: *g, Source: "set(v){}"}}, readonly); err == nil {
		t.Fatal("setter inserted into val")
	}
}

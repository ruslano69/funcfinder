package internal

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func TestGooseCodeObjectsSpans(t *testing.T) {
	cases := []struct{ src, kind, name, want string }{
		{`fn a() {} fn b() { return 2; }`, "function", "a", `fn a() {}`},
		{`fn a() {} fn b() { return 2; }`, "function", "b", `fn b() { return 2; }`},
		{`namespace n; import .other; checkfree simd recursive fn n::a<T: Box<u8[>..]>>(x: T) -> const u8[:], fn(i64)->(i64, bool) { fn local() {} }`, "function", "n::a", `checkfree simd recursive fn n::a<T: Box<u8[>..]>>(x: T) -> const u8[:], fn(i64)->(i64, bool) { fn local() {} }`},
		{`export "entry" fn ::a() -> u32 { return 1; }`, "function", "::a", `export "entry" fn ::a() -> u32 { return 1; }`},
		{`extern "native" fn a(x: i64) -> i64; fn keep() {}`, "function", "a", `extern "native" fn a(x: i64) -> i64;`},
		{`extern fn a(x: i64) -> i64 = "native";`, "function", "a", `extern fn a(x: i64) -> i64 = "native";`},
		{`thread_fn worker() {}`, "function", "worker", `thread_fn worker() {}`},
		{`struct Box<T> { data: T[>..], ref: T&<u16> } fn keep(){}`, "type", "Box", `struct Box<T> { data: T[>..], ref: T&<u16> }`},
		{`enum Result<T> { Ok { value: T }, Err }`, "type", "Result", `enum Result<T> { Ok { value: T }, Err }`},
		{`type Buffer = const (Node..&<varint>)[varint];`, "type", "Buffer", `type Buffer = const (Node..&<varint>)[varint];`},
		{`type Callback = fn(u8[:])->(i64, bool); fn keep(){}`, "type", "Callback", `type Callback = fn(u8[:])->(i64, bool);`},
		{`/* outer /* fn fake(){} */ */ fn real() { let s = """} fn fake(){} /*"""; let c = '}'; } // suffix`, "function", "real", `fn real() { let s = """} fn fake(){} /*"""; let c = '}'; }`},
	}
	for _, tc := range cases {
		t.Run(tc.name+tc.want[:2], func(t *testing.T) {
			src := []byte(tc.src)
			obj, err := ReadCodeObject("test.goose", "goose", tc.kind, tc.name, 0, 0, src)
			if err != nil {
				t.Fatal(err)
			}
			if obj.Source != tc.want {
				t.Fatalf("source %q, want %q", obj.Source, tc.want)
			}
			out, next, err := ReplaceCodeObject(*obj, src)
			if err != nil || !bytes.Equal(out, src) || next.Source != obj.Source {
				t.Fatalf("identity %q %#v %v", out, next, err)
			}
		})
	}
	for _, tc := range []struct{ src, name string }{
		{`fn outer() { fn local() {} }`, "local"},
		{`extern "library" { fn hidden(); } fn real(){}`, "hidden"},
		{`let callback = fn() { fn hidden() {} }; fn real(){}`, "hidden"},
	} {
		_, err := ReadCodeObject("opaque.goose", "goose", "function", tc.name, 0, 0, []byte(tc.src))
		if e, ok := err.(*ObjectError); !ok || e.Code != "not_found" {
			t.Fatalf("opaque scope %s: %v", tc.name, err)
		}
	}
}

func TestGooseCodeObjectsTransactions(t *testing.T) {
	src := []byte("\xef\xbb\xbfnamespace n;\r\n// docs\r\nfn a() {} fn b() {} fn c() {}\r\nstruct Box { data: u8[>..] }\r\n")
	path := filepath.Join(t.TempDir(), "test.goose")
	read := func(kind, name string) CodeObject {
		t.Helper()
		o, err := ReadCodeObject(path, "goose", kind, name, 0, 0, src)
		if err != nil {
			t.Fatal(err)
		}
		return *o
	}
	a, b, c, box := read("function", "a"), read("function", "b"), read("function", "c"), read("type", "Box")
	changed := b
	changed.Source = "fn b() {\n let value = helper();\n}"
	box.Source = "struct Box { data: u8[>..], next: Box&<u32>? }"
	out, results, err := PlanCodeEdits([]CodeEdit{
		{Operation: "replace", Object: changed},
		{Operation: "insert_after", Object: b, Source: "fn helper() -> i64 { return 2; }\n type Bytes = u8[>..];"},
		{Operation: "remove", Object: a},
		{Operation: "replace", Object: box},
	}, src)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(out, []byte("\xef\xbb\xbfnamespace n;\r\n// docs\r\n ")) || !bytes.Contains(out, []byte("\r\n fn c() {}\r\n")) || bytes.Count(out, []byte("\r\n")) != bytes.Count(out, []byte("\n")) {
		t.Fatalf("surrounding bytes/endings: %q", out)
	}
	if len(results) != 4 || len(results[1].Objects) != 2 || results[1].Objects[0].Name != "helper" || results[1].Objects[1].Name != "Bytes" || len(results[2].Objects) != 0 {
		t.Fatalf("results: %#v", results)
	}
	for _, r := range results {
		if r.Revision != codeRevision(out) {
			t.Fatal("revision differs")
		}
	}
	if _, _, err := PlanCodeEdits([]CodeEdit{{Operation: "remove", Object: c}}, out); err == nil {
		t.Fatal("stale revision accepted")
	}
	next := results[0].Objects[0]
	next.Source = strings.ReplaceAll(next.Source, "value", "answer")
	if _, _, err := ReplaceCodeObject(next, out); err != nil {
		t.Fatal("applied object cannot be reused:", err)
	}
	if _, _, err := PlanCodeEdits([]CodeEdit{{Operation: "remove", Object: b}, {Operation: "replace", Object: changed}}, src); err == nil {
		t.Fatal("overlap accepted")
	}
}

func TestGooseCodeObjectsRefusals(t *testing.T) {
	src := []byte(`namespace n; fn a() {} let global = 3; import module; extern "lib" { fn e(); } fn b() {}`)
	o, err := ReadCodeObject("test.goose", "goose", "function", "a", 0, 0, src)
	if err != nil {
		t.Fatal(err)
	}
	for _, payload := range []string{
		`fn renamed() {}`, `inline fn a() {}`, `recursive fn a() {}`, `thread_fn a() {}`, `fn n::a() {}`,
		`fn a() {} fn extra(){}`, `fn a() {} //`, `fn a() {} /*`, `fn a() { let s="broken; }`, `fn a() {[)]}`,
		`fn a() {} namespace changed;`, `fn a() {} let injected=1;`, `fn a() fn swallowed(){}`,
	} {
		changed := *o
		changed.Source = payload
		if _, _, err := ReplaceCodeObject(changed, src); err == nil {
			t.Fatalf("accepted replacement %q", payload)
		}
	}
	for _, suffix := range []string{`let global = 1;`, `import module;`, `namespace n;`, `extern "lib" { fn e(); }`, `struct Box {x:i64}`, `fn b(){}`} {
		data := []byte(`fn a() {} ` + suffix)
		obj, err := ReadCodeObject("test.goose", "goose", "function", "a", 0, 0, data)
		if strings.HasPrefix(suffix, "namespace") {
			if err == nil {
				t.Fatal("late namespace accepted")
			}
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		obj.Source += " //"
		if _, _, err := ReplaceCodeObject(*obj, data); err == nil {
			t.Fatalf("swallowed neighbor %q", suffix)
		}
	}
	for _, tail := range []string{`/* unterminated`, `fn b(){`, `fn b(){ let s="unterminated; }`, `fn b(){ let s="""unterminated; }`, `fn b(){[)]}`, "\x00"} {
		if _, err := ReadCodeObject("test.goose", "goose", "function", "a", 0, 0, []byte(`fn a() {} `+tail)); err == nil {
			t.Fatalf("invalid file accepted %q", tail)
		}
	}
	for _, payload := range []string{`fn a(){}`, `namespace x; fn c(){}`, `let injected=1;`, `extern "lib" { fn c(); }`} {
		if _, _, err := PlanCodeEdits([]CodeEdit{{Operation: "insert_after", Object: *o, Source: payload}}, src); err == nil {
			t.Fatalf("invalid insertion accepted %q", payload)
		}
	}
	for _, tc := range []struct{ original, changed string }{
		{`extern "c" fn e();`, `extern "other" fn e();`},
		{`extern fn e() = "c";`, `extern fn e() = "other";`},
		{`export "c" fn e(){}`, `export "other" fn e(){}`},
		{`struct e {}`, `enum e {}`},
	} {
		kind := "function"
		if strings.HasPrefix(tc.original, "struct") {
			kind = "type"
		}
		obj, err := ReadCodeObject("test.goose", "goose", kind, "e", 0, 0, []byte(tc.original))
		if err != nil {
			t.Fatal(err)
		}
		obj.Source = tc.changed
		if _, _, err := ReplaceCodeObject(*obj, []byte(tc.original)); err == nil {
			t.Fatal("binding/category changed")
		}
	}
}

func TestGooseCodeObjectsAmbiguity(t *testing.T) {
	src := []byte(`fn f(x: i64){} fn f(x: string){}`)
	_, err := ReadCodeObject("test.goose", "goose", "function", "f", 0, 0, src)
	e, ok := err.(*ObjectError)
	if !ok || e.Code != "ambiguous" || len(e.Candidates) != 2 {
		t.Fatalf("candidates: %#v", err)
	}
	obj, err := ReadCodeObject("test.goose", "goose", "function", "f", 1, e.Candidates[1].Column, src)
	if err != nil || obj.Source != `fn f(x: string){}` {
		t.Fatalf("coordinate selector: %#v %v", obj, err)
	}
	if _, _, err := PlanCodeEdits([]CodeEdit{{Operation: "insert_after", Object: *obj, Source: `fn f(x: bool){}`}}, src); err != nil {
		t.Fatal("overload refused:", err)
	}
	if _, _, err := PlanCodeEdits([]CodeEdit{{Operation: "insert_after", Object: *obj, Source: obj.Source}}, src); err == nil {
		t.Fatal("duplicate accepted")
	}
}

func TestGooseObjectLexicalBoundaries(t *testing.T) {
	src := []byte("\xef\xbb\xbf/* гусь */ fn f() { let raw = \"\"\"\r\n    } /* fn fake(){}\r\n    \"\"\"; } fn keep(){}\r\n")
	obj, err := ReadCodeObject("test.goose", "goose", "function", "f", 0, 0, src)
	if err != nil || obj.Column != bytes.Index(src, []byte("fn f"))+1 {
		t.Fatalf("physical byte column: %#v %v", obj, err)
	}
	out, _, err := ReplaceCodeObject(*obj, src)
	if err != nil || !bytes.Equal(out, src) {
		t.Fatalf("CRLF literal identity: %q %v", out, err)
	}
	// Navigation recovers after an incomplete single-line string; editing
	// validates the whole file and must refuse that same source.
	incomplete := []byte("let s=\"broken\nfn later(){}")
	ts, err := gooseLex(incomplete, false)
	if err != nil || len(ts) == 0 || ts[len(ts)-1].text != "}" {
		t.Fatalf("navigation recovery: %#v %v", ts, err)
	}
	if _, err := gooseObjectTokens(incomplete); err == nil {
		t.Fatal("editing accepted incomplete literal")
	}
}

func FuzzGooseObjectCatalogue(f *testing.F) {
	for _, seed := range []string{`fn f(){}`, `namespace n; type T = u8[>..];`, `extern "c" fn f();`, `fn f<T:Box<u8[>..]>>() -> fn(i64)->bool {}`, `/* /* */ */ fn f(){let raw="""}""";}`} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, src []byte) {
		items, err := gooseCodeItems("fuzz.goose", src, "", "module")
		if err != nil {
			return
		}
		last := 0
		for _, item := range items {
			if item.start < last || item.end <= item.start || item.end > len(src) {
				t.Fatalf("invalid/overlapping byte span: %#v", item)
			}
			last = item.end
		}
	})
}

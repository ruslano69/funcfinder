package internal

import (
	"reflect"
	"strings"
	"testing"
)

func TestGooseCallGraphTokenOwnership(t *testing.T) {
	cfg := langConfigOf(t, "goose")
	lines := strings.Split(`fn Alpha() {} fn Beta() { Alpha(); }
fn outer() {
    fn nested<T>() { Beta(); }
    nested<i64>();
    let text = """fn Fake() { Beta(); }""";
}
recursive fn recur() { recur(); }`, "\n")
	r, err := NewFinder(cfg, nil, true, false, false).FindFunctionsInLines(lines, 1, "calls.goose")
	if err != nil {
		t.Fatal(err)
	}
	got := buildFileCallGraph("calls.goose", lines, r.Functions, cfg, nil, nil)
	want := []CallEdge{{Caller: "Beta", Callee: "Alpha", Line: 1}, {Caller: "nested", Callee: "Beta", Line: 3}, {Caller: "outer", Callee: "nested", Line: 4}, {Caller: "recur", Callee: "recur", Line: 7}}
	if !reflect.DeepEqual(got.Calls, want) {
		t.Fatalf("calls=%+v want=%+v", got.Calls, want)
	}
}

func TestGooseDeclarations(t *testing.T) {
	cfg := langConfigOf(t, "goose")
	lines, err := readFileLines("../test_examples/test_example.goose")
	if err != nil {
		t.Fatal(err)
	}
	r, err := NewFinder(cfg, nil, true, true, false).FindFunctionsInLines(lines, 17, "test.goose")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"native", "identity", "Alpha", "Beta", "main", "nested"}
	if len(r.Functions) != len(want) {
		t.Fatalf("functions: %+v", r.Functions)
	}
	for i, fn := range r.Functions {
		if fn.Name != want[i] || fn.Start < 17 || fn.End < fn.Start || strings.Join(fn.Lines, "\n") != strings.Join(lines[fn.Start-17:fn.End-17+1], "\n") {
			t.Fatalf("bad declaration: %+v", fn)
		}
	}
	if r.Functions[2].Start != r.Functions[3].Start {
		t.Fatal("shared-line functions lost")
	}
	factory := NewStructFinderFactory().CreateStructFinder(cfg, "", true, true)
	types, err := factory.FindStructuresInLines(lines, 17, "test.goose")
	if err != nil || len(types.Types) != 3 {
		t.Fatalf("types=%+v error=%v", types, err)
	}
	for i, name := range []string{"Point", "Value", "Count"} {
		if types.Types[i].Name != name {
			t.Fatalf("wrong types: %+v", types)
		}
	}
	selected, err := CreateFinder(cfg, "nested", "extract", true, false).FindFunctions("../test_examples/test_example.goose")
	if err != nil || len(selected.Functions) != 1 || selected.Functions[0].Name != "nested" {
		t.Fatalf("selection=%+v error=%v", selected, err)
	}
}

func TestGooseLexicalAndHeaderEdges(t *testing.T) {
	cfg := langConfigOf(t, "goose")
	src := `extern "lib" { fn delay(ms: u32) = "delay"; }
checkfree
inline fn ns::run<T>(x: T, n: i64 = (if true { 1 } else { 2 })) -> fn(i64) -> i64 {
    let c = '\'';
    let text = "fn Nope() { }";
    let raw = """fn AlsoNope() {}""";
    /* outer /* inner */ fn Comment() {} */
    fn inner() { }
    return fn(n: i64) { return n; };
}
thread_fn worker(n: i64) -> i64 { return n; }
struct Box<T> { value: T, cb: fn(i64) -> i64 }
type Callback = fn(i64) -> i64;`
	lines := strings.Split(src, "\n")
	r, err := NewFinder(cfg, nil, true, true, false).FindFunctionsInLines(lines, 1, "edges.goose")
	if err != nil || len(r.Functions) != 4 {
		t.Fatalf("functions=%+v error=%v", r, err)
	}
	for i, name := range []string{"delay", "ns::run", "inner", "worker"} {
		if r.Functions[i].Name != name {
			t.Fatalf("functions: %+v", r.Functions)
		}
	}
	if r.Functions[1].Start != 2 || r.Functions[1].End != 10 || r.Functions[1].SignatureLine != 3 || r.Functions[1].SignatureName != "run" {
		t.Fatalf("header/body boundary: %+v", r.Functions[1])
	}
	s := NewSanitizer(cfg, false)
	facts := s.CleanLinesFacts([]string{"/* outer", "/* inner */ fn Fake() {}", "*/ fn Real() {}"})
	if strings.TrimSpace(facts[1].Cleaned) != "" || !strings.Contains(facts[2].Cleaned, "fn Real") {
		t.Fatalf("nested comment leaked: %+v", facts)
	}
}

func TestGooseLanguageRegistration(t *testing.T) {
	config, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.GetLanguageByExtension(".goose")
	if cfg == nil || cfg.LangKey != "goose" {
		t.Fatalf("config=%+v", cfg)
	}
	if cfg.ImportRegex().FindStringSubmatch("import .utils.text;")[1] != ".utils.text" {
		t.Fatal("relative import lost")
	}
}

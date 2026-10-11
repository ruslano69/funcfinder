package internal

import (
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// langConfigOf returns the language config by its key.
func langConfigOf(t *testing.T, lang string) *LanguageConfig {
	t.Helper()
	config, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	cfg := config[lang]
	if cfg == nil {
		t.Fatalf("%s config not found", lang)
	}
	return cfg
}

// mapFunctions runs the function finder in map mode, the way --map does.
func mapFunctions(t *testing.T, lang, src string) []FunctionBounds {
	t.Helper()
	cfg := langConfigOf(t, lang)
	res, err := NewFinder(cfg, nil, true, false, false).FindFunctionsInLines(strings.Split(src, "\n"), 1, "test."+lang)
	if err != nil {
		t.Fatalf("FindFunctionsInLines: %v", err)
	}
	return res.Functions
}

// spans returns "name:start-end" for each function, sorted.
func spans(fns []FunctionBounds) []string {
	var out []string
	for _, f := range fns {
		out = append(out, f.Name+":"+strconv.Itoa(f.Start)+"-"+strconv.Itoa(f.End))
	}
	sort.Strings(out)
	return out
}

// A trait's method declaration ends with ';' and has no body. It is listed on its
// own line and must not take the body of the next function.
func TestDeclarations_RustTraitDoesNotSwallowNextFunction(t *testing.T) {
	src := `pub trait Predictor {
    fn get(&mut self) -> i32;
    fn update(&mut self, bit: u8);
}

fn write_var_int(v: u32) -> u32 {
    v + 1
}

fn after() {
    let x = 1;
}
`
	got := spans(mapFunctions(t, "rust", src))
	want := []string{"after:10-12", "get:2-2", "update:3-3", "write_var_int:6-8"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("functions = %v, want %v", got, want)
	}
}

// A file whose only function-like lines are declarations and one function must
// still yield that function.
func TestDeclarations_OnlyDeclarationsAndOneFunction(t *testing.T) {
	src := `trait P {
    fn a(&self);
    fn b(&self);
}
fn f() {}
`
	got := spans(mapFunctions(t, "rust", src))
	want := []string{"a:2-2", "b:3-3", "f:5-5"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("functions = %v, want %v", got, want)
	}
}

// A signature can span lines and end with ';'. The ';' inside [u8; 4] is not the
// end of the signature.
func TestDeclarations_MultiLineSignatureEndingInSemicolon(t *testing.T) {
	src := `trait T {
    fn long(
        a: [u8; 4],
        b: u32,
    ) -> u32;
}
fn sized(a: [u8; 4]) -> u8 {
    a[0]
}
fn after() {
}
`
	got := spans(mapFunctions(t, "rust", src))
	want := []string{"after:10-11", "long:2-5", "sized:7-9"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("functions = %v, want %v", got, want)
	}
}

// A C prototype is a declaration: it must not swallow main. The C header pattern
// needs the line to end at ')', so a prototype ending in ';' is not a candidate
// at all; main's brace is on its own line, as the C pattern expects.
func TestDeclarations_CPrototypeDoesNotSwallowMain(t *testing.T) {
	src := `int add(int a, int b);
int main(void)
{
    return 0;
}
`
	got := spans(mapFunctions(t, "c", src))
	want := []string{"main:2-5"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("functions = %v, want %v", got, want)
	}
}

// An abstract Java method is a declaration. The method after it is found.
func TestDeclarations_JavaAbstractMethodDoesNotSwallowNext(t *testing.T) {
	src := `abstract class A {
    abstract void f();
    void g() {
    }
}
`
	got := spans(mapFunctions(t, "java", src))
	want := []string{"g:3-4"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("functions = %v, want %v", got, want)
	}
}

// The nested path (TypeScript): an overload signature is a declaration on its own
// line; the implementation and the function after it keep their own spans.
func TestDeclarations_TypeScriptOverloadOnNestedPath(t *testing.T) {
	src := `function f(a: number): void;
function f(a: any) {
}
function g() {
}
`
	got := spans(mapFunctions(t, "ts", src))
	want := []string{"f:1-1", "f:2-3", "g:4-5"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("functions = %v, want %v", got, want)
	}
}

// A C# expression body on the line after its signature ends at its own ';'. The
// C header pattern needs the signature line to end at ')', so the body is on the
// next line here, as C# code commonly lays it out.
func TestDeclarations_CSharpExpressionBody(t *testing.T) {
	src := `class C {
    int G()
        => 5;
    int H()
    {
        return 1;
    }
}
`
	got := spans(mapFunctions(t, "cs", src))
	want := []string{"G:2-3", "H:4-7"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("functions = %v, want %v", got, want)
	}
}

// On the nested path an arrow expression body ends at its own ';', and a body
// brace alone on the line under the signature closes on that line too.
func TestDeclarations_JSArrowExpressionAndBraceOnOwnLine(t *testing.T) {
	src := `const f = (a) => a + 1;
function g()
{ return 2; }
function h() {
}
`
	got := spans(mapFunctions(t, "js", src))
	want := []string{"f:1-1", "g:2-3", "h:4-5"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("functions = %v, want %v", got, want)
	}
}

// sigScanner reads a signature line by line until it decides, as the finder does.
func TestSigScanner(t *testing.T) {
	tests := []struct {
		name  string
		lines []string
		want  sigEvent
	}{
		{"prototype ends with semicolon", []string{"int add(int a, int b);"}, sigEnd},
		{"brace opens body", []string{"int main(void) {"}, sigBody},
		{"semicolon inside brackets is part of the type", []string{"fn f(a: [u8; 4]) -> u8;"}, sigEnd},
		{"multi-line signature ends in semicolon", []string{"fn long(", "    a: [u8; 4],", "    b: u32,", ") -> u32;"}, sigEnd},
		{"lambda in a parameter default stays inside brackets", []string{"void F(Func<int> f = () => 1);"}, sigEnd},
		{"arrow expression body ends at its semicolon", []string{"int G() => 5;"}, sigEnd},
		{"expression body on the next line", []string{"int G()", "    => Compute(1);"}, sigEnd},
		{"brace after arrow opens a body", []string{"int G() => new X {"}, sigBody},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := &sigScanner{}
			var got sigEvent
			for _, line := range tc.lines {
				got = s.feed(line)
				if got != sigOpen {
					break
				}
			}
			if got != tc.want {
				t.Fatalf("feed %q = %v, want %v", tc.lines, got, tc.want)
			}
		})
	}
}

// A signature with neither a body nor a ';' (a Kotlin interface method) ends on
// its own line when the next function's header begins, so that function is not
// swallowed.
func TestDeclarations_KotlinBodylessWithoutSemicolon(t *testing.T) {
	src := `interface Shape {
    fun area(): Double
    fun name(): String {
        return "x"
    }
}
fun other() {
}
`
	got := spans(mapFunctions(t, "kotlin", src))
	want := []string{"area:2-2", "name:3-5", "other:7-8"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("functions = %v, want %v", got, want)
	}
}

// The same on the nested path: a Swift protocol requirement has no ';' either.
func TestDeclarations_SwiftRequirementWithoutSemicolonOnNestedPath(t *testing.T) {
	src := `protocol Shape {
    func area() -> Double
    func name() -> String {
        return "x"
    }
}
func other() {
}
`
	got := spans(mapFunctions(t, "swift", src))
	want := []string{"area:2-2", "name:3-5", "other:7-8"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("functions = %v, want %v", got, want)
	}
}

// A function declared inside another function's body is a function of its own:
// Rust allows it, and the outer span still covers it.
func TestDeclarations_RustNestedFunctionIsMapped(t *testing.T) {
	src := `fn outer() -> i32 {
    fn inner(x: i32) -> i32 {
        x + 1
    }
    inner(1)
}
fn after() {}
`
	got := spans(mapFunctions(t, "rust", src))
	want := []string{"after:7-7", "inner:2-4", "outer:1-6"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("functions = %v, want %v", got, want)
	}
}

// A Kotlin member with a modifier before fun (override, abstract), an annotation,
// or an extension receiver is a function like any other.
func TestFuncPattern_KotlinModifiersAnnotationsAndReceiver(t *testing.T) {
	src := `class App : Application() {
    override fun onCreate() {
        super.onCreate()
    }
    protected abstract fun AlertDialog.Builder.prepare(listener: Int)
    @Composable private fun Greeting(name: String) {
    }
}
`
	got := spans(mapFunctions(t, "kotlin", src))
	want := []string{"Greeting:6-7", "onCreate:2-4", "prepare:5-5"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("functions = %v, want %v", got, want)
	}
}

// Swift members carry attributes and modifiers before func (override, @objc,
// @IBAction): they are functions too.
func TestFuncPattern_SwiftAttributesAndModifiers(t *testing.T) {
	src := `class DetailViewController: UIViewController {
    override func viewDidAppear(_ animated: Bool) {
        super.viewDidAppear(animated)
    }
    @IBAction func refresh() {
    }
    @objc private func toggle(_ sender: Any) {
    }
}
`
	got := spans(mapFunctions(t, "swift", src))
	want := []string{"refresh:5-6", "toggle:7-8", "viewDidAppear:2-4"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("functions = %v, want %v", got, want)
	}
}

// A balanced brace inside an open parameter list (a default value with a block)
// is part of the signature: the body starts at the brace after the parentheses.
func TestDeclarations_BalancedBraceInParameterDoesNotEndSignature(t *testing.T) {
	src := `fun Chip(
    selected: Boolean,
    interactionSource: MutableInteractionSource = remember { MutableInteractionSource() },
) {
    val x = 1
}
fun after() {
}
`
	got := spans(mapFunctions(t, "kotlin", src))
	want := []string{"Chip:1-6", "after:7-8"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("functions = %v, want %v", got, want)
	}
}

// A header whose parameter list stays open after a balanced brace is not closed
// on that line; its expression ends on the line before the next function.
func TestDeclarations_HeaderWithBalancedBraceStaysOpen(t *testing.T) {
	src := `fun Pref(name: String, defaultValue: () -> String = { "" },
) = Make(name, defaultValue)
fun after() {
}
`
	got := spans(mapFunctions(t, "kotlin", src))
	want := []string{"Pref:1-2", "after:3-4"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("functions = %v, want %v", got, want)
	}
}

// An annotation and a property after an expression body are not part of it.
func TestDeclarations_AnnotationAfterExpressionIsNotPartOfIt(t *testing.T) {
	src := `class A {
    fun exitTween() = tween(100)
    @Composable
    val x = 1
    fun other() {
    }
}
`
	got := spans(mapFunctions(t, "kotlin", src))
	want := []string{"exitTween:2-2", "other:5-6"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("functions = %v, want %v", got, want)
	}
}

// The same on the nested path: a property after a Swift requirement is not part of it.
func TestDeclarations_PropertyAfterRequirementIsNotPartOfIt(t *testing.T) {
	src := `protocol Shape {
    func area() -> Double
    var name: String
    func other() {
    }
}
`
	got := spans(mapFunctions(t, "swift", src))
	want := []string{"area:2-2", "other:4-5"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("functions = %v, want %v", got, want)
	}
}

// An expression body followed by a class ends on its own last line. An anonymous
// object on the continuation line is part of the expression.
func TestDeclarations_ExpressionBodyEndsBeforeClassDeclaration(t *testing.T) {
	src := `class A {
    fun exitTween() = tween(100)
    class Inner {
    }
    fun listen() =
        object : Listener {
            override fun on() {
            }
        }
}
`
	got := spans(mapFunctions(t, "kotlin", src))
	want := []string{"exitTween:2-2", "listen:5-9"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("functions = %v, want %v", got, want)
	}
}

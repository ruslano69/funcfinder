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

// A C prototype is a declaration: it is listed on its own line and does not swallow
// main.
func TestDeclarations_CPrototypeDoesNotSwallowMain(t *testing.T) {
	src := `int add(int a, int b);
int main(void)
{
    return 0;
}
`
	got := spans(mapFunctions(t, "c", src))
	want := []string{"add:1-1", "main:2-5"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("functions = %v, want %v", got, want)
	}
}

// An abstract Java method is a declaration: it is listed on its own line, and the method
// after it is found with its own span.
func TestDeclarations_JavaAbstractMethodDoesNotSwallowNext(t *testing.T) {
	src := `abstract class A {
    abstract void f();
    void g() {
    }
}
`
	got := spans(mapFunctions(t, "java", src))
	want := []string{"f:2-2", "g:3-4"}
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

// A block inside an expression body closes on its own brace; the expression goes
// on with a call chain, so the function ends after the chain.
func TestDeclarations_ChainAfterBlockInExpressionBody(t *testing.T) {
	src := `fun openFile(path: String) =
    path
        .runCatching {
            open(this)
                ?: throw Exception()
        }
        .onFailure { report(it) }
fun after() {
}
`
	got := spans(mapFunctions(t, "kotlin", src))
	want := []string{"after:8-9", "openFile:1-7"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("functions = %v, want %v", got, want)
	}
}

// An if-expression whose else starts the line after the closing brace of the then-block
// is one expression: the function ends after the else block.
func TestDeclarations_ElseAfterBlockInExpressionBody(t *testing.T) {
	src := `fun pick(x: Int) = if (x > 0) {
    1
}
else {
    2
}
fun after() {
}
`
	got := spans(mapFunctions(t, "kotlin", src))
	want := []string{"after:7-8", "pick:1-6"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("functions = %v, want %v", got, want)
	}
}

// Braces inside the parentheses of a call that forms an expression body (a transition
// lambda with a block) do not end the function; the expression ends after the call.
func TestDeclarations_BracesInsideCallArgumentsDoNotEndFunction(t *testing.T) {
	src := `fun Nav.screen(route: String) =
    composable(
        enterTransition = { slide(1) },
        exitTransition = {
            fade()
        },
    )
fun after() {
}
`
	got := spans(mapFunctions(t, "kotlin", src))
	want := []string{"after:8-9", "screen:1-7"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("functions = %v, want %v", got, want)
	}
}

// A chain that keeps going after a block ends at its last line, even when that line
// has no brace of its own.
func TestDeclarations_ChainContinuesPastBlockUntilItEnds(t *testing.T) {
	src := `fun load() =
    fetch()
        .map {
            it
        }
        .flow()
fun after() {
}
`
	got := spans(mapFunctions(t, "kotlin", src))
	want := []string{"after:7-8", "load:1-6"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("functions = %v, want %v", got, want)
	}
}

// A line that starts with a closing brace but sits inside the parentheses of the
// expression is part of it: the expression ends on that line.
func TestDeclarations_ClosingBraceInsideCallEndsExpressionOnItsLine(t *testing.T) {
	src := `fun pick(x: Int) = listOf(x, {
    x
}, x)
fun after() {
}
`
	got := spans(mapFunctions(t, "kotlin", src))
	want := []string{"after:4-5", "pick:1-3"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("functions = %v, want %v", got, want)
	}
}

// An expression body that is the last thing in the file ends on its last line.
func TestDeclarations_ExpressionAtEndOfFileEndsOnItsLine(t *testing.T) {
	src := `fun pick(x: Int) =
    listOf(
        x,
    )
`
	got := spans(mapFunctions(t, "kotlin", src))
	want := []string{"pick:1-4"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("functions = %v, want %v", got, want)
	}
}

// A Kotlin raw string closed by a run of quotes ("""..."""") and a multi-line raw
// string containing quotes do not change the quote state of the rest of the file.
func TestDeclarations_KotlinRawStringRunOfQuotes(t *testing.T) {
	src := `const val A = """--x "ext""""
val s = """
    say "hi"
"""
fun after() {
}
`
	got := spans(mapFunctions(t, "kotlin", src))
	want := []string{"after:5-6"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("functions = %v, want %v", got, want)
	}
}

// A single-line header whose braces balance but which ends in '=' keeps its expression
// on the next line.
func TestDeclarations_HeaderEndingInEqualsContinuesOnNextLine(t *testing.T) {
	src := `fun String.getInt(default: Int = Defaults.getOrElse(this) { 0 }): Int =
    kv.decodeInt(this, default)
fun after() {
}
`
	got := spans(mapFunctions(t, "kotlin", src))
	want := []string{"after:3-4", "getInt:1-2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("functions = %v, want %v", got, want)
	}
}

// A C function whose opening brace is on its header line is found, and the if
// inside it is not a function.
func TestDeclarations_CBraceOnHeaderLine(t *testing.T) {
	src := `static int add(int a, int b) {
  return a + b;
}
int main(void) {
  if (add(1, 2))
    return 0;
  return 1;
}
`
	got := spans(mapFunctions(t, "c", src))
	want := []string{"add:1-3", "main:4-8"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("functions = %v, want %v", got, want)
	}
}

// A control statement whose condition ends a line, with its block on the next line,
// is not a function.
func TestDeclarations_CControlStatementIsNotAFunction(t *testing.T) {
	src := `void f(void)
{
  if (x)
  {
    y();
  }
}
`
	got := spans(mapFunctions(t, "c", src))
	want := []string{"f:1-7"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("functions = %v, want %v", got, want)
	}
}

// A pointer return type written against the name (static T *f (...) {) is a function.
func TestDeclarations_CPointerReturnTypeWrittenAgainstName(t *testing.T) {
	src := `static int *first (int *a) {
  return a;
}
`
	got := spans(mapFunctions(t, "c", src))
	want := []string{"first:1-3"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("functions = %v, want %v", got, want)
	}
}

// A throws clause does not become the method's name.
func TestDeclarations_JavaThrowsClauseKeepsMethodName(t *testing.T) {
	src := `class A {
    public void write(int x) throws IOException {
        y();
    }
}
`
	got := spans(mapFunctions(t, "java", src))
	want := []string{"write:2-4"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("functions = %v, want %v", got, want)
	}
}

// A Java signature that spans lines is found from its first line.
func TestDeclarations_JavaMultiLineSignature(t *testing.T) {
	src := `class A {
    public static <T> B<T> of(
        int x) {
        return null;
    }
}
`
	got := spans(mapFunctions(t, "java", src))
	want := []string{"of:2-5"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("functions = %v, want %v", got, want)
	}
}

// A C++ member definition with a qualified name and a body on its header line, and a
// trailing return type, are functions.
func TestDeclarations_CppQualifiedNameAndTrailingReturnType(t *testing.T) {
	src := `void Foo::bar(int x) const {
  y();
}
auto g() -> int {
  return 1;
}
`
	got := spans(mapFunctions(t, "cpp", src))
	want := []string{"bar:1-3", "g:4-6"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("functions = %v, want %v", got, want)
	}
}

// A C# member whose expression body starts on the header line (=>) is a function.
func TestDeclarations_CSharpExpressionArrowOnHeaderLine(t *testing.T) {
	src := `class A {
    public void Bulk() =>
        Foo();
    public int Baz() {
        return 1;
    }
}
`
	got := spans(mapFunctions(t, "cs", src))
	want := []string{"Baz:4-6", "Bulk:2-3"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("functions = %v, want %v", got, want)
	}
}

// A Ruby method ends with the end at its header's indentation; a nested if has its own end,
// and a singleton method is found.
func TestDeclarations_RubyMethodEndsAtItsEnd(t *testing.T) {
	src := `class Greeter
  def hello
    puts "hi"; puts "there"
  end

  def bye(name)
    if name
      puts name
    end
  end

  def self.make
    new
  end
end
`
	got := spans(mapFunctions(t, "ruby", src))
	want := []string{"bye:6-10", "hello:2-4", "make:12-14"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("functions = %v, want %v", got, want)
	}
}

// The body of a heredoc is text: a backtick or a quote in it does not change what
// follows.
func TestDeclarations_RubyHeredocBodyIsNotCode(t *testing.T) {
	src := strings.Join([]string{
		"class A",
		"  def doc",
		"    <<~MD",
		"      Set `x` to \"y\"",
		"    MD",
		"  end",
		"  def after",
		"  end",
		"end",
	}, "\n") + "\n"
	got := spans(mapFunctions(t, "ruby", src))
	want := []string{"after:7-8", "doc:2-6"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("functions = %v, want %v", got, want)
	}
}

// An endless method and a method closed on its own line are one-line functions.
func TestDeclarations_RubyOneLineMethods(t *testing.T) {
	src := `def a = 1
def b
end
`
	got := spans(mapFunctions(t, "ruby", src))
	want := []string{"a:1-1", "b:2-3"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("functions = %v, want %v", got, want)
	}
}

// TypeScript class members are functions: a constructor, a getter, a static and an async
// method with a block inside. The if is a statement, not a method.
func TestDeclarations_TSClassMethods(t *testing.T) {
	src := `export class Box {
  private value: number;
  constructor(value: number) {
    this.value = value;
  }
  get size(): number {
    return this.value;
  }
  static of(v: number): Box {
    return new Box(v);
  }
  async load<T>(x: T): Promise<T> {
    if (x) {
      return x;
    }
    return x;
  }
}
`
	got := spans(mapFunctions(t, "ts", src))
	want := []string{"constructor:3-5", "load:12-17", "of:9-11", "size:6-8"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("functions = %v, want %v", got, want)
	}
}

// A const bound to a value that awaits (no arrow, no semicolon) is not a function.
func TestDeclarations_AwaitingConstValueIsNotAFunction(t *testing.T) {
	src := `const rows = (await db.query("x"))
function f() {
}
`
	got := spans(mapFunctions(t, "js", src))
	want := []string{"f:2-3"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("functions = %v, want %v", got, want)
	}
}

// A regular expression literal with a quote in it does not open a string: the functions
// after it are found.
func TestDeclarations_JSRegexLiteralWithQuote(t *testing.T) {
	src := `const f = (v) => {
  return /\"/.test(v) ? 'x' : 'y';
};
function after() {
}
`
	got := spans(mapFunctions(t, "js", src))
	want := []string{"after:4-5", "f:1-3"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("functions = %v, want %v", got, want)
	}
}

// A header whose balanced braces sit inside an open parenthesis (a generic argument) is
// not complete: the expression goes on to the line that closes it.
func TestDeclarations_JSHeaderBracesInsideParenthesisStayOpen(t *testing.T) {
	src := `const history = async () => (await query<{ f: string }>(
  "x"
));
function after() {
}
`
	got := spans(mapFunctions(t, "js", src))
	want := []string{"after:4-5", "history:1-3"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("functions = %v, want %v", got, want)
	}
}

// A TypeScript class method whose parameter list continues on the next lines is found from
// its first line, and the member after it keeps its own span.
func TestDeclarations_TSClassMethodSignatureOnSeveralLines(t *testing.T) {
	src := `class A {
  async run(
    x: number,
  ): Promise<void> {
    return;
  }
  get size() {
    return 1;
  }
}
`
	got := spans(mapFunctions(t, "ts", src))
	want := []string{"run:2-6", "size:7-9"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("functions = %v, want %v", got, want)
	}
}

// Braces in a default value of a multi-line parameter list are not the body.
func TestDeclarations_TSClassMethodDefaultObjectDoesNotEndIt(t *testing.T) {
	src := `class A {
  f(
    o: { a: number } = {},
  ): void {
    return;
  }
  g() {
  }
}
`
	got := spans(mapFunctions(t, "ts", src))
	want := []string{"f:2-6", "g:7-8"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("functions = %v, want %v", got, want)
	}
}

// A Java method whose parameter list continues on the next lines is found from its first
// line, and the member after it keeps its own span.
func TestDeclarations_JavaMethodSignatureOnSeveralLinesInClass(t *testing.T) {
	src := `class A {
    public static <T> B<T> of(
        int x) {
        return null;
    }
    void g() {
    }
}
`
	got := spans(mapFunctions(t, "java", src))
	want := []string{"g:6-7", "of:2-5"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("functions = %v, want %v", got, want)
	}
}

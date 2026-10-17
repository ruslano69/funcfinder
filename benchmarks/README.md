# benchmarks — the spec-sheet harness

funcfinder's pitch is **comprehension at ~5% of the token cost and a fraction of
the time**, with a map that's *good enough to navigate* — not a perfect parser.
This harness produces the honest, per-project numbers that back that claim, and
it does so reproducibly so anyone can re-run them.

## The AST ruler (not a competitor — a measuring tape)

`cmd/astoracle` is a real Go parser (`go/ast`, zero heuristics). It emits the
exact same JSON shape as `funcfinder --dir <d> --all --json`, so the two can be
diffed directly. It is the **ground truth** against which funcfinder's regex
output is scored. Go only, on purpose: the ruler must be unimpeachable on the
flagship language. Other languages get their own native oracle (Python `ast`,
the TS compiler API, …) or tree-sitter as a second tier.

The ruler walks the whole AST (`ast.Inspect`), not just top-level `f.Decls`, so
it captures **function-local type declarations** (`func f() { type T … }`) that
funcfinder — being line-based — also reports. An earlier top-level-only version
undercounted these and unfairly dented funcfinder's precision; a ruler that
undercounts is a broken ruler.

A **type alias** (`type A = S`, or a line `A = S` of a `type ( ... )` group) is a
name for an existing type, not a new one. Both tools list aliases apart from the
types, in a file's `aliases` list (`name`, `line`, `target`), so one struct with
several names is counted once. The `classes` list compared by `specsheet.py` holds
only real types. On the working tree of 2026-10-13 the self-host run gives types
2376/2376 (recall and precision 100%) and aliases 21/21 against the ruler.

The ruler exists to *measure*, never to become the product. The target is not
"match the AST 1:1" — past "the agent reaches the right code", extra accuracy
costs tokens and time, which are the product. The ruler tells us **where** the
misses are so we can split them into cheap regex fixes (community engine) vs
fundamentally-hard cases (the parser-tier's reason to exist), quantified.

## The two columns this produces (phase 1)

- **Accuracy** — recall + precision of symbols vs the ruler, matched by
  `(name, line ±2)` per file, computed over the file intersection so
  file-selection differences don't corrupt the number.
- **Token savings** — tokens of the funcfinder map vs the raw source it
  replaces.

Time-to-comprehension and task-success are **phase 2**: they need an
agent-in-the-loop runner and are deliberately not faked here.

## Run it

```bash
# build the funcfinder + oracle binaries (from repo root)
go build -o funcfinder.exe ./cmd/funcfinder
go build -o astoracle.exe  ./cmd/astoracle

# point at a checked-out reference project (pin a commit for reproducibility)
funcfinder.exe --dir <proj> --all --json --no-gitignore > ff.json
astoracle.exe  <proj> > oracle.json

python benchmarks/specsheet.py \
  --funcfinder ff.json --oracle oracle.json \
  --root <proj> --label "tdtp (Go)" --sha <commit>
```

`tiktoken` is used for token counts if installed; otherwise it falls back to a
`chars/4` approximation (flagged in the output), which is conservative.

## Recorded rows

### Python function spans

`python_ast_eval.py` uses Python's native `ast` and scans every tracked `.py` file
in a checkout. It retains duplicate method names by parsing plain `--map` output.
Names match one-to-one between the first decorator and the `def` line; end lines
are compared after removing trailing blank/comment lines from funcfinder spans.
It reports missing functions, false positives and wrong ends separately, including
files with no functions and syntax errors. It does not score classes or tokens.

```bash
python benchmarks/python_ast_eval.py --root <checkout> --label flask \
  --binary ./funcfinder.exe --output build/flask.ast-eval.json
```

Use a Python runtime that supports the target syntax. The output records the
runtime, checkout SHA, dirty states, binary SHA-256 and funcfinder repository SHA.
The repository SHA identifies the evaluator checkout; the binary hash identifies
the exact executable. Build with `build.ps1` on Windows or `build.sh` elsewhere.
Python measurements and project pins from 2026-10-12 are recorded in `TODO.md`.

### PHP function spans

`php_tree_eval.py` uses tree-sitter PHP and scans all tracked `.php` files. Truth
includes named functions, methods and bodyless interface/abstract declarations;
anonymous closures are excluded. Names match one-to-one between the declaration
start and name token line, and ends must match the syntax tree exactly. Files
with parse errors are reported and excluded rather than scored from partial trees.

```bash
python -m pip install --target build/eval-runtime tree-sitter==0.26.0 tree-sitter-php==0.25.1
python benchmarks/php_tree_eval.py --root <checkout> --label monolog \
  --binary ./funcfinder.exe --module-dir build/eval-runtime \
  --output build/monolog.php-eval.json
```

Validated with Python 3.14.6 on Windows. The native Point wrapper crashed during
large traversals in this runtime; byte offsets are converted to exact line numbers
instead. The output records both grammar versions, project/source SHAs, dirty
states and binary SHA-256. PHP results and project pins are in `TODO.md`.

### Scala and D function spans

`scala_d_tree_eval.py` scans tracked `.scala` or `.d` files. Scala truth includes
definitions and bodyless declarations; D includes named functions, constructors,
destructors and postblits. Anonymous lambdas and eponymous value/type templates are
excluded. Reference endpoints use the last non-extra syntax-tree token, avoiding
comments/whitespace attached to Scala indentation blocks. Parse-error files are
reported and excluded from scoring.

The D grammar can label `final switch` as a function and constructor-delegation
calls as constructors. The evaluator rejects these using syntax and enclosing
function context, stopping at local class boundaries so their constructors remain
in truth. Each rejection is recorded as `oracle_rejected`, with its reason and
location, and counted separately from parse-error files and finder mismatches.
`final switch` is a statement in the [D specification](https://dlang.org/spec/statement.html#FinalSwitchStatement).

```bash
python -m pip install --target build/eval-runtime tree-sitter-scala==0.26.2
python benchmarks/scala_d_tree_eval.py --language scala --root <scopt-checkout> \
  --label scopt --binary ./funcfinder.exe --module-dir build/eval-runtime \
  --output build/scopt.scala-eval.json

python -m pip install --target build/d-eval-runtime tree-sitter-language-pack==1.22.0
python benchmarks/scala_d_tree_eval.py --language d --root <d-yaml-checkout> \
  --label d-yaml --binary ./funcfinder.exe --module-dir build/d-eval-runtime \
  --grammar-cache build/d-grammar-cache --output build/d-yaml.d-eval.json
```

The first D run downloads its grammar into the explicit cache and needs network
access; later runs reuse it. Output records the D grammar DLL hash as well as
runtime/package versions, checkout SHAs, dirty states and executable hash. Current
pins, measurements and excluded-file limitations are in root `TODO.md`.

### C definitions and prototypes

`c_tree_eval.py` scans tracked `.c` and `.h` files after GCC preprocessing.
Compiler line markers map definitions and prototypes to original source lines;
function-pointer variables and typedefs are excluded. Candidate declarations in
inactive branches are recorded separately. Preprocessing or grammar failures are
explicit exclusions, never successful coverage.

```bash
python -m pip install --target build/c-eval-runtime tree-sitter==0.26.0 tree-sitter-c==0.24.1
python benchmarks/c_tree_eval.py --root <lua-checkout> --label lua \
  --binary ./funcfinder.exe --module-dir build/c-eval-runtime --gcc <gcc-executable> \
  --include-dir <lua-checkout> --include-header <lua-checkout>/lua.h \
  --define LUA_USE_JUMPTABLE=0 --output build/lua.c-eval.json
```

The recorded Lua profile uses C99 and MSYS2 UCRT64 GCC 15.2.0 (Rev8) on Windows.
The include header supplies context for standalone headers; disabling the jump
table selects Lua's standard dispatch path. The evaluator masks GCC va_arg and
offsetof expressions that the grammar cannot parse, preserving bytes/newlines
and function boundaries. Counts include both definitions and prototypes, so
they cannot be compared directly with earlier definition-only measurements.

Output retains project and finder SHAs, dirty states, parser versions, compiler
version/hash, preprocessing flags, executable hash and every mismatch/inactive
candidate. Results describe the selected compiler/configuration; other conditional
variants need separate runs. On Windows the evaluator adds the GCC binary directory
to its child process PATH so the compiler subprocesses can load their dependencies.

### C++ functions and declarations

`cpp_tree_eval.py` scans tracked `.cc`, `.cpp`, `.cxx`, `.hpp` and `.h` files with
libclang, the front end the compilers use, so macros such as `FMT_CONSTEXPR` are
expanded as the compiler sees them. Truth includes constructors, destructors and
bodyless declarations; a declaration's end is the `;` after its trailing return type.
Active lines come from `g++ -E` with the same standard, so candidates inside disabled
`#if` branches are recorded separately. Files clang cannot parse are explicit exclusions.

```bash
python -m pip install libclang
python benchmarks/cpp_tree_eval.py --root <fmt-checkout> --label fmt \
  --binary ./funcfinder.exe --gxx <g++-executable> \
  --include-dir <fmt-checkout>/include \
  --system-include <gcc-lib>/include/c++ --system-include <gcc-lib>/include \
  --system-include <gcc-lib>/include-fixed --system-include <sysroot>/include \
  --pathspec include --pathspec src --output build/fmt.cpp-eval.json
```

The recorded fmt profile uses `b4ffa39b9994c961418406f47358791d89a5aa48`, C++17,
the `x86_64-w64-mingw32` target, MinGW-Builds GCC 15.2.0 and libclang 18.1.1. The
evaluator measures `include/` and `src/` only: the tests need a vendored gtest that
is not on the include path. `src/fmt.cc` (a C++20 module unit) and `src/os.cc` (an
x86 builtin this target does not provide) are excluded, so 18 files are scored.

Output retains the project and finder SHAs, dirty states, libclang and compiler
versions and hashes, the standard, target, system include directories and every
mismatch. The remaining misses are conversion operators (named by their canonical
type, which funcfinder does not reproduce), macro-generated declarations and
template specializations on their own line; the remaining false positives are
explicit instantiations (`extern template`, `template FMT_API`).

### TypeScript functions and declarations

`ts_tree_eval.py` scans tracked `.ts` and `.tsx` files with tree-sitter-typescript. Truth
covers function declarations and signatures (overloads, `declare function`), class and
interface methods (bodyless ones included), constructors, and functions bound to a variable
(`const f = () => ...`). Anonymous callbacks and computed method names are not truth; they are
counted apart. A string-named method keeps its quotes, as the finder names it.

```bash
python -m pip install tree-sitter-typescript
python benchmarks/ts_tree_eval.py --root <zod-checkout> --label zod \
  --binary ./funcfinder.exe --output build/zod.ts-eval.json
```

The recorded zod profile uses `b500a2ee95134f81d74da7911d7378dd2853d042` and the
hr_gemini profile `5b69a1eb`; both are measured on 2026-10-13 with tree-sitter-typescript 0.23.2.

### C# named-function spans

`csharp_roslyn_eval.py` compiles `csharp_oracle.cs` against Roslyn assemblies in
the installed .NET SDK. It needs no NuGet packages or project compilation. The
generated project, build output and CLI home stay under `--work-dir` in `build/`.

```bash
python benchmarks/csharp_roslyn_eval.py --root <polly-checkout> --label polly \
  --binary ./funcfinder.exe --work-dir build/csharp-eval/oracle \
  --output build/csharp-eval/polly.json
```

The reference includes methods, explicit constructors/destructors, local
functions and operators, including bodyless declarations. It excludes anonymous
lambdas, implicit primary constructors and property/event accessors. Conversion
types are named by compact syntax tokens. Locations match one-to-one from the
declaration's first attribute through its name; end lines match Roslyn exactly.

The recorded run uses Python 3.14.6, SDK 9.0.308 and Roslyn 4.14.0.0 with Preview
syntax and `FileBasedProgram` enabled (for Polly's `cake.cs`). Repeat `--define`
to supply preprocessor symbols; the default is empty. Syntax errors exclude a
file explicitly; candidates starting in disabled text are recorded separately.
The finder retains all source branches and spans a conditional expression
through its last branch, so other symbol profiles require separate measurements.

Output records project/finder SHAs, dirty states, SDK/Roslyn versions, features,
symbols and oracle/executable SHA-256 hashes. Exact project pins and results are
in root `TODO.md`; these are syntax/location measurements, not semantic binding
or a claim that every call-graph edge is type resolved.

### C# linear versus recursive scope traversal

`csharp_traversal_compare.py` compares the production scanner with the prototype
in `internal/csharp_recursive_test.go`. Only brace-scope traversal changes;
sanitizing, tokenization, declaration recognition and endpoint rules are shared.
This measures explicit scope state versus recursive calls, not a complete C#
grammar parser or AST construction. The prototype stays out of production builds.
A Go overlay builds it as the CLI finder and runs the existing tests unchanged.

```bash
python benchmarks/csharp_traversal_compare.py \
  --root build/csharp-eval-20261016/repos/polly \
  --root build/csharp-eval-20261016/repos/dapper \
  --work-dir build/csharp-traversal-20261016
```

Use the Python runtime and local SDK required by the C# evaluator. Defaults are
10 in-process samples per variant (five per traversal order, one-second target,
one Go CPU), then 20 paired CLI samples per variant after two warmups. In-process
runs preload tracked `.cs` files and include finder construction, sanitizing,
tokenization and traversal. CLI runs use `--dir --all --json --no-gitignore`,
default workers and warm filesystem caches; they include process startup, disk
reads, type discovery and captured JSON output. Entire CLI outputs must agree.
Both builds are also scored independently by the Roslyn evaluator.

`comparison.json` retains samples, versions, project pins, source/executable
hashes, flags, memory allocations and Roslyn summaries. `in-process.txt` contains
the raw Go benchmark output. Size counts cover LF-normalized UTF-8 source for
the full effective finder and its traversal separately, excluding tests/harness;
binary sizes use identical Go build flags. Complete-result parity (including
class scope, signature metadata and extracted text) runs through
`TestCSharpTraversalParity` with the same roots, in three modes and with a
1024-block nested example. [The recorded comparison](CSHARP_TRAVERSAL_COMPARISON.md)
contains results and limitations; root `TODO.md` records the status.

### Shared recursive pilot: C#, Java and D

[BRACE_PILOT.md](BRACE_PILOT.md) describes the event-based walker and its language
adapters. The implementation is test-only; `brace_pilot_build.py` builds/tests a
CLI overlay without changing production dispatch. Python and Ruby are excluded.
The previous C# traversal timings apply to its standalone prototype, not this
new shared implementation.

```bash
python benchmarks/brace_pilot_build.py --work-dir build/brace-pilot-20261016 \
  --cs-root build/csharp-eval-20261016/repos/polly \
  --cs-root build/csharp-eval-20261016/repos/dapper
```

Use `csharp_roslyn_eval.py` for C# and the existing `scala_d_tree_eval.py` for D.
That evaluator now also accepts `--language java` with `tree-sitter-java`:

```bash
python -m pip install --target build/eval-runtime --no-deps tree-sitter-java==0.23.5
python benchmarks/scala_d_tree_eval.py --language java --root <gson-checkout> \
  --label gson --binary build/brace-pilot-20261016/funcfinder-pilot.exe \
  --module-dir build/eval-runtime --output build/brace-pilot-20261016/gson.pilot.json
```

Java truth covers methods, explicit/compact constructors and annotation-type
elements, including methods of anonymous/local classes and bodyless members.
It excludes implicit record constructors and lambdas. The recorded Gson pin is
`e2292f9c9f2e07a4791827e8eea34aa7d80545d6`, with tree-sitter 0.26.0 and
tree-sitter-java 0.23.5; all 264 tracked files parse.

### D speed: production versus shared pilot

After `brace_pilot_build.py`, measure the verified binaries and the two complete
D finder paths with:

```bash
python benchmarks/d_traversal_compare.py \
  --root build/language-eval-20261012/repos/arsd \
  --root build/language-eval-20261012/repos/d-yaml \
  --build-dir build/brace-pilot-20261016 --work-dir build/d-traversal-20261016
```

The default run takes ten serial parser samples per variant (five per order,
one-second target, one Go CPU) on preloaded tracked `.d` files, including all
six files excluded by the reference grammar. Parsing includes D preparation,
class discovery and function discovery. It compares the complete production
implementation with the shared pilot and its adapter; it does not isolate the
cost of recursive calls.

Twenty paired CLI samples per variant follow two warmups. Commands use the
original clones with `--dir --all --json --no-gitignore`, default workers and
warm file caches. Arsd also has one PHP source that is unchanged between builds.
CLI time includes startup, walking, reads, type discovery and captured output.
Output must be stable within a variant; both initial maps, their hashes/counts
and equality are recorded without assuming equivalent raw output on unscored
files or identical declaration starts/class scopes.

`comparison.json` retains all samples, versions, pins, build/source hashes and
allocation metrics; `in-process.txt` holds the raw Go benchmark output. Allocated
bytes are total allocations rather than peak RAM. Results are recorded in
`BRACE_PILOT.md` and root `TODO.md`.

### Production Java fix

The default Java parser uses a linear token walk with an explicit scope stack.
On Gson `e2292f9`, it matches all 3492 named reference functions with correct
ends and no missing/false functions, and all 767 named types by header location.
Type ends are not independently scored. The previous production path missed
367 functions and reported 55 false functions. See [JAVA_FIX.md](JAVA_FIX.md)
for reproduction, complete timing ranges and corpus limits. The recursive
pilot measurements above describe the earlier baseline and remain separate.

`java_finder_compare.py` accepts `--before`, `--after`, optional `--pilot`,
`--root` and `--work-dir`; defaults are twenty shuffled samples and two warmups
for one/CPU-count file workers. It retains maps and hashes and requires stable
output within each variant. It does not assume before/after map equality or
measure parser-only time.

### Optional scope-parallel pilot

After building the shared serial pilot, compare bounded parallel scope bodies
against the verified serial and production binaries:

```bash
python benchmarks/brace_parallel_compare.py \
  --root build/language-eval-20261012/repos/arsd \
  --root build/language-eval-20261012/repos/d-yaml \
  --serial-build-dir build/brace-pilot-20261016 \
  --work-dir build/brace-parallel-20261016 \
  --parity-project cs=build/csharp-eval-20261016/repos/polly \
  --parity-project cs=build/csharp-eval-20261016/repos/dapper \
  --parity-project java=build/brace-pilot-20261016/repos/gson
```

The runner builds/tests/vets a third overlay, requires exact serial-pilot parity
on supplied corpora and all timed CLI output, then measures file workers 1/CPU
count and scope budgets 4/16/32 (clamped to CPU count). Defaults: 2048-token
threshold, one warmup, three shuffled sample rounds. `GOMAXPROCS` uses all
logical CPUs. Windows process CPU time supplies average whole-run utilization;
other platforms retain wall timings without this CPU reading. The scheduler
uses one global extra-task budget across file workers. It leaves preparation
and tokenization sequential within each file. See [BRACE_PARALLEL.md](BRACE_PARALLEL.md)
for the negative scaling result, complete matrix, verification and limits.

For Java/Gson, use `--language java --root build/brace-pilot-20261016/repos/gson`
and a separate work directory. The recorded Java run used ten samples and two
warmups (`--samples 10 --warmups 2`) with
`--work-dir build/brace-parallel-java-20261016`; omit the D
roots and optional extra parity projects from the command above. `--language`
defaults to `d` and accepts `cs`, `java` and `d`.

### Go symbol maps

| Project | Commit | Lang | Recall | Precision | Token savings | Notes |
|---|---|---|---|---|---|---|
| ruslano69/tdtp-framework | `4ff012e` | Go | 98.1% → 99.1% | 99.5% | 88.8%* | after the defined-type fix (loop #1) |
| ruslano69/tdtp-framework | `2028d38` | Go | 99.1% → **100.0%** | **100.0%** | 88.8%* | after char-literal + inline-brace fixes; ruler corrected (loop #2) |
| **funcfinder (self-host)** | `HEAD` | Go | 99.3% → **100.0%** | **100.0%** | — | the bootstrap test: funcfinder on its own source (loop #3) |

\* verbose `--all --json`, chars/4 approx; higher with `--split`/compact map.
Recall/precision are over the file intersection with the ruler.

**Loop #3 — the bootstrap test ("the compiler must compile itself").** Running
funcfinder on its *own* source surfaced two more clean categories, both fixed in
config/regex:

1. **Generics.** `func GenericFunction[T any](…)` — the func pattern expected
   `(` right after the name and choked on the `[type params]` list. Fixed with a
   non-capturing `(?:\[[^\]]*\])?` between name and args (methods and multi-param
   constraints included). Pinned by `TestGoFinder_GenericFunctions`.
2. **Composite defined types.** `type ShardGraph map[string]map[string]struct{}`
   — the `named` pattern forbade *any* `{`, to avoid grabbing multi-line
   `type X struct {`. But a nested `struct{}` / `interface{}` in a `map` value is
   balanced. Relaxed to allow interior braces while still rejecting a *trailing*
   unmatched `{`. Pinned by `TestGoStructFinder_CompositeDefinedTypes`.

Result: **606 / 606 symbols, recall and precision both 100.0%.** funcfinder maps
its own codebase symbol-for-symbol with `go/ast`. The tdtp row is unchanged
(3366/3366) — no regression.

**The dovodka loop, demonstrated end-to-end over two passes:**

*Loop #1 — defined types.* The first 1.9% recall miss was one clean category:
Go *defined types* — `type X string`, `type X func(...)`, `type X []byte` —
which funcfinder didn't detect (its `struct_type_patterns` only covered
`struct` / `interface` / `= alias`). 66 instances. Fixed with a `named` pattern
plus single-line closing for brace-less type kinds. Recall **98.1% → 99.1%**,
misses 66 → 30. Pinned by `TestGoStructFinder_DefinedTypes`.

*Loop #2 — the residual 30, which turned out to be two more clean categories,
not "only AST gets it":*

1. **Char/rune literals leaking braces (22 of 30).** `char_delimiters` was
   *unset for every language* in `languages.json`, so the sanitizer's
   char-literal machinery was dead code. A lexer line like `if c == '{' {` fed
   a phantom `{` into `CountBraces`, and the enclosing function stayed open and
   swallowed everything after it. Fix: set `"char_delimiters": ["'"]` for the
   languages where `'` is a rune/char literal (Go, C#, Java, D, Kotlin — C/C++
   already carried `'` in `string_chars`). *Not* Rust (lifetimes `'a`) or Scala
   (symbol literals `'sym`), where a lone `'` is legal and unpaired. Pinned by
   `TestCharLiteralBraces_NoSwallow` / `TestCharLiteralSanitized`.
2. **Inline balanced braces (the last 7).** `type Foo struct{}` /
   `struct{ io.Writer }` net to `braceCount == 0`, which the struct finder read
   as "multi-line signature, wait for a brace" — so it stayed open and swallowed
   the next type. Fix: when the line has no unmatched brace but *did* contain a
   `{`, close the type on its own line. Pinned by
   `TestGoStructFinder_InlineBraces_NoSwallow`.

Result: recall **99.1% → 100.0%** on the file intersection, misses 30 → 0.

*Loop #2b — the ruler audited itself.* Investigating the remaining <1% precision
gap showed the "false positives" weren't false at all: 14 were function-local
`type` declarations funcfinder correctly reports, which the top-level-only ruler
didn't emit. So the ruler was fixed (walk the whole AST). After that the two
agree symbol-for-symbol — **3366 / 3366**, recall and precision both **100.0%**,
zero misses in either direction. The lesson cuts both ways: the ruler measures
funcfinder, and funcfinder's output can expose blind spots in the ruler.

**What 100/100 does and doesn't mean.** It means: on tdtp, over the shared Go
files, funcfinder *locates* every function and type the parser sees (matched by
name and line ±2), and reports no phantom symbols. It does **not** mean "regex ==
AST": we compare symbol *presence*, not exact spans/signatures/fields; we model
two node kinds (functions, types), not the full grammar; and it's one project.

**The lesson for the tier boundary.** What looked like "harder, parser-only"
territory after loop #1 was, on inspection, two more *cheap* config/regex fixes
plus a bug in the measuring tape — no parser needed. The ruler earns its keep
precisely here: it stops us from hand-waving "the rest needs AST" and makes us
look. The genuine parser-tier cases are the ones that survive *after* the cheap
fixes are exhausted — and on tdtp there are none left to point at. The next
honest test is a project that still has residuals here.

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

### JavaScript named functions

`js_tree_eval.py` scans tracked `.js`, `.jsx`, `.mjs` and `.cjs` files with
tree-sitter-javascript. Truth includes named declarations/expressions,
generators, literal method names and anonymous function/arrow values directly
bound to identifier variables. Named expressions keep their internal name.
Anonymous callbacks, anonymous assignment/property values and computed methods
are outside this profile. Duplicate names are retained; names/header lines
match one-to-one and end lines are checked independently. Parse errors are
reported as exclusions. These line-based scores do not establish byte/column
extraction accuracy on minified files.

```bash
python -m pip install --target build/js-eval-runtime tree-sitter==0.26.0 tree-sitter-javascript==0.25.0
PYTHONPATH=build/js-eval-runtime python benchmarks/js_tree_eval.py \
  --root <checkout> --label lodash --binary ./funcfinder.exe \
  --output build/lodash.js-eval.json
```

On PowerShell set `$env:PYTHONPATH` to the absolute runtime directory before
running Python. `--pathspec` restricts tracked paths and may be repeated. Full
before/after scores, pins, limitations and CLI timings are in
[JAVASCRIPT_FIX.md](JAVASCRIPT_FIX.md). The expanded lodash profile includes
vendor code and four minified bundles; it supersedes the historical
1918-function measurement rather than pretending to replay its old scope.

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

## D implementation experiment

`dlang_port.d` ports the D finder path to D; it is a standalone measurement
probe, outside production dispatch. `dlang_port_compare.py` freezes a Go
revision, builds both probes, checks ordered function/class results including
duplicate names and scopes, and scores the accepted D tree-sitter profile
before repeated serial timing. It builds runtime-regex and ctRegex D variants.

On Windows, initialize the user's LDC/MSVC environment with
`F:\D\ldcvars64.bat`. Then use Python 3.14 with the existing D grammar runtime:

```powershell
python benchmarks/dlang_port_compare.py --help
python benchmarks/dlang_port_compare.py `
  --revision b744fa0130db40816e3528e1ca36ceb868d7481a `
  --root build/language-eval-20261012/repos/d-yaml `
  --root build/language-eval-20261012/repos/arsd `
  --work-dir build/dlang-port-20261018-predicates `
  --baseline-report build/dlang-port-20261018-postscanner-baseline/report.json `
  --samples 10 --repeats 2
```

The baseline report is optional: it reuses hash-verified primary old Go/D binaries
in the same shuffled rounds and requires identical pattern configuration. Older
baseline variants nested in that report are not imported again. `--samples 0`
builds and verifies parity/independent quality without running timing samples.
Omit it for a fresh comparison of the current D source against the selected
Go revision. The saved original binaries/source hashes describe the initial
pilot; current source includes Go's plain-line fast paths, reusable per-file
buffers, lazy rune decoding and additional matcher gates. `AuditHints` checks
cached matches and attribute/type/statement gates against their bare patterns;
`AuditFastPaths` compares the lexer/prefix shortcuts with their slow paths.
Both are excluded from timed release builds. Build them separately with
assertions enabled and `-boundscheck=on`; the corpus/fixture/stress checks and
phase/allocation diagnostics are recorded in `DLANG_PORT.md`.

The current D pilot also reads function headers and ordinary attribute/type/
statement prefixes with scanners. `AuditScanner` checks every header against
its function regex; `AuditPredicates` checks the three prefix predicates against
their previous gated regexes. Rare Unicode whitespace/word boundaries retain
the old regex path. Build all audit versions together for corpus verification.
The differential predicate checker also exercises generated inputs:

```powershell
ldc2 -O2 -boundscheck=on -d-version=PredicateCheck -I=benchmarks `
  -of=build/dlang-port-20261018-predicates/predicate-check.exe `
  benchmarks/dlang_port.d benchmarks/dlang_predicate_check.d
build/dlang-port-20261018-predicates/predicate-check.exe `
  build/dlang-port-20261018-predicates/config.json
```

That output directory and config are created by the comparison harness. Baseline
reports describe preserved binaries, not the current source: retain a baseline
source snapshot and run with `--samples 0` before editing the candidate. Choose
a new work directory for a different Go revision. The current baseline uses the
header scanner but predates the predicate/join changes.

The probe covers finder API bounds. It does not implement the five-tool CLI,
directory discovery, file parallelism, selection/offset/extraction, structural
field analysis or call graphs. The Go probe retains generic finder machinery;
its size difference from D includes specialization. Read `DLANG_PORT.md` before
using the results to decide on a rewrite. Reports and generated sources stay
under ignored `build/`.

## JS/TS finder API experiment in D

`JS_TS_D_PORT.md` describes the complete experimental JS/TS finder path and
shared cleaner, the frozen Go comparison, independent grammar limits and audit
checks. This is a separate serial probe; the production CLI remains Go.

```powershell
python benchmarks/js_ts_port_compare.py --help
python benchmarks/js_ts_port_compare.py `
  --revision b744fa0130db40816e3528e1ca36ceb868d7481a `
  --project js=build/javascript-eval-20261018/repos/express `
  --project js=build/javascript-eval-20261018/repos/lodash `
  --project ts=build/javascript-eval-20261018/repos/zod `
  --project ts=G:/DEV/OmniRoute --project js=G:/DEV/OmniRoute `
  --work-dir build/js-ts-d-port-current --samples 7 --repeats 1
```

The harness checks exact cleanup facts and full ordered function/class/signature
arrays before timing, then reports the existing JS/TS independent profiles.
`--samples 0` skips measurements. `--finder-only` times only the finder while
still checking complete cleanup parity; the report records `timed_modes`.
Each run retains D/Python source snapshots in `sources/`.
Use the installed LDC/MSVC environment and
Python grammar runtimes described in the report. Build a separate
`-O2 -boundscheck=on -d-version=AuditTSHeader -d-version=AuditJSTokens` probe, then run
`python benchmarks/js_ts_port_checks.py --work-dir build/js-ts-d-port-current`;
see the report for the full command.

To compare an optimization with preserved D binaries, choose a new work directory
and add `--baseline-report build/js-ts-d-port-20261018/report.json`. The harness
verifies reference/binary/config identity, checks all three maps and times old D,
new D and Go in the same shuffled rounds. Per-file input hashes are recorded and
rechecked after each corpus. Keep phase diagnostics separate from release timing.
The candidate directory must differ from the baseline directory; this is checked
before building. The Go 1.15.9 TS recovery-gate comparison uses revision
`bdbc81b` and is recorded in `JS_TS_D_PORT.md` under the recovery-gate stage.

## C#/C++ finder API experiment in D

`CSHARP_CPP_D_PORT.md` describes the separate serial D port, frozen Go 1.15.9
comparison, Unicode 15.0.0 tables and independent Roslyn/libclang limits.
C++ now uses tokenization; the initial regex experiment is preserved separately.
`brace_port_compare.py --help` lists the four-project comparison; use
`--samples 0` for correctness only and a new work directory for a new revision.
Use `--baseline-report build/cs-cpp-d-port-20261018/report.json` and a distinct
`build/cpp-tokens-20261018/` candidate directory to include the prior regex D
binary in the same rounds. New C++ output may differ; the harness requires no
lost correct function and no quality-count regression against Go before timing.
Build the separate `--audit --samples 0 --skip-oracle` variant (same projects),
then run `brace_port_checks.py --work-dir <work>` for fixtures, raw cleanup and
every consumed pattern's capture groups. Full commands and classifier-table
provenance are in the report. Preserve snapshots, pins, per-file/binary hashes
and raw timing samples; run diagnostics separately from timing. The probe
does not implement the production CLI or file parallelism.

## Go/Java finder API experiment in D

`GO_JAVA_D_PORT.md` describes `go_port.d`/`java_port.d`, the four-language shared
probe, pinned gin/Gson and installed Go `encoding/json` corpora. The Go reference
is native `go/parser`/`go/ast`, including exact named-function and all TypeSpec
bounds; Java uses the existing grammar profile plus a named-type boundary audit.
Go's broader type output and one corrected Java record end intentionally change
whole maps. `go_java_port_compare.py --help` and `go_java_port_checks.py --help`
describe the entrypoints. Use a new work directory for another reference revision,
run separate bounds/token/cleaner audits and unit tests before paired timings,
then recheck snapshots, binary/input hashes and prior C#/C++ maps. Full commands,
measurement limits and build-tag/alias profiles are in the report.

## C finder API experiment in D

`C_D_PORT.md` describes the standalone C token/declarator port and its GCC plus
tree-sitter Lua reference. Run `python benchmarks/c_port_compare.py --help`,
then the default command for release/assertion/unit builds, all 68 Lua files,
11 fixtures and seven paired timing rounds. `--samples 0` skips timing. It uses
the hash-verified frozen Go probe from `build/go-java-d-port-20261018/` and the
installed LDC/MSVC environment. Python grammar packages live in
`build/c-eval-runtime/`; use the pinned versions from the C evaluation above.
Choose a new `--work-dir` under `build/` for changed sources or inputs.

Both APIs retain all 1,568 accepted functions/prototypes with correct ends and
135 separate inactive candidates. D adds raw tag definitions without a type
oracle and is currently 11.0% slower in serial parsing. Size is a specialized
C-only probe versus generic Go, not an equivalent toolkit comparison. Macro
expansion, typedef-dependent ambiguity, some declaration-list shapes and the
complete CLI remain open; the report specifies these measurement limits.

## Rust finder API experiment in D

`RUST_D_PORT.md` describes the standalone Rust lexer/token finder and its
unexpanded tree-sitter profile. Install tree-sitter 0.26.0 and
tree-sitter-rust 0.24.0 under `build/rust-eval-runtime/`, then run:

```powershell
python benchmarks/rust_port_compare.py --help
python benchmarks/rust_port_compare.py `
  --project kanzi=G:/DEV/Go/kanzi/rust_kanzi `
  --project serde=build/rust-d-port-20261019/repos/serde `
  --work-dir build/rust-d-port-20261019 --samples 7 --repeats 3
```

The harness builds retained snapshots with installed LDC/MSVC, checks unit and
separate token/bounds assertion builds, then scores functions and named types
and times against the verified frozen Go probe. `--samples 0` skips timings.
Serde selects tracked Rust files only under `serde/src` and `serde_core/src`.
Kanzi's actual dirty tracked files are hashed; its untracked tree is excluded.
Choose a new work directory for changed sources or inputs.

Keep the explicit kanzi grammar exclusion and all 58 timed inputs separate from
57 accepted files (1,379 functions and 470 types). Macro token trees are not
expanded and cfg branches are not evaluated. Full maps differ from Go because
macro candidates and impl-scope/alias type profiles differ. Native rustfmt syntax
acceptance does not establish function spans. The Rust-only probe, serial timing
and specialized binary size are not a complete nine-language CLI measurement;
full Unicode XID and file workers remain open.

## Kotlin finder API experiment in D

`KOTLIN_D_PORT.md` describes the standalone Kotlin lexer/token finder and its
native compiler PSI reference. The final harness uses Kotlin compiler 2.2.0 and
a portable JDK under `build/kotlin-oracle-runtime/`; tree-sitter is not required.
`kotlin_psi_oracle.java.txt` parses syntax without Gradle/Android dependencies,
normalizes CRLF in memory and reports names/header/end lines in UTF-8.

```powershell
python benchmarks/kotlin_port_compare.py --help
python benchmarks/kotlin_port_compare.py `
  --project neko=build/kotlin-d-port-20261019/repos/NekoBoxForAndroid `
  --project seal=build/kotlin-d-port-20261019/repos/Seal `
  --work-dir build/kotlin-d-port-20261019 --samples 7 --repeats 3
```

Use `--samples 0` for correctness only and a new work directory for changed
sources/inputs. The harness compiles retained D/Java snapshots, runs separate
module unit and token/bounds assertion builds, scores all 311 tracked `.kt`/`.kts`
files against PSI, then times the two serial APIs. All 1,995 functions and 413
named types match without parse exclusions. Compiler libraries/classes, binaries,
input bytes, earlier backend hashes and raw samples are retained and verified.
Named fun/type truth excludes constructors/accessors, enum entries, anonymous or
unnamed declarations and declarations in string interpolation. See the report
for language limits, expanded type profile and specialized-probe size/speed
limits; the common CLI and file workers remain open.

## Swift finder API experiment in D

`SWIFT_D_PORT.md` describes the standalone Swift token finder and its
tree-sitter 0.26.0 / tree-sitter-swift 0.7.4 reference. Install those pinned
packages under `build/swift-eval-runtime/`, then run:

```powershell
python benchmarks/swift_port_compare.py --help
python benchmarks/swift_port_compare.py `
  --project alamofire=build/swift-d-port-20261019/repos/Alamofire `
  --project kingfisher=build/swift-d-port-20261019/repos/Kingfisher `
  --work-dir build/swift-d-port-20261019 --samples 7 --repeats 3
```

The harness builds retained snapshots using LDC/MSVC, checks module unit tests
and separate token/bounds assertion maps, then scores functions and nominal/
alias/associated types against the grammar before paired serial timings.
`--samples 0` skips timing; choose a new work directory for changed sources or
inputs. Reference input normalizes CRLF with original line numbers preserved;
error auditing includes missing unnamed syntax tokens.

Keep 268 accepted files (3,249 functions and 835 types, zero mismatches) separate
from all 276 audited/timed inputs and eight explicit grammar exclusions. Named
func truth includes operators/protocol requirements, not constructors/accessors;
extensions supply scopes rather than new types. Full maps intentionally differ
from Go. The dedicated algorithm, specialized Swift-only executable and serial
API timings are not a complete eleven-language CLI or language-only benchmark.

## PHP finder API experiment in D

`PHP_D_PORT.md` records the standalone token finder and its tree-sitter 0.26.0 /
tree-sitter-php 0.25.1 reference under `build/eval-runtime/`.

```powershell
python benchmarks/php_port_compare.py --help
python benchmarks/php_port_compare.py `
  --project monolog=build/language-eval-20261012/repos/monolog `
  --project routing=build/language-eval-20261012/repos/symfony-routing `
  --work-dir build/php-d-port-final-20261019 --samples 7 --repeats 3
```

All 495 tracked files independently verify 2,727 named functions/methods and 480
named class/interface/trait/enum types with zero misses, false entries, wrong ends
or parse exclusions. The harness checks separate module units and token/bounds
assertion maps, snapshots, prior backends and hashes before/after serial timing.
Twenty-one fixtures include heredoc/nowdoc, interpolation, attributes, anonymous
classes, mixed HTML, Unicode/CRLF and long/deep inputs. Incomplete/invalid-UTF-8
fixtures are explicit robustness inputs outside independent truth. Use a new work
directory for changed inputs and `--samples 0` for correctness only. The expanded
type profile and specialized PHP-only API are not Go-map/cleanup parity, a language-
only speed comparison or a complete twelve-language CLI.

## Scala finder API experiment in D

`SCALA_D_PORT.md` records the standalone lexer/finder with tree-sitter 0.26.0 /
tree-sitter-scala 0.26.2 under `build/eval-runtime/`.

```powershell
python benchmarks/scala_port_compare.py --help
python benchmarks/scala_port_compare.py `
  --project scopt=build/language-eval-20261012/repos/scopt `
  --project oslib=build/language-eval-20261012/repos/os-lib `
  --work-dir build/scala-d-port-final-20261019 --samples 7 --repeats 3
```

All 88 tracked files independently verify 1,023 named functions/signatures and
328 named class/object/package-object/trait/enum/type declarations with no missing
or false entries, wrong ends or parse exclusions. Functions include parameterless,
operator and auxiliary-constructor names; aliases, abstract and nested/local types
broaden the older Go type profile. The harness verifies separate module units,
token/bounds assertion maps, snapshots, prior backends and final hashes around
paired serial timing. Twenty-two fixtures include Scala 3 indentation/end markers,
symbols/characters, interpolation and long/deep input; incomplete/invalid-UTF-8
fixtures remain explicit unscored robustness checks. These predominantly Scala 2
corpora and limited Scala 3 fixtures do not establish complete language coverage,
Go-map/cleanup parity, language-only gains or a combined thirteen-language CLI.
Use `--samples 0` for correctness only and a new work directory for changed inputs.

## Python finder API experiment in D

`PYTHON_D_PORT.md` records the standalone logical-statement/indentation finder
against CPython 3.14.6 native AST. The Go baseline uses production `PythonFinder`
through an exact-body preloaded-lines overlay; the older generic Go probe is not
the Python production path. The adapter keeps `os.ReadFile`/`strings.Split` input
semantics and retains its frozen source/probe hashes.

```powershell
python benchmarks/python_port_compare.py --help
python benchmarks/python_port_compare.py `
  --project flask=build/language-eval-20261012/repos/flask `
  --project requests=build/language-eval-20261012/repos/requests `
  --project hr=G:/DEV/Python/hr_portal `
  --work-dir build/python-d-port-production-20261019 --samples 7 --repeats 3
```

All 180 tracked files independently verify 2,884 functions and 343 classes with
zero D errors or parse exclusions. Separate units and token/bounds maps pass;
15 fixtures cover Python 3.14 f/t strings, nested/inline/decorated suites,
continuations, tabs/newlines/BOM and malformed/long/deep inputs. Keep Go's raw
ends and its separately normalized blank/comment ends distinct from D's exact
AST ends. Verify the interpreter/AST library, adapter body/reader, prior backends
and all final snapshots/input hashes outside timing. No application dependencies
are needed. Full XID/NFKC/legacy encodings, CLI/workers and other analyses remain
open. Use a new work directory for changed sources/inputs and `--samples 0` for
correctness only; earlier generic-Go measurements are exploratory.

## Ruby finder API experiment in D

`RUBY_D_PORT.md` records the standalone lexer/token/block finder against
tree-sitter 0.26.0 / tree-sitter-ruby 0.23.1 under `build/ruby-eval-runtime/`.

```powershell
python benchmarks/ruby_port_compare.py --help
python benchmarks/ruby_port_compare.py `
  --project huginn=build/ruby-d-port-20261019/repos/huginn `
  --project maybe=build/ruby-d-port-20261019/repos/maybe `
  --work-dir build/ruby-d-port-final-20261019 --samples 7 --repeats 3
```

All 1,294 tracked `.rb` files independently verify 4,146 methods and 1,315 named
class/module types with zero misses, false entries, wrong ends or parse exclusions.
Pair heredoc beginnings/bodies and extend declaration spans through physical
terminators, including endless methods. The frozen Go comparison uses its original
generic Ruby path, retained as `go_generic` in the preceding Python report.
Separate units and token/bounds maps pass; 22 fixtures cover nested/endless/
singleton/operator methods, condition modifiers/values, loops/blocks, regex/
percent/interpolated literals, queued heredocs and malformed/long/deep input.
Keep dynamic/alias/literal declarations outside this named profile and do not
claim full grammar/Go-map/cleanup/semantic parity. Final snapshots, prior backends,
compiler/grammar/binary/input hashes and stable per-variant maps are verified.
Use a new work directory for changed inputs and `--samples 0` for correctness only.
All 15 languages now have finder prototypes; the common CLI, file workers and
other toolkit analyses remain open.

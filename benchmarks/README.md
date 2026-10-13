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

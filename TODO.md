# TODO

Known issues and follow-up work, tracked here until they become GitHub issues.

---

## Language coverage on real projects: status and next steps (2026-10-12)

Earlier measurements used `main` at `2022d5b` against syntax-tree ground truth (tree-sitter, one grammar per
language); subsequent runs and their revisions are recorded below. Python, PHP, Scala/D, C and C++ evaluators now live in `benchmarks/`. Truth
includes declarations without a body (interface, abstract, prototype members): the design lists
them with their own span, so a missing C# or Java declaration counts as missing.

| Language | Project (truth functions) | Correct | Wrong end | Missing | State |
|---|---|---|---|---|---|
| Go | gin (1342), funcfinder itself (603) | 100% | 0 | 0 | done |
| Rust | kanzi-rs (490) | 100% | 0 | 0 | done |
| Swift | Alamofire (1890), Kingfisher (109) | 99.6%, 100% | 2, 0 | 6, 0 | good |
| Kotlin | NekoBoxForAndroid (1166), Seal (781) | 99.7%, 99.2% | 1, 0 | 3, 6 | good |
| Ruby | huginn (1865), maybe (2284) | 99.0%, 99.9% | 0, 3 | 19, 0 | good |
| TypeScript | hr_gemini (2988), zod (2072) | 98.8%, 99.0% | 12, 1 | 36, 21 | measured 2026-10-13 with the tree-sitter TypeScript reference; 23 and 10 false functions |
| JavaScript | lodash (1918, minified) | 81.0% | 7 | 357 | open |
| Java | gson (3480) | 89.5% | 0 | 364 | open |
| C# | Polly (5070) | 74.9% | 3 | 1272 | open |
| C++ | fmt (1450 truth) | 97.5% | 0 | 36 (19 conversion operators, 11 macro-generated) | measured 2026-10-13; 7 false functions (explicit instantiations) |
| C++ | tinyxml2 (627 truth) | 99.8% | 0 | 1 | measured 2026-10-13; 0 false functions; `8224e427b655b83dae5e2298f1e6919523a78737` |
| C | Lua (1568, including 391 prototypes) | 100% | 0 | 0 | local fix verified; no false functions; one GCC configuration |
| Python | Flask (1462), Requests (711), hr_portal (711) | 100%, 100%, 100% | 0, 0, 0 | 0, 0, 0 | local fix verified; no false functions |
| PHP | Monolog (1602), Symfony Routing (1125) | 100%, 100% | 0, 0 | 0, 0 | local fix verified; no false functions |
| Scala | scopt (380), os-lib (643) | 100%, 100% | 0, 0 | 0, 0 | local fix verified; no false functions |
| D | D-YAML (539), arsd (11705) | 100%, 100% | 0, 0 | 0, 0 | local fix verified; no false functions; oracle accepts 34/35 and 120/125 files |

Python was measured separately on 2026-10-12 on a working-tree fix based on `7b9131e`, built with `build.ps1`, against
Python 3.14 `ast`. All tracked `.py` files were included (83/37/60 files, no parse errors).
Names are matched one-to-one within the AST declaration/decorator span; correct ends exclude
trailing blank/comment lines. Duplicate method names are retained using plain `--map` output,
since file-mode JSON overwrites repeated names. Recall and precision are 100% for all three
projects. Before the fix, wrong ends were 20/30/97 and Flask had 29 false functions.
Pins: Flask `d086db856be187255b8ec61ef409357393020f32`,
Requests `611c6162cbc4ac2020a2f91c7cfa4f3abf9bbb60`, hr_portal `87a2909d85ee173209afcecdd12f530257e5d2ab`.
The evaluator is `benchmarks/python_ast_eval.py`; raw outputs from this run are in ignored
`build/language-eval-20261012/`.

PHP was measured on 2026-10-12 on a working-tree fix based on `34d7dc8` using
tree-sitter 0.26.0 / tree-sitter-php 0.25.1 under Python 3.14.6. All tracked PHP
files were included (228 Monolog, 267 Symfony Routing; zero parse errors). Truth
includes 16/18 bodyless declarations, excludes anonymous closures, and matches
names one-to-one within the declaration header with exact end lines. Baseline:
27/43 missing, zero wrong ends or false functions. Both projects now have 100%
recall and precision. Pins: Monolog `3bed304b1905fed5c30d172872f4d890cbb03c6a`,
Symfony Routing `57970507cf869e358330cb336772a7dcb31fa67c`. Evaluator:
`benchmarks/php_tree_eval.py`; raw before/after JSON is in the same ignored build
directory. Line numbers are derived from byte positions because the native Point
wrapper crashed on this Windows Python runtime.

Scala was measured on 2026-10-12 on a working-tree fix based on `bd1575e`;
D's follow-up fix is based on `25a1e52`,
using Python 3.14.6 / tree-sitter 0.26.0. Scala grammar: tree-sitter-scala 0.26.2;
D grammar: tree-sitter-language-pack 1.22.0, DLL SHA-256
`acf35033a28e91d15620505ee2b2e06efc5be01c8caa0c6fef5de6925f9588f8`.
Reference ends exclude syntax-tree extras/trailing whitespace. D truth includes
constructors, destructors and postblits; anonymous lambdas and value/type templates
are excluded. All 29 scopt and 59 os-lib files parsed. D excludes
`D-YAML/examples/tojson/source/app.d` and arsd's `cli.d`, `core.d`, `ini.d`,
`script.d`, `uda.d` because the oracle reports syntax errors; their symbols are
not scored. The D grammar mislabels two `final switch` statements and 18
constructor-delegation calls as declarations; the evaluator rejects these by
syntax/context and records every rejection (`oracle_rejected`). Final truth is
11705 functions, all found with correct ends and no false functions. With this
audited reference, `25a1e52` missed 249 functions and produced 93 false functions;
the follow-up fix has zero in both categories. Before the first fixes: Scala
missed 120/203; D-YAML missed 486 with 300 false functions. Evaluator:
`benchmarks/scala_d_tree_eval.py`, raw JSON in the ignored build directory above.
Pins: scopt `fb371a7cf997fe053c9e1496cc6e13fbff3ff9de`,
os-lib `86e5ddb65fe62305afb5885fcc0f8da50aba4b48`,
D-YAML `09aeac155c9d0f85a0e7442dffb442de53528a41`,
arsd `953107183178fccbe8af7340b230c37c2071aef8`.

C was re-measured on 2026-10-12 on a working-tree fix based on `c2cf2fe`,
against Lua `0b29f408433e92953cc72b1d3e06c7ac8139e439`. All 68 tracked `.c`/`.h`
files were included, with no preprocessing/parse failures. Runtime: Python 3.14.6,
tree-sitter 0.26.0, tree-sitter-c 0.24.1, MSYS2 UCRT64 GCC 15.2.0 (Rev8).
The oracle preprocesses C99, with the Lua root on the include path, forced
`lua.h` for standalone headers and `LUA_USE_JUMPTABLE=0`. It maps GCC line markers
back to original source and scores active definitions/prototypes. All 135 finder
candidates in inactive branches are recorded separately, not scored. Thirty-eight
GCC va_arg/offsetof expressions are masked for grammar compatibility, preserving
bytes/newlines and function boundaries. This is one Windows/compiler configuration,
not all conditional variants. Before the fix: 157 missing (156 prototypes, one
definition), 136 false names, zero wrong ends; after: zero in all three categories.
The earlier 99.9% did not measure prototypes. Evaluator: `benchmarks/c_tree_eval.py`;
exact flags, compiler/binary hashes and findings are in the ignored build directory.

TypeScript was measured on 2026-10-13 with `benchmarks/ts_tree_eval.py` (tree-sitter-typescript 0.23.2 as the reference). Truth: function declarations and signatures, class and interface methods (including bodyless overloads and abstract methods), constructors, and functions bound to a variable; anonymous callbacks are not truth. zod `b500a2ee95134f81d74da7911d7378dd2853d042` (511 tracked files, 4 unparseable): on `main` 2007 of 2072 found, recall 96.9%, precision 99.0%; now 2051 found, recall 99.0%, precision 99.5%. hr_gemini `5b69a1eb` (455 tracked files, 9 unparseable): on `main` recall 91.3%, precision 96.2%; now recall 98.8%, precision 99.2%. Still open: 12 wrong ends in hr_gemini, all React components whose JSX body is cut short; 23 and 10 false functions; 36 and 21 missing. Lodash is not in this table: its reference is a minified bundle, and a non-minified JavaScript project is still needed.

C++ was also measured on tinyxml2 `8224e427b655b83dae5e2298f1e6919523a78737` (`tinyxml2.h` and
`tinyxml2.cpp`, 627 truth, libclang 18.1.1, same target): on `main` 332 found, recall 53.0%, precision
100%; now 626 found, recall 99.8%, precision 100%, no wrong ends or false functions. The override and
final specifiers were the largest remaining miss. CLI11 (`07c9f13b15a70c367a406e1cfe2c363aa1b66894`)
is not measured: 22 of its 29 headers fail to parse with the clang reference (a GCC-only x86 builtin in
the mingw headers, and redefinitions from its include structure).

C++ was measured on 2026-10-13 on a working-tree fix based on `edc5f94`, against fmt
`b4ffa39b9994c961418406f47358791d89a5aa48`, with libclang 18.1.1 and MinGW-Builds GCC 15.2.0
(C++17, `x86_64-w64-mingw32`). `include/` and `src/` were measured: 18 files are scored, and
`src/fmt.cc` (a C++20 module unit) and `src/os.cc` (an x86 builtin this target lacks) are unparseable
exclusions. Truth is 1450 functions and declarations, 138 of them bodyless. On `main` at `edc5f94`:
727 missing, 21 false functions, 2 wrong ends (recall 49.9%, precision 97.2%). Now: 36 missing,
7 false functions, 0 wrong ends (recall 97.5%, precision 99.5%). Evaluator:
`benchmarks/cpp_tree_eval.py`; raw JSON is in the ignored build directory.

- [ ] **Verify CI on `main` at `2022d5b`.** PR #77 was merged while four of its checks (Analyze
  (go), Build, Build Windows, Build macOS) were still running. Read the result once; do not poll.

### Open items, in order of measured gap

- [x] **TypeScript, zod: 267 missing, now 31.** Three shapes were missed: a const annotated with
  a function type (`const f: (x) => T = (x) => {`), a method whose name is a string literal
  (`"=== INVALID"() {`, read from the raw line because the sanitizer blanks the literal), and a
  method with a function-typed parameter (`add(fn: (v) => any) {`). Correct 88.8% -> 98.6%, wrong
  ends unchanged. The 31 left: generic or multi-line method headers (`refine(` with its parameters
  below, `extend<...>(` over several lines); methods named `catch` or `with`, which the statement
  words exclude; a `const x =` whose value starts on the next line; a function whose return type
  holds braces (`loadPriorTriage(...): Map<string, { verdict: string }> {` ends at the return type,
  and it is not found at all); a computed name (`get [SYM]() {`).
- [x] **TypeScript, hr_gemini: 46 missing, now 26; lodash: 473 missing, now 357.** A function can
  start after a separator on a line (`let n=0;const f=…`, `cookie='';constructor(){`, `}, m(){`).
  Each such header is read from its own start, and its body closes where its braces do. Flag
  `inline_headers`, TypeScript and JavaScript only. Two closing rules came with it: a body closed
  mid-line is over when a header follows the close (minified `}function a(){…}`), and a depth that
  drops below zero ends the function. The 26 left in hr_gemini: 8 `React.FC<{` headers whose type
  argument runs over several lines, 11 headers whose parameters or destructuring run over several
  lines (`export const X: React.FC<P> = ({` and `async issue(input: {`), and 7 others, including
  `export default function App()`, which may be a cascade from an earlier frame that did not close
  (not checked yet). Also `notification=` after a comma in a `const` list.
- [x] **TypeScript, hr_gemini: 23 wrong ends, now 13.** Four causes fixed: a template literal's
  `${...}` holds strings and nested templates (the sanitizer closed the outer template at the
  first inner backtick, which hid whole files), an arrow header ending in `=>` is not complete,
  a parenthesized arrow body runs to its closing `);`, and a brace inside a type argument
  (`React.FC<{...}>`) does not stop the bracket count. The 13 left are mostly early ends in React
  components; not analysed further yet.
- [ ] **C#, Polly: 1272 missing.** Two causes. Multi-line signatures, and about 172 declarations
  without a body (`void Foo();` in interfaces), which funcfinder does not list at all. A `;` at the
  end of the header pattern was tried and rejected: +76 correct, but +211 false functions and 35
  wrong ends, because calls inside bodies (`throw new X(a);`) match. A rule has to tell a
  declaration from a statement call; the statement context is not visible to the pattern.
- [x] **C++, fmt: 727 missing, now 36.** Fixed with header patterns and a few engine rules:
  nested parentheses in parameters, bodies on the header line, signatures that continue on later
  lines, constructors and destructors (in-class and out-of-class), operator symbols, template
  prefixes, export macros in class headers (`class FMT_API file`), braced initializations in
  constructor initializer lists, and one-line classes that held the next class open. Remaining:
  conversion operators (19, which clang names by canonical type), declarations generated by
  `FMT_*` macros (11), template specializations and `template <...>` headers that do not fit
  one line (6), and the 7 false functions in the table. The C++20 module unit and the x86 builtin
  file are not measured.
- [ ] **Java, gson: 364 missing (10.5%).** Not analysed yet. Suspects: methods of nested classes and
  constructors. Measure before changing anything.
- [ ] **Kotlin, NekoBox `ServiceButton.kt`: `counters` ends late.** A local expression body is
  followed by an `if` block, and the `if` is read as the body. It was missing before PR #76, so this
  is not a regression, but it is a wrong span now. A rule for a statement keyword that cannot
  continue an expression (`if`, `for`, `while`, `return`) would cover it; check the previous line
  does not end in an operator first.
- [ ] **Kotlin, Seal: 6 missing, NekoBox: 3 missing.** Not analysed yet (`FormatPage.kt`,
  `DownloaderV2.kt`, `Utils.kt`).
- [ ] **Ruby, huginn: 19 missing.** Methods defined inside another method's body, such as `authors`
  and `_links` in `lib/feedjira_extension.rb` (inside `self.included`). The block-end finder does not
  nest. Also a few in `spec/`. Ruby `maybe`: 3 wrong ends, not analysed.
- [ ] **Swift: Alamofire 2 wrong ends, 6 missing.** Not analysed. Project `Ice` is not measured.
- [ ] **JavaScript, lodash: 357 missing (19%), 7 wrong ends.** Minified bundles: several functions
  per line, and the wrong ends are in the same bundles (`lodash.min.js`, `lodash.fp.min.js`). A
  non-minified JavaScript project is needed to measure the rest of the language. Decide whether
  minified code is in scope.
- [x] **Python: wrong spans and docstring false positives fixed locally.** Logical statements
  now account for strings, bracket continuations, explicit backslashes and multiline decorators.
  Function indentation comes from the `def` line; suite colons are read outside brackets, with
  trailing comments ignored. Inline suites close at the end of their logical statement. Triple
  strings match their own delimiter and scanning resumes after the close. Finder class ranges and
  `AnalyzePythonScopes` use the same boundaries. All 2884 real functions now have correct ends;
  the 29 Flask docstring false functions are gone. Regression tests are in `internal/python_bounds_test.go`.
- [x] **PHP: modifiers, references, nested methods and heredoc fixed locally.** The function
  pattern accepts abstract/final/static/visibility modifiers and return-by-reference names.
  Nested scanning finds methods of classes declared inside other functions. PHP-only heredoc
  preprocessing hides literal code/quotes/braces and retains code after the closing label;
  extracted source and slice offsets are preserved. Both reference projects now have zero
  missing functions, false functions and wrong ends. Tests: `internal/php_finder_test.go`.
- [x] **Scala: modifiers, parameterless/operator names and expression ends fixed locally.**
  Inline annotations/members, function-typed returns, multiline signatures, string-only bodies,
  chains after brace blocks and triple-string closing runs are covered. Both projects now match
  all 1023 reference functions exactly. Shared nested-parser implementation was not changed.
- [x] **D-YAML: nested discovery, lexical masking and contract ends fixed locally.** All 539
  oracle functions match. Attribute continuations, templates, return-by-reference/type modifiers,
  nested comments, raw/delimited/token strings and inline classes are covered by dedicated code.
- [x] **D, arsd: declaration prefixes and constructor calls fixed locally.** Conditional and
  parameterized attribute/linkage prefixes, pointer/type forms, inferred properties and multiline
  return types are found. Constructor calls are excluded using function/class context, including
  callback arguments; local constructors and postblits remain visible. All 11705 audited reference
  functions match exactly on 120 accepted files. Tests also cover selected-function extraction and offsets.
- [ ] **D: audit six excluded files with a stronger/native oracle.** One D-YAML and five arsd files
  have grammar errors; the accepted-file results do not establish whole-repository coverage.
- [x] **C, Lua: parenthesized function names and function-pointer typedefs fixed locally.**
  Definitions and prototypes such as `int (name)(...)` now retain their names;
  `typedef void (*Callback)(...)` and function-pointer variables are not functions.
  All 1568 active declarations match exactly on all 68 tracked C/header files in the
  recorded GCC configuration. Tests verify original extraction, offsets and selection.

### Regression sweep for any change to the nested path

The nested path is shared. Before a PR that touches `findFunctionsWithNesting`, `sigScanner`,
`continuesExpression` or the sanitizer, run the same projects and compare with the table above:
TypeScript (`hr_gemini`, `zod`), JavaScript (`lodash`), Go (`gin`, funcfinder itself), Rust
(`kanzi-rs`), Swift (`Alamofire`, `Kingfisher`). A PR that moves any of these numbers down has to
say so and why. The 2026-10-13 change to class bounds (`findClassesWithOffset`, net brace depth) and
to `sigScanner` (braced initializers, used by C++ only) touches the class lookup of every language
with classes. Differential check on 2026-10-13 (`main` binary against this change, same checkouts, no
truth oracle): zod `b500a2ee95134f81d74da7911d7378dd2853d042` 2472 functions, lodash
`2b5e6f7399a7b48005140b5d5c6bc6c0e62919a8` 2120, hr_gemini `5b69a1eb` 3277 — identical function sets.
Class attribution changed only for one-line class declarations (`class Test {}`: 11 in zod, 9 in
hr_gemini); methods after them no longer belong to that class. `abstract class X {` was not a class in the
TS/JS `class_pattern` (the `--struct` pattern already accepted it). Fixed on 2026-10-13: zod gains 11 methods of
its abstract classes (`ZodType`, `Metabench`), lodash and hr_gemini are unchanged. Abstract methods without a body are listed too (zod: `run`, `_parse`). Truth-based numbers were not re-measured.
The fixtures in `test_examples/` and `test_files/` show no change outside C++.

### Harness

- [ ] **Move the evaluators into the repo** (`tools/eval/`): `lang_eval.py` (all languages but
  Kotlin), `kt_eval.py` (Kotlin). They currently sit in a scratch directory. Run them with
  `python3`, where the tree-sitter grammars are installed.
- [ ] **Pin each project to a commit.** The projects were cloned with `--depth=1` and the commits were
  not recorded, so the numbers cannot be reproduced exactly.
- [ ] **Fix the evaluator's header comment.** It says declarations without a body are not truth, but
  the code counts them, and the table above relies on that.
- [ ] **Record the build command** next to each number. Use
  `GOPROXY=https://goproxy.io GONOSUMDB='*' go build ./cmd/funcfinder/`.

---

## CI: `Test` job failing on every push since 2026-07-01 (2026-07-17)

- [x] **FIXED.** Root cause had nothing to do with `-race` despite every
  failure looking like a bare `exit code 1` right after the last package's
  `ok` line, with no `FAIL`/`panic` anywhere — the actual error, buried in
  the full (not `--log-failed`) log: `go: no such tool "covdata"` for
  `internal/embed`, the sole package with zero test files. Computing its
  coverage entry needs `covdata`, which the CI runner's Go install lacks.
  `-race` was a red herring — the original single `go test -race
  -coverprofile=... ./internal/...` bundled both concerns into one exit
  code, so a coverage-tooling failure looked like a test/race failure.
  Confirmed by bisection: dropping `-coverprofile`/`-covermode` alone made
  every job pass.

  Fix in `.github/workflows/ci.yml`: split into `Run tests (race)` (no
  coverage) and `Run tests (coverage)` (no `-race`, excludes
  `internal/embed` via `go list ./internal/... | grep -v
  '/internal/embed$'`). A real data race still fails CI on its own; a
  missing toolchain component for an untested package no longer masquerades
  as one. Verified: `Test (1.23)`, `Test (1.24)`, and all three `Build`
  jobs green on the next push.

## deps --shards: shardForDir prefix match non-deterministically drops parent-package edges (2026-07-17)

- [x] **FIXED.** Found while mapping this project with its own tools:
  `deps . -l go --shards` showed `cmd_funcfinder.json` depending on
  `internal_knowledge.json` — but `cmd/funcfinder/main.go` only imports
  `github.com/ruslano69/funcfinder/internal`, not `internal/knowledge` at
  all. `internal/importresolver.go`'s `shardForDir` matched by string prefix
  (`rel == dirRel || strings.HasPrefix(rel, dirRel+"/")`), which any
  subpackage's files also satisfy (`internal/knowledge/db.go` starts with
  `internal/`) — so which shard it returned for the plain `internal` import
  depended on Go's unspecified map iteration order over `relToShard`.
  Verified non-deterministic against the real repo: 8 repeated runs of the
  pre-fix binary alternated between `internal.json` (correct) and
  `internal_knowledge.json` (wrong) 5:3.

  Fix: `shardForDir` now matches a file's *immediate* containing directory
  (`filepath.Dir(rel)`) exactly against `dirRel`, instead of a string
  prefix — matching Go's own import semantics (a package is the files in one
  directory, never its subdirectories). The now-unused `splitBy` parameter
  threaded through `shardForDir`/`resolveImportToShard` (it never actually
  affected the returned shard, since `relToShard`'s values already encode
  `splitBy`) was removed. Post-fix, 5 repeated real-repo runs all
  consistently show `internal.json`.

  Test: `TestBuildShardGraph_ParentAndSubpackageBothResolve`
  (`internal/importresolver_test.go`), 20 iterations per run to catch the
  map-iteration-order flakiness; fails deterministically against the pre-fix
  code on iteration 0.

## findFunctionsSimple: single-line function body treated as "no brace yet" (2026-07-17)

- [x] **FIXED.** Found while verifying the `callgraph -l` filter fix (below):
  `internal/finder.go`'s `findFunctionsSimple` — the non-nested path used by
  Rust, C, C++, C#, Java, D, Kotlin, PHP — closes a function on `depth == 0 &&
  prevDepth > 0`. That condition is never satisfied when a function's opening
  AND closing brace land on the *same* line, since the net brace delta for
  that line is 0 and `prevDepth` was also still 0 (nothing open yet) — so it
  read identically to "no brace at all, still waiting for a multiline
  signature's brace". The function stayed open indefinitely, silently
  absorbing every following line (including an entire next function) until an
  unrelated brace-balance event happened to close it. Two real-world shapes:
  - Rust idiomatic single-line bodies: `fn helper() -> i32 { 42 }`.
  - K&R-style C with a one-line body on the brace line: a signature line
    ending bare (C's `func_pattern` requires end-of-line after `)`), then
    `{ return 42; }` on the next line.

  This is exactly why `callgraph --dir` on a Rust-only file set was reporting
  0 functions/0 calls (found while verifying the `-l` filter fix — see
  below), and would affect `funcfinder --map`/`--struct` output quality on
  any of the 8 affected languages whenever a function's body fits on one
  line.

  Fix: both places `findFunctionsSimple` decides "is this function closed"
  now also check whether the line actually contained a `{` (`hasBrace :=
  strings.Contains(cleaned, "{")`), not just the net brace delta — mirroring
  the already-correct pattern in `findFunctionsWithNesting`
  (`braceDelta == 0 && strings.Contains(cleaned, "{")`). Verified the
  Rust `where`-clause multiline-signature case (the reason `prevDepth > 0`
  was required in the first place) still works.

  Tests: `TestFindFunctionsSimple_SingleLineBodyOnSignatureLine`,
  `TestFindFunctionsSimple_SingleLineBodyOnBraceLine`
  (`internal/finder_multiline_test.go`).

## Rust/Scala char literals vs single-quote ambiguity (deferred)

Loop #2 of the tdtp spec-sheet fixed char/rune-literal brace leakage by setting
`"char_delimiters": ["'"]` for Go, C#, Java, D, Kotlin. Rust and Scala were
**intentionally left out**: a lone `'` there is legal and unpaired — Rust
lifetimes (`&'a str`, `fn f<'a>()`) and Scala symbol literals (`'sym`). Treating
`'` as a char delimiter would open `StateCharLiteral` and blank the rest of the
line. If Rust/Scala char literals (`'{'`, `'}'`) ever show up as a measured
recall miss on a reference project, the fix is a *context-aware* char rule
(e.g. require a closing `'` within N chars, or a lookahead that rejects
`'<ident>` lifetime/symbol shapes), not a blanket delimiter.

---

## Code review: hot-path optimizations, bugs, stdlib wheels (2026-06-30)

Findings from reviewing the most heavily-executed code paths. Ordered by the
fix sequence (safe → risky). Checkboxes track progress.

### Stdlib "wheels" — low risk, output-identical

- [x] **`checksum_stdlib.go` hand-rolled hex** (`hexUint64Pair` + `uint64Hex`)
  → `encoding/hex.EncodeToString(h.Sum(nil))`. Byte order identical; verified
  by the incremental split round-trip tests.
- [x] **`config.go` bubble sort** in `GetSupportedLanguages` → `sort.Strings`.
- [x] **`dirprocessor.go` `itoa`** — inlined to `strconv.Itoa`.
- [x] **`callgraph_test.go` `containsSubstr`** → `strings.Contains`.

### Hot-path performance — no output change, benchmark before/after

- [x] **`enhanced_sanitizer.go` `matchesAt`** — rewritten to range over the
  pattern's runes directly (no `[]rune(pattern)`, no `string(runes[pos:...])`).
  **~2.5x faster** on the realistic CleanLine benchmark (16148 → 6592 ns/op).
- [x] **`enhanced_sanitizer.go` repeated `[]rune(s.config.X)`** — replaced with
  allocation-free `runeLen`/`firstRune` helpers; delimiter rune lengths are no
  longer recomputed via `[]rune(...)` slices.

### Bugs

- [x] **`enhanced_sanitizer.go` byte/rune index mismatch** (`CleanLine`) —
  FIXED. Buffer is now sized by rune count, and `handleBlockComment`,
  `handleMultiLineString`, `tryHandleBlockComment`, `tryHandleMultiLineString`
  scan via the rune-based `indexRunesFrom` instead of byte-slicing `line[idx:]`
  at a rune index. Covered by `sanitizer_nonascii_test.go` (6 tests:
  rune-length invariance, code preservation, block/line comment and multiline
  docstring handling across multibyte runes).
- **Unicode identifiers** (NEW, found during smoke test) — DONE across all 15
  languages. funcfinder, findstruct, callgraph and stat now detect
  function/type/call names with non-ASCII letters (`func Привет()`, `type Café`).
  - [x] **Single source of truth** — `internal/identifiers.go` defines
    `identClass` (`[\p{L}\p{Nd}_]`, Unicode equivalent of `\w`) and `identStart`
    (`[\p{L}_]`, no leading digit). Language patterns reference it via the
    `{IDENT}` placeholder, expanded at config-load time (`expandIdentPlaceholder`,
    applied in `config.go` to every compiled pattern). callgraph.go builds its
    call-site regex from the same constants — identifier recognition can no
    longer drift between "where is X defined" and "who calls X".
  - [x] **All 15 languages** — every name capture in `func_pattern`,
    `class_pattern`, `call_pattern`, `struct_type_patterns` switched
    `(\w+)` → `({IDENT}+)` (112 patterns). `\w` inside char classes
    (return-type matchers) and keyword groups left untouched.
  - [x] **callgraph** — hardcoded ASCII `callIdentRe` (ASCII-only `\b`) replaced
    with one built from `identStart`/`identClass`, no `\b`. Resolves
    `Привет → Старт`.
  - [x] **stat** — picks it up for free via each language's `call_pattern`.
  - Out-of-the-box, no flag: ASCII output unchanged (regression-guarded in
    `finder_unicode_test.go`). Cost: Unicode RE2 classes ~28% slower on the
    affected patterns (microseconds/line), dwarfed by the 2.5x sanitizer speed-up
    and negligible end-to-end. Benchmarks in `identifiers_bench_test.go`.
  - Note (left as-is, minor): return-type matchers in C/C++/C#/Java/D
    (`[\w\s\*]+`) and decorator/annotation patterns stay ASCII — a Unicode-named
    *return type* or attribute won't match, though the *name* will.
- [x] **`finder.go` dead branch** (`findFunctionsSimple`) — REMOVED. Confirmed
  dead: `} else if currentFunc != nil ...` sat inside the `else` of
  `if currentFunc != nil`, so `currentFunc` was always nil there. Multiline
  signatures (e.g. Rust where-clauses) are actually continued by the
  `if currentFunc != nil` branch on following lines. Pinned by
  `finder_multiline_test.go` (passes identically before and after removal).

### Resolved

- [x] **`dirprocessor.go` hand-rolled JSON** (`formatDirResultsJSON`,
  `formatManifestJSON`) + **incomplete `escapeJSON`** — MIGRATED to
  `encoding/json` (decision: accept the one-time format change). This fixes the
  control-char escaping bug and removes ~120 lines. Bonus: the manifest format
  is now byte-identical to what `deps --update-manifest` already wrote
  (cmd/deps used `json.MarshalIndent` all along), so the two tools no longer
  disagree on format. `escapeJSON` removed. Covered by
  `TestFormatDirResultsJSON_EscapesSpecialChars` (round-trips special chars
  through `json.Unmarshal`) plus the existing manifest round-trip tests.

---

## Reference-project findings (meetily run, 2026-06-30)

Surfaced while running the toolkit on Zackriya-Solutions/meetily (Tauri app:
TS frontend + Rust `src-tauri` + Python backend) as a candidate golden
reference project. Both are about honesty of output, which matters most for an
enterprise/Pro correctness story.

### `deps` silently returns an empty graph when rooted wrong (trust bug)

- [x] **FIXED (2026-07-17).** Two stderr diagnostics, both in `internal/importresolver.go`:
  - `DetectTSConfigAbove` — the precise signal for "rooted one level too deep,
    below the tsconfig": walks up from the analyzed root (bounded, 8 levels)
    looking for a `tsconfig.json`/`tsconfig.base.json` that `DetectTSAliases`
    (which only looks at the root itself and one level below) would never find.
    If one exists above, warns and names the directory to re-run from.
  - `ShardGraphStats.Warning()` — a general ratio-based fallback: `BuildShardGraph`
    now returns `(ShardGraph, ShardGraphStats)`, tracking how many imports
    *looked* intra-project (relative, or under a known modulePrefix/alias
    prefix) versus how many actually resolved to a shard. Below 20% resolved
    (min sample 3, to avoid noise on tiny dirs) triggers a warning — covers
    Go's `modulePrefix` misrooting too, not just TS/JS aliases.

  Wired into `cmd/deps/main.go`'s `--shards` path. Reproduced the exact
  meetily scenario with a synthetic tsconfig+alias fixture: correctly-rooted
  run is silent, `deps frontend/src --shards` (misrooted) now prints
  `WARNING: found .../frontend/tsconfig.json above .../frontend/src, ...`
  before the (still leaf-only, but now explained) graph. Tests:
  `TestDetectTSConfigAbove`, `TestBuildShardGraph_MisrootedShardsWarns`
  (`internal/importresolver_test.go`).

### `callgraph --dir` `-l` flag is a hint, not a filter — decide & document

- [x] **FIXED (2026-07-17).** Decided (b): `-l` now restricts the scan.
  `runDirMode` in [cmd/callgraph/main.go](cmd/callgraph/main.go) (~line 125)
  used to always pass the full multi-language `config` map to
  `internal.NewDirProcessor`, so the function-extraction pass auto-detected
  and processed every supported language in `dir` regardless of `-l` — only
  the (unused-for-extraction) import-alias collection was actually scoped to
  `-l`. Now passes a single-entry `internal.Config{lang: langConfig}` when
  `-l` is set, so extraction is scoped too. `--help` for `-l` updated to
  document the `--dir` behavior explicitly.

  Verified before/after on a synthetic Go+Python mixed directory: pre-fix,
  `callgraph --dir <mix> -l go` still "Analyzed 2 files" (both `main.go` and
  `app.py`); post-fix, "Analyzed 1 files" (`main.go` only). Consistent with
  `deps -l`'s existing shard-mode behavior, which already fully filters.

  **Found but out of scope, spun off separately:** Rust-only file sets
  return 0 functions/0 calls from `callgraph` (both `--dir` and `--inp`,
  independent of this fix) — Rust extraction only seems to work when mixed
  with another language's files in the same run. Pre-existing, unrelated to
  the `-l` filter change (reproduced identically on the pre-fix binary).

---

## Bugs

### HybridStructFinder: brace-less `type_alias` swallows following types (TS/JS)

- [x] **FIXED (2026-07-17).** `findAllTypes` in
  [internal/struct_finder_factory.go](internal/struct_finder_factory.go)
  (around line 212): the `braceCount == 0`, non-`IndentBased` branch used to
  leave a brace-less single-line construct (e.g. a TS `type Handler = (req:
  Request) => void;`) open with `depth = 0`, so it absorbed every following
  line — including a subsequent `class Server {...}` — until an unrelated
  brace-balance event coincidentally closed it. Now closes the type
  immediately at the line that defines it, the same way the `braceCount > 0
  && depth == 0` branch already did.

  Verified live: `funcfinder --struct --map --inp handler.ts --source ts` on
  the exact repro now reports `Handler: 1-1; Server: 3-5;` instead of
  swallowing `Server`. The test that pinned the buggy behavior
  (`TestHybridStructFinder_TypeAliasWithoutBraces_SwallowsFollowingLines`)
  is renamed `..._DoesNotSwallowFollowingLines` and now asserts both types
  are found separately.

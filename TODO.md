# TODO

Known issues and follow-up work, tracked here until they become GitHub issues.

---

## Language coverage on real projects: status and next steps (2026-10-17)

Earlier measurements used `main` at `2022d5b` against syntax-tree ground truth (tree-sitter, one grammar per
language); subsequent runs and their revisions are recorded below. Python, PHP, Scala/D, C, C++, TypeScript and C# evaluators now live in `benchmarks/`. Truth
includes declarations without a body (interface, abstract, prototype members): the design lists
them with their own span, so a missing C# or Java declaration counts as missing.

| Language | Project (truth functions) | Correct | Wrong end | Missing | State |
|---|---|---|---|---|---|
| Go | gin (1342), funcfinder itself (603) | 100% | 0 | 0 | done; re-measured on gin HEAD `0f09c3a` (1554 truth): 4 misses — one-line `struct{}` members of `type (...)` groups and multi-line `[]struct{...}` composites — fixed, now 1554/1554, 100%/100% |
| Rust | kanzi-rs (490) | 100% | 0 | 0 | done |
| Swift | Alamofire (1890), Kingfisher (109) | 99.6%, 100% | 2, 0 | 6, 0 | good |
| Kotlin | NekoBoxForAndroid (1166), Seal (781) | 99.7%, 99.2% | 1, 0 | 3, 6 | good |
| Ruby | huginn (1865), maybe (2284) | 99.0%, 99.9% | 0, 3 | 19, 0 | good |
| TypeScript | hr_gemini (2988), zod (2072) | 100%, 99.3% | 0, 1 | 0, 14 | hr_gemini measured 2026-10-13 after the declarator fix (tree-sitter TypeScript reference, 0 false functions); zod measured before it, 3 false functions |
| JavaScript | lodash (1918, minified) | 81.0% | 7 | 357 | open |
| Java | gson (3492) | 100% | 0 | 0 | production linear-token fix verified on `e2292f9`; 264/264 files, 0 false functions; 767/767 named types match header locations; recursive pilot remains separate |
| C# | Polly (5116), Dapper (1592) | 100%, 100% | 0, 0 | 0, 0 | measured 2026-10-16 with Roslyn; no false functions; 801/801 and 157/157 files; default symbols |
| C++ | fmt (1450 truth) | 97.5% | 0 | 36 (19 conversion operators, 11 macro-generated) | measured 2026-10-13; 7 false functions (explicit instantiations) |
| C++ | tinyxml2 (627 truth) | 99.8% | 0 | 1 | measured 2026-10-13; 0 false functions; `8224e427b655b83dae5e2298f1e6919523a78737` |
| C | Lua (1568, including 391 prototypes) | 100% | 0 | 0 | local fix verified; no false functions; one GCC configuration |
| Python | Flask (1462), Requests (711), hr_portal (711) | 100%, 100%, 100% | 0, 0, 0 | 0, 0, 0 | local fix verified; no false functions |
| PHP | Monolog (1602), Symfony Routing (1125) | 100%, 100% | 0, 0 | 0, 0 | local fix verified; no false functions |
| Scala | scopt (380), os-lib (643) | 100%, 100% | 0, 0 | 0, 0 | local fix verified; no false functions |
| D | D-YAML (539), arsd (11705) | 100%, 100% | 0, 0 | 0, 0 | local fix verified; no false functions; oracle accepts 34/35 and 120/125 files |

C# was re-measured on 2026-10-16 on a working-tree fix based on `bf62c23`, using
Python 3.14.6, .NET SDK 9.0.308 / Roslyn 4.14.0.0, Preview syntax and the
`FileBasedProgram` feature. No preprocessor symbols were supplied. All 958 tracked
`.cs` files parse successfully; 20 Polly and 113 Dapper candidates in disabled text
are recorded outside scoring. Truth includes named methods, constructors,
destructors, local functions and operators, including 175/29 bodyless declarations;
anonymous lambdas, implicit primary constructors and property/event accessors are
excluded. Conversion type names are compact syntax tokens. All 6708 measured
declarations have correct names/header locations and exact end lines, with zero
missing or false functions. On the same oracle, the `bf62c23` baseline had 1322/629
missing, 80/14 false functions and 3/1 wrong ends. The finder keeps source ranges
through the last branch of a conditional expression without evaluating symbols;
these results describe the recorded default-symbol profile, not every build variant.
Pins: Polly `ea41f708cfc33ba5ae01738ee8c5a1cf211fb9e9`,
Dapper `eb47546a408bbf6c178bda4f04dc205bc51ffbfe`. Evaluator:
`benchmarks/csharp_roslyn_eval.py` with `csharp_oracle.cs`; raw before/after JSON
and executable/oracle hashes are in ignored `build/csharp-eval-20261016/`.

C# scope traversal was compared on the same pins against the test-only recursive
prototype on 2026-10-16 (base `8e5cf67`, Go 1.25.7, Windows/Ryzen 9 5950X).
Ten serial in-process samples per variant give median parser times 125.152 →
123.609 ms for Polly and 38.101 → 37.240 ms for Dapper, with 0.75%/1.50% fewer
allocated bytes. Twenty paired CLI samples per variant are effectively equal:
146.369/146.373 ms and 70.226/70.254 ms. Traversal is 83 → 69 lines; the whole
effective finder is 447 → 433 lines. Both independently retain 6708/6708 correct
Roslyn functions and complete-result parity in map/extraction/offset/selection
modes. Default production traversal is unchanged. Shared tokenization/signature
rules are identical; this does not compare full grammar parsers. See
`benchmarks/CSHARP_TRAVERSAL_COMPARISON.md`; raw samples and hashes are in
ignored `build/csharp-traversal-20261016/`.

The shared recursive pilot now covers C#, Java and D, with Python and Ruby
excluded. On the same C# and D pins plus Gson `e2292f9`, independent references
find all 22444 measured declarations with correct ends and no false functions:
C# 6708, Java 3492, D 12244. D retains the six existing parse-error exclusions
(120/125 arsd and 34/35 D-YAML files); C# retains its default-symbol profile.
The walker owns nesting/context/extraction; adapters own signature, block and
preparation rules. Default production dispatch is unchanged. See
`benchmarks/BRACE_PILOT.md`; artifacts are in `build/brace-pilot-20261016/`.

D speed of the shared pilot was measured separately on all tracked D files
(125 arsd, 35 D-YAML), including the six reference exclusions. Ten serial parser
samples and twenty paired CLI samples per variant: arsd 2238.230 → 4888.042 ms
parser (2.18x slower), 441.851 → 1957.523 ms complete CLI (4.43x slower), allocated
bytes 231.212 → 1434.969 decimal MB (6.21x). D-YAML 267.529 → 279.766 ms parser;
113.920 → 110.128 ms CLI (overlapping ranges), allocations 12.306 → 23.220 MB.
The current D pilot must not replace production yet. Profiling points to repeated
header reconstruction/normalization and regex matching in the adapter. CLI type
maps also differ (arsd 4451/2857, D-YAML 90/74 entries), outside the named-function
oracle; establish type-map parity before promotion. Raw samples/profiles and
maps: `build/d-traversal-20261016/`; method/results: `benchmarks/BRACE_PILOT.md`.

After the user stopped background work, identical binaries/inputs/settings were
remeasured in `build/d-traversal-20261016-quiet/`: arsd 2234.783/4808.390 ms
serial parsing (2.15x) and 396.890/1809.914 ms CLI (4.56x). D-YAML
263.803/233.988 ms parsing, 89.293/89.196 ms CLI. Arsd's regression persists;
allocation totals are unchanged. Parser benchmarks deliberately use one Go CPU;
directory CLI uses file-level parallelism with the default CPU-count workers.

A bounded within-file scope scheduler was tried separately on the same pins
(`benchmarks/BRACE_PARALLEL.md`). With 32 file workers, adding up to 31 scope
tasks changes arsd CLI median 1804.244 → 1828.307 ms and D-YAML 86.579 →
86.739 ms (three samples/case). Arsd mean total CPU utilization rises only
12.4% → 13.5%. One file worker gives 4965.057 → 4884.765 ms on arsd, too
small a change to establish a gain in this exploratory run. Exact ordered
serial-pilot parity holds on all 1382 tracked C#/Java/D files; scheduler race
checks pass. Keep it test-only: serial preparation and parent-side header
recognition remain, and production type-map parity is still unresolved.

Java/Gson was then measured with the same scope scheduler (ten samples/case,
two warmups; `build/brace-parallel-java-20261016/`). With 32 file workers:
production 47.165 ms, serial pilot 68.369 ms, parallel pilot with scope budget
32: 69.646 ms. With one file worker: 146.582 / 158.297 / 156.413 ms.
Scope parallelism again gives no useful scaling; ordered results on all 264
Java files and timed CLI text match the serial pilot exactly. Tests/vet pass.
The pre-fix production/pilot timing gap includes different recognition: the unchanged
pilot matches all 3492 reference declarations; the old production has 367 misses and
55 false functions. This is not an isolated recursion-cost comparison.

The production Java path is now fixed independently of the recursive pilot:
`java_finder.go` uses a linear token walk and explicit scope stack. All 3492
Gson reference functions have correct ends, zero misses and zero false names;
all 767 named types match names/header locations without duplicates. The type
audit does not score end boundaries. Shared bracket pairing retains C# output
byte-for-byte on Polly/Dapper. Unit tests, complete tests, vet, race checks and
all five tool builds pass. Raw artifacts: `build/java-fix-20261016/`;
method, final timing ranges and limits: `benchmarks/JAVA_FIX.md`. The final
20-sample CLI run is noisy (an old-parser outlier reaches 2.27 s); its medians
are 70.501 → 113.450 ms at 32 file workers and 179.799 → 181.555 ms at one.
The correct parser is slower than the old incomplete path; no precise speed
ratio is established under this load.

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

TypeScript was measured on 2026-10-13 with `benchmarks/ts_tree_eval.py` (tree-sitter-typescript 0.23.2 as the reference). Truth: function declarations and signatures, class and interface methods (including bodyless overloads and abstract methods), constructors, and functions bound to a variable; anonymous callbacks are not truth. zod `b500a2ee95134f81d74da7911d7378dd2853d042` (511 tracked files, 4 unparseable): on `main` 2007 of 2072 found, recall 96.9%, precision 99.0%; now 2057 found, recall 99.3%, precision 99.85%. hr_gemini `5b69a1eb` (455 tracked files, 9 unparseable): on `main` recall 91.3%, precision 96.2%; now recall 99.9%, precision 100%, 2 wrong ends (down from 12: a closing JSX tag was read as a regular expression, and a typed component React.FC<{...}> lost its body). Still open: the 2 wrong ends (onData, an object-literal method closed by a brace and a semicolon on its line, and loadBonusGrantsForYear); 0 false functions; 2 missing (a second declarator in one const, and OrderCreationForm whose parameter destructuring runs over lines). zod: 3 false functions and 15 missing. Lodash is not in this table: its reference is a minified bundle, and a non-minified JavaScript project is still needed.

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
- [x] **TypeScript, hr_gemini: 23 wrong ends, now 0 (2026-10-13).** Causes fixed: a template literal's
  `${...}` holds strings and nested templates (the sanitizer closed the outer template at the
  first inner backtick, which hid whole files), an arrow header ending in `=>` is not complete,
  a parenthesized arrow body runs to its closing `);`, a brace inside a type argument
  (`React.FC<{...}>`) does not stop the bracket count, a closing JSX tag (`</h4>`) and a
  self-closing `}/>` are not regular expressions, and a body that opens and closes on its header
  line ends there.
- [ ] **TypeScript, zod: 14 missing, 3 false functions, 1 wrong end (2026-10-13; not re-measured after the declarator fix).**
  zod missing: `catch`, `create`, `args`, `returns`, `$constructor` (generic header over lines), the
  `formatContributor` and `createToJSONSchemaMethod` consts whose `=` sits on the previous line,
  `createStandardJSONSchemaMethod`, `registries.add<` over lines, `async "array-objects"`, and
  `loadPriorTriage` (return type with braces). zod false functions: the methods named `async` and `new`.
  zod wrong end: `validate-abort-sweep.ts` `deepInput` (truth 78-81): funcfinder ends it at 80.
- [x] **C#, Polly/Dapper: declaration context and function ends fixed locally.**
  The C# token scan uses shared cleaned facts and distinguishes typed declarations from calls,
  control statements, properties/indexers/events, primary type headers and object patterns.
  It finds multiline/generic and explicit-interface methods, constructors/destructors, local
  functions, operators and bodyless declarations, retaining original extraction and offsets.
  `callgraph` and `funcstats` exclude declaration tokens using finder metadata; real recursion
  remains, and C# local callers use the innermost scope. All 5116/1592 reference functions match
  exactly in the documented profile. Tests: `internal/csharp_finder_test.go` and funcstats tests.
- [x] **C++, fmt: 727 missing, now 36.** Fixed with header patterns and a few engine rules:
  nested parentheses in parameters, bodies on the header line, signatures that continue on later
  lines, constructors and destructors (in-class and out-of-class), operator symbols, template
  prefixes, export macros in class headers (`class FMT_API file`), braced initializations in
  constructor initializer lists, and one-line classes that held the next class open. Remaining:
  conversion operators (19, which clang names by canonical type), declarations generated by
  `FMT_*` macros (11), template specializations and `template <...>` headers that do not fit
  one line (6), and the 7 false functions in the table. The C++20 module unit and the x86 builtin
  file are not measured.
- [ ] **Promote the shared C#/Java/D recursive pilot after performance and wider-corpus validation.**
  On Gson `e2292f9`, the pilot and the independent production Java fix both retain
  3492 correct functions, including annotation elements. C# and measured D quality are retained.
  The recursive implementation is still test-only. First remove the measured D regression (arsd 2.18x slower
  serial parsing, 4.43x slower CLI, 6.21x allocated bytes) and establish type-map parity;
  optimize repeated header construction/normalization and matching in the D adapter.
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
its abstract classes (`ZodType`, `Metabench`), lodash and hr_gemini are unchanged. Abstract methods without a body are listed too (zod: `run`, `_parse`). The truth-based numbers for the 2026-10-13 TypeScript fixes are in the table above.
The fixtures in `test_examples/` and `test_files/` show no change outside C++.

### Profile and duplicates (2026-10-13)

Measured with a temporary `BenchmarkProfileRepo` (whole repository, `all` mode; 124 tracked files, 29.7k lines):
about 95 ms and 39 MB per run, 335k allocations. `LoadConfig` costs about 17 ms per CLI run. Profile reports:
`build/profile-cpu-top.txt`, `build/profile-cpu-cum.txt`, `build/profile-mem-top.txt` (ignored by git).

Open speed items, by share of the cumulative CPU profile:
- `Sanitizer.CleanLine` about 36%: every line is converted to `[]rune` and back. A byte path for ASCII lines is the largest single gain.
- Struct finder second pass about 19% (`FindStructures`): the whole file is scanned again for types the class pattern has already found in `all` mode.
- Regex execution about 21%: function patterns run on every line; a cheap pre-check could skip most lines.
- Repeated file reads: finder, struct finder and the Go alias pass each open the file (`os.Open` about 4.5%); the alias pass also sanitizes the file again (about 5% on Go files).
- `LoadConfig` compiles every language's patterns on each run; a lazy compile per language would remove most of its 17 ms.

Duplicates in production code:
- Open and scan into `[]string`, four copies (`finder.go`, `python_struct_finder.go`, `struct_finder_factory.go`, `structfinder.go`); `readAllLines` already exists.
- The type-scan loop is copied between `structfinder.go` and the hybrid finder in `struct_finder_factory.go`: the largest duplicate.
- The shard-key choice (`splitBy == "file"`) is repeated three times in `dirprocessor.go`.
- Rune-slice cleaning is repeated in `d_lines.go`, `python_finder.go` and `scala_lines.go`.

Done in the same pass (2026-10-13), each checked against the reference output on a fixed corpus (725 files, byte
identical in four output modes): each file is read once; regex patterns compile on first use; `CleanLine` has a fast
path for lines with no literal or comment start (checked against the slow path); the four open-and-scan copies are
one helper; the struct-pattern kind is fixed by sorted order. Corpus full run 1.75 s to 1.53 s; a single-file run
150 ms to 129 ms (the process itself is about 70 ms of that).

Left, and why:
- Merging `HybridStructFinder` into `StructFinder` changes TypeScript and JavaScript output: the hybrid closes a
  class or interface header that has no brace on its line, the generic finder waits for the brace. That needs a
  decision on which behaviour is wanted before the merge.
- The `all` mode still scans each file a second time for types (`FindStructures`, about 19% of CPU before these
  changes); the classes of the first pass could be reused if the dedupe rule is kept.
- The shard-key choice in `dirprocessor.go` (three copies) and the rune-slice cleaning in `d_lines.go`,
  `python_finder.go` and `scala_lines.go` are duplicates that are not yet merged.

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
  languages. funcfinder, findstruct, callgraph and funcstats now detect
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
  - [x] **funcstats** — picks it up for free via each language's `call_pattern`.
  - Out-of-the-box, no flag: ASCII output unchanged (regression-guarded in
    `finder_unicode_test.go`). Cost: Unicode RE2 classes ~28% slower on the
    affected patterns (microseconds/line), dwarfed by the 2.5x sanitizer speed-up
    and negligible end-to-end. Benchmarks in `identifiers_bench_test.go`.
  - Note (left as-is, minor): regex return-type matchers in C/C++/D
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

### `.gitignore` globs: `*` is a regex quantifier, `**` lines are dropped (2026-10-13)

- [ ] **Open.** `IgnoreMatcher.patternToRegex` (`internal/dirprocessor.go`) escapes regex characters with a class that
  leaves out `*`, so its `\*` and `\*\*` replacements never fire. A `*` stays a regex quantifier on the character before
  it: `*.log` becomes `(^|/)*\.log($|/)`, which works by accident, but `ab*c` matches `ac`, and `/_*` becomes `^_*$`,
  which matches only paths made of underscores instead of every top-level name starting with `_`. A line with `**` (`.agents/**`, `**/build`, `foo/**/bar`) does not compile
  and `parsePatterns` drops it without a word: OmniRoute's `.gitignore` has a dozen such lines that never apply.
  Fixing it changes which files every tool reads, so it needs its own change, a re-measure, and a CHANGELOG entry.
  `globLiteral` follows the current regexes and must change with them (`TestIgnoreMatcher_LiteralGateMatchesRegex`).

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

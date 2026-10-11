# TODO

Known issues and follow-up work, tracked here until they become GitHub issues.

---

## Language coverage on real projects: status and next steps (2026-10-11)

Measured on `main` at `2022d5b` against syntax-tree ground truth (tree-sitter, one grammar per
language). The evaluator scripts live outside the repo for now (see the last item). Truth
includes declarations without a body (interface, abstract, prototype members): the design lists
them with their own span, so a missing C# or Java declaration counts as missing.

| Language | Project (truth functions) | Correct | Wrong end | Missing | State |
|---|---|---|---|---|---|
| Go | gin (1342), funcfinder itself (603) | 100% | 0 | 0 | done |
| Rust | kanzi-rs (490) | 100% | 0 | 0 | done |
| Swift | Alamofire (1890), Kingfisher (109) | 99.6%, 100% | 2, 0 | 6, 0 | good |
| Kotlin | NekoBoxForAndroid (1166), Seal (781) | 99.7%, 99.2% | 1, 0 | 3, 6 | good |
| Ruby | huginn (1865), maybe (2284) | 99.0%, 99.9% | 0, 3 | 19, 0 | good |
| TypeScript | hr_gemini (3096), zod (2416) | 98.4%, 98.7% | 13, 1 | 38, 31 | hr_gemini open |
| JavaScript | lodash (1918, minified) | 75.1% | 4 | 473 | open |
| Java | gson (3480) | 89.5% | 0 | 364 | open |
| C# | Polly (5070) | 74.9% | 3 | 1272 | open |
| C++ | fmt | about 39% | not measured | about 61% (earlier run) | open |
| C | Lua | 99.9% (earlier run) | not measured | not measured | re-measure |
| Python, Scala, D, PHP | none | not measured | | | not started |

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
- [ ] **TypeScript, hr_gemini: 38 missing (was 46).** Mostly several declarations on one line
  (`let checks=0;const check=(ok)=>{…}`). The arrow pattern matches only the first declaration of a
  line. Needs a match per declaration in the nested path, with each one's braces counted from its
  own position, not the whole line's.
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
- [ ] **C++, fmt: about 61% missing.** Multi-line signatures, constructors, functions with macro
  prefixes. Regex will not get far; this needs a parser-level approach.
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
- [ ] **JavaScript, lodash: 473 missing (25%).** The file is minified bundles, so the function shape
  is not readable by the pattern. Decide whether minified code is in scope. If yes, measure a
  non-minified JavaScript project as well; if no, drop lodash from the table.
- [ ] **Scala, D, PHP, Python: no measurement.** Scala uses the nested path, which Go, JavaScript,
  TypeScript, Rust and Swift share, so any change there needs the regression sweep below. Python has
  its own finder (`python_finder.go`) and is not covered by it. Add tree-sitter-python as ground
  truth.
- [ ] **C, re-measure Lua** with the current binary; the 99.9% is from an earlier run.

### Regression sweep for any change to the nested path

The nested path is shared. Before a PR that touches `findFunctionsWithNesting`, `sigScanner`,
`continuesExpression` or the sanitizer, run the same projects and compare with the table above:
TypeScript (`hr_gemini`, `zod`), JavaScript (`lodash`), Go (`gin`, funcfinder itself), Rust
(`kanzi-rs`), Swift (`Alamofire`, `Kingfisher`). A PR that moves any of these numbers down has to
say so and why.

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

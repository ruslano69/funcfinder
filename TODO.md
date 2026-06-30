# TODO

Known issues and follow-up work, tracked here until they become GitHub issues.

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

## Bugs

### HybridStructFinder: brace-less `type_alias` swallows following types (TS/JS)

**File:** [internal/struct_finder_factory.go](internal/struct_finder_factory.go) — `findAllTypes` (around line 159)

**Found while:** adding test coverage for `struct_finder_factory.go`
(`internal/struct_finder_factory_test.go`).

**Symptom:** A single-line TypeScript `type` alias with no braces, e.g.:

```ts
type Handler = (req: Request) => void;

class Server {
    start() {}
}
```

causes `Server` to never be reported as its own type. `Handler` ends up
"open" indefinitely and absorbs everything after it (including `Server`)
until some later brace pair drives the running `depth` counter from `>0`
back to `0` — at which point *that* event closes the alias, not the class
that triggered it.

**Root cause:** in `findAllTypes`, when a type is opened with `braceCount ==
0` (no `{` on the same line) and the language is not `IndentBased`, `depth`
is initialized to `0`. The closing condition is `depth == 0 && prevDepth >
0` — but since `depth` starts at `0` and stays there until something opens a
brace, that condition is never satisfied by the alias's own lines. The type
only gets closed by an unrelated later brace-balance event, or — if no such
event ever occurs — by the EOF fallback (`if currentType != nil &&
currentType.End == 0`), which is why a *lone* type alias at end-of-input
works correctly (see `TestHybridStructFinder_TypeAliasWithoutBraces_SingleType`)
but one followed by more code does not (see
`TestHybridStructFinder_TypeAliasWithoutBraces_SwallowsFollowingLines`).

**Suggested fix:** when `braceCount == 0` and the language is not
`IndentBased`, the type is by construction single-line — close it
immediately the same way the `braceCount > 0 && depth == 0` branch already
does, instead of leaving it open with `depth = 0`.

**Test coverage:** the buggy behavior is pinned (not silently relied upon)
by `TestHybridStructFinder_TypeAliasWithoutBraces_SwallowsFollowingLines` in
`internal/struct_finder_factory_test.go`. When this is fixed, that test's
expectation should be updated to assert both `Handler` and `Server` are
found as separate types.

**Impact:** Affects `funcfinder --struct` / `--struct --extract` output
quality on TypeScript/JavaScript codebases that mix brace-less type aliases
with classes/interfaces/enums — type aliases are common in real TS code, so
this can silently drop types from `--struct` output on affected files.

# JS/TS finder API port to D

## Scope

This extends the D experiment to a separate JS/TS-only probe. VERSION and
releases are unchanged. The initial Go reference is frozen at
`b744fa0130db40816e3528e1ca36ceb868d7481a`; its source snapshot and generated
probe remain under `build/js-ts-d-port-20261018/reference-src/`.
The missing-function follow-up below changes production Go and the D prototype
together; its reference is a separately recorded base-plus-source-overlay.

The D port includes the complete named-function/class discovery path exercised
by the serial finder API, not only cleanup or header recognition:

- `js_ts_cleaner.d`: per-file lexical state, comments, quoted/raw strings,
  templates, regex literals versus division and JSX, plus cleaned text,
  starting state, indentation and code-presence facts. It intentionally follows
  the frozen Go sanitizer's behavior, rather than introducing another grammar.
- `javascript_port.d`: tokens/bracket pairs, declarations and named expressions,
  directly bound anonymous values/arrows, class/object methods, executable
  scopes and stable result order. Named expressions retain their inner name;
  computed methods/callbacks follow the same exclusions as Go.
- `typescript_header.d`: Go's leftmost-first header scanner, including all
  eight result strings and lazy type annotations. The configured function
  pattern must exactly equal the frozen constant. Remaining header patterns
  use `std.regex`.
- `typescript_port.d`: pending multiline signatures, type-literal braces,
  typed arrow/value distinctions, inline/list declarators, bodyless members,
  expression continuations and function-end boundaries. Scan contexts and
  event/depth storage are reused across lines.
- `typescript_candidates.d`: gated balanced-token recovery for headers outside
  the canonical line pattern: nested generics, split/curried values and
  multiline, quoted or keyword members. It mirrors `typescript_candidates.go`.
- `js_ts_model.d` / `js_ts_probe.d`: raw class discovery, class scope and
  signature metadata, configuration checks, input, serial timing and JSON.

The probes do not implement CLI directory flags, file workers, structural
fields/aliases, selection, offsets/extraction, dependencies, complexity or call
graphs. Go retains generic machinery for other languages, while this D binary
implements JS/TS; size differences include specialization. These are API and
lexical facts measurements, not an equivalent five-tool toolkit comparison.

## Input and ownership

ASCII lines are cleaned directly into reusable byte buffers; Unicode uses a
rune path, with exactly one masked space per original code point. Returned
text owns its storage. JavaScript tokenization reads ASCII joined text directly;
the Unicode path scans runes while tracking separate byte offsets in raw and
cleaned text. Tokens retain slices of immutable owned joined UTF-8 input, avoiding
one allocation/conversion per Unicode-path token. Raw and cleaned offsets differ
because masking changes byte lengths. Returned names can keep the whole joined
input allocation alive. Malformed source UTF-8 is decoded one invalid byte at a time
as Go does, rather than rejected by D's validating `readText`. A fresh mutable
read/decoded allocation transfers ownership through `assumeUnique`; scratch
buffers are never exposed this way. Lexical configuration changes are rejected
when unsupported by this specialized port.

## Verification

The comparison first requires exact complete cleanup facts, then exact ordered
function name/start/end/class/signature-line/signature-name arrays and raw
class name/start/end arrays. Duplicates are preserved. Every timed probe output
is rechecked, and timed parser checksums must match the saved maps.

`js_ts_port_checks.py` additionally exercises repository fixtures and generated
JS/TS programs: nested/template/regex text, JSX, private/quoted/numeric methods,
named expressions versus IIFEs, inline neighbors, multiline TS types, a 128 KiB
line, Unicode and malformed UTF-8. It compares normal/raw cleanup and finder
results for Go, release D and a separate assertions-enabled
`-O2 -boundscheck=on -d-version=AuditTSHeader -d-version=AuditJSTokens` build. The checked build compares
each TS scanner invocation with the function pattern. Another 100,000 seeded
random/header-shaped strings compare all eight TS groups directly with Go.
The JS audit compares every token's text/lines/literal flag and every bracket
pair with the original rune-to-string path; it also checks fast ASCII character
predicates against the Unicode sets. Fixtures include mixed-width comments,
literals and identifiers so raw and cleaned byte offsets diverge.

Independent references reuse `js_tree_eval.symbols` and `ts_tree_eval.symbols`,
recording grammar versions and every exclusion/finding. Exact Go parity does
not remove existing Go limitations. In the preflight accepted profiles:

| Corpus | Accepted / tracked files | Truth | Missing | False entries | Wrong ends |
|---|---:|---:|---:|---:|---:|
| Express JS | 141 / 141 | 218 | 0 | 0 | 0 |
| lodash JS | 55 / 55 | 2458 | 0 | 0 | 0 |
| zod TS | 507 / 511 | 2072 | 14 | 3 | 1 |
| OmniRoute TS | 8845 / 8853 | 28871 | 20 | 49 | 38 |
| OmniRoute JS | 448 / 449 | 2380 | 37 | 2 | 0 |

The final report records project pins/dirty state, API counts, all oracle
findings, source/binary hashes, compiler versions/options and raw measurements.
Class bounds/signature metadata are compared with Go; independent scoring here
is the existing named-function grammar profile, not a class/type oracle.

The full API results agree on **10,009 files, 36,702 functions and 4,159 raw
class entries**, including signature metadata and files outside the independent
grammar subset. References are tree-sitter 0.26.0, JavaScript grammar 0.25.0 and
TypeScript grammar 0.23.2. Project pins are Express
`0dee2c52c7c873498f8ed7d8ee4ddd0a04d3ada8`, lodash
`2b5e6f7399a7b48005140b5d5c6bc6c0e62919a8`, zod
`b500a2ee95134f81d74da7911d7378dd2853d042` and OmniRoute base
`ee221d870c199bc1aa1f3b90303ff2bc7c74509b`. The first three are clean clones.
OmniRoute is the existing working tree: `package-lock.json` and
`scripts/build/colocate-standalone.mjs` are modified. Only the latter enters the
JS corpus; TS inputs have no tracked source changes. All input hashes and
status are retained in `corpus-inputs.json`, and the modified JS source is saved
under `corpus-overrides/OmniRoute/`. These OmniRoute results describe that input
state, not the base commit alone. Input byte totals, maps and hashes were
rechecked after timing; external sources were not edited.

## Initial timings and size

Windows 10 x64 / Ryzen 9 5950X, Go 1.25.7 (`GOAMD64=v1`, stripped probe) and
LDC 1.43.0 (`-O3 -release -mcpu=x86-64`). Seven shuffled paired rounds,
seed 20261018, one timed corpus pass per parser sample. Both programs preload
source, initialize patterns and discard one complete warmup result before
timing. Outer result slots are preallocated; measured passes retain all
results. File read/configuration/JSON are outside parser timing. Complete probe
process timing includes startup, input and output; all timed outputs/checksums
are verified. Audits are compiled separately and absent from timed builds.

Medians in milliseconds:

| Corpus | Cleanup Go | Cleanup D | Finder Go | Finder D |
|---|---:|---:|---:|---:|
| Express JS | 9.552 | 1.735 | 30.053 | 23.967 |
| lodash JS | 40.998 | 9.607 | 146.569 | 150.255 |
| zod TS | 43.001 | 12.154 | 129.024 | 126.968 |
| OmniRoute TS | 667.500 | 180.146 | 1743.758 | 1639.100 |
| OmniRoute JS | 35.998 | 10.925 | 110.373 | 132.488 |

Cleanup is 3.29–5.50x faster in this port. Whole finder performance varies:
Express is faster in D; zod is approximately equal (sample ranges overlap);
lodash D takes 2.5% more time, and OmniRoute JS takes 20.0% more. OmniRoute TS
takes 6.0% less time (Go 1729.022–1752.972 ms versus D 1620.515–1647.521 ms).
The faster cleaner does not establish a language-wide or whole-tool speedup;
remaining JS tokenization/recognition merits a separate profile.

| Corpus | Finder process Go | Finder process D | Cleanup-facts process Go | Cleanup-facts process D |
|---|---:|---:|---:|---:|
| Express JS | 43.575 | 39.782 | 28.726 | 41.231 |
| lodash JS | 180.497 | 175.788 | 102.801 | 191.516 |
| zod TS | 165.886 | 163.775 | 114.244 | 195.984 |
| OmniRoute TS | 2186.382 | 1954.194 | 1842.863 | 2871.193 |
| OmniRoute JS | 140.654 | 160.933 | 91.098 | 152.596 |

The diagnostic cleanup-facts process exports every cleaned line and fact as
JSON. Its startup/input/output costs outweigh the faster in-memory cleanup;
it is slower in D on every corpus. These measurements do not isolate the
individual input/formatting operations or represent a shipping CLI mode.

Unpacked probe sizes: Go **2,776,576 bytes**, D **1,270,272 bytes**. The D binary
is 54.25% smaller, including the specialization difference described above.
The release maps, complete samples, source hashes, binary hashes, config and
verification are in `build/js-ts-d-port-20261018/`; experiment logs and the
summary are in `build/dlang-clean-js-ts-20261018/`. Post-timing bounds/audit,
fixture, raw-cleanup, malformed UTF-8 and vector checks all passed.

## JS tokenization follow-up (2026-10-18)

A separate three-run phase diagnostic located the JS slowdown in tokenization:
OmniRoute JS spent 94.7637 ms there, including 72.4798 ms in the Unicode token
loop. This is phase timing, not a sampled profile of individual operations.
Two changes address it:

- ASCII identifier/digit/whitespace checks use direct ranges, with unchanged
  Unicode-set fallbacks.
- Unicode tokens retain immutable UTF-8 slices, using independent raw/cleaned
  byte cursors while scanning runes. Malformed direct API input is normalized
  before slicing; the original conversion path remains in the audit build.

The intermediate ASCII-only change reduced the diagnostic OmniRoute tokenization
phase to 55.3423 ms; adding slices reduced it to 31.1183 ms. Diagnostics and their
instrumented source snapshots are separate from the release measurements under
`build/js-ts-profile-20261018/{before,ascii,slices}/`.

Seven new shuffled paired rounds include frozen Go, the hash-verified initial D
binary and the optimized D binary together, with the same warmup/retention and
one-pass protocol above. Old/new cleanup facts and ordered finder results agree
with Go on all 10,009 files; independent grammar findings are unchanged. Input
hashes are recorded and checked again after each corpus. These are the same
project pins and OmniRoute working-tree input state described above.

Serial finder medians, milliseconds:

| Corpus | Go | D before | D after |
|---|---:|---:|---:|
| Express JS | 29.009 | 23.534 | 12.552 |
| lodash JS | 145.039 | 150.737 | 75.185 |
| OmniRoute JS | 108.048 | 127.191 | 61.358 |
| zod TS | 129.532 | 127.007 | 125.014 |
| OmniRoute TS | 1725.450 | 1626.192 | 1606.324 |

JS parsing takes 46.7–51.8% less time than the previous D binary and is
1.76–2.31x as fast as Go on these corpora. OmniRoute JS ranges do not overlap:
Go 106.620–110.592 ms, old D 126.619–127.668 ms, new D 60.632–62.417 ms.
There is no TS-specific change; its small shifts have overlapping ranges and
are not evidence of a TypeScript speedup from this patch.

Complete finder-process medians, including startup/input/JSON:

| Corpus | Go | D before | D after |
|---|---:|---:|---:|
| Express JS | 45.745 | 40.119 | 29.712 |
| lodash JS | 179.498 | 175.760 | 100.426 |
| OmniRoute JS | 143.798 | 156.578 | 91.776 |
| zod TS | 166.051 | 162.413 | 162.372 |
| OmniRoute TS | 2160.998 | 1937.477 | 1915.454 |

The optimized probe is 1,272,832 bytes (+2,560 bytes); Go is still 2,776,576
bytes. Raw samples/maps, independent findings, per-file hashes, frozen Go and
candidate source snapshots are under `build/js-ts-d-port-20261018-js-opt/`.
This remains a serial specialized API prototype, with the scope and result-name
storage-retention limitations above. The diagnostic cleanup-facts export was
not optimized and remains slower end to end than Go.

Post-timing `AuditJSTokens`/`AuditTSHeader` assertions and bounds checks passed
on all corpora and JS/TS normal/raw-cleanup/stress fixtures. All 100,000 TS
capture-group vectors still agree. The standalone checked token test additionally
visits all 1,112,064 Unicode scalar values for word membership, all 128 ASCII
controls/characters and six direct malformed-UTF-8 API cases. Its source/build
script/output are retained in `build/js-ts-profile-20261018/`. `post-checks.json`
confirms unchanged source/binary hashes, identical initial/current corpus bytes
and independent counts; `verification.json` retains the full audit results.

## Missing-function follow-up (2026-10-18)

Production Go and the D prototype now recover all **71 previously missing named
functions** in these accepted profiles. This is a correctness change, so its
output intentionally differs from the previous binaries.

| Corpus | Truth | Missing before → after | False before → after | Wrong ends after |
|---|---:|---:|---:|---:|
| Express JS | 218 | 0 → 0 | 0 → 0 | 0 |
| lodash JS | 2458 | 0 → 0 | 0 → 0 | 0 |
| zod TS | 2072 | 14 → 0 | 3 → 2 | 1 |
| OmniRoute JS | 2380 | 37 → 0 | 2 → 2 | 0 |
| OmniRoute TS | 28871 | 20 → 0 | 49 → 45 | 38 |

The reference profile, versions, pins, input hashes and **13 parse-error
exclusions** are unchanged. All 35,999 accepted truth functions are found;
false entries and wrong ends remain, so this is not complete parser accuracy.

The lexical fixes use an incremental template frame stack shared by line
cleaning and whole-file token recovery. Regex character classes, comments,
escaped backticks and nested/multiline template expressions no longer hide
later declarations or leak braces. JS/TS comments end at their first `*/`.
Regexes with unknown identifier flags are rejected instead of consuming JSX
bracketed text and its closing tag.

The canonical TS header scanner still matches its pattern exactly. A supplemental
token pass runs when a line suggests an unresolved header. It reads balanced
nested type arguments, multiline parameters, split/curried arrows, dollar names,
async quoted methods and keyword methods in member scopes. Executable case
blocks and asserted/invoked function values do not add reference functions.
Existing duplicate names are merged one-to-one; old bounds are retained, and
the false `async` name of a quoted method is replaced. Custom Go function
patterns bypass this recovery.

Go and D have exact cleanup/finder/signature/class parity on all **10,009 files,
36,783 function entries and 4,158 raw class entries**, including the unscored
files. Class bounds/signature metadata remain API-parity checks, not independent
class-oracle results. Separate bounds/assertion builds pass all corpus maps,
normal/raw cleanup, the new lexical/header fixtures, malformed UTF-8 and the
100,000 TS capture-group vectors. Go tests additionally cover original source
extraction, selected functions, class scopes and offsets 1/71. `go test ./...`,
`go vet ./...` and builds of all five tools pass.

Artifacts and reproducible check scripts are under `build/js-ts-misses-20261018/`.
`report.json` labels the Go reference as base `b744fa0` plus the exact hashed
working-source overlay, retained in `reference-src/`; it does not claim the
clean base includes these fixes. Candidate D sources are retained in `sources/`.
`verify.py` builds that reference and checks lexical/API/oracle results;
`compile.bat`, `compile-audit.bat` and `js_ts_port_checks.py --work-dir
build/js-ts-misses-20261018` reproduce the builds/audits. The probes are now
Go 2,806,784 bytes and D 1,294,848 bytes, still with the specialization difference
described above. Historical performance tables describe their preserved binaries.

Five shuffled paired parser-only rounds compare the preserved pre-fix binaries
with these exact fixed binaries (one preloaded corpus pass, discarded warmup,
per-variant checksum validation). Medians, milliseconds:

| Corpus | Go before | Go fixed | D before | D fixed |
|---|---:|---:|---:|---:|
| Express JS | 29.224 | 29.001 | 12.272 | 12.352 |
| lodash JS | 147.188 | 145.321 | 74.219 | 76.055 |
| zod TS | 127.020 | 172.463 | 124.278 | 148.350 |
| OmniRoute JS | 107.827 | 110.552 | 61.436 | 65.717 |
| OmniRoute TS | 1710.505 | 2158.723 | 1593.788 | 1919.023 |

The additional TS token recovery has a measurable cost: Go takes 26.2% more
time on OmniRoute TS and 35.8% more on zod; D takes 20.4%/19.4% more. The
candidate-line gate avoids the pass in ordinary files, but optimizing the
remaining recovery work is still open. JS Express/lodash changes are small;
OmniRoute JS now recovers the 37 missing entries and takes 2.5%/7.0% more time
in Go/D. These are parser measurements, not complete-process/CLI timings.
Raw samples and verified binary/input hashes are in `timing.json`; diagnostic
CPU profiles are kept separately and were not run during these measurements.

The owning internal, benchmark and fixture contracts are updated. Root and
other child indexes/contracts remain unchanged because no directory boundary
or toolkit/CLI ownership changed. Outstanding quality work is the false entries
and wrong ends listed above.

## Reproduction

Initialize `F:\D\ldcvars64.bat` for the installed LDC 1.43.0 / MSVC toolchain.
Use the installed Python 3.14 runtime and the existing grammar packages under
`build/js-eval-runtime/` and `build/eval-runtime/`.

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

`--samples 0` builds and checks parity/oracle without timing. Choose a new work
directory for another Go revision. Save an audit build as `d-audit-probe.exe`
in that directory, then run:

```powershell
ldc2 -O2 -boundscheck=on -d-version=AuditTSHeader -d-version=AuditJSTokens -I=benchmarks `
  -of=build/js-ts-d-port-current/d-audit-probe.exe `
  benchmarks/js_ts_cleaner.d benchmarks/js_ts_model.d `
  benchmarks/javascript_port.d benchmarks/typescript_header.d `
  benchmarks/typescript_port.d benchmarks/typescript_candidates.d `
  benchmarks/js_ts_probe.d
python benchmarks/js_ts_port_checks.py --work-dir build/js-ts-d-port-current
```

The owning benchmark contract and README cover these modules and checks. Root
and other child contracts/indexes remain unchanged: no production API or
directory boundary changed.

For the optimized comparison, use the same projects/revision with
`--work-dir build/js-ts-d-port-20261018-js-opt` and
`--baseline-report build/js-ts-d-port-20261018/report.json`. Build the separate
audit executable and run fixture/corpus checks in the new directory too. Never
reuse the baseline directory for a candidate build.

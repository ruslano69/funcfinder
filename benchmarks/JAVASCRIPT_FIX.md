# JavaScript discovery and minified boundaries — 2026-10-18

The JS production path now uses shared lexical facts, opaque literal tokens,
bracket pairs and an explicit delimiter stack. It is independent of the
TypeScript line scanner and of the test-only recursive traversal experiments.

## Fix

The previous scanner missed expression functions and dollar identifiers, and
several functions on one minified line could inherit a neighboring body's
closing line. The JS token walk recognizes named declarations/expressions,
generators, direct identifier-variable function/arrow values and literal
class/object methods. Methods may use quoted, numeric, private or keyword
names. Comments, strings and regex literals cannot supply function headers.
Uninitialized/continued declarators and function braces on the line after the
parameters retain their declaration context. An anonymous IIFE is not named
after the variable receiving its result.

Original extraction, selected-name mode, nonzero line offsets and duplicate
names remain available. Extraction is line-based: functions sharing a line
can include the same whole line in their extracted text. JS supplies actual
name/parameter declaration coordinates to callgraph/funcstats; variable arrows
and anonymous values have no such name token. Genuine recursion must remain
counted. Nested callers use the innermost named function, and JS call/name
patterns support dollars/private identifiers. Directory discovery now includes
`.cjs` as well as `.js`, `.jsx` and `.mjs`.

## Independent profile

Reference: Python 3.14.6, tree-sitter 0.26.0,
tree-sitter-javascript 0.25.0. `js_tree_eval.py` scores all tracked JS-family
files, retains duplicate names through plain-map output, matches names/header
lines one-to-one and checks end lines independently. Runtime/package versions,
pins, pathspec, parse errors and binary hashes are recorded. Git paths are
decoded as UTF-8 on Windows, including Express's Unicode fixture paths.

Truth includes named function declarations/expressions, generators, literal
method definitions and anonymous function/arrow values directly bound to an
identifier variable. A named expression uses its internal name, without a
second alias entry (`const alias = function inner(){}` scores `inner`). An
anonymous direct value uses its variable name. Computed methods, anonymous
callbacks and anonymous assignment/property values are outside this profile.
Locations are line-based; byte/column extraction, type ranges and semantic
resolution of calls are not scored by these results.

## Before / after

Base revision `1fef10405494e4c693fcb7b3e9e24ea3011e786d` (v1.15.1) plus the
working-tree fix. Both project clones are clean and every tracked JS file
parses successfully; no files are excluded for syntax errors.

| Project | Files | Truth | Missing before → after | False before → after | Wrong ends before → after |
|---|---:|---:|---:|---:|---:|
| lodash | 55 | 2458 | 403 → 0 | 4 → 0 | 4 → 0 |
| Express | 141 | 218 | 78 → 0 | 0 → 0 | 0 → 0 |

Pins:

- lodash `2b5e6f7399a7b48005140b5d5c6bc6c0e62919a8`.
- Express `0dee2c52c7c873498f8ed7d8ee4ddd0a04d3ada8`.
- TypeScript regression / CommonJS fixture: zod
  `b500a2ee95134f81d74da7911d7378dd2853d042`.

All **2676** measured lodash/Express functions now match names/header/end
lines. The lodash score includes vendor sources and four minified bundles
containing **650** reference functions. Correct-span rate rises from 83.44%
to 100% on this lodash profile and from 64.22% to 100% on Express.
The historical 1918-function / 81% lodash entry used a different scope and
does not serve as a directly comparable baseline for this expanded profile.

On zod, all **338 TypeScript files with symbols (2693 functions)** have exactly
the same complete per-file JSON before and after. No previously mapped file
changes. Adding `.cjs` discovers one additional generated JS file; its **7/7**
functions independently match the JS reference. This check does not re-score
or broaden the earlier TypeScript quality claims.

Coverage applies to these pinned corpora and the named-function profile; it is
not a claim of complete JavaScript grammar support.

## Complete CLI timing

Windows amd64, Ryzen 9 5950X (16 cores / 32 logical CPUs), Go 1.25.7. Both
variants use the normal `build.ps1` flags, version 1.15.1, the same stripping
and version ldflags. Ten shuffled rounds (seed 20261018), two warmups per case,
`GOMAXPROCS=32`, warm caches. CLI uses the original roots and
`--dir --all --json --no-gitignore --workers <count>`.

| Project | File workers | Before, ms | After, ms |
|---|---:|---:|---:|
| lodash | 1 | 305.969 | 255.935 |
| lodash | 32 | 123.103 | 117.203 |
| Express | 1 | 98.999 | 78.393 |
| Express | 32 | 49.360 | 51.486 |

These are whole-run wall-time medians, including startup, walking, reads,
function/type discovery and captured JSON, not parser-only timings. Default
file parallelism gives a small improvement on lodash and a small slowdown on
Express; these short-run deltas should not be generalized. Type counts are
unchanged on both measured roots; function output intentionally changes.
Each variant must produce stable output. JSON omits files without symbols, so
its total file count is smaller than the tracked-file count used for scoring.

## Verification and reproduction

Passed: `go test ./... -count=1`, `go vet ./...`,
`go test -race ./internal ./cmd/funcstats -count=1`, and all five `build.ps1`
builds. Regressions cover same-line neighbors, named/anonymous/generator/arrow
contexts, literal guards, methods, IIFE results, continued declarations,
extraction/offsets/selection, `.cjs`, declaration-call exclusion and real
recursion. The race build uses the working MSYS2 UCRT64 GCC/cgo toolchain
outside the restricted Windows sandbox.

Run the commands in `README.md` with the pinned JS grammar. Score the same
roots with saved before/after binaries. Ignored `build/javascript-eval-20261018/`
retains the executables, four complete lodash/Express reference reports,
`zod-cjs.after.json`, exact maps and `typescript-regression.json`.
`compare_cli.py` reproduces the paired CLI timing and retains commands, pins,
source/binary hashes, order, samples and average process CPU readings in
`comparison.json`. CPU readings include startup/I/O and are not peak usage.
`tests.txt`, `vet.txt`, `race.txt` and `build.txt` retain verification logs.

Measured binary SHA-256:

- Before: `55b949a4bdaa6d3d2e54bbb49f9e3e34091dd24f2f9dc3d0e1a00470e5545fa0`.
- After: `ffdb30034c9a23662c594a82884e82c4d79a86b6cface926185e3f10912de72f`.

DOX: updated internal, cmd and benchmarks contracts, measurement usage,
root TODO status and the v1.15.2 changelog. Root and other child contracts
and all child indexes remain unchanged: no hierarchy, public API or CLI flag
changes. The measured before/after binaries above both report 1.15.1; their
hashes identify the measurement builds. This fix ships in v1.15.2, whose
versioned binaries are checked separately during release verification.

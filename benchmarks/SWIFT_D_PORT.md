# Swift finder experiment in D — 2026-10-19

`swift_port.d` adds a Swift lexer/token finder; `swift_port_probe.d` is its
standalone serial API entrypoint. It shares immutable result types, UTF-8
helpers and Unicode tables without modifying earlier language backends. Normal
Swift parsing compiles and calls no regex.

The lexer retains immutable source slices and byte/first/final-line coordinates.
It handles nested comments, backtick identifiers, ordinary/triple-quoted/raw
strings, nested interpolation, raw and context-gated bare regex literals, and
conditional-directive lines. Brackets are paired once. The finder handles named
`func` declarations, operator names, generic/async/throws/where headers, local
functions, protocol requirements and extension members. A following attribute
must not extend a bodyless requirement's span. Named types include nominal
class/struct/enum/actor/protocol declarations, aliases and associated types;
extensions supply a lexical owner scope, not another type declaration.

## Independent reference and scope

The reference is tree-sitter 0.26.0 with tree-sitter-swift 0.7.4 on Python
3.14.6. It walks named function/protocol-function declarations, including
operators, and nominal/alias/associated-type declarations. Names are taken from
the syntax token after the declaration keyword: the grammar has multiple `name`
fields, and an operator function's field may otherwise point to its return type.
Initializers/deinitializers, subscripts, accessors, closures, enum cases and
extensions as new types are outside this profile. String/regex content and named
declarations inside string interpolation are not discovery input.

Names/header intervals match one-to-one, preserving duplicates, and final syntax
token lines must match exactly. This does not validate exact annotation-inclusive
starts or semantic owner metadata. The in-memory reference input normalizes
CRLF to LF, preserving original line numbers; byte offsets and line tables are
both derived from that normalized input. This avoids two false grammar failures
on multiline escaped CRLF strings. Disk inputs remain unchanged and hashed.
Conditional branches remain source syntax: there is no macro expansion,
platform selection, module resolution or type checking.

Every error or missing syntax token is a file exclusion. Missing punctuation can
be an unnamed child, so error auditing visits all children, not only named ones.
The grammar binary and full diagnostics are retained. The installed Swift 4.1.3
toolchain was inspected but is not used as a reference for these modern sources.

## Corpus quality

Both checkouts are clean; the pathspec is all tracked `*.swift`, including tests,
examples and package manifests:

| Project | Pin | Accepted / input files | Functions | Named types |
|---|---|---:|---:|---:|
| Alamofire | `bda9ed57d72988a3a2ada33d824583541f86eac6` | 96 / 101 | 1,811 | 416 |
| Kingfisher | `95b3c81386a4fa6cbb9111f3e6323de304e5fe13` | 172 / 175 | 1,438 | 419 |
| Total | | 268 / 276 | 3,249 | 835 |

D has **zero missing functions/types, false entries or wrong ends** on the
accepted profile. Under the same fresh pins/reference, frozen Go 1.15.9 function
missing/false/wrong-end counts are Alamofire **4/0/2**, Kingfisher **48/0/2**.
D recovers all 52 accepted misses and fixes four ends; no previously correct
accepted function is lost. Whole maps intentionally differ: type discovery is
broader, including aliases and associated types, and signatures/owners/order are
not Go-parity contracts. Go's narrower class-map type-profile counts are
Alamofire **246/1/0** and Kingfisher **164/0/1** (missing/false/wrong end).
These are new pinned measurements, not a replay of the historical Swift table.

The eight grammar exclusions are retained, audited and timed, with no independent
accuracy claim:

- Alamofire `Source/Core/Protected.swift`.
- Alamofire `Source/Core/WebSocketRequest.swift`.
- Alamofire `Source/Features/Validation.swift`.
- Alamofire `Tests/Request+AlamofireTests.swift`.
- Alamofire `Tests/ValidationTests.swift`.
- Kingfisher `Sources/Cache/ImageCache.swift`.
- Kingfisher `Sources/Utility/Delegate.swift`.
- Kingfisher `Sources/Views/AnimatedImageView.swift`.

Full raw Go/D function counts are 1,886/1,892 for Alamofire and 1,513/1,562 for
Kingfisher. D's 205 functions in excluded files are raw output, not extra verified
truth. Full raw types Go/D are 181/447 and 269/441. The distinction between all
276 timed inputs and 268 independently scored inputs is part of the profile.

## Checks

Release/assertion maps agree on all 276 corpus inputs and 13 fixture/stress
files. Eleven accepted fixtures independently verify 21 functions and seven
types with no mismatches. Incomplete and invalid-UTF-8 fixtures have explicit
expected output instead of accepted grammar coverage.

Fixtures and separate module unit assertions cover protocol requirements,
annotations on following declarations, generic constraints, operators, extension
and local functions, class methods versus class declarations, nested comments,
raw/interpolated/triple strings, regex versus division, all conditional branches,
backticks/emoji/Unicode, CRLF continuation, a 128 KiB line and 128 nested blocks.
Assertion builds use `-O2 -boundscheck=on -d-version=AuditSwiftTokens` and check
source slices, byte/line ordering and pair symmetry. Timed release builds contain
no audit/unit code. There is no Go lexical-fact parity claim: Swift has a separate
lexer and the probe does not expose earlier cleanup/raw diagnostics.

All 15 earlier C/C++/C#/Go/Java/Rust/Kotlin D source hashes remain unchanged.
The new entrypoint does not link the earlier language finders.

## Speed and size

Seven shuffled paired rounds, three preloaded passes per sample, with warmups:

| Project | Go parser ms | D parser ms | Go / D | Go process ms | D process ms |
|---|---:|---:|---:|---:|---:|
| Alamofire | 148.556 | 17.392 | 8.54x | 184.337 | 55.808 |
| Kingfisher | 160.907 | 16.548 | 9.72x | 198.859 | 54.003 |

Complete-process speedups are 3.30x and 3.68x. Serial parsing includes lexing
and finding on preloaded source; complete-process time also includes startup,
input and JSON generation, with Python decoding outside the timer. Builds,
grammar scoring and audits finish before timing. Each variant's own maps and
checksums stay stable; all samples and input bytes are retained.

These compare the generic Go recognizer with a dedicated token algorithm in D,
with intentionally different discovery profiles. They do not isolate implementation
language effects or establish a complete-CLI speedup. All 276 inputs, including
grammar exclusions, are timed.

LDC 1.43.0 uses `-O3 -release -mcpu=x86-64`. The Swift-only D executable is
936,448 bytes versus generic Go 1.25.7's 2,807,296 bytes (`GOAMD64=v1`, stripped
linking). Specialization contributes to size; this is not equivalent full-toolkit
scope or a combined eleven-language D executable.

## Reproduction and limits

```powershell
python benchmarks/swift_port_compare.py --help
python benchmarks/swift_port_compare.py `
  --project alamofire=build/swift-d-port-20261019/repos/Alamofire `
  --project kingfisher=build/swift-d-port-20261019/repos/Kingfisher `
  --work-dir build/swift-d-port-20261019 --samples 7 --repeats 3
```

The harness expects pinned Python packages under `build/swift-eval-runtime/`,
the installed LDC/MSVC environment under `F:\D`, and the hash-verified frozen Go
probe/report referenced by `build/kotlin-d-port-20261019/report.json`. It never
builds the dirty Go working tree. `--samples 0` runs correctness only. Use a new
work directory for changed sources/inputs to retain previous measurements.

The work directory retains source/helper snapshots, builds/configs/manifests,
full Go/D/assertion maps, independent truth with every exclusion diagnostic,
fixture inputs/unit logs, source/compiler/grammar/binary/input hashes and raw
samples. Final hashes are checked after timing; an empty accepted function
profile fails the measurement.

Remaining limits include the eight reference exclusions, complete Swift Unicode
identifier/operator rules, regex-context ambiguity beyond tested cases,
declarations inside string interpolation and semantic owners. String interpolation
nesting stops at 256. Platform conditions and macros are not evaluated. The shared
CLI, file workers, extraction/selection and other toolkit analyses remain open.

DOX: benchmark ownership/contracts, README and root TODO are updated. Root and
other child AGENTS/indexes are unchanged because production ownership, CLI
contracts and repository boundaries are unchanged.

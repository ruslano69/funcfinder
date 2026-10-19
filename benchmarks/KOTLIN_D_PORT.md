# Kotlin finder experiment in D — 2026-10-19

`kotlin_port.d` adds a Kotlin lexer/token finder; `kotlin_port_probe.d` is its
standalone serial API entrypoint. It reuses immutable result types, UTF-8
helpers and Unicode tables without modifying earlier language backends.
Normal parsing compiles and calls no regex.

The lexer retains immutable source slices and both first/final token lines.
It handles nested comments, character literals, backtick identifiers, ordinary
and triple-quoted strings, nested quotes/braces in interpolation, and longer
closing quote runs. Brackets are paired once. String contents/interpolation
remain a literal token rather than named-code discovery input.

Functions include named `fun` declarations, local/extension/generic functions,
block/expression bodies and bodyless signatures. Expression scanning retains
trailing lambdas/member chains, `if`/`when`/`try`/`catch`/`finally` and infix
right operands. A finished expression stops before the next declaration or
statement, fixing the `ServiceButton.kt` local `counters` boundary. Type
discovery includes named classes/interfaces/objects/companions and aliases,
including primary-constructor annotations and classes without bodies.

## Independent reference

`kotlin_psi_oracle.java.txt` uses the **Kotlin compiler 2.2.0 PSI parser** without
semantic analysis, Gradle or Android dependencies. It walks `KtNamedFunction`,
named `KtClassOrObject` and `KtTypeAlias` elements; enum entries, constructors,
property accessors, anonymous functions/lambdas/objects and unnamed companions
are outside this named-declaration profile. Named declarations nested inside
string interpolation are explicitly excluded by both implementations.

The oracle normalizes CRLF to LF as IntelliJ documents do, preserving original
line numbers, and uses PSI UTF-16 offsets only against the corresponding Java
string. Output is explicitly UTF-8. Names/header intervals match one-to-one,
retaining duplicates, and ends must match exactly. This validates header/name
anchors and end lines, not exact annotation-inclusive starts or semantic owners.
Every `PsiErrorElement` is an explicit file exclusion; these corpora have none.

The initial tree-sitter-kotlin 1.1.0 diagnostic misparsed a valid multiline
`try`/`catch`/`finally` expression without reporting an error. It is **not** used
as the final reference or required by the final harness.

The oracle runs on portable Temurin JDK `21.0.12.1+1-LTS` under
`build/kotlin-oracle-runtime/`. The pinned compiler distribution came from
JetBrains' `v2.2.0` release; the JDK came from Adoptium's Windows x64 JDK 21
endpoint. Every compiler-library JAR, Java/Javac executable, oracle source and
compiled class is hashed in the report. Annotation processors are disabled
when compiling the oracle (`javac -proc:none`).

## Corpus results

Both checkouts are clean; only tracked `.kt` and `.kts` files are scanned:

| Project | Pin | Files | Functions | Named types |
|---|---|---:|---:|---:|
| NekoBoxForAndroid | `5768494d8ae3c74a057bb6d46c0f8dc071b0d821` | 166 | 1,171 | 232 |
| Seal | `7677f61a20fda4210e84215fef9e9b51d25daa47` | 145 | 824 | 181 |
| Total | | 311 | 1,995 | 413 |

D retains every reference function and type with **zero missing, false or
wrong-end entries**, with no parse exclusions. Frozen Go 1.15.9 under the same
PSI profile has function missing/false/wrong-end counts NekoBox **3/0/1** and
Seal **6/0/0**. No previously correct function is lost. Go raw class output
omits many aliases/bodyless/modified declarations: type-profile missing/false/
wrong-end counts are **89/0/12** and **106/0/9**; D is **0/0/0** on both.
Its named-type discovery is intentionally broader than the older Go class map.
Whole maps, signatures, order and owner metadata are not claimed to match Go.
These are freshly pinned PSI measurements, not a direct replay of the historical
Kotlin grammar table or identical historical input counts.

Release/assertion maps agree on all 311 corpus files and 12 fixture/stress files.
Ten valid fixtures independently verify 19 functions and five named types with
zero mismatches. Incomplete and invalid-UTF-8 fixtures have explicit expectations
instead of accepted PSI coverage. Cases include `counters` followed by an `if`,
bodyless annotated signatures, generic receivers, `init` extension names,
functional return types, interpolation/raw strings, infix continuations,
primary constructors, CRLF/Unicode/emoji, a 128 KiB line and 128 nested blocks.
Additional module unit assertions cover these declaration boundaries.

Bounds/token audits use `-O2 -boundscheck=on -d-version=AuditKotlinTokens`;
timed release builds contain no audit/unit code. Token source slices, byte/line
ordering and pair symmetry are checked. Kotlin has its own lexer; this probe
does not implement older cleanup/raw diagnostics or claim Go lexical-fact parity.
All 13 prior C/C++/C#/Go/Java/Rust D source hashes remain unchanged.

## Timing and binary size

Seven shuffled paired rounds, three preloaded passes each, with warmups:

| Project | Go parser ms | D parser ms | Go / D | Go process ms | D process ms |
|---|---:|---:|---:|---:|---:|
| NekoBox | 48.855 | 11.846 | 4.12x | 86.132 | 46.428 |
| Seal | 84.858 | 17.119 | 4.96x | 120.787 | 53.108 |

Complete-process speedups are 1.86x and 2.27x. Parser times include lexing and
finding on preloaded source; complete-process timing also includes startup,
input and JSON generation, with Python JSON decoding outside the timer. Each
variant's own maps/checksums remain stable. Builds, unit/assertion checks and
PSI JVM work finish before its project's timing rounds. Raw samples retain the
179.565 ms Go NekoBox process outlier; medians do not discard samples.

LDC 1.43.0 uses `-O3 -release -mcpu=x86-64`. The Kotlin-only D executable is
939,520 bytes versus the frozen generic Go 1.25.7 probe's 2,807,296 bytes
(`GOAMD64=v1`, stripped linking). Specialization and different discovery profiles
contribute to these measurements. This is not the size or speed of a complete
ten-language D CLI, and oracle/JDK assets are not runtime D dependencies.

## Reproduction and limits

```powershell
python benchmarks/kotlin_port_compare.py --help
python benchmarks/kotlin_port_compare.py `
  --project neko=build/kotlin-d-port-20261019/repos/NekoBoxForAndroid `
  --project seal=build/kotlin-d-port-20261019/repos/Seal `
  --work-dir build/kotlin-d-port-20261019 --samples 7 --repeats 3
```

The harness expects portable `jdk-*/bin/java.exe` and `javac.exe` plus
`kotlinc/lib/` in `build/kotlin-oracle-runtime/`, and the existing LDC/MSVC
environment at `F:\D`. It compiles the oracle into the selected work directory
and reuses the hash-verified frozen Go probe/report from
`build/rust-d-port-20261019/`; it does not build the dirty Go working tree.
`--samples 0` runs correctness only. Choose a new work directory for changed
sources/inputs to retain prior measurements.

The work directory retains D/helper/Java source snapshots, build commands,
configs, manifests, full Go/D/assertion maps, PSI truth and diagnostics,
fixtures/unit logs, compiler/runtime/binary/input hashes and every timing sample.
Final hashes are rechecked after timing; the harness rejects empty accepted
function truth as a failed measurement.
`postflight.py`/`postflight.json` repeat native PSI parsing, count/map and hash
checks after timing and exercise eight unsupported-configuration cases across
release/assertion builds. Neither corpus contains an excluded named function
inside string interpolation.

This is a lexical prototype, not the complete Kotlin grammar or semantic model.
It retains all platform/source-set branches without resolution. Full Unicode
identifier categories, context-sensitive syntax beyond the measured profiles,
semantic owner validation and named declarations inside string interpolation
remain outside scope. String interpolation nesting is capped at 256. The shared
CLI, file workers, extraction/selection and other toolkit tools remain open.

DOX: benchmark ownership/contracts, README and root TODO are updated. Root and
other child AGENTS/indexes stay unchanged because production ownership, CLI
contracts and repository boundaries are unchanged.

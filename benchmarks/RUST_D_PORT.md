# Rust finder experiment in D — 2026-10-19

`rust_port.d` adds a Rust lexer and token finder; `rust_port_probe.d` is its
standalone serial API entrypoint. It shares immutable result types, UTF-8 helpers
and Unicode 15.0.0 tables with earlier experiments without modifying their
sources. Normal Rust parsing compiles and calls no regex.

The lexer handles nested block comments, ordinary/byte/C strings, raw strings
with arbitrary hash counts, escaped/Unicode character literals, lifetimes,
labels and raw identifiers. Tokens retain immutable source slices and byte/line
coordinates; balanced brackets are paired once. The finder recognizes named
functions, trait/foreign signatures, nested functions, generic/where headers,
structs/enums/unions/traits, aliases and associated types. Trait/impl scopes
supply owner metadata, including `impl Trait for !`. Opaque `impl Trait` return
types do not open an impl-item scope. Attributes and macro token trees are
skipped; macro input is not expanded into Rust items.

## Corpora and independent quality

The reference is tree-sitter 0.26.0 with tree-sitter-rust 0.24.0 on Python
3.14.6. It walks parsed named function/signature items and named type items,
including associated types and aliases. It excludes anonymous closures, function
pointer types, macro definitions/invocations and unexpanded generated items.
Conditional attributes are not evaluated: all syntax branches remain in scope.
Names/header lines match one-to-one, preserving duplicates; final lines must
match exactly. Types are scored separately under the same header/end rule.

| Corpus | Files scored / input | Functions | Types | D missing / false / wrong end |
|---|---:|---:|---:|---|
| kanzi-rs | 33 / 34 | 470 | 51 | 0 / 0 / 0 for both kinds |
| Serde core source trees | 24 / 24 | 909 | 419 | 0 / 0 / 0 for both kinds |
| Total | 57 / 58 | 1,379 | 470 | 0 / 0 / 0 for both kinds |

kanzi-rs is the existing checkout at
`3a991f39f587c20b23ef8f4d9ac7a3efcaad73ae`, **dirty** in CHANGELOG,
`container.rs` and `lib.rs`, with an untracked `python_kanzi/` tree. The run reads
only tracked `.rs` files, records actual input hashes and leaves that checkout
unchanged. This is a base-plus-working-changes measurement, not a clean-pin run.
Serde is clean at `0db9b3b504bcb0a7fa9cf001429869519afd3e9a`; the pathspec is
tracked `.rs` files under `serde/src/` and `serde_core/src/`, excluding the
derive crate and syntax-negative tests.

`crates/kanzi/src/text_codec.rs` is an explicit grammar exclusion in kanzi-rs: the
reference reports `, &` at line 978 in `forward(&raw, &mut enc, ...)`. The
installed `rustfmt 1.9.0-stable (48a229ceae 2026-09-01)` accepts this file with
`--emit stdout --edition 2021`; no source is rewritten. This confirms a
reference-parser limitation, not function-span truth. Its 20 raw functions are
retained and audited, but are not added to the independently verified total.

Frozen Go 1.15.9 finds all accepted 1,379 functions with correct ends but adds
63 Serde candidates inside macro token trees. All 63 are verified against the
retained macro intervals; D excludes them. Raw function totals are kanzi
Go/D 490/490 and Serde 972/909. D raw types are 53/419; Go raw classes are
87/460. Go's old class profile includes impl scopes and omits many aliases and
associated types, so these type maps intentionally describe different things.
Whole maps, order, signatures and owner metadata are not claimed to match Go.
Impl/trait owner metadata has unit coverage but no separate semantic oracle.

## Verification

Release/assertion maps match on all 58 corpus files, including the grammar
exclusion, and 11 fixture/stress files. Nine accepted fixtures independently
verify 16 functions and six types with zero missing/false/wrong-end entries.
The incomplete declaration and invalid-UTF-8 fixture have explicit expected
outputs instead of accepted grammar coverage.

Cases include nested functions/blocks/comments, trait signatures, `!` as a
never type versus macro invocation, opaque return types, raw identifiers,
lifetimes/labels, byte/C strings, eight-hash raw strings, Unicode escapes,
malformed UTF-8, a 128 KiB line and 128 nested blocks. Separate module unit
tests pass. Assertion builds use `-O2 -boundscheck=on -d-version=AuditRustTokens`
to check token slices, byte/line ordering and pair
symmetry. There is no Go lexical-fact parity claim: Rust has its own lexer and
this entrypoint does not implement the older probes' cleanup/raw diagnostics.

All 11 earlier C/C++/C#/Go/Java D source hashes remain identical to their
verified reports. The new probe does not link the earlier language finders;
their existing snapshots/binaries remain intact.

## Timing and size

Seven shuffled paired rounds, three preloaded passes per sample, with warmups:

| Corpus | Go parser ms | D parser ms | Go / D | Go process ms | D process ms |
|---|---:|---:|---:|---:|---:|
| kanzi-rs | 15.194 | 12.799 | 1.19x | 45.653 | 42.783 |
| Serde | 14.795 | 8.088 | 1.83x | 44.517 | 37.720 |

D uses 15.8%/45.3% less parser time and 6.3%/15.3% less complete-process time.
Parser timing includes lexing/finding on preloaded input; complete-process time
also includes startup, input and JSON generation, with Python JSON decoding
outside that timer. All 58 files, including the grammar exclusion, are timed.
Builds, reference parsing, assertion tests and syntax checks run before timing.
All raw samples and each variant's own checksum/map are retained.

LDC 1.43.0 uses `-O3 -release -mcpu=x86-64`; Go 1.25.7 uses `GOAMD64=v1` with
stripped linking. The Rust-only D binary is 932,864 bytes versus generic Go's
2,807,296 bytes. These differ in specialization, discovery profile and toolkit
scope; this is not the size or speed of a complete nine-language D CLI.

## Reproduce and remaining limits

```powershell
python benchmarks/rust_port_compare.py --help
python benchmarks/rust_port_compare.py `
  --project kanzi=G:/DEV/Go/kanzi/rust_kanzi `
  --project serde=build/rust-d-port-20261019/repos/serde `
  --work-dir build/rust-d-port-20261019 --samples 7 --repeats 3
```

The evaluator needs the installed LDC/MSVC environment and
`build/rust-eval-runtime/` containing tree-sitter 0.26.0 and
tree-sitter-rust 0.24.0. It reuses the hash-verified frozen Go probe/report
under `build/go-java-d-port-20261018/`, rather than building the dirty Go tree.
`--samples 0` runs correctness only. Use a new work directory for changed
sources/inputs to preserve earlier measurements.

The work directory retains `report.json`, D/helper source snapshots, configs,
manifests, full Go/D/assertion maps, independent truth with macro intervals and
parse diagnostics, fixture inputs, unit logs, build commands, hashes and raw
samples. Final source/binary/input hashes are checked after timing.
`postflight.py`/`postflight.json` repeat hash, map and independent-count checks,
confirm the excluded file's 20 raw function bounds agree with Go, and test eight
unsupported-configuration cases across release/assertion builds. Go agreement
on the excluded file is regression evidence, not independent span accuracy.

Macro expansion, cfg selection, complete Rust parsing and full Unicode XID
classification remain outside scope. Identifiers use Go Unicode 15.0.0
Letter/Digit plus U+0300–U+036F combining marks, not Rust's full XID tables.
The reported grammar exclusion still needs another span oracle. The shared CLI,
file parallelism, selection/extraction and other toolkit analyses remain open.

DOX: benchmark ownership/contracts and README plus root TODO are updated. Root
and other child AGENTS/indexes are unchanged because production ownership, CLI
contracts and repository boundaries are unchanged.

# C++ types-only allocation step — 2026-10-21

Directory C++ `--struct` now passes `includeFunctions=false` to the existing token scanner. The optional API
argument defaults to true. Cleaning, tokenization/pairing, type discovery and function recognition/traversal remain
unchanged. Only function header-start calculation, result construction and copying into the neutral model are omitted.
File field/ownership consumers retain function results.

Function recognition cannot simply be disabled: the current scanner uses it to skip function bodies. Otherwise local
types would enter the output and change the analysis scope. Constructors, operators, parameter validation and end
recognition therefore still run. This introduces neither a second parser nor a new public flag.

## Measured result

Intel i3-12100, Windows, LDC 1.43.0 / LLVM 22.1.8, `-O3 -release -mcpu=x86-64`. Warm-cache full CLI,
`--dir --no-gitignore --struct --json`, identical absolute paths, warmups, seven shuffled paired samples per case.
Median milliseconds:

| Corpus | Workers | Before | After |
|---|---:|---:|---:|
| fmt | 1 | 64.05 | 62.54 |
| fmt | 8 | 56.03 | 56.35 |
| tinyxml2 | 1 | 35.74 | 35.81 |
| tinyxml2 | 8 | 29.75 | 29.55 |

There is no demonstrated consistent full-CLI speedup. A second eight-worker comparison gave fmt 57.69/55.64 ms
and tinyxml2 29.05/29.40 ms. Function/all mode medians show no slowdown above the plan's 3% budget.

Serial parser timing preloads C++ source, initializes configurations and warms buffers. Seven shuffled processes per
variant each time five passes. Allocation counters are sampled outside the stopwatch; no audit/allocation diagnostics
run in timed CLI builds. `FFDReferenceScan` forces full results from the same existing scanner for parser reference.

| Corpus | Parser before / after, ms | Repeat before / after, ms | Allocated bytes before / after per pass |
|---|---:|---:|---:|
| fmt | 27.195 / 25.944 | 26.740 / 26.406 | 12,798,556 / 11,783,456 |
| tinyxml2 | 2.431 / 2.305 | 2.303 / 2.259 | 1,027,488 / 940,688 |

Allocation savings are repeatable at 7.9%/8.4%; these are warmed cumulative thread allocations, not peak/live memory.
Parser medians favor the candidate by 1–5%, but the varying small effect does not establish a fixed speed gain.
Full CLI scans include unchanged non-C++ inputs; the serial probe includes C++ only.

## Verification and accuracy scope

- All 2,504 existing CLI checks and 29 module unit suites pass. A scanner unit checks full/type-only class equality
  and empty function results while preserving the existing exclusion of local types in function bodies.
- `cpp_types_only_checks.py` adds 42 frozen-output/libclang/audit checks: all directory modes, JSON/names/trees,
  file fields/extraction/docs, constructor/operator traversal, directives, CRLF/Unicode/invalid UTF-8, a 128 KiB literal,
  malformed input and 64 nested blocks. libclang independently confirms fixture types and identifies excluded locals.
- Complete directory outputs match frozen D before/after in all modes at one/eight workers on fmt and tinyxml2.
  `FFDTypeAudit` checks owned type names, bounds, kinds and ordering against full results, including names mode before
  annotation. The assertions build enables bounds checks, `AuditCppTokens` and `AuditBraceCleaner`.
- Independent corpus reference is libclang 18.1.1, C++17, `x86_64-w64-mingw32`, installed GCC 16.1.0 system includes
  and GCC-preprocessed active lines. Record flags, compiler/DLL hashes, input pins and every exclusion in the evidence.
  fmt scores tracked `include/` and `src/`; tinyxml2 scores `tinyxml2.cpp` and `tinyxml2.h`.

| Corpus | Accepted / parse-excluded files | Named compiled type definitions | Matched types | Wrong starts / ends among matches | Matched functions / truth |
|---|---:|---:|---:|---:|---:|
| fmt | 4 / 16 | 105 | 89 | 2 / 0 | 336 / 338 |
| tinyxml2 | 2 / 0 | 26 | 26 | 0 / 0 | 627 / 627 |

These counts are unchanged by the optimization. fmt's 16 unmatched compiled definitions are 14 macro-generated
`type_constant` specializations and the typedef-named anonymous C types `fmt_arg`/`fmt_type`, differing from raw
named-tag discovery. Two existing start errors include earlier conditional/export fences for `basic_string_view`
and `basic_appender`; four additional raw rows are in inactive branches. Function checks report no false entries or
wrong ends in accepted files, with two existing fmt misses. Anonymous cursors and local type scopes are counted
separately; repeated exposure of the same libclang type cursor is deduplicated by source identity.

The 16 fmt exclusions include incomplete `__normal_iterator` diagnostics with this libclang/GCC-header combination.
They prevent a whole-fmt compiler-accuracy claim; all excluded inputs still pass before/after output and token/bounds
audits. This stage preserves existing grammar/scope behavior rather than treating D or Go output as independent truth.
The historical C#/C++ harness help was checked; its full C#/Roslyn workflow was not rerun (no dotnet on PATH).
C# production source/behavior is unchanged and its controlled CLI checks pass.

## Evidence

Base revision: `4853f59`, with recorded C++ scanner/ffd routing overlays.
fmt pin: `0fc31633e7d53ac895ae0a688b83b7d44f0b0c1f`;
tinyxml2 pin: `8224e427b655b83dae5e2298f1e6919523a78737`.
Baseline executable SHA-256: `dfa020832d24e943f2162609bb8b65ed37aabe4193a7fe1694138383456b61e4`.
Candidate SHA-256: `caa425ad141b58d8281d016888330da89d42f9c8da5646ad82248cada71bafae`.

Ignored `build/cpp-types-only/` retains frozen binaries, baseline/current sources, build scripts, source/input/binary
hashes, `measure.py`, `parser_probe.d`, all primary/repeat samples, regression logs and independent findings. Primary
comparison has 168 CLI and 28 parser samples; repeat has 28 CLI and 28 parser samples. Inputs/binaries are rechecked
after verification. Temporary clones are removed after checks.

```powershell
python benchmarks/ffd/build.py --test --out build/cpp-types-only/audit --flag=-boundscheck=on --flag=-d-version=AuditCppTokens --flag=-d-version=AuditBraceCleaner --flag=-d-version=FFDTypeAudit
python benchmarks/ffd/cpp_types_only_checks.py --before build/cpp-types-only/before/ffd.exe --after build/cpp-types-only/after/ffd.exe --audit build/cpp-types-only/audit/ffd-test.exe --runtime build/cpp-types-only/runtime
```

Next: continue consumer specialization in Go/C#/Java. Further C++ speed work needs a profile of tokenization and
required boundary recognition; removing output allocation alone does not produce a material CLI gain.

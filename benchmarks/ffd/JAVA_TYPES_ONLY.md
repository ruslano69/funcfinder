# Java-language types-only step in D — 2026-10-21

Directory Java `--struct` now passes `includeFunctions=false` to the existing `findJava` token finder. The optional
API argument defaults to true. Its append helper omits method/constructor result construction and neutral-model
copies, while retaining method recognition, end scans and scope traversal. These control visibility of local classes,
anonymous-class members, enum bodies, records and compact constructors; disabling them would change type discovery.
File field/ownership consumers and function/all modes retain full results. No public flag or second parser is added.

## Measurements

Intel i3-12100, Windows, LDC 1.43.0 / LLVM 22.1.8, `-O3 -release -mcpu=x86-64`. Seven shuffled paired process
samples per case, warm filesystem cache, per-variant warmups, identical paths, `--dir --no-gitignore --struct --json`.
Median milliseconds:

| Gson | Before | After |
|---|---:|---:|
| Full CLI, one worker | 60.44 | 59.20 |
| Full CLI, eight workers | 36.16 | 36.14 |
| Preloaded serial parser | 17.235 | 16.545 |
| Parser repeat | 17.304 | 16.619 |

Parser medians improve about 4%; the small full-CLI effect does not establish a general speedup. Function/all modes
show no slowdown above the plan's 3% investigation budget (largest observed slowdown about 1.8%). The firm result
is warmed parser allocations: **2,563,136 -> 1,522,064 bytes per pass**, a **40.6% reduction**.

Parser timing preloads source, initializes configurations and warms buffers. Each of seven shuffled processes times
five passes; allocation counters are sampled outside the stopwatch. The separate repeat uses seven further pairs.
The current-source parser reference uses `FFDReferenceScan` to force full results from the same finder. Timed release
CLI builds contain no audit/allocation diagnostics. Allocations are cumulative thread bytes, not peak/live memory.

## Correctness

- All 2,504 existing CLI checks and 29 module unit suites pass. A Java unit compares full/type-only types inside a
  method and an anonymous object while checking that the optimized result contains no functions.
- `java_types_only_checks.py` adds 42 checks: frozen CLI output, independent fixture type names/lines, complete
  type audits, all modes/JSON/names/trees, file fields/extraction/docs, annotations with array values in record headers,
  compact constructors, local/anonymous scopes, annotation elements, enums, CRLF/Unicode/dollar names, invalid UTF-8,
  a 128 KiB literal, malformed input and 64 nested blocks.
- Full directory output matches before/after in three modes at one/eight workers. The separate assertions build
  enables bounds checks, `AuditBraceTokens`, `AuditBraceCleaner` and `FFDTypeAudit`; it compares owned core type
  names, starts, ends, kinds and order against full results before annotation, including names-mode audits.
- Independent reference: tree-sitter 0.25.2, tree-sitter-java 0.23.5. Named class/interface/enum/record/annotation types
  use keyword starts and final syntax-token ends; methods, explicit/compact constructors and annotation elements use
  the existing reference profile. Anonymous class scopes themselves are unnamed, but their methods/named types are
  included. Exact name/start/end multisets retain duplicates. Syntax errors would be explicit exclusions.

| Gson input | Accepted / excluded files | Named types | Methods/constructors/elements | Missing / false / wrong spans |
|---|---:|---:|---:|---:|
| Tracked Java files | 264 / 0 | 767 | 3,492 | 0 / 0 / 0 |

The assertion API probe also checks full/type-only class equality for every file and serializes each result before
token storage can be reused. The accepted grammar profile is a syntax reference, not Java semantic compilation,
field-accuracy proof or universal grammar coverage. Shared tokenizers/cleaners and other language finders are unchanged.

## Evidence

Base revision: `11d2b18`, with recorded `java_port.d` and `ffd_langs.d` overlays.
Gson pin: `e2292f9c9f2e07a4791827e8eea34aa7d80545d6`.
Baseline executable SHA-256: `c1cc4b2a6a6ed36c46ab91be2a2f82d65c5663a9818afddca90895225595c10f`.
Candidate SHA-256: `b1a74dc9747467f1880b2a874f9344fc5fa060fec0dc0f752939b21fed551f27`.

Ignored `build/java-types-only/` retains frozen binaries, baseline/current sources, grammar binary hash, input/source/
binary hashes, build scripts, `verify.py`, `java_api_probe.d`, parser probe, complete maps and regression logs. Primary
timing includes 84 CLI and 14 parser samples; repeat includes 14 parser samples. Final hashes are checked after
verification/timing. The temporary Gson clone is removed; retained pins and hashes permit reproducing its inputs.

```powershell
python benchmarks/ffd/build.py --test --out build/java-types-only/audit --flag=-boundscheck=on --flag=-d-version=AuditBraceCleaner --flag=-d-version=AuditBraceTokens --flag=-d-version=FFDTypeAudit
python benchmarks/ffd/java_types_only_checks.py --before build/java-types-only/before/ffd.exe --after build/java-types-only/after/ffd.exe --audit build/java-types-only/audit/ffd-test.exe --runtime build/java-types-only/runtime
```

Next: C# consumer specialization with independent reference checks before accepting scope or performance claims.

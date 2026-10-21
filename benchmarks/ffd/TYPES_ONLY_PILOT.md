# JS/TS types-only pilot — 2026-10-21

Directory `--struct` now requests types before parsing. JavaScript/TypeScript reuse the existing cleaner and
`hybridTypes` scanner and skip the native function finder, recovery and unused function/native-class bounds.
No new declaration parser was introduced. Other languages retain their full native scan for now.
File parsing retains function ownership context for fields; function and combined directory modes retain full scans.

## Measured result

Windows, Intel i3-12100 (4 cores / 8 logical processors), LDC 1.43.0 / LLVM 22.1.8,
`-O3 -release -mcpu=x86-64`, warm filesystem cache. Same absolute corpus paths, one warmup per variant/case,
seven shuffled paired process samples, median milliseconds. These compare D before/after, not D versus Go.

| Corpus | Workers | Before `--struct` | After | Speedup |
|---|---:|---:|---:|---:|
| Express | 1 | 46.49 | 36.61 | 1.27x |
| Express | 8 | 31.75 | 26.28 | 1.21x |
| Zod | 1 | 176.76 | 97.45 | 1.81x |
| Zod | 8 | 64.92 | 45.63 | 1.42x |

All function/combined mode medians were within 2.3% of the baseline; no regression exceeded the plan's 3% budget.
Full CLI includes discovery, reads, startup and formatting, limiting the gain on small inputs.

In-process serial parsing preloads source bytes, initializes configurations and warms the finders first.
Each of seven shuffled processes measures five passes; allocation counters are sampled outside the stopwatch.
Both variants use the same current source; `FFDReferenceScan` forces the original full finder for the reference.

| Corpus | Files | Parser before / after, ms per pass | Speedup | Allocated bytes before / after per pass |
|---|---:|---:|---:|---:|
| Express | 141 | 14.990 / 5.576 | 2.69x | 4,347,904 / 7,216 |
| Zod | 517 | 109.301 / 39.955 | 2.74x | 14,607,443 / 1,288,800 |

Allocation figures are warmed current-thread allocation counters, not retained memory or peak RSS.
No audit/statistics instrumentation was enabled in timed release binaries.

## Correctness and scope

- All 2,504 existing CLI checks and 29 module unit suites pass, alongside the 64 new checks below.
- All before/after directory JSON bytes match in three modes at one/eight workers, including every timed run.
- A separate assertions-enabled `FFDTypeAudit` build compares each real file's type names, starts, ends, kinds and
  ordering against the full existing finder. Both worker counts pass on all 658 files.
- Diagnostic `FFDScanStats` reports zero native calls/function bounds for the optimized request, versus
  141 calls / 218 function bounds on Express and 517 calls / 2,729 bounds on Zod. Hybrid calls/type counts remain
  141/1 and 517/1,054. Diagnostic native time includes tokenization/conversion; it is not a separate lexer profile.
- `types_only_checks.py` adds 64 generated CLI/baseline/AST/audit checks. Independent references are tree-sitter
  0.25.2, JavaScript grammar 0.25.0 and TypeScript 0.23.2, checking type names and declaration lines in controlled
  fixtures. Local classes, fake declarations in literals/comments, CRLF, Unicode paths, file field/context consumers,
  names and both tree formats are covered.
- Corpus equality proves preservation of the existing type profile, not complete JS/TS grammar accuracy. Existing
  hybrid finder limitations remain. This pilot does not change function recognition or claim a new corpus AST score.

## Evidence and reproduction

Baseline source: `5decf3c`; baseline executable SHA-256
`a6c9c3509ad40f2969300154e0c84a812074cac828cf72de2ff998101094a6e0`.
Candidate executable SHA-256: `547ee69ee6c43d2e125cdef87d517ba03eef0c547490b1b40583d454906e1d03`.
The final rebuilt release (64 fixture checks repeated) is
`0b3be86e7ff32ae2b6480a3b30a5a427a63550bc7a4af11b556bd06748e2066e`; the frozen timed candidate is retained separately.
Express pin: `0dee2c52c7c873498f8ed7d8ee4ddd0a04d3ada8`;
Zod pin: `afdac65e0f80c0ccf9e8909da97c411567908b57`.
The temporary clones are removed after verification. Ignored `build/types-only-pilot/` retains frozen binaries,
`measure.py`, `parser_probe.d`, build scripts, all 168 CLI samples, 28 parser samples, diagnostic logs and
`results.json` with input hashes checked before/after. The parser checksum agrees across variants.

```powershell
python benchmarks/ffd/build.py --test --out build/types-only-pilot/audit --flag=-d-version=FFDTypeAudit --flag=-d-version=FFDScanStats
python benchmarks/ffd/types_only_checks.py --before build/types-only-pilot/before/ffd.exe --after build/ffd/ffd.exe --audit build/types-only-pilot/audit/ffd-test.exe --runtime build/accuracy-runtime
```

Next: introduce types-only requests inside the existing D/C++ native scanners, retaining token pairing, local type
visibility and independent language oracles. `ScanNeeds` defaults remain conservative until each backend is verified.

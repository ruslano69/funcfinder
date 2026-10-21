# D types-only step — 2026-10-21

Directory D `--struct` now passes the consumer request into the existing `dlang_port.find` API.
The optional `includeFunctions` argument defaults to true. When false, the API returns the existing class results
after shared preparation, skipping function recognition, context stacks, function bounds and class-owner assignment.
File consumers retain function context. Function/all modes and all other languages retain their previous behavior.

Shared cleaning, normalization, header joining, hints and contract masking remain unchanged: these can affect type
visibility and coordinates. This is a conservative restriction of existing work, not a new D type grammar or a
complete migration of its remaining class regex to tokens. That migration and removal of function-only preparation
need separate accuracy work.

## Measurements

Intel i3-12100, Windows, LDC 1.43.0 / LLVM 22.1.8, `-O3 -release -mcpu=x86-64`. Full CLI uses warm filesystem
cache, the same absolute paths, `--dir --no-gitignore --struct --json`, one/eight workers, warmups and seven shuffled
paired samples. All sample output bytes match the frozen baseline. Medians in milliseconds:

| Corpus | Workers | Before | After | Ratio |
|---|---:|---:|---:|---:|
| arsd | 1 | 252.24 | 212.45 | 1.19x |
| arsd | 8 | 78.16 | 75.26 | 1.04x |
| D-YAML | 1 | 39.45 | 36.21 | 1.09x |
| D-YAML | 8 | 29.61 | 28.90 | 1.02x |

A second seven-pair check of the small eight-worker effect gave arsd 76.73/72.24 ms and D-YAML 30.02/28.50 ms.
Both sets favor the candidate, but the absolute gain is small and its percentage varies. Do not generalize a fixed
parallel gain from these two corpora. Function/all mode medians changed by at most 2.6%, within the plan's 3% budget.

Serial parser timing preloads D source, initializes configurations and warms buffers. Each of seven shuffled process
samples times five passes; allocation counters are read outside the stopwatch. The current-source reference uses
`FFDReferenceScan` to force the full existing API, not a different parser.

| Corpus | D files | Parser before / after, ms | Ratio | Allocated bytes before / after per pass |
|---|---:|---:|---:|---:|
| arsd | 126 | 202.566 / 173.776 | 1.17x | 15,881,315 / 12,339,648 |
| D-YAML | 35 | 13.208 / 10.779 | 1.23x | 1,179,136 / 925,296 |

Allocation figures are cumulative warmed thread allocations, not live/peak memory. Complete arsd directory scans
also include its unchanged PHP file; parser measurements include D only. No audit/allocation diagnostics run inside
the timed release CLI.

## Correctness and independently observed gaps

- All 2,504 existing CLI checks pass; 29 module unit suites pass, including a D request/context regression.
- `d_types_only_checks.py` adds 36 checks: frozen output, independent AST fixture names/lines, complete-type audit,
  names/trees/file fields/extraction/docs, CRLF/Unicode, multiline headers/contracts, a 128 KiB literal and 64 nested functions.
- All three directory modes match before/after at one/eight workers on both corpora. `FFDTypeAudit` compares owned
  type names, starts, ends, kinds and ordering against the full API for every D file, including grammar exclusions.
  The assertions build also enables bounds checks, `AuditHints` and `AuditFastPaths`.
- Full D API results (ordered functions/types/class scopes) also match frozen before/after runtime-pattern and
  compiled-pattern builds. Independent function scoring finds all 11,757 arsd and 539 D-YAML accepted functions,
  with zero missing/false/wrong-end entries; 20 arsd grammar misclassifications are explicitly rejected by the
  existing oracle profile. This check isolates each source file in a process: the historical standalone batch
  `dlang_port.main` retains borrowed function names across arena resets and corrupts its saved batch maps.
  Its original Go/D parity gate therefore failed and is not an accuracy result. `verify_api.py` retains the isolated
  maps/checks and grammar results. Production `ffd` owns names through `toSyms`; the timed parser consumes each
  result before the next file. Repair of the historical batch probe is separate work.
- Independent type reference: tree-sitter 0.25.2, language-pack 1.22.1, downloaded D grammar DLL hash retained in
  `verification.json`. Named class/struct/interface/union/enum declarations are scored, including locals; starts are
  normalized to the declaration keyword, excluding preceding attributes. Parse-error files are explicitly excluded.

| Corpus | Accepted / excluded files | Nominal type truth | Matched | Missing | Wrong ends among matched |
|---|---:|---:|---:|---:|---:|
| arsd | 121 / 5 | 2,365 | 2,326 | 39 | 0 |
| D-YAML | 34 / 1 | 65 | 65 | 0 | 0 |

These counts apply equally to baseline and candidate. arsd misses comprise 25 structs, 13 unions and one class.
The existing line-header profile also emits rows outside nominal type truth: arsd 1,667 and D-YAML 25.
AST matching on their declaration lines classifies 1,395/23 as manifest-constant scope rows and 272/2 as manifest
name errors (for example `enum long value` reported as `long`, or `wchar`/`dchar` reported instead of the declared name).
The classification is syntactic, not semantic type resolution. The fixture explicitly records the inherited union
omission rather than treating baseline equality as proof of accuracy. Accuracy fixes should be separate from this
optimization so changed output is explainable.

## Evidence

Base revision: `a3f41c6`, with explicit overlays of `benchmarks/dlang_port.d` and `benchmarks/ffd/ffd_langs.d`.
arsd pin: `2d687063ed8cce41ff32588e816ed0c53aa968ab`;
D-YAML pin: `09aeac155c9d0f85a0e7442dffb442de53528a41`.

Frozen baseline executable SHA-256: `2d9fe8f9db020c8532c22c5348264955dca20b816322b87a0101bc2eae5f3265`.
Candidate SHA-256: `dfa020832d24e943f2162609bb8b65ed37aabe4193a7fe1694138383456b61e4`.

Ignored `build/d-types-only/` retains binaries, baseline/current sources, build scripts, source/binary/input hashes,
`measure.py`, the preloaded `parser_probe.d`, all 168 primary CLI samples, 28 parser samples and 28 repeat CLI samples,
maps, corpus grammar findings, classification and regression logs. Input hashes are verified before/after timing;
parser checksums agree. Temporary clones are removed after checks.

```powershell
python benchmarks/ffd/build.py --test --out build/d-types-only/audit --flag=-boundscheck=on --flag=-d-version=AuditHints --flag=-d-version=AuditFastPaths --flag=-d-version=FFDTypeAudit
python benchmarks/ffd/d_types_only_checks.py --before build/d-types-only/before/ffd.exe --after build/d-types-only/after/ffd.exe --audit build/d-types-only/audit/ffd-test.exe --runtime build/d-types-only/runtime --grammar-cache build/d-types-only/grammar
```

Next performance stage: restrict function work in the existing C++ token scanner, retaining token pairing and local
types. Track the D union/struct omissions and manifest-name errors as independent accuracy work.

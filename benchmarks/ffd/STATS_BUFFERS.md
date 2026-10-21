# D statistics/call scratch reuse, 2026-10-21

This step reduces allocation and unused work after the static-configuration phase. It preserves native parsers,
the default eight workers and default GC. No lock or atomic primitive was replaced; their individual costs have
not been profiled. Production remains Go; these are the three experimental D tools.

## Changes and ownership

- The neutral token adapter reuses a thread-local `Buffer!Token` and line-classification arrays. Bracket pairs borrow
  the native token arrays instead of being copied. Token/line/pair/text slices are synchronous scratch: consume
  them before calling another lexer, finder or `tokensFor` on the same thread. Clear old token references before
  reuse so unused capacity does not retain old source strings.
- Call recognition reuses declaration masks, Go non-call/range masks, declaration-position storage and site buffers.
  The occurrence index contains only names of actual native function declarations; it still indexes every
  occurrence of those names, preserving overloads, signatures and recursive calls on declaration lines.
- Default `scanCalls` output owns its strings, masks, positions and site array, as graph consumers need.
  The synchronous stats consumer explicitly borrows it, then copies each distinct retained map key once.
  Borrowed output expires on reuse of call or lexer scratch. Returned file results never retain borrowed slices.
- Calls and identifier uses copy a key on first insertion only. Stats line/code/metadata arrays use current-file
  scratch. CLI metrics do not construct the identifier-use map; `--uncalled` enables it. Direct `analyzeStats`
  callers retain the previous full behavior through the default `collectUses=true` argument.

Parser and statistics tokenization still run separately. Sharing their complete token representation is future
work; this step only reuses storage and removes copies/work that these consumers do not need.

## Validation

- 3,394 CLI/truth checks passed: 3,137 ffd, 191 stats and 66 graph. The two added CLI regressions process 40 files
  with repeated calls and check retained names/counts and uncalled candidates after buffer reuse/growth.
- Assertions-enabled unit builds passed for 32 ffd, 25 stats and 24 graph modules. New tests verify token reset
  across backends and that an owned graph scan survives later borrowed scans and native-token reuse.
- Syntax-only Go AST count scoring matches every file/name count on 266 files: internal 137 files/27,687 lines,
  cmd 30/5,877 and Gin 99/24,556. Baseline and candidate both have zero differing pairs. This reference includes
  named identifier conversions and does not resolve types or dispatch. Fifteen-language fixture truth remains
  distinct from full grammar coverage.
- Every full JSON result matched the frozen baseline in timed runs, including all three tools and uncalled mode.
  Input/source/binary hashes and source snapshots are retained under ignored `build/d-stats-buffers-20261021/`.

## Complete CLI timings

Windows / Intel i3-12100, LDC 1.43.0 (DMD frontend 2.113.0, LLVM 22.1.8),
`-O3 -release -mcpu=x86-64`, ordinary GC and eight workers. Both variants include the prior static-config and
callgraph work on base revision `5c989f70cefb96b04a8ce1aecf1c4dd0e99ea2c5`; these are frozen working-tree builds.
Gin's commit and each source file hash are in `real/accuracy.json`.

Three warmups and 31 shuffled rounds, medians in milliseconds, including process startup/output/shutdown:

| Command | Before | After | Ratio |
|---|---:|---:|---:|
| funcstats / internal | 58.746 | 22.267 | 2.64× |
| callgraph / internal | 41.400 | 25.516 | 1.62× |
| ffd --all / internal | 13.755 | 13.771 | unchanged |

The independent Go-corpus batch also checks ordinary and uncalled output on every invocation:

| Input / funcstats mode | Before | After |
|---|---:|---:|
| internal / metrics | 59.022 | 22.263 |
| internal / uncalled | 57.010 | 26.276 |
| cmd / metrics | 23.422 | 16.216 |
| cmd / uncalled | 22.354 | 16.228 |
| Gin / metrics | 55.278 | 23.417 |
| Gin / uncalled | 51.761 | 23.964 |

Skipping unused use counts explains part of the ordinary-metrics gain; the uncalled and callgraph improvements
also exercise the allocation/index changes. The ffd directory all-mode does not request field tokenization, so
its near-zero change is expected. Small commands remain dominated by startup; --version changes are noise/layout,
not evidence of this optimization. These samples do not establish uniform speedups on other corpora/languages.
Binary sizes change only slightly: ffd 2,115,584 → 2,117,632 bytes, funcstats 1,868,288 → 1,871,360,
and callgraph 1,794,560 → 1,797,120.

## Phase and allocation diagnostic

Separate frozen time-only and allocation/profile overlays retain output equality. Their generator, source/binary
hashes, build commands and 31 shuffled rounds are in `phases/`; allocation runs use the thread-local
`GC.allocatedInCurrentThread()` counter and `--DRT-gcopt=profile:1`. Primary phase timing does not call GC stats.

The time-only file/worker wall phase drops **44.712 → 8.384 ms**. Plain binaries in the same diagnostic batch
take 60.165 → 22.815 ms. These are separate samples from the CLI table above. Aggregation drops 1.035 → 0.123 ms
because the unused use map is absent; JSON DOM/serialization time remains approximately 1.1/1.5 ms.

Per-file cumulative allocation, summed across all workers, has a median of **25.729 → 15.948 MB**, a **38.0%**
reduction. MB is decimal traffic, not peak/live memory or total process allocation. The caller participates in
file work; its main/worker allocation counter overlaps file counters and must not be added again.

| File subphase | Before allocated MB | After allocated MB |
|---|---:|---:|
| Read | 0.492 | 0.500 |
| Finder | 4.684 | 4.749 |
| Neutral token preparation | 12.968 | 8.828 |
| Call scan/counts | 5.083 | 1.530 |
| Uses and line/import/decorator metrics | 2.470 | 0.352 |

The small finder/read changes reflect scheduling and pool-growth differences; their algorithms are unchanged.
Per-phase medians are not additive. Output-stage allocations depend on available append capacity: serialization
allocates more in this candidate profile (0.234 → 0.803 MB), despite identical output and nearly unchanged time.
Do not describe the file-only reduction as a whole-process or peak-memory reduction.

The allocation/profile run reports median worker-phase collection time **3.640 → 1.280 ms**, with **3 → 2**
collections in that phase. Runtime shutdown is outside the checkpoints. This supports lower GC pressure;
it does not quantify atomic-operation or allocator-lock time. All published speedups above use ordinary GC.

## Artifacts and DOX

- `before/` and `after/`: source snapshots, manifests and uninstrumented binaries.
- `cli/report.json`: 15-case three-tool shuffled timings, result checks and input/binary hashes, produced by
  `static_config_compare.py`.
- `checks/`: existing CLI checker logs; `real/accuracy.json` and `real/timing.json`: source/AST truth, hashes,
  counts and repeated ordinary/uncalled timings. `real_compare.py` retains the scoring/timing driver.
- `phases/report.json` and the source overlays: separate diagnostic phases and allocation samples;
  `phases_driver.py` retains the overlay generator.

The nearest AGENTS.md documents borrowed/owned lifetimes and optional use analysis. Parent/root AGENTS.md files
are unchanged: ownership boundaries and project workflow are unchanged. TODO and CHANGELOG record this step.

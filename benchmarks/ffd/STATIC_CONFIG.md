# Immutable D configuration, 2026-10-21

The three experimental D CLIs now read generated immutable tables instead of decoding the same embedded JSON
several times per invocation. Go's resolved configuration remains canonical. `genconfig -d` emits sorted objects;
lookup uses binary search without allocation. Ordinary arrays remain ordered sequences, including duplicates.
No configuration array has been converted into a set.

Registry validation, extensions, documentation and stats/graph profiles use the tables directly. The existing
C++/JS/TS/D native APIs receive a JSONValue adapter only for a selected language. Their parsers and recognition
rules are unchanged. JSON output, incremental manifest decoding, associative arrays, GC and Druntime remain.
This is the configuration stage, not a replacement JSON parser or a BetterC build.

## Validation

- A D unit test converts the entire generated table back to JSONValue and compares it with the JSON reference,
  including nulls, empty arrays, booleans and escaped strings. Go tests require both generated artifacts to be current
  and reject unsupported schema kinds.
- All 3,392 existing CLI checks passed: 3,137 discovery/file/type/doc/tree/raw/flag/split/incremental checks,
  189 funcstats checks and 66 callgraph checks. Unit builds passed for 31/23/22 modules respectively.
- Before/after complete-CLI output matched on every measured invocation. This establishes configuration and
  behavior preservation on these checks; independent parser grammar coverage is unchanged.

## Measurements

Windows, Intel i3-12100, LDC 1.43.0 (DMD frontend 2.113.0, LLVM 22.1.8), release `-O3 -release -mcpu=x86-64`.
Base revision: `5c989f70cefb96b04a8ce1aecf1c4dd0e99ea2c5`, with the uncommitted callgraph/false-call fixes
present in both variants. The frozen source snapshots, compiler inputs, binary hashes, outputs and raw samples are
under ignored `build/d-static-config-20261021/`; these are working-tree builds, not clean release revisions.

Configuration-only diagnostic: warm registry, Go backend and graph profiles; nine shuffled process repetitions,
each measuring 1,000 repetitions of `initLanguages(); prepareLanguages([Lang.go]); prepareGraph();`.
GC stays enabled, with a collection before the measured loop. The extension map already exists. This excludes
process startup, file parsing, workers and output; it is not a cold CLI measurement.

| Per initialization | Before | After |
|---|---:|---:|
| Median time | 940.409 µs | 10.970 µs |
| Cumulative allocated bytes | 804,672 | 8,960 |

This configuration path is 85.7× faster and allocates 98.9% fewer bytes. Allocation is cumulative traffic,
not peak or resident memory. Other tools/languages initialize different profiles; the ratio does not describe
their complete execution time.

Complete CLI: three warmups, 31 shuffled rounds, medians in milliseconds. `internal` contains 137 Go files;
fixtures use `test_examples` (all languages for ffd, Go for stats/graph); the file case is `internal/shardutil.go`.
Each elapsed time includes process startup, stdout capture and shutdown.

| Command/input | Before | After |
|---|---:|---:|
| ffd / version | 10.003 | 9.354 |
| ffd / empty | 11.052 | 9.646 |
| ffd / internal | 15.033 | 13.699 |
| ffd / fixtures | 22.883 | 20.610 |
| ffd / file | 10.991 | 9.662 |
| funcstats / version | 9.539 | 10.180 |
| funcstats / empty | 11.820 | 10.856 |
| funcstats / internal | 56.789 | 59.730 |
| funcstats / fixtures | 13.189 | 12.076 |
| funcstats / file | 11.285 | 10.823 |
| callgraph / version | 9.648 | 10.020 |
| callgraph / empty | 11.012 | 10.277 |
| callgraph / internal | 42.298 | 41.511 |
| callgraph / fixtures | 12.625 | 11.306 |
| callgraph / file | 10.711 | 10.088 |

Short configured commands generally save about 0.5–1.4 ms. `--version` bypasses configuration, so its fluctuations
cannot be attributed to this optimization. No uniform whole-tool speedup is established.

Funcstats/internal was slower. A separate 51-pair repeat gave 58.003 → 59.478 ms (2.5% slower, overlapping
10th–90th percentile ranges). A diagnostic 31-pair run with automatic GC disabled gave 54.305 → 52.101 ms;
this suggests sensitivity to heap/collection behavior, but does not isolate its cause. Untimed GC profiles recorded
four collections in each variant, with different heap sizes/times. No GC flags or defaults were changed.
Retain this regression as an open measurement limit for subsequent allocation work.

## Phase follow-up on funcstats/internal

Separate instrumented copies of both frozen variants retain the same eight-worker pipeline and complete JSON
output. Production source/binaries are unchanged. The overlay generator, modified source snapshots, build commands,
binary/input hashes, full output and raw samples are under `build/d-static-config-20261021/phases/`;
the generator is `build/d-static-config-20261021/phases_driver.py`.

The time-only builds use monotonic timestamps without GC statistics calls. Separate allocation builds read
`GC.allocatedInCurrentThread()` (the thread-local counter, without `GC.stats()` heap locking) and sample main-thread
GC profiles with `--DRT-gcopt=profile:1`. Their elapsed times are diagnostic, not the primary time comparison.
All six variants (plain/time/allocation, before/after) ran in 51 shuffled rounds after three warmups;
every complete JSON result matched. Input and binary hashes remained unchanged.

Time-only medians, wall milliseconds per phase:

| Phase | Before | After |
|---|---:|---:|
| Registry and stats profiles | 1.116 | 0.492 |
| Walk and job selection | 1.541 | 1.071 |
| Selected language preparation | 1.100 | 0.002 |
| File processing and worker lifetime | 35.005 | 38.771 |
| Aggregation | 1.092 | 1.062 |
| JSON DOM construction | 1.048 | 1.053 |
| JSON serialization | 1.504 | 1.491 |
| Output write/flush | 0.154 | 0.152 |

Preparation saves about 2.2 ms, but the parallel file phase costs about 3.8 ms more. The per-invocation sum of
measured main phases has a median of 42.399 → 44.150 ms; medians of individual phases are not additive.
Uninstrumented binaries in these same rounds took 51.791 → 54.474 ms end to end, confirming that the regression
also exists without diagnostic code. These current absolute times differ from the earlier session's runs.

File subphase medians are **sums of elapsed intervals across files/workers**, including GC pauses and allocator
or scheduling waits. They overlap in wall time and are not CPU time or additions to the worker wall phase:

| File subphase | Before summed ms | After summed ms | Before/after allocated MB |
|---|---:|---:|---:|
| Size lookup and read | 5.621 | 5.919 | 0.500 / 0.504 |
| Native declaration finder | 20.549 | 21.119 | 4.702 / 4.701 |
| Line preparation and tokenization | 27.218 | 30.008 | 13.042 / 13.066 |
| Call recognition and count map | 113.209 | 128.678 | 5.087 / 5.075 |
| Identifier uses, line/import/decorator metrics | 103.633 | 115.252 | 2.470 / 2.470 |

MB is decimal cumulative allocation traffic from the separate allocation runs. These values are not live/peak
memory. The caller participates in file processing; its `main/workers` allocation count overlaps file counters,
so do not add that counter to the per-file allocation total.

The separate GC profile localizes a collection shift within measured main: the baseline has a median of one
collection during selected-language preparation and two during file processing; the candidate has zero and three.
Collection time inside file processing rises 2.305 → 3.794 ms. Final runtime cleanup occurs after these phase
checkpoints and is excluded from this GC breakdown.

A diagnostic 31 shuffled pairs on the same time-only binaries with automatic GC disabled gives worker-wall
30.368 → 29.926 ms and complete diagnostic CLI 48.385 → 46.842 ms, with identical output. Thus the slowdown
is localized to the allocation/GC-sensitive parallel file phase, especially the call/use work; aggregation and JSON
do not explain it. Moving collections into that phase is directly measured. Allocation-lock contention versus
individual pause/scheduling costs has not been isolated, so the observations do not prove an algorithmic regression
in call recognition or justify disabling GC by default.

Next optimization target: reduce call/use-map allocation and duplicate token preparation, then remeasure default
GC with these phase and output checks. Tokenization is the largest measured file-allocation source here.

## Binary size and reproduction

| Binary | Before bytes | After bytes | Increase |
|---|---:|---:|---:|
| ffd | 2,068,992 | 2,115,584 | 2.25% |
| funcstats | 1,842,688 | 1,868,288 | 1.39% |
| callgraph | 1,769,984 | 1,794,560 | 1.39% |

Tables trade a small size increase for less initialization work; std.json remains linked for other consumers.

To repeat the CLI comparison with separately built frozen variants:

```powershell
python benchmarks/ffd/static_config_compare.py --before build/baseline/bin --after build/candidate/bin --out build/static-config-repeat --rounds 31
```

The harness saves input copies/hashes, binary hashes, outputs and raw shuffled samples and rejects changed inputs,
binaries or results. Keep the sources/toolchain records corresponding to each binary alongside the report.
The initial measurement's input snapshot was recorded after the runs; its binary hashes and per-run output equality
were checked during the runs. The later harness smoke run exercises input hashing before and after execution.

DOX pass: settings contracts and measurement ownership are updated in `ffd/AGENTS.md` and `benchmarks/AGENTS.md`.
The root AGENTS.md is unchanged because the top-level boundary and workflow are unchanged.

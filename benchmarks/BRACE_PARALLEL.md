# Bounded scope parallelism experiment — 2026-10-16

Adding parallel scope traversal to the shared C#/Java/D pilot gives no useful
speedup on the measured D projects or Java/Gson with normal file parallelism. The scheduler
remains test-only; production dispatch is unchanged.

The production columns below use the frozen pre-fix baseline. Java was later
fixed independently; see [JAVA_FIX.md](JAVA_FIX.md) for current default-parser
quality and the separate before/after comparison.

## Implementation

`internal/brace_parallel_test.go` adds 140 lines of experimental implementation.
It reuses the serial pilot's preparation, tokenization and adapters. Each file
prepares its tokens sequentially, then independent scope bodies may run in Go
tasks. A process-wide nonblocking semaphore permits up to scope-workers minus
one extra tasks across all file workers; saturated or small scopes run inline.
The measured threshold is 2048 tokens. Each task owns its result slices; child
placeholders preserve source order when results join. With 32 file workers and
32 scope workers, at most 63 parsing tasks can be active; `GOMAXPROCS=32` bounds
simultaneous Go execution. The scope setting is an extra-task budget, not a
replacement for the file worker count.

## Method

Base revision `8e5cf6757ba4c96946121e13285c9f7241b9e9a0` plus working-tree
experiments. Ryzen 9 5950X, 16 cores / 32 logical CPUs, Windows amd64,
Go 1.25.7 and Python 3.14.6. Reuse the verified production and serial binaries
from `build/brace-pilot-20261016/build.json`; build the parallel variant with
the same `-trimpath -buildvcs=false` flags through a Go overlay.

Pins, complete tracked-file counts and corpus limits are recorded in
[BRACE_PILOT.md](BRACE_PILOT.md). Timing uses all 125 arsd D files and all 35
D-YAML files, including the six reference-grammar exclusions. Original-root
arsd CLI runs also include its unchanged PHP source.

For each project: file workers 1/32; production and serial pilot; parallel
pilot with scope workers 4/16/32. All cases use `GOMAXPROCS=32`, one warmup and
three samples, shuffled in rounds with seed 20261016. Commands use
`--dir <clone> --all --json --no-gitignore --workers <count>`. Wall time includes
process startup, directory walking, reads, serial preparation, all type/function
discovery and captured JSON. This is an exploratory scaling comparison; a small
difference in three samples does not establish a repeatable improvement.

Windows `GetProcessTimes` measures the child's kernel plus user CPU time.
Average total CPU utilization is CPU seconds / wall seconds / 32 × 100.
It includes startup and I/O and does not measure peak utilization. CPU-time
granularity makes the short D-YAML CPU readings particularly noisy.

## Results

Medians of whole-run wall time, milliseconds. Each CPU percentage is the
median of the three whole-run average utilization readings.

| File workers | Variant / scope workers | arsd, ms | arsd CPU | D-YAML, ms |
|---:|---|---:|---:|---:|
| 1 | Production | 2450.985 | 3.4% | 293.183 |
| 1 | Serial pilot | 4965.057 | 3.4% | 262.891 |
| 1 | Parallel / 4 | 4895.803 | 3.6% | 258.797 |
| 1 | Parallel / 16 | 4905.009 | 3.4% | 258.297 |
| 1 | Parallel / 32 | 4884.765 | 3.6% | 260.972 |
| 32 | Production | 391.426 | 27.6% | 90.580 |
| 32 | Serial pilot | 1804.244 | 12.4% | 86.579 |
| 32 | Parallel / 4 | 1808.204 | 12.7% | 87.350 |
| 32 | Parallel / 16 | 1822.897 | 13.1% | 88.173 |
| 32 | Parallel / 32 | 1828.307 | 13.5% | 86.739 |

Arsd with one file worker improves only 1.6% at scope-workers 32; with existing
file parallelism it becomes 1.3% slower. D-YAML remains effectively unchanged.
All timed parallel CLI outputs match the serial pilot exactly, including order
and types. Production versus pilot map differences remain as documented in
`BRACE_PILOT.md`; the faster production path is not a whole-map-equivalent
control.

Serial preparation/tokenization stays on each file's path. Header recognition
also runs in the parent before a recognized body's traversal can fork; flat
and bodyless declarations offer little scope work to distribute. A diagnostic
corpus run creates 154 tasks on arsd and four on D-YAML at the measured threshold.
Together with the earlier D CPU/allocation profile, the weak scaling points to
header construction, normalization and matching as the next optimization target.
This result applies to the measured scope-body scheduler, not every possible
parallel parsing algorithm.

## Java / Gson

The same matrix was measured on Gson `e2292f9c9f2e07a4791827e8eea34aa7d80545d6`
(264 tracked Java files). Same machine, verified serial/production binaries,
parallel implementation, 2048-token threshold and `GOMAXPROCS=32`. This run
uses two warmups and ten shuffled sample rounds per case. Whole-root CLI time
includes startup, reads, all supported source files, type discovery and JSON.

| File workers | Variant / scope workers | Median, ms |
|---:|---|---:|
| 1 | Production | 146.582 |
| 1 | Serial pilot | 158.297 |
| 1 | Parallel / 4 | 158.020 |
| 1 | Parallel / 16 | 156.463 |
| 1 | Parallel / 32 | 156.413 |
| 32 | Production | 47.165 |
| 32 | Serial pilot | 68.369 |
| 32 | Parallel / 4 | 67.904 |
| 32 | Parallel / 16 | 69.232 |
| 32 | Parallel / 32 | 69.646 |

The parallel-32 pilot is 1.9% slower than the serial pilot with 32 file workers;
with one file worker it is 1.2% faster. The observed ranges overlap, and neither
change establishes useful scaling. File-level parallelism itself helps the
serial pilot: 158.297 → 68.369 ms (2.32x). At 32 file workers, median average
total CPU utilization is 19.5% serial and 23.0% parallel-32; these short-run CPU
readings are coarse and are not peak measurements.

The serial pilot takes 45.0% longer than production at 32 file workers, but the
implementations have different recognition quality. The previously scored,
unchanged serial pilot finds all 3492 reference functions with correct ends and
zero false functions, while production has 367 misses and 55 false functions.
These timings therefore cannot isolate the cost of recursion. Independent
scoring details and limits remain in `BRACE_PILOT.md`.

The runner verifies exact parallel/serial ordered results on all 264 tracked
Java files and exact CLI text equality in every measured case. Full overlay
tests and vet pass again. Raw measurements, pins, commands, CPU readings,
source/binary hashes and verification logs are preserved separately in
`build/brace-parallel-java-20261016/`. Reproduction is in `README.md`; only the
harness gained a corpus-language argument. Parser and scheduler sources are
unchanged, so the earlier scheduler race check still applies.

## Verification and artifacts

- Exact ordered `FindResult` parity with the serial pilot on all 1382 tracked
  files: Polly 801, Dapper 157, Gson 264, arsd 125, D-YAML 35. This comparison
  includes the D files excluded by the independent grammar; it is not a new
  independent quality score.
- Fixtures cover extraction, selected names, nonzero offsets, source order,
  worker budgets 1/2/4/32, and 64 simultaneous file jobs sharing one budget.
- Complete parallel-overlay `go test ./... -count=1` and `go vet ./...` pass.
- Focused scheduler tests pass under `go test -race`. On this Windows host the
  GCC/cgo race build requires execution outside the restricted sandbox; the
  successful check used MSYS2 UCRT64 GCC 15.2.0.

Ignored `build/brace-parallel-20261016/` retains `comparison.json` (all samples,
commands, pins, CPU readings and hashes), overlay source, executable,
`parallel.tests.txt`, `parallel.vet.txt`, verbose `parity-tests.txt` and
successful `race-tests.txt`.

Parallel implementation SHA-256:
`3d60e717d31ab72f0c8b4d5301804dafb8aca35b3ef131aef6351dee656b06ae`.
Parallel binary SHA-256:
`96e6486a44a0417a922adce7b054ccaf53a68918f6d8449835d84724e4f6e012`.
Serial walker/adapter and serial/production binary hashes remain in the prior
build record and report. The scheduler's timing build and race check use the
same implementation source.

DOX: `internal/AGENTS.md` and `benchmarks/AGENTS.md` own the new experiment and
verification contracts; their child indexes stay unchanged. Root/other child
contracts and the release changelog stay unchanged because no hierarchy,
public API or default CLI behavior changes.

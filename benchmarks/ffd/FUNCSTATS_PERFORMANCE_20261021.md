# funcstats CLI performance, 2026-10-21

Historical baseline at `5c989f7`; current main includes later optimizations documented in [STATS_BUFFERS.md](STATS_BUFFERS.md).

The initial D implementation is slower for ordinary statistics on most measured inputs. On the local 137-file Go
tree it takes about 56–57 ms versus Go's 45 ms. The equal-output `--uncalled` case is about 1.50x faster in wall time.
This compares complete implementations and their current analysis profiles, not programming languages in isolation.

## Environment and method

- Revision: `5c989f70cefb96b04a8ce1aecf1c4dd0e99ea2c5`; Intel Core i3-12100, 8 logical processors, Windows amd64.
- Go: funcstats 1.15.10, built with Go 1.27.1. D: funcstats 0.1.0, LDC 1.43.0 / LLVM 22.1.8,
  `-O3 -release -mcpu=x86-64`, without diagnostics. Both binaries are the previously validated builds.
- Go SHA-256: `fb561db4c87c677461d713f0b5f0bd39735b6ab7be2e2b4e3e7abeb04067a069`.
- D SHA-256: `41afa48950dbfc16581338c4e31aa7b2352898369eedbda6500465c35b4cb590`.
- Two clean series of nine shuffled paired runs per case: 324 timed process invocations total. Two warm-ups per
  variant/case precede each series. Processes run sequentially; references, checks and builds are outside timing.
- Timings include process startup, walking/reading, analysis, aggregation and JSON serialization. Stdout goes to
  the null device, stderr is captured and must be empty. These are warmed-filesystem full-CLI measurements,
  not cold-cache or preloaded-parser results. Defaults are used: D has eight file workers, Go uses its CPU default.
- Every variant's complete JSON is retained and must remain byte-stable before/after its series; binary and input
  byte hashes must remain unchanged. Raw wall/child CPU samples include all outliers. Child CPU readings are coarse
  on this Windows host; wall medians below are the main result.

## Results

All times are milliseconds; each cell is the median of nine launches. Go/D above one means D is faster.

| Input / mode | Files | Lines | Go primary | D primary | Go/D | Go confirmation | D confirmation |
|---|---:|---:|---:|---:|---:|---:|---:|
| Go `cmd/funcstats/main.go` | 1 | 583 | 16.45 | 22.37 | 0.735 | 16.32 | 21.86 |
| Go `internal/finder.go` | 1 | 1,592 | 18.70 | 24.12 | 0.775 | 18.63 | 24.19 |
| Go `cmd/funcstats`, statistics | 5 | 850 | 17.67 | 23.40 | 0.755 | 17.29 | 23.27 |
| Go `internal`, statistics | 137 | 27,687 | 45.16 | 56.35 | 0.801 | 44.98 | 56.91 |
| Go `internal`, `--uncalled` | 137 | 27,687 | 81.11 | 54.04 | 1.501 | 82.13 | 54.55 |
| Installed Go `encoding/json` | 97 | 53,133 | 80.28 | 78.08 | 1.028 | 78.94 | 78.75 |
| Installed Go `net/http` | 157 | 97,534 | 96.42 | 122.92 | 0.784 | 95.94 | 123.53 |
| Python `benchmarks` | 49 | 6,003 | 36.36 | 45.46 | 0.800 | 36.01 | 44.34 |
| D `benchmarks` | 57 | 8,848 | 38.57 | 50.83 | 0.759 | 38.76 | 51.57 |

Commands use `-j` and explicit `-l go|py|d`; directory cases use `--dir`. `--uncalled` adds that flag to the
otherwise identical local Go directory command. No top-N limit is applied. Standard library paths are
`C:\Program Files\Go\src\encoding\json` and `C:\Program Files\Go\src\net\http` from the installed Go 1.27.1 tree.

The JSON values are equal only for the uncalled case. Its wall-time gain is therefore the strongest equal-output
comparison here. Ordinary statistics retain intentional accuracy/profile differences, including call and import
counts and some native line classifications. `encoding/json` is effectively tied: its 2.8% primary advantage shrinks
to 0.2% in confirmation. Do not claim a stable D speedup for ordinary statistics from these data.

## Accuracy accompanying the timing

The existing independent named-call Go AST reference was run outside timing on all four nonoverlapping Go trees:
396 files / 179,204 physical lines. Both variants' calls are compared with the same per-file AST maps.

| Input | D excess / missing occurrences | Go excess / missing occurrences |
|---|---:|---:|
| `cmd/funcstats` | 0 / 0 | 3 / 0 |
| `internal` | 0 / 0 | 105 / 0 |
| `encoding/json` | 0 / 0 | 622 / 447 |
| `net/http` | 0 / 0 | 618 / 34 |

D has zero differing file/name counts in this profile. Go has 1,348 excess and 481 missing occurrences across
383 differing file/name pairs. The reference counts named syntactic `CallExpr` identifiers/selectors and
generic/indexed callees; configured exclusions apply. Identifier type conversions remain included, while
composite-type conversions, declarations and control syntax are excluded. This is not semantic call resolution.
The extra Python/D workloads have fixture-level coverage, not an independent full-corpus call-count audit.

## Fixed costs and next work

A separate 15-pair check gives `--version` medians of Go 12.94 ms / D 19.10 ms and an empty Go file of
Go 14.43 ms / D 20.87 ms. Version handling exits before language/profile preparation. This suggests that fixed
process/runtime/module initialization costs explain much of the small-input difference; it is not a phase profile.

Source inspection identifies work to profile next, without assuming its share of the measured slowdown:

1. Profile startup/module initialization before adding algorithmic changes.
2. Propagate consumer needs: normal statistics currently build all identifier uses, while `--uncalled` also computes
   ordinary statistics/calls/imports it does not print. Apply the same conservative needs pattern already used by finders.
3. Share lexical facts/tokens between function discovery and statistics instead of tokenizing the source twice.
4. Profile copying and associative-array allocation for names/uses. Preserve owned outputs across arena resets.

No implementation is changed by this measurement. Any optimization must preserve each variant's complete output
and the independent reference checks; do not regain speed by copying Go's inaccurate counts.

## Retained evidence

`build/funcstats-speed-20261021/` retains `measure.py`, `primary.json`, `confirmation.json`, `startup.json`, complete
variant outputs, all timed samples, hashes and `quality.json`. Source/binary/input snapshots are retained there.
An additional repeat that overlapped a source inspection is explicitly excluded from the final comparison and kept
as `repeat-inspection-overlap.json`. Neither clean series overlaps checks/builds. Existing validation artifacts and
the AST reference remain under `build/funcstats-validation/`. No repositories were cloned for this measurement.

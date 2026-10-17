# C# scope traversal comparison — 2026-10-16

The recursive prototype is slightly shorter and slightly faster in serial parser
measurements. Complete CLI timings are indistinguishable at the observed
variation. Both retain the measured C# discovery quality. The production finder
is unchanged; the prototype is test-only.

## Scope

Base revision: `8e5cf6757ba4c96946121e13285c9f7241b9e9a0`, with the comparison
harness/prototype added in the working tree.

- Linear: `internal/csharp_finder.go`, with an explicit type-scope stack and
  maps for brace boundaries and block classification.
- Recursive: `internal/csharp_recursive_test.go`. Each brace scope is visited
  by a recursive call carrying the nearest type name and a local statement
  boundary. Returning restores enclosing context without the scope maps.
- Both share cleaning, tokenization, bracket pairing, declaration recognition
  and endpoint rules. No AST is constructed. This compares scope traversal,
  not two complete C# grammar implementations.
- A Go overlay replaces only the production entry function for the candidate
  CLI. Existing regression tests run unchanged against both builds.

## Inputs and environment

| Project | Pinned commit | Tracked C# files | Lines | Roslyn functions |
|---|---|---:|---:|---:|
| Polly | `ea41f708cfc33ba5ae01738ee8c5a1cf211fb9e9` | 801 | 105936 | 5116 |
| Dapper | `eb47546a408bbf6c178bda4f04dc205bc51ffbfe` | 157 | 26790 | 1592 |

Both clones are clean. Windows amd64, AMD Ryzen 9 5950X, 16 cores / 32 logical
CPUs. Go 1.25.7, Python 3.14.6, SDK 9.0.308, Roslyn 4.14.0.0. Reference settings:
Preview, `FileBasedProgram=true`, no preprocessor symbols. Named-function truth
and conditional-source span normalization are defined in `README.md`.

## Speed and allocations

Serial benchmarks preload tracked C# lines. Each operation processes the entire
project, including finder construction, sanitizing, tokenization and traversal,
without disk I/O. Ten samples per variant: five in each traversal order,
one-second target per sample, `GOMAXPROCS=1`. Values are medians.

| Metric | Linear | Recursive | Change |
|---|---:|---:|---:|
| Polly parser, ms/project | 125.152 | 123.609 | -1.23% |
| Dapper parser, ms/project | 38.101 | 37.240 | -2.26% |
| Polly allocated bytes/project | 87907870.5 | 87250664.5 | -0.75% |
| Dapper allocated bytes/project | 25511843.5 | 25129506.5 | -1.50% |
| Polly CLI, ms/project | 146.369 | 146.373 | +0.003% |
| Dapper CLI, ms/project | 70.226 | 70.254 | +0.040% |

Allocated bytes are total allocations, not peak memory. Common cleaning and
tokenization are included; these timings do not isolate traversal cost.

CLI commands use `--dir <root> --all --json --no-gitignore`, default workers,
warm filesystem caches, two warmups and twenty paired samples per variant.
First-variant order is balanced and shuffled with a recorded seed. Timing
includes startup, directory walk, reads, type discovery, JSON generation and
captured output. No Roslyn process runs during timing. Complete directory JSON
text agrees on every sample.

CLI p10–p90 ranges overlap: Polly 141.602–210.740 ms (linear) versus
139.620–275.120 ms (recursive); Dapper 68.236–72.524 ms versus 68.618–72.143 ms.
There is no measurable end-to-end advantage. The modest serial difference is
specific to these corpora and this machine.

## Size

Counts use LF-normalized UTF-8, excluding tests, harness and the test-file
package/import/comment preamble. Common code is counted in both full effective
finders. Traversal bodies contain no blank/comment lines.

| Metric | Linear | Recursive | Change |
|---|---:|---:|---:|
| Traversal lines | 83 | 69 | -14 (-16.9%) |
| Traversal bytes | 2613 | 2275 | -338 (-12.9%) |
| Full effective finder lines | 447 | 433 | -14 (-3.1%) |
| Full effective finder bytes | 12305 | 11967 | -338 (-2.7%) |
| Windows CLI executable bytes | 4406784 | 4405248 | -1536 (-0.035%) |

Both binaries use `-trimpath -buildvcs=false -ldflags=-buildid=` with the same
compiler and no stripping. Binary-size differences at this scale are minor.

## Correctness

Both binaries independently scored 5116/5116 and 1592/1592 against Roslyn:
zero missing functions, false functions, wrong ends or unparseable files.
The same 20/113 disabled-source candidates are outside scoring. Directory JSON
includes those branches (5136/1705 functions); file counts 789/156 include only
files with mapped symbols.

Complete-result parity covers all 958 real files and two synthetic examples in
three modes: map, extraction with start-line offset 71, and selected-function
extraction. It includes class scopes, signature metadata and original text.
One example has 1024 nested blocks. Full Go tests and `go vet ./...` pass for
both builds.

## Reproduction and artifacts

```bash
python benchmarks/csharp_traversal_compare.py \
  --root build/csharp-eval-20261016/repos/polly \
  --root build/csharp-eval-20261016/repos/dapper \
  --work-dir build/csharp-traversal-20261016
```

Use Python 3.14.6 from the C# evaluation environment. Generated files stay in
ignored `build/csharp-traversal-20261016/`: `comparison.json` retains samples,
sizes, commands, project pins, versions, source/executable hashes and reference
summaries; `in-process.txt` contains raw benchmarks; `*.tests.txt` contains test
results; `*.roslyn.json` contains reference findings. The overlay source and
both measured executables are adjacent.

| Artifact | SHA-256 |
|---|---|
| Production source (on-disk bytes) | `6d904769747c81e9d624ffa159684d635e9f213952d7ec64185891f7d550fef3` |
| Effective recursive source | `a78508c592c7c345a37d18017a67ae068e6da29411e843e9f7837e63573db240` |
| Linear executable | `827d0091159157a692fa84fc659b8a882d944e2f28083499443fe7e37eeb0828` |
| Recursive executable | `6ef04f24eeee04ac5e836e2cb0d3ee014629e97a5d8301964e35b604745adc41` |

DOX: `internal/AGENTS.md` and `benchmarks/AGENTS.md` describe the experiment.
Root and other child contracts stay unchanged because the hierarchy, default
parser, public APIs and CLI behavior are unchanged.

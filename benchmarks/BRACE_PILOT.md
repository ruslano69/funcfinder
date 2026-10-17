# Shared recursive scope pilot

The first pilot covers C#, Java and D. Python and Ruby are excluded; Scala's
indentation syntax is outside this brace-based pilot. The implementation stays
in test files and does not change default production dispatch.

The production Java parser was subsequently fixed independently using a linear
token walk. Baseline quality/timings below refer to the earlier `8e5cf67`
implementation; current Java results are in [JAVA_FIX.md](JAVA_FIX.md).

## Structure

- `internal/brace_recursive_test.go`: the language-neutral event walker.
  `findBraceRecursive` owns traversal, enclosing type/function context, selected
  functions, offsets and extraction from original lines. It contains no language
  keywords or token-specific recognition rules.
- `internal/brace_adapters_test.go`: token-event adapters and preparation.
  Shared token/endpoint helpers retain their existing C# names in this pilot;
  this is not a new lexer, AST or complete grammar parser.
- C# retains the measured declaration/block/conditional-expression rules and
  non-serialized signature metadata. Full result parity against production is
  tested on both pinned C# corpora in three extraction/selection/offset modes.
- Java handles annotations (including annotation array arguments), generic
  headers, return-type annotations, constructors, compact record constructors,
  bodyless annotation elements and methods of anonymous/local classes.
- D reuses its existing nested-comment/token-string/contract preparation and
  configured signature patterns. The adapter accounts for access/attribute
  sections, conditional declarations, case labels, static destructors and
  constructor delegation, including calls with lambdas as arguments. Local
  types reset the enclosing-function context for genuine constructors/postblits.

The adapter emits a declaration, nested scope, statement boundary or skipped
token range. The common walker restores parent context on return. Statements
inside expressions and language-specific header rules stay in the adapter.

## Independent measurements — 2026-10-16

Base revision: `8e5cf6757ba4c96946121e13285c9f7241b9e9a0`; pilot source is added
in the working tree. Python 3.14.6; Go 1.25.7 windows/amd64.

| Language / project | Accepted / tracked files | Truth | Missing | False | Wrong ends |
|---|---:|---:|---:|---:|---:|
| C# / Polly | 801/801 | 5116 | 0 | 0 | 0 |
| C# / Dapper | 157/157 | 1592 | 0 | 0 | 0 |
| Java / Gson | 264/264 | 3492 | 0 | 0 | 0 |
| D / D-YAML | 34/35 | 539 | 0 | 0 | 0 |
| D / arsd | 120/125 | 11705 | 0 | 0 | 0 |

Pins:

- Polly `ea41f708cfc33ba5ae01738ee8c5a1cf211fb9e9`.
- Dapper `eb47546a408bbf6c178bda4f04dc205bc51ffbfe`.
- Gson `e2292f9c9f2e07a4791827e8eea34aa7d80545d6`.
- D-YAML `09aeac155c9d0f85a0e7442dffb442de53528a41`.
- arsd `953107183178fccbe8af7340b230c37c2071aef8`.

All clones are clean. C# uses SDK 9.0.308 / Roslyn 4.14.0.0, Preview,
`FileBasedProgram=true`, no symbols; the same 20/113 disabled-source candidates
are outside scoring. Java uses tree-sitter 0.26.0 / tree-sitter-java 0.23.5.
Truth includes the nine annotation-type elements as bodyless named members.
Gson production on the same reference has 367 misses and 55 false functions.

D uses tree-sitter-language-pack 1.22.0. Its DLL hash is
`acf35033a28e91d15620505ee2b2e06efc5be01c8caa0c6fef5de6925f9588f8`.
The oracle rejects the same two final-switch statements and eighteen constructor
calls. Excluded files remain arsd's `cli.d`, `core.d`, `ini.d`, `script.d`,
`uda.d` and D-YAML's `examples/tojson/source/app.d`. Coverage claims apply only
to accepted files and the recorded syntax profiles, not arbitrary configurations.

## Build and verify

```bash
python benchmarks/brace_pilot_build.py --work-dir build/brace-pilot-20261016 \
  --cs-root build/csharp-eval-20261016/repos/polly \
  --cs-root build/csharp-eval-20261016/repos/dapper
```

The builder adds test-only helpers as virtual ordinary Go files and injects
the three-language dispatch through an overlay. It builds both variants, runs
`go test ./... -count=1` and `go vet ./...` for both, and retains logs/hashes.
Existing D regressions run unchanged against the pilot, including selected
constructor extraction, callbacks, nonzero offsets and nearest local type scope.
`TestBracePilot_JavaAndD` adds the cross-language boundary cases. All pass.
After adding Java support to the evaluator, its unchanged Scala profile was
also rechecked: scopt 380/380 and os-lib 643/643, with no discrepancies.

Score the resulting CLI with `csharp_roslyn_eval.py` and `scala_d_tree_eval.py`
(`--language java` or `d`), as documented in `README.md`. Generated artifacts
stay under ignored `build/brace-pilot-20261016/`: `build.json`, overlay source,
`*.tests.txt`, `*.vet.txt` and the five `*.pilot.json` reference reports.

Measured candidate SHA-256:
`d491b77bb0fd9e05dfe3bae7c27b24c0d38412220ed2e01e7529c0047dd92759`.
Walker source SHA-256:
`03bcb70cc48b5a98d568cf9c23dc4d20e17ca904ad8c9d4d2b045ced70a25f19`.
Adapter source SHA-256:
`c88f1b7d0f27a29bc9738e8db32910387ddb9c30e673b2ab81af04e3912476e5`.

## D performance — 2026-10-16

The complete production D parser was compared with this shared pilot on the
same verified binaries and pins. All 125 arsd D files (287204 lines, 9526724
bytes) and all 35 D-YAML files (14147 lines, 495062 bytes) are timed, including
the six reference-grammar exclusions. Machine: Ryzen 9 5950X, Windows amd64.

Medians of ten serial in-process samples per variant, five per order, one-second
target, `GOMAXPROCS=1`. Lines are preloaded; finder construction, D preparation,
class/function discovery and allocation costs are timed. This compares complete
implementations, including adapters, rather than recursive calls in isolation.

| Metric | Production | Shared recursive pilot | Pilot / production |
|---|---:|---:|---:|
| arsd parser, ms | 2238.230 | 4888.042 | 2.18x |
| D-YAML parser, ms | 267.529 | 279.766 | 1.05x |
| arsd allocated bytes, decimal MB | 231.212 | 1434.969 | 6.21x |
| D-YAML allocated bytes, decimal MB | 12.306 | 23.220 | 1.89x |
| arsd complete CLI, ms | 441.851 | 1957.523 | 4.43x |
| D-YAML complete CLI, ms | 113.920 | 110.128 | 0.97x |

Allocated bytes are total allocations, not peak RAM. CLI commands use
`--dir <original-clone> --all --json --no-gitignore`, default workers, warm file
caches, two warmups and twenty paired samples per variant. Variant order is
balanced and shuffled. Timings include startup, walking, reads, type discovery,
JSON generation and captured output. Arsd also has one unchanged PHP source.

Arsd CLI p10–p90: production 415.618–501.144 ms, pilot 1830.093–2047.636 ms.
D-YAML CLI ranges overlap: 109.789–118.899 ms versus 106.604–117.014 ms; its
end-to-end difference is small. Serial D-YAML ranges also overlap.

The large arsd regression prevents promotion of the current D adapter. A
separate single-operation profile identifies declaration handling as a main
cost: `braceTokenAdapter.functionName` takes 51.3% cumulative CPU samples,
`header` 11.5%, and shared D preparation 27.4% (nested percentages must not be
added). The recursive walker itself accounts for 0.38% flat CPU samples.
Allocation-profile leaders are `normalizeDPrefix` (31.7%, including preparation
and adapter calls), string-builder writes (25.5%), tokenization (15.3%) and
string replacement (12.0%). This points to repeated header reconstruction,
normalization and regex matching in the adapter as optimization targets.
Profiling is separate from the repeated timing samples.

Both CLI outputs are deterministic within their own variants. Function counts
agree (12858 arsd including PHP; 541 D-YAML). D-YAML function entries agree
after sorting; arsd has three header-start differences of one line. Type maps
also differ: arsd 4451 versus 2857 entries, D-YAML 90 versus 74. Those entries
have no independent type oracle in this experiment; named-function coverage
above must not be interpreted as whole-map equivalence. Type-map parity is a
separate prerequisite for promotion.

Reproduce with `benchmarks/d_traversal_compare.py` as documented in `README.md`.
Raw samples, counts, hashes and maps are in ignored
`build/d-traversal-20261016/comparison.json`, `in-process.txt` and `*.map.json`.
Separate profiles are `recursive.cpu.pprof` and `recursive.mem.pprof`, with
readable `cpu-top.txt`, `cpu-cumulative.txt` and `alloc-top.txt` summaries.

### Repeat after the user stopped background processes

The same binaries, pins and settings were remeasured without parser changes.
Ten serial parser samples and twenty paired CLI samples per variant, with
identical warmups/order policy. Raw results are separately preserved in
`build/d-traversal-20261016-quiet/`.

| Metric | Production | Shared recursive pilot | Pilot / production |
|---|---:|---:|---:|
| arsd parser, ms | 2234.783 | 4808.390 | 2.15x |
| D-YAML parser, ms | 263.803 | 233.988 | 0.89x |
| arsd complete CLI, ms | 396.890 | 1809.914 | 4.56x |
| D-YAML complete CLI, ms | 89.293 | 89.196 | 1.00x |

Arsd's regression persists after removing background work. Small-project timings
improve: D-YAML's pilot parser is now 11.3% faster in process, while its CLI is
effectively equal. Allocation totals remain essentially unchanged: arsd 231.212
versus 1434.967 decimal MB, D-YAML 12.306 versus 23.220 MB.

The serial benchmark deliberately sets `-cpu=1` (`GOMAXPROCS=1`) and iterates
files sequentially: one busy logical CPU is about 3.125% of this 32-logical-CPU
machine. Directory CLI jobs already process files concurrently using
`runtime.NumCPU()` workers by default (`NewDirProcessor` / `processFilesParallel`).
The serial pilot visits scopes within each file sequentially. An optional
bounded parallel scheduler with independent buffers and deterministic merging
was measured separately: it gives no useful gain on the two D corpora with
normal file parallelism. See [BRACE_PARALLEL.md](BRACE_PARALLEL.md) for the
worker matrix, CPU utilization and parity checks. The measured D adapter costs
remain the main optimization target.

The earlier standalone C# timing/size comparison is preserved separately; its
timings do not establish shared-pilot performance for other languages. The
pilot binary retains old fallback code, so its size is not a replacement-size
measurement. Production dispatch remains unchanged.

DOX: internal and benchmarks contracts record the pilot scope and verification.
Root and other child contracts remain unchanged because the hierarchy, public
API and default CLI behavior are unchanged. The release changelog is unchanged
because no production parser behavior is switched by this experiment.

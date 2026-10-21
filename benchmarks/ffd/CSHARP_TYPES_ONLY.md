# C# types-only step in D — 2026-10-21

Directory C# `--struct` now calls `findCSharp(facts, includeFunctions = false)`. The optional argument defaults to true.
The existing cleaner/tokenizer, function recognition, body-end checks and block/scope traversal remain: method-body
markers affect subsequent boundaries and type visibility. The optimized request omits function bounds/signature
copies, the conditional-range pass and expression-alternative end extension used only for function output.
File field/ownership consumers and function/all modes retain full behavior. No public flag or second parser is added.

## Measurements

Intel i3-12100, Windows, LDC 1.43.0 / LLVM 22.1.8, `-O3 -release -mcpu=x86-64`. Warm filesystem cache, identical
absolute paths, warmups, seven shuffled paired full-CLI samples per case, `--dir --no-gitignore --struct --json`.
Median milliseconds:

| Corpus | Workers | Before / after | Repeat before / after |
|---|---:|---:|---:|
| Polly | 1 | 132.02 / 126.52 | 135.22 / 128.48 |
| Polly | 8 | 69.90 / 69.01 | 69.68 / 67.96 |
| Dapper | 1 | 46.26 / 45.47 | 46.34 / 45.79 |
| Dapper | 8 | 32.06 / 31.91 | 33.84 / 32.23 |

Both sets favor the candidate, but the small CLI effect varies; do not promise a fixed percentage. Function/all mode
medians show no slowdown above the plan's 3% investigation budget. The serial parser preloads source and warms
configurations/buffers; seven shuffled processes each measure five passes, followed by a seven-pair repeat.

| Corpus | Parser before / after, ms | Repeat before / after, ms | Allocated bytes before / after per pass |
|---|---:|---:|---:|
| Polly | 41.725 / 39.451 | 41.312 / 39.509 | 25,668,326 / 23,954,400 |
| Dapper | 12.738 / 12.167 | 12.685 / 12.166 | 6,913,628 / 6,451,088 |

Parser improvement is about 4–6%; warmed thread allocations fall about 6.7%. Allocation counters are read outside
the stopwatch and measure cumulative allocated bytes, not live/peak memory. `FFDReferenceScan` forces full work
from the same current finder for the parser reference. Timed release CLI builds contain no audit/allocation diagnostics;
Roslyn, compilation and correctness checks run outside timing.

## Verification and independent accuracy

- All 2,504 existing CLI checks and 30 module unit suites pass, including a new C# full/type-only nested/local/
  conditional-expression unit. `csharp_types_only_checks.py` adds 47 checks: frozen output, independent Roslyn type
  names/starts/ends, complete audits, all modes/JSON/names/trees, file fields/extraction/docs, records, operators,
  local functions, verbatim strings, attributes, conditional branches, CRLF/Unicode/invalid UTF-8, long/deep/malformed
  input. A conditional type fixture equals the union of Roslyn's FEATURE-defined and undefined configurations.
- Complete before/after directory output matches in three modes at one/eight workers on both corpora. Assertions
  builds enable bounds checks, `AuditBraceCleaner`, `AuditBraceTokens` and `FFDTypeAudit`; every file's core type
  names/bounds/kinds/order match the full route before annotation. The API probe consumes results before buffer reuse.
- Independent reference: local .NET SDK 10.0.100, Roslyn 5.0.0.0, Preview syntax, `FileBasedProgram=true`, empty defines
  for corpus scoring. This is syntax parsing, not dependency/semantic compilation. SDK files are under ignored
  `build/csharp-types-only/dotnet/`, without system installation; archive/compiler/source hashes are retained.
- `csharp_oracle.cs` now adds `classes` for class/struct/interface/enum/record declarations (including bodyless ones),
  with attribute/modifier starts, closing-brace ends for body types, and syntax ends for bodyless declarations.
  Delegates/using aliases remain outside this nominal-type profile. Existing function fields retain their meaning;
  additive `header_start` excludes leading attributes to describe the finder's established function-header convention.

| Corpus | Accepted / excluded files | Active named types matched / truth | Missing types | Type start / end errors | Active named functions matched / truth |
|---|---:|---:|---:|---:|---:|
| Polly | 801 / 0 | 933 / 950 | 17 | 2 / 0 | 5,116 / 5,116 |
| Dapper | 157 / 0 | 424 / 427 | 3 | 0 / 0 | 1,592 / 1,592 |

These counts are identical for baseline and candidate. All 20 missing types are declarations without brace bodies;
the old finder recognizes types at `{` only. Polly's two starts include attributes from inactive conditional branches
before `BulkheadRejectedException`/`RateLimitRejectedException`. Raw discovery additionally returns 3/45 inactive
type rows and 20/113 inactive function rows: report those as preprocessor analysis-scope differences, not active false
entries. No active false function/type rows or wrong ends were found in this profile.

The 2,909/552 raw function-start differences from full Roslyn spans are explained by leading attributes. After the
explicit `header_start` normalization all 6,708 functions have correct names/starts/ends. Preserve full spans as well;
do not silently relabel the header convention as complete declaration-span equality. Implicit primary constructors,
accessors and lambdas remain outside named-function truth. Bodyless-type/start fixes should be separate accuracy work.

## Evidence and reproduction

Base revision: `690f3af`, with recorded finder/routing/oracle overlays.
Polly pin: `3f3adad9d5af85658fc3585018f02a2d8c2173e8`;
Dapper pin: `eb47546a408bbf6c178bda4f04dc205bc51ffbfe`.
Baseline executable SHA-256: `b1a74dc9747467f1880b2a874f9344fc5fa060fec0dc0f752939b21fed551f27`.
Candidate SHA-256: `a04cfa0ca00da30278b158944ba09e42a24f1221f3280cf26f554dc428457f03`.

Ignored `build/csharp-types-only/` retains frozen binaries/sources, manifests, all maps/findings, Roslyn output,
SDK/archive/oracle hashes, build scripts, `verify.py`, parser/API probes, regression logs and all samples. Primary timing
has 168 CLI/28 parser samples; repeat has 56 CLI/28 parser samples. Input/binary hashes are checked after verification
and timing. Temporary Polly/Dapper clones are removed; the local SDK/cache remain available for further reference checks.

```powershell
python benchmarks/ffd/build.py --test --out build/csharp-types-only/audit --flag=-boundscheck=on --flag=-d-version=AuditBraceCleaner --flag=-d-version=AuditBraceTokens --flag=-d-version=FFDTypeAudit
python benchmarks/ffd/csharp_types_only_checks.py --before build/csharp-types-only/before/ffd.exe --after build/csharp-types-only/after/ffd.exe --audit build/csharp-types-only/audit/ffd-test.exe --dotnet build/csharp-types-only/dotnet/dotnet.exe --oracle build/csharp-types-only/oracle/bin/Release/net10.0/Oracle.dll
```

The first native D/C++/Go/Java/C# group is complete. Prioritize remaining-language work by profiles; keep the new
accuracy findings separate from optimization. Go C++ token reuse and remaining allocation sources are still planned.

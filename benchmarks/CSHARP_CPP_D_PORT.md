# C#/C++ finder API port to D

The later shared Go/Java extension is recorded in `GO_JAVA_D_PORT.md`. Current
workspace sources build four languages; all measurements below belong to their
preserved source snapshots and binaries. The extension rechecks these C#/C++ maps.

## Scope

The six `brace_port_*.d`/`csharp_port.d`/`cpp_port.d` modules implement a separate
serial C#/C++ finder probe. Production tools remain Go. Reference revision is
Go 1.15.9, `bdbc81bdc9be260782163d62c49e8b6135a0923d`, archived under
`build/cs-cpp-d-port-20261018/reference-src/`; D and harness sources are retained
in `sources/` and compiled from that snapshot.

- `brace_port_cleaner.d` shares one ASCII/rune state machine for comments,
  strings, character literals and C# verbatim strings. It preserves Go's rules,
  including nested comment opening on a single line, first-closing continuation
  comments, multiline regular strings and raw-mode behavior. Results own their
  bytes; malformed UTF-8 becomes one replacement character per invalid byte.
- `csharp_port.d` retains the production token scan, attribute/type checks,
  declaration boundaries, constructors/destructors/operators, local functions,
  property/block distinctions and conditional expression alternatives.
- `cpp_port.d` retains the production simple finder, header/declaration/
  constructor patterns, conversion-operator exclusions, pending signatures,
  braced initializer lists, function ends and class scopes.
- `brace_port_model.d` owns the supported configuration, raw class discovery,
  six-field function/three-field class results and JSON encoding. RE2's ASCII
  Perl classes and word boundaries and absolute end anchors are translated for
  D `std.regex`; original pattern strings remain in the configuration artifact.
- `brace_port_unicode.d` pins Go Unicode 15.0.0 Letter/Digit ranges, including
  the regex identifier classes. The newer D library categories otherwise accept
  characters Go does not. A different reference Unicode version is rejected.
  The bitmap, Go/D classifier probes, generator and provenance hashes are kept
  in the work directory; there are 659 letter and 64 digit intervals.
- `brace_port_probe.d` preloads sources, discards one warmup pass for parser
  timing, retains results and validates checksums. Whole-process runs include
  startup/read/parse/JSON. JSONL readers split only at LF, preserving Unicode
  line separators inside string values.

These probes do not implement complete CLI directory selection, file workers,
offsets/extraction/selection modes, structural fields/aliases or the remaining
toolkit analyses. Go retains generic language machinery; D is specialized for
C#/C++. Binary-size differences include this specialization. C++ syntax and
raw literal limits remain those of the frozen Go implementation.

## Quality and inputs

All **980 files, 8,978 raw function entries and 1,797 raw class entries** have
exact ordered Go/D parity, including class scopes, signature metadata and
normal/raw cleanup facts (text, start state, indentation and code presence).
Unscored and inactive sources are included in this API comparison. Type and
signature parity is not an independent type-boundary oracle.

| Corpus | Tracked/scored files | Truth | Missing | False | Wrong end |
|---|---:|---:|---:|---:|---:|
| Polly C# | 801/801 | 5116 | 0 | 0 | 0 |
| Dapper C# | 157/157 | 1592 | 0 | 0 | 0 |
| fmt C++ (`include/`, `src/`) | 20/18 | 1450 | 36 | 7 | 0 |
| tinyxml2 C++ (two core files) | 2/2 | 627 | 1 | 0 | 0 |

Pins (clean clones): Polly `ea41f708cfc33ba5ae01738ee8c5a1cf211fb9e9`,
Dapper `eb47546a408bbf6c178bda4f04dc205bc51ffbfe`,
fmt `b4ffa39b9994c961418406f47358791d89a5aa48`,
tinyxml2 `8224e427b655b83dae5e2298f1e6919523a78737`.

C# uses SDK 9.0.308 / Roslyn 4.14.0.0, Preview, `FileBasedProgram=true` and
empty preprocessor symbols. Polly/Dapper retain 20/113 candidates in disabled
text, separately excluded from scoring. C++ uses libclang 18.1.1 and MSYS2 GCC
15.2.0 Rev8, C++17, `x86_64-w64-mingw32`, no defines, GCC's C++/target/backward/
builtin/fixed/sysroot include directories and fmt's `include/`. This reproduces
the historical counts with a different GCC distribution; exact compiler hash
and include arguments are retained. `fmt.cc` and `os.cc` remain explicit parse
exclusions. There are 54 fmt and two tinyxml2 inactive candidates. Known C++
misses/false entries are inherited from Go; this port does not fix them.

Separate `-O2 -boundscheck=on -d-version=AuditBraceCleaner` builds agree on every
corpus map and cleanup fact. Checks include 130 C# and 131 C++ fixture/stress
files, malformed input, Unicode/ASCII coordinates, verbatim strings, comments,
operators, local functions, initializers, 128 KiB lines and 64 nested blocks.
The configured language patterns' complete capture groups match Go on 20,000 seeded
vectors: two C# and seven C++ patterns, **180,000 comparisons** in total.
All **1,114,112 code points** have identical Letter/Digit/Space classifications.
Unsupported lexical configurations are rejected.

## Performance

Seven shuffled paired rounds with three preloaded parser passes per sample,
one discarded warmup and retained-result checksum/output validation. Medians
in milliseconds, on the same 32-logical-CPU Windows host as the JS/TS work:

| Corpus | Go parser | D parser | Go full process | D full process |
|---|---:|---:|---:|---:|
| Polly C# | 115.694 | 68.649 | 161.965 | 119.812 |
| Dapper C# | 36.484 | 18.264 | 54.371 | 37.674 |
| fmt C++ | 35.034 | 90.810 | 45.937 | 127.362 |
| tinyxml2 C++ | 10.758 | 28.088 | 19.180 | 62.614 |

C# parser time is 40.7%/49.9% lower in D (1.69x/2.00x throughput). The direct
C++ port takes 2.59x/2.61x as much parser time as Go. A port does not by itself
ensure a speedup; C++ recognition still uses runtime `std.regex`, including
Go-compatible classes and boundaries. Profiling those patterns, then comparing
a specialized header scanner or compiled patterns, is the next experiment;
the current measurements do not isolate individual regex operations.

The combined C#/C++ D probe is **1,232,896 bytes** versus Go **2,807,296 bytes**.
This is 56.1% smaller, with the specialization limits described above. Full
process timings include pattern/config initialization, input and JSON output;
parser timings exclude them. Neither measurement includes production file
parallelism. Builds/oracles/checks ran between stages, never concurrently with
timed children. Full ranges and samples are in `report.json`; `postflight.json`
verifies source snapshots, binaries, corpus bytes, Unicode provenance and
unchanged independent counts against `correctness-report.json`.

## C++ tokenization follow-up (2026-10-18)

The normal C++ probe now uses `cpp_tokenizer.d` and `cpp_tokens_port.d`.
It tokenizes cleaned UTF-8 once, maps byte coordinates back to raw source,
pairs parentheses/brackets/braces and identifies declarations, classes/enums/
unions and bodies from tokens. It skips continued preprocessor directives,
explicit instantiations and balanced parameter/default-value expressions.
Constructors use class/qualified names and skip braced initializers; operators
include `operator()`, `operator[]`, builtin conversions and user-defined literals.
`final` does not replace a class name. Normal parsing supplies lexical settings
with an empty regex map; `--headers` separately enables the old pattern diagnostic.
Custom regex recognition overrides are outside the token backend's scope.

The initial regex D binary/report/snapshots above remain preserved under
`build/cs-cpp-d-port-20261018/`; `cpp_port.d` retains that experiment. Current
artifacts use `build/cpp-tokens-20261018/`. Source hashes prove that cleanup,
the pinned Unicode table and the C# finder did not change. C# still exactly
matches Go; both normal/raw cleanup modes match Go on all 980 inputs.

| Corpus | Go missing/false/wrong-end | Token D missing/false/wrong-end |
|---|---:|---:|
| fmt, 1450 truth | 36 / 7 / 0 | **22 / 0 / 0** |
| tinyxml2, 627 truth | 1 / 0 / 0 | **0 / 0 / 0** |

No previously correct named function was lost: the harness compares individual
missing/wrong-end truth entries, not only total counts. The same libclang/GCC
profile and two fmt exclusions apply. Remaining fmt misses are 12 conversions
requiring alias/dependent type resolution and 10 macro-expanded methods. The
token backend retains the old exclusion for conversions it cannot name without
semantic analysis; builtin `unsigned`/`signed` spellings are normalized.

Token function matching uses the recorded function-name line within the oracle
header range and exact end. Raw declaration starts are tested independently;
they may include an outer class-template clause before libclang's function extent.
The Go reference uses its original start-line anchor (its signature field is
zero), retaining the previous 36/7/0 and 1/0/0 counts. Token type/scoping output
and signature metadata intentionally differ from the old line-pattern maps;
they are not independently scored as class-boundary accuracy.

Separate unit tests cover multiline templates, constructors, defaults/nested
types, directives, operators/literals, Unicode raw offsets, final classes and
numeric variable initialization. Bounds/cleaner/token audits compare every
current map to its release variant, validate pair symmetry/raw slices and assert
that the normal token finder has no compiled regex patterns. Existing stress,
malformed UTF-8 and diagnostic-pattern comparisons also pass; diagnostic regex
checks are not evidence that the token scanner calls those patterns.

Use the four projects below with `--work-dir build/cpp-tokens-20261018` and
`--baseline-report build/cs-cpp-d-port-20261018/report.json`. The harness verifies
reference/probe/config/binary/input identity and includes all three binaries
in the same shuffled rounds. Each variant retains its own output/checksum.
Run the separate `-d-version=AuditCppTokens` audit and `compile-unit.bat` tests
before timing; unit-test executables run tests only unless explicitly given
`--DRT-testmode=run-main`. `postflight.json` records final hashes and input/quality
stability. This remains a serial API experiment, without full CLI/file workers.

Seven paired shuffled rounds, three preloaded passes per parser sample and a
discarded warmup, give these medians (milliseconds):

| Corpus | Go parser | Regex D parser | Token D parser | Go process | Regex D process | Token D process |
|---|---:|---:|---:|---:|---:|---:|
| fmt | 35.362 | 89.927 | **12.184** | 46.613 | 129.043 | **24.070** |
| tinyxml2 | 11.067 | 28.524 | **2.632** | 18.891 | 66.138 | **12.219** |

Token parser throughput is **2.90x/4.20x Go** and **7.38x/10.84x previous D**.
Complete-process speedups over Go are 1.94x/1.55x. C# parser medians in the same
rounds are Go/regex-D/current-D 115.289/67.040/67.268 ms (Polly) and
37.025/18.024/18.142 ms (Dapper); the C# finder source/output is unchanged.

The current combined probe is **1,240,064 bytes**, +7,168 bytes versus regex D;
Go is 2,807,296 bytes. The specialized/generic comparison limit still applies.
Current raw maps contain 8,991 functions and 1,803 types across 980 files;
counts include inactive/unscored input and are distinct from independent truth.
All timing checksums use each variant's own maps. Final postflight verifies
source snapshots, all three binaries, corpus bytes, per-function quality and
the baseline-directory guard. No checks/builds/profiles ran concurrently with
timed children. The historical scalar-classifier audit is reused through
unchanged Unicode/cleaner/C# source hashes; it was not silently rerun against
different tables.

## Reproduction of the current token stage

Use Python 3.14.6, Go 1.25.7 (`GOAMD64=v1`), LDC 1.43.0 / LLVM 22.1.8 and
`F:\D\ldcvars64.bat` for the installed MSVC x64 linker. Install libclang 18.1.1
under `build/cpp-eval-runtime/`. The C# evaluator builds its oracle using the
installed SDK assemblies without project dependencies.

```powershell
python benchmarks/brace_port_compare.py --help
python benchmarks/brace_port_checks.py --help
python benchmarks/brace_port_compare.py `
  --revision bdbc81b --work-dir build/cpp-tokens-20261018 `
  --baseline-report build/cs-cpp-d-port-20261018/report.json `
  --project cs=build/csharp-eval-20261016/repos/polly `
  --project cs=build/csharp-eval-20261016/repos/dapper `
  --project cpp=build/cs-cpp-d-port-20261018/repos/fmt `
  --project cpp=build/cs-cpp-d-port-20261018/repos/tinyxml2 `
  --samples 7 --repeats 3
```

`--samples 0` runs parity/oracles without timing; `--skip-oracle` is for explicit
parity-only diagnostics. `--audit --samples 0 --skip-oracle` with the same
projects builds `d-audit-probe.exe` and writes a separate `audit-report.json`.
Alternatively run the retained `compile-audit.bat`. Its current flags include
`-d-version=AuditCppTokens`. Run
`python benchmarks/brace_port_checks.py --work-dir build/cpp-tokens-20261018`
after the release/audit builds, separately from timed measurements.

The current work directory retains `report.json`, `correctness-report.json`, complete
maps, independent truth/inputs, fixture/header vectors, source/config/compiler/
binary hashes and build scripts. Initial regex binaries can be rebuilt using
their preserved `build/cs-cpp-d-port-20261018/compile.bat` and snapshot sources;
do not rebuild them through the changed workspace harness. Unicode rechecks
remain in that initial work directory and use `classifiers.go`,
`classifiers.d`, `compile-classifiers.bat` and `classifiers.bin`; regenerating
the pinned ranges uses `generate_unicode.py`. Keep those diagnostics out of
timed rounds. Recheck hashes, source snapshots, corpus bytes and independent
counts after measuring.

The owning benchmark DOX and README are updated. Root and other child
contracts/indexes are unchanged: this experiment adds no production ownership
boundary or CLI contract.

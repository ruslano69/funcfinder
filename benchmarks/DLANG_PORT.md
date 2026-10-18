# D implementation pilot

## Scope and toolchain

`dlang_port.d` ports the production D preparation, comment/literal lexer,
declaration-prefix normalization, contracts, multiline headers, nested function
discovery, constructor-call filtering and nearest class scopes to D. It exposes
a small serial finder API probe reading an explicit JSON file manifest.
It is experimental and does not participate in production dispatch or releases.

LDC 1.43.0 (DMD frontend 2.113.0, LLVM 22.1.8) is installed in
`F:\D\ldc-1.43.0` alongside the existing DMD 2.085.1. The official x64 archive
SHA-256 is `60ae3d5e34287aa25433c7550520060a52f9f212e3d5ff6472a9067d8d17e47e`.
`F:\D\ldcvars64.bat` initializes Visual Studio 2022 x64 and adds LDC to that
shell's PATH. Without the Visual Studio environment, this installation's linker
discovery crashes with `0xC000001D`; compilation and execution work after
initialization. Both ordinary D and BetterC smoke programs were built and run.
The parser pilot uses ordinary D with its runtime and GC; BetterC was not ported.

Two D variants share the same algorithm: runtime `std.regex` and `ctRegex`,
which generates the pattern matcher at compile time. Both use
`-O3 -release -mcpu=x86-64`; the latter adds `-d-version=CompiledPatterns`.
The Go API probe uses Go 1.25.7, `GOAMD64=v1`, `-ldflags=-s -w`, and a frozen
source snapshot of `v1.15.3`, commit `3fecced988380372181f306a4f85ffc627dd0373`.
Concurrent edits in the active checkout do not enter the Go reference build.
The generated probe and pattern module are retained under the work directory.

Both probes return ordered arrays of function name/start/end/class scope and
raw class name/start/end, retaining duplicate names. This does not port CLI
flags, directory discovery, file parallelism, offsets/selection/extraction,
structural fields, aliases, dependency analysis, complexity or call graphs.
The Go probe retains generic finder machinery that can support other languages;
the D probe implements only D. Their size difference includes specialization,
and cannot establish the size reduction of an equivalent 15-language toolkit.

## Quality

Pins are unchanged from the existing D experiments:

- D-YAML: `09aeac155c9d0f85a0e7442dffb442de53528a41`, 35 tracked D files,
  495,062 bytes, 541 functions and 90 raw class entries from the Go API.
- arsd: `953107183178fccbe8af7340b230c37c2071aef8`, 125 tracked D files,
  9,526,724 bytes, 12,829 functions and 4,337 raw class entries from the Go API.

Both D variants exactly match the ordered Go results on all 160 files, including
the six files outside the independent grammar's accepted subset. They also pass
the independent `scala_d_tree_eval.symbols` profile with tree-sitter 0.26.0 and
tree-sitter-language-pack 1.22.0: 539/539 D-YAML and 11,705/11,705 arsd functions,
zero missing/false functions and wrong end lines. As before, 1/35 D-YAML and
5/125 arsd files are excluded for grammar parse errors; 20 arsd grammar candidates
are explicitly rejected by the existing audit rules. Full-result parity on
excluded files is not independent evidence of accuracy. Exact class boundaries
are compared with Go, not scored by a separate class oracle. Results describe
these pins and this named-function profile, not arbitrary D syntax.

## Measurement contract

The machine runs Windows 10 x64, Ryzen 9 5950X, 32 logical CPUs. Python 3.14.6
orchestrates 10 rounds, shuffling the three variants with seed 20261018.
Parser timings preload files, initialize patterns and warm once outside the
timer; each sample averages two complete serial corpus passes. Runtime
allocations/GC and result collection remain inside the timer. Probe process
timings include startup, config/pattern loading, file reading, a single serial
parse and JSON output. Two process warmups precede measured samples. These are
one-file-at-a-time probes, not the shipping CLI's parallel directory benchmark.
All 160 files, including reference exclusions, contribute to timing. Every
process result is checked against the saved full map; parser timing checks
result counts. The runtime defaults for GC threads remain in effect.

Initial reports, samples, maps, corpus manifests, binary/source hashes, grammar
DLL hash, compiler flags and reference snapshot are retained in
`build/dlang-port-20261018-final/`. The measured D source is preserved as
`build/dlang-port-20261018-optimized/dlang_port.before.d`; current source includes
the follow-up optimizations below. `README.md` gives the current comparison command.

## Initial binary size

| Probe | Bytes | Decimal MB |
|---|---:|---:|
| Go production finder API | 2,737,152 | 2.737 |
| D, runtime regex | 1,205,760 | 1.206 |
| D, ctRegex | 1,561,600 | 1.562 |

The runtime-regex D probe is 55.9% smaller than the Go probe; ctRegex is 42.9%
smaller. Generated matchers cost about 356 KB. These are unpacked Windows
executables, not archive sizes or the total installed toolchain. BetterC smoke
program sizes are not evidence about a BetterC port of this parser.

## Initial timings and interpretation

Medians in milliseconds (10 samples per cell):

| Corpus / operation | Go | D runtime regex | D ctRegex |
|---|---:|---:|---:|
| D-YAML, serial parser | 168.246 | 260.410 | 207.502 |
| D-YAML, complete probe process | 179.597 | 297.015 | 231.574 |
| arsd, serial parser | 1285.069 | 2982.511 | 2013.926 |
| arsd, complete probe process | 1335.386 | 3032.865 | 2061.610 |

Parser sample ranges: D-YAML 166.294–171.128 / 255.692–273.390 /
205.602–210.840 ms; arsd 1265.319–1325.330 / 2943.174–3078.888 /
1990.987–2034.016 ms (Go / runtime D / ctRegex D). Thus the ctRegex port takes
1.23x Go's parser time on D-YAML and 1.57x on arsd. Runtime regex takes 1.55x
and 2.32x respectively. The gaps exceed the observed sample ranges; these are
results for this implementation and machine, not language-wide speed rankings.

Porting the D finder is feasible without losing the measured quality. A direct
language rewrite does not ensure faster parsing. `ctRegex` improves the D
implementation but costs binary space. This pilot provides a smaller binary,
with slower parsing; it does not justify replacing production Go for speed.

Before considering broader migration, profile D preparation, string/rune
conversions, allocations and matcher execution. Compare any optimized D variant
with an equivalent Go optimization under the same quality gates. A fair whole
tool comparison also needs a D-only Go specialization, the missing CLI modes,
equivalent file-worker scheduling, and the same deployment dependencies.
Only then can results support a decision about porting the full toolkit.

## Go 1.15.5 optimizations in D

The follow-up ports `b5a450ab41567037a9a486f61bdd2baf0250ba9c` to D:

- Preparation returns a per-line unknown/match/no-match hint. The main scan
  skips the function pattern on known negative lines; positive lines still
  run it to capture the name. A hint is retained only when final prepared text
  equals the exact string that was checked. Contract masking can invalidate it.
- Reuse each line's original UTF-8 string; convert its rune buffer back only
  after a mutation, rather than for each attribute/header check and final write.
- Attribute matching first checks the initial non-whitespace character against
  `@pencisr`, also in prefix normalization and multiline-header preparation.

The Go reference is now the frozen `4ef44059a45aec74502dcf908bb5ece31314767b`
release revision (VERSION 1.15.5). Changes to directory scheduling or gitignore
handling do not participate in this serial, explicit-manifest probe.
`--baseline-report` retains the verified original Go/D executables as three
additional variants. All six variants run in the same shuffled rounds with
10 samples and one timed corpus pass after the parser warmup. Thus these
before/after results are contemporaneous; they are not a comparison between
medians from separate sessions. Both corpora retain the same ordered maps and
independent quality as the initial pilot.

`AuditHints`, compiled separately with `-O2 -boundscheck=on` and assertions
enabled, compares every known hint with the function pattern and the attribute
gate with its ungated pattern on all 160 files. It also reproduces the Go maps,
including the synthetic and repository fixtures. Audit code is absent from
timed builds. The source snapshot, original D source, maps, logs and complete
samples are retained in `build/dlang-port-20261018-optimized/`.

Contemporaneous medians (milliseconds, old -> optimized):

| Corpus / variant | Serial parser | Complete probe process |
|---|---:|---:|
| D-YAML, Go | 171.265 -> 114.027 | 180.839 -> 123.606 |
| D-YAML, D runtime regex | 260.307 -> 185.133 | 294.993 -> 219.539 |
| D-YAML, D ctRegex | 207.342 -> 144.242 | 230.872 -> 165.151 |
| arsd, Go | 1272.144 -> 824.552 | 1329.284 -> 870.708 |
| arsd, D runtime regex | 2959.919 -> 2213.134 | 3036.705 -> 2287.609 |
| arsd, D ctRegex | 2006.095 -> 1385.388 | 2052.075 -> 1433.379 |

D ctRegex reduces parser time by 30.43%/30.94% (D-YAML/arsd); runtime regex by
28.88%/25.23%. Go improves by 33.42%/35.18%, so optimized D ctRegex still takes
1.27x/1.68x optimized Go's time. Current parser sample ranges (Go / runtime D /
ctRegex D) are 110.000–116.120 / 183.148–191.672 / 142.489–144.863 ms for
D-YAML and 817.574–837.987 / 2204.817–2223.357 / 1373.876–1435.567 ms for arsd.
D binary sizes remain 1,205,760 / 1,561,600 bytes; the Go probe grows from
2,737,152 to 2,739,712 bytes. The same specialization limits still apply.

## Remaining speed reserves: measured phases and D guidance

A separate instrumented ctRegex build records six non-overlapping phases and
`GC.allocatedInCurrentThread()` deltas, plus `GC.profileStats()` with
`--DRT-gcopt=profile:1`. Three process runs per corpus reproduce the complete
Go maps. These diagnostic timings include instrumentation and GC profiling;
they are not additional leaderboard samples. Medians for arsd:

| Phase | Milliseconds | Cumulative allocated MB |
|---|---:|---:|
| Cleaning and prefix normalization | 439.324 | 378.807 |
| Multiline header joining | 224.895 | 8.765 |
| Contracts and function hints | 631.709 | 156.923 |
| Raw class discovery | 37.757 | 0.353 |
| Nested function scan | 70.035 | 33.495 |
| Nearest scopes / constructor-call filtering | 1.841 | 1.347 |

Total parser time is 1406.516 ms; cumulative managed allocations across phases
are about 579.690 MB for 9.527 MB of tracked D input. This is an allocation
counter, not peak/live memory, and excludes file reading and JSON output.
Fifteen GC cycles account for 11.030 ms: under 1% of parser time. About 92% of
time is preparation, versus 5% in the entire nesting scan. On D-YAML preparation
is about 95%: cleaning 23.022 ms, header joins 68.181 ms, contracts/hints
58.087 ms; total 157.806 ms, about 31.176 MB allocated and 1.331 ms in seven GC
cycles. Per-phase medians need not sum exactly to the total median.

The strongest next candidates, inferred from these measurements and code:

1. Avoid repeated UTF-8 -> UTF-32 -> UTF-8 materialization. `clean`, `normalize`
   and `prepare` still create rune arrays; `to!dstring(line).dup` materializes
   and then duplicates a buffer. Test a normal-state fast path for lines without
   literal/comment markers, byte scanning for ASCII syntax, and Unicode decoding
   only where identifiers/delimiters require it. Compare slices with fixed
   attribute/contract tokens without converting each token to a new string.
2. Reuse scratch buffers across lines, reserving capacity for the current file;
   use small local static buffers where lifetimes permit, with a growing buffer
   for longer input. A per-file arena is a later option, with retained names and
   output owned separately. This targets allocation/copy work, rather than just
   collection pauses.
3. Reduce matcher and materialization work in multiline-header joins and
   contract preparation. Build a candidate once or scan spans; preserve cheap
   gates and exact-text cache invalidation. The current phase profile does not
   isolate regex execution from conversion/scanning within a phase, so it does
   not assign a measured percentage to regex alone.
4. Reuse nesting arrays (`events`, `previousDepths`, current/next contexts) and
   reserve result capacity. This is a smaller target: the whole nesting scan
   accounts for about 5% of arsd's parser time. D's
   [Appender](https://dlang.org/library/std/array/appender.html) is explicitly
   recommended for frequent appends and keeps local capacity metadata.
5. Use `@nogc` on allocation-free inner helpers as a checked contract, and
   `scope`/`return scope` to express lifetimes. These do not automatically move
   arbitrary dynamic arrays to the stack or remove existing allocations; see
   [D function attributes](https://dlang.org/spec/function.html#nogc-functions).
   Raw pointers are an option only after an allocation/lifetime design and an
   assembly/profile comparison. Current `-release` already leaves bounds
   checks enabled only in `@safe` code; the pilot's own helpers default to
   `@system`, so changing indexes to pointers is not inherently a new win.

Go commit `08f3531ff54aa9d784cd6c1d998ff76e9ebe5e9f` subsequently added
plain-line lexer/prefix fast paths targeting this reserve. The next experiment
below ports these and implements reusable buffers. The six-variant results
above remain the frozen Go 1.15.5 comparison.

The installed LDC also exposes GC-to-stack optimization, LTO and PGO options
(`--disable-gc2stack`, `--flto`, `--fprofile-*`); none was newly benchmarked here.
Treat CPU-specific builds and PGO/LTO as separate experiments after fixing the
preparation path, retaining a portable build and representative training inputs.
This profile does not justify a speed estimate for those changes or a blanket
GC disable/BetterC rewrite. Collection time and allocation overhead are distinct;
see the official [GC allocation counter](https://dlang.org/library/core/memory/gc.allocated_in_current_thread.html),
[GC profile statistics](https://dlang.org/library/core/memory/gc.profile_stats.html)
and [memory management specification](https://dlang.org/spec/garbage.html).

`make_phase_profile.py`, `compile-profile.bat`, `dlang_port_profile.d`,
`run_phase_profile.py`, `profile-build.txt` (`-vgc` diagnostics), raw stdout/stderr
and `phase-profile.json` retain this diagnostic experiment under the optimized
work directory. The measured release source and binaries remain unchanged.

## Plain-line paths, scratch buffers and matcher gates

The current experiment freezes Go at
`08f3531ff54aa9d784cd6c1d998ff76e9ebe5e9f`. It ports the normal-state ASCII
lexer shortcut and prefix shortcut, retaining Unicode validation and the slow
paths. Further D changes address the measured allocation/copy work:

- Decode UTF-8 directly into growing per-file rune buffers. Reuse the lexer's
  input/output buffers and preparation scratch space; decode lazily in the
  contract pass. Converted UTF-8 results own copies, and retained multiline
  delimiters own their storage, so subsequent lines cannot overwrite them.
- Compare contract keywords as rune slices, and use a switch for ordinary
  prefix classification instead of constructing an array literal per line.
- Reuse event/depth arrays and two `Appender` context buffers across lines.
  Buffers grow for long lines; there is no fixed line-length or nesting cap.
- Before type/statement regexes, test mandatory type keywords or the possible
  initial statement characters. These gates avoid regex work on impossible
  candidates without changing the patterns or acceptance rules.

Both release builds retain full ordered Go parity on all 160 files and the
independent 12,244-function accepted profile. A separate build uses
`-O2 -boundscheck=on -d-version=AuditHints -d-version=AuditFastPaths`, with
assertions enabled. It checks every cached hint and all three matcher gates
against their bare patterns; fast lexer checks include the resulting lexer
state, and prefix checks compare the slow normalization result.

The checked build, runtime-regex build, ctRegex build, Go reference and previous
D build also agree on generated stress input: 9,133 lines, 1,066 functions,
250 Cyrillic names, nested/token/multiline strings and comments, a 128 KiB
literal line and 64 nested functions. Three smaller fixtures additionally cover
contracts, constructors/delegation and split signatures. The compiled-pattern
configuration mismatch is still rejected. These checks cover the exercised
inputs; they do not expand the independent grammar's six excluded files.

Current artifacts are in `build/dlang-port-20261018-buffers/`: the report and
raw maps/samples, frozen Go source, diagnostic sources/build scripts,
`verification.json`, `stress.py`, generated fixture and
`stress-verification.json`. Intermediate `fast-report.json` and
`buffer-report.json` retain verified builds before buffer reuse and before the
additional matcher gates, respectively.

Ten shuffled contemporaneous rounds compare the three new probes with the
primary binaries in the previous Go 1.15.5 report, using one timed corpus pass
after a parser warmup. Medians in milliseconds (previous -> current):

| Corpus / variant | Serial parser | Complete probe process |
|---|---:|---:|
| D-YAML, Go | 111.841 -> 106.000 | 122.444 -> 116.551 |
| D-YAML, D runtime regex | 185.578 -> 124.756 | 220.788 -> 179.733 |
| D-YAML, D ctRegex | 145.608 -> 103.986 | 168.109 -> 130.782 |
| arsd, Go | 827.228 -> 723.677 | 875.941 -> 773.386 |
| arsd, D runtime regex | 2222.547 -> 1151.857 | 2301.542 -> 1233.776 |
| arsd, D ctRegex | 1397.629 -> 726.832 | 1441.158 -> 776.825 |

D ctRegex parser time decreases by 28.59% on D-YAML and 48.00% on arsd
(1.40x/1.92x throughput). Runtime-regex D improves by 32.77%/48.17%; Go's
plain-line paths improve it by 5.22%/12.52%. The new ctRegex/Go parser ratios
are 0.981/1.004. Their ranges overlap: D-YAML Go 102.978–109.176 versus D
102.983–109.917 ms; arsd Go 715.859–829.700 versus D 722.580–779.877 ms.
Thus this experiment establishes approximately equal parser performance,
not a meaningful D lead. Complete arsd probe times are also close; Go retains
a process-time advantage on smaller D-YAML. Preserve the outliers in the report.

Current unpacked binaries are Go 2,741,760 bytes, runtime D 1,210,880 bytes
and ctRegex D 1,566,720 bytes. Each D build grows by 5,120 bytes; ctRegex is
42.86% smaller than this Go probe. The generic-Go/D-only specialization limits
still apply. This is a serial finder API result for two pinned projects, not
a speed/size estimate for a complete multi-language toolkit. Final source and
binary hashes, all sample counts, ordered maps and the checked build were
verified after timing (`summarize.py`, `verification.json`).

Separate three-run ctRegex phase diagnostics retain full-map parity. For arsd,
cumulative parser allocations fall from 579.690 to 42.526 MB (92.66% less);
D-YAML falls from 31.176 to 3.339 MB (89.29% less). These counters describe
allocation traffic, not peak RAM. The final diagnostic arsd phases are:

| Phase | Milliseconds | Cumulative allocated MB |
|---|---:|---:|
| Cleaning and prefix normalization | 129.537 | 26.454 |
| Multiline header joining | 196.253 | 8.757 |
| Contracts and function hints | 340.738 | 4.878 |
| Raw class discovery | 32.977 | 0.267 |
| Nested function scan | 38.499 | 0.947 |
| Nearest scopes / constructor-call filtering | 2.443 | 1.224 |

The diagnostic parser median is 740.600 ms, with 2.164 ms in four GC cycles.
D-YAML is 120.661 ms, with 0.852 ms in two GC cycles. This remains an
instrumented profile, separate from the repeated release measurements. The
same improvements were obtained with managed growing buffers and immutable
results; no raw-pointer arena or blanket GC disable was introduced. Header
joining and contract/hint preparation remain the main measured time targets.

## Header scanner instead of the function pattern

Go `perf/d-header-scanner` (PR #114, commit `660e102`) replaces the D
`func_pattern` regexp with `dHeaderSubmatch` (`internal/d_header.go`): a scanner
that returns exactly the pattern's match and captured name, in its
leftmost-first order. The D pilot now carries a direct translation
(`headerMatch` and helpers in `dlang_port.d`); `header()` and the scan's name
use it, and the function pattern is compiled only for audit builds.
`configure()` rejects a `func_pattern` other than the frozen
`headerPatternText`, in both D builds. The class, attribute, type, statement,
declaration and continuation patterns are unchanged.

Verification: a `-O2 -boundscheck=on -d-version=CompiledPatterns
-d-version=AuditScanner` build compares every scanner call with the bare
pattern (match and name) and threw nothing on all 160 tracked files. All three
new D builds reproduce the ordered Go maps on D-YAML (541 functions) and arsd
(12,829), and the function/class bounds of the three fixture files. On the Go
side, the scanner was compared with the pattern on 1.8 million real D-YAML/arsd
lines and 3 million generated header-shaped lines, with no difference.

Ten shuffled contemporaneous rounds, one warmup, two timed corpus passes per
sample; serial parser medians in milliseconds. The baselines are the final
binaries of `build/dlang-port-20261018-buffers/` (Go `08f3531`, i.e. 1.15.6).

| Variant | D-YAML | range | arsd | range |
|---|---:|---:|---:|---:|
| Go 1.15.6, regexp | 104.9 | 102.1–107.4 | 720.4 | 706.5–739.9 |
| Go + scanner | 35.0 | 34.7–35.8 | 376.6 | 372.3–385.7 |
| D ctRegex, buffers | 104.2 | 102.4–105.5 | 720.6 | 713.9–727.7 |
| D ctRegex + scanner | 27.2 | 27.0–27.5 | 389.9 | 388.3–393.4 |
| D runtime regex, buffers | 122.0 | 117.7–125.9 | 1139.0 | 1129.3–1163.7 |
| D runtime regex + scanner | 34.4 | 34.0–34.5 | 509.1 | 507.9–510.9 |

The scanner removes the largest shared cost from both implementations: Go
falls 3.0x/1.9x (D-YAML/arsd), D ctRegex 3.8x/1.85x. The new ctRegex/Go ratios
are 0.78/1.04 and the ranges do not overlap: D leads on D-YAML by about 22%,
Go leads on arsd by about 3.5%. The remaining runtime-vs-ctRegex gap in D
(509 vs 390 ms on arsd) is the cost of the patterns still compiled at runtime;
porting those to scanners is the remaining regex path. Binary sizes: ctRegex D
1,431,040 bytes (buffers build 1,566,720: the generated function matcher is
gone), runtime D 1,219,072, Go + scanner probe 2,756,608. The same serial,
D-only, two-project limits apply.

Artifacts are in `build/dlang-port-scanner/`: the pre-change source
(`dlang_port.before.d`), the generator for the D block (`gen_block.py`,
`expanded_pattern.txt`), `compile.bat`, the three D probes and the Go probe
built from the scanner branch.

## Post-scanner preparation predicates and header joins

This experiment freezes the production Go API at
`b744fa0130db40816e3528e1ca36ceb868d7481a` (D header scanner, plus the
subsequent TS changes). The unmodified D scanner source, fresh release binaries,
ordered maps, independent oracle report and three-run phase diagnostics are
retained in `build/dlang-port-20261018-postscanner-baseline/`.

The new profile, rather than the pre-scanner profile above, guided the change.
On arsd, the diagnostic parser median is 398.059 ms: contracts/hints 158.501 ms
(about 40%), cleaning/normalization 129.354 ms (32%), header joins 48.777 ms
(12%), raw classes 33.569 ms, nested scan 24.649 ms and scope/filtering 2.516 ms.
GC collection takes 2.275 ms in four cycles. D-YAML totals 38.484 ms. These
cold, instrumented process measurements include GC profiling and are separate
from warm release-parser medians.

The candidate makes two changes:

- Read ordinary ASCII attribute/type/statement prefixes directly, retaining
  the previous alternatives, whitespace rules, required separators and keyword
  boundaries. Unicode identifiers after `@` use the L/Nd sets already used by
  the header scanner. Rare non-ASCII whitespace or keyword boundaries fall back
  to the previous gated regexes; these patterns remain compiled for that path.
  `AuditPredicates` compares each call with its old predicate.
- A multiline-header candidate is materialized only once a `(` occurs. One
  `Appender!string` extends its immutable prefix across lookahead steps instead
  of joining every prefix from scratch. Successful output owns the builder's
  storage; that builder is never reused after retaining the output. The same
  maximum five lines, terminator stops and original header end coordinates
  remain in effect.

All 160 ordered function/class maps still agree with the frozen Go API; the
independent accepted profile remains 12,244 functions with zero missing,
spurious or wrong-end entries, with the same six grammar exclusions. A bounds-
checked assertions-enabled build combines `AuditScanner`, `AuditPredicates`,
`AuditHints` and `AuditFastPaths` and passes both corpora and three fixtures.
`dlang_predicate_check.d` performs 328,051 comparisons with the previous gated
regexes, covering all keywords, concatenated attributes, near misses, Unicode
boundaries/whitespace, generated lines and a 128 KiB tail. The expanded stress
fixture contains 9,149 lines and 1,069 functions, including 64 nested functions,
a 128 KiB literal and split names/parentheses/multiline return types. Go,
previous D, both new release builds and the checked build agree exactly.

Three-run candidate diagnostics give arsd 251.983 ms: cleaning/normalization
114.249 ms, joins 19.640 ms, contracts/hints 58.152 ms, classes 34.264 ms,
nested scan 23.936 ms and scope/filtering 1.935 ms. An intermediate predicate-
only profile puts joins at 20.432 ms/7.945 MB allocated; the final join uses
3.702 MB. Thus the join change primarily reduces allocation traffic (53.4%
of that intermediate join phase), while the predicates supply most of the
measured time reduction. D-YAML final diagnostic time is 17.144 ms.

Candidate artifacts, phase generators/raw samples and verification scripts are
in `build/dlang-port-20261018-predicates/`. `dlang_port.predicates.d` and
`predicate-phase-profile.json` retain the intermediate implementation/profile.

Ten shuffled release rounds retain the three baseline binaries in the same
series, with one parser warmup and two timed corpus passes per sample. Both Go
probes use the same frozen revision; their small differences show measurement
variation, not a Go optimization. Medians in milliseconds (before -> after):

| Corpus / variant | Serial parser | Complete probe process |
|---|---:|---:|
| D-YAML, Go (unchanged) | 35.503 -> 35.004 | 46.181 -> 46.074 |
| D-YAML, runtime D | 34.670 -> 15.974 | 47.550 -> 28.346 |
| D-YAML, ctRegex D | 27.555 -> 16.181 | 47.764 -> 26.764 |
| arsd, Go (unchanged) | 380.954 -> 384.121 | 432.076 -> 432.862 |
| arsd, runtime D | 511.982 -> 260.189 | 555.761 -> 307.118 |
| arsd, ctRegex D | 398.570 -> 253.035 | 442.132 -> 294.618 |

ctRegex D reduces parser time by 41.28%/36.51% (D-YAML/arsd), runtime D by
53.93%/49.18%. New ctRegex D takes 46.23%/65.87% of this Go API's parser time
(2.16x/1.52x throughput). Current parser ranges (Go/runtime D/ctRegex D) are
33.965–36.265 / 15.874–16.286 / 14.796–17.076 ms for D-YAML and
377.792–395.932 / 258.061–263.766 / 249.651–255.434 ms for arsd. Runtime D
and ctRegex D overlap on D-YAML; the small median reversal is not evidence
that runtime regex is intrinsically faster. Both new D builds beat this Go
reference on these two pinned serial-probe workloads; this is not a language-
wide or complete-toolkit comparison.

Current binary sizes are Go 2,768,384 bytes, runtime D 1,224,192 bytes and
ctRegex D 1,435,648 bytes. Relative to this fresh baseline, D grows by
5,120/4,608 bytes. The previous specialization limits remain: Go links generic
finder machinery, while D implements only D. Total diagnostic allocation
traffic falls from 42.428 to 37.500 MB on arsd and 3.339 to 2.342 MB on
D-YAML (11.62%/29.87%); these are not peak RAM measurements. After this change,
cleaning/normalization is the largest measured phase (about 45% on arsd).
Source/binary hashes and all ten-sample counts were rechecked after timing.

The production Go sources, release version and dispatch are unchanged by this
experiment. The nearest benchmark contract and README are updated; root and
other child contracts/indexes remain unchanged because this adds no production
API or directory boundary.

## Byte-oriented ASCII cleanup and JS/TS expansion

`Lexer.cleanUnits` now runs the same state machine over bytes for ASCII lines
and runes for Unicode. The ASCII path avoids decoding to UTF-32 and encoding
back to UTF-8; its output is an owned immutable copy of reusable byte scratch.
Normal-state plain-code returns remain unchanged. A Unicode delimiter carried
from another line still uses the same persistent lexer state and comparison
rules. `AuditFastPaths` compares ASCII output and every persistent state field
with the slow rune path, in addition to its previous shortcut checks.

Ten shuffled contemporaneous rounds reuse the verified previous predicate/join
binaries from `build/dlang-port-20261018-predicates/report.json`; Go remains
frozen at `b744fa0`. One warmup and two timed passes per sample; milliseconds:

| Corpus / variant | Serial parser, before -> after | Complete probe, before -> after |
|---|---:|---:|
| D-YAML, Go (unchanged) | 35.632 -> 35.131 | 46.223 -> 46.595 |
| D-YAML, runtime D | 16.260 -> 13.300 | 28.555 -> 26.030 |
| D-YAML, ctRegex D | 16.414 -> 13.682 | 26.785 -> 24.474 |
| arsd, Go (unchanged) | 382.847 -> 382.914 | 431.949 -> 434.237 |
| arsd, runtime D | 261.261 -> 201.314 | 306.530 -> 244.522 |
| arsd, ctRegex D | 253.640 -> 195.963 | 296.113 -> 239.137 |

ctRegex parser time falls another 16.6%/22.7% (D-YAML/arsd). Both variants
retain full ordered parity on 160 files and the same independent 12,244-function
accepted profile. The combined audit build and the 1,069-function stress input
also pass. Current binaries: Go 2,768,384 bytes, runtime D 1,227,264 bytes,
ctRegex D 1,438,720 bytes; each D build grows by 3,072 bytes. Generic-Go versus
D-only specialization and serial API limits remain unchanged.

A separate three-run phase profile after all release timing gives arsd
198.286 ms, with cleaning/normalization 57.962 ms (previous diagnostic
114.249 ms). That phase allocates 16.078 MB instead of 26.470 MB. Total phase
allocation traffic falls 37.500 -> 27.178 MB; GC is 1.124 ms in two cycles.
D-YAML diagnostic time is 13.690 ms and 1.827 MB allocated. These are
instrumented/cold-process diagnostics, not extra release samples or peak RAM.
Contracts/hints and cleaning now each account for about 30% of arsd time.
Profiles, binaries, frozen Go, samples, hashes and verification are retained in
`build/dlang-clean-js-ts-20261018/d-clean/`; the original source is saved at
`build/dlang-clean-js-ts-20261018/dlang_port.before.d`.

The same byte/rune strategy also underlies a separate complete JS/TS finder
API port. Its source, quality limits, cleanup/finder/process measurements and
reproduction commands are in [JS_TS_D_PORT.md](JS_TS_D_PORT.md).

## Initial pilot additional verification

A separate `-O2 -boundscheck=on` build, with assertions enabled, reproduces the
Go maps on all 160 corpus files. The production Go probe and all three D builds
also agree on the two repository D fixtures and a synthetic fixture covering
nested comments, token/delimited strings, a Unicode name, nested functions,
contracts, constructors/delegation and a split return-type/header. The ctRegex
release build rejects a mismatched runtime pattern configuration. Binary/source
hashes were rechecked after measurement. Commands, fixture text and results are
retained in `verify.py`, `compile-checked.bat` and `verification.json` under the
same ignored work directory. No production Go code was changed by this pilot.

DOX: benchmark ownership, reproduction, measurement limits and Windows toolchain
guidance are recorded in `benchmarks/AGENTS.md` and `README.md`. The root and other
child contracts and child indexes remain unchanged: no production API, workflow
or hierarchy changed.

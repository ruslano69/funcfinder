# TODO

Known issues and follow-up work, tracked here until they become GitHub issues.

---

## Language coverage on real projects: status and next steps (2026-10-19)

Ruby D port: all 1,294 tracked Huginn/Maybe `.rb` files independently verify 4,146
methods and 1,315 named class/module types, with zero D misses, false entries,
wrong ends or grammar exclusions. Native grammar is tree-sitter 0.26.0 /
tree-sitter-ruby 0.23.1, with explicit heredoc beginning/body pairing and physical
terminator span extension. Fresh frozen-Go function missing/false/wrong-end counts
are 20/1/0 and 0/0/3; D closes these accepted gaps. Production Go and its Ruby
TODO remain unchanged. Separate units/token audits pass; 22 fixtures cover nested/
endless/singleton/operator methods, value/modifier conditions, blocks/loops,
literal forms, division, queued heredocs, Unicode/CRLF and malformed/long/deep input.
Twenty valid fixtures independently verify 94 methods/six types. All 23 previous
D sources are unchanged. Seven paired Go/D parser medians ms: Huginn 57.299/34.772
(1.65x), Maybe 42.503/27.921 (1.52x); process medians 107.463/79.471 and
100.600/81.006. Ruby-only D is 944,640 bytes versus generic Go 2,807,296;
algorithms/profiles/specialization differ. All 15 languages now have finder
prototypes; common CLI/workers/other analyses and complete lexical/semantic
coverage remain open. Fresh pins, snapshots, truth/maps and all samples:
`build/ruby-d-port-final-20261019/`; see `benchmarks/RUBY_D_PORT.md`.

Python D port: all 180 tracked Flask/Requests/hr_portal files independently verify
2,884 functions and 343 classes against native CPython 3.14.6 AST, with zero D
misses, false entries, wrong ends or parse exclusions. The final frozen-Go baseline
uses production PythonFinder through an exact-body preloaded-lines adapter and
the original os.ReadFile/strings.Split reader. The generic NewFinder and earlier
ReadLines-based probes are exploratory. Go raw maps retain trivia; after separate
blank/comment end normalization, function missing/false/wrong-end counts are
0/0/0, 29/0/1 and 0/0/0. The frozen Requests result needs separate investigation;
it is not a replay of the earlier local Go fix. D has exact native ends, no trimming.
Separate units/token audits and 15 fixtures pass; 13 valid fixtures independently
verify 87 functions/three classes. All 21 previous D sources are unchanged. Seven
paired Go/D parser medians ms: Flask 15.672/9.815 (1.60x), Requests 10.242/6.744
(1.52x), hr_portal 21.736/12.973 (1.68x); process medians 48.636/43.797,
41.048/37.810 and 55.020/46.026. Python-only D is 932,352 bytes versus the Go
PythonFinder probe's 2,555,392. Full XID/NFKC/legacy encodings, common CLI/workers
and other analyses remain open. Evidence: `build/python-d-port-production-20261019/`;
see `benchmarks/PYTHON_D_PORT.md`.

Scala D port: all 88 tracked scopt/os-lib files independently verify 1,023 named
functions/signatures and 328 named types with zero misses, false entries, wrong
ends or grammar exclusions. Frozen Go also has no function errors; its narrower
type map misses 44/91 types. The D profile includes aliases/abstract/nested types
and package objects. Separate module units and token/bounds maps pass; 22 fixtures
cover parameterless/operator/setter/auxiliary-constructor names, annotations,
curried/context clauses, expression/control bodies, characters/symbols/interpolation,
Scala 3 indentation/enums/extensions/end markers, Unicode/CRLF and long/deep input.
Twenty valid fixtures independently verify 107 functions/15 types; malformed and
incomplete fixtures remain explicit unscored robustness checks. All 19 earlier D
backend hashes are unchanged. Seven paired Go/D parser medians ms: scopt
8.634/3.309 (2.61x), os-lib 23.835/5.863 (4.07x); process medians 37.797/32.977
and 55.829/36.913. Scala-only D is 940,032 bytes versus generic Go 2,807,296;
algorithms/profiles/specialization differ. Thirteen of fifteen finder prototypes
are present; Python/Ruby, common CLI/workers/other analyses and complete Scala 3/
Unicode/quote/XML grammar coverage remain open. Pins, snapshots, truth/maps and
all raw samples: `build/scala-d-port-final-20261019/`; see `benchmarks/SCALA_D_PORT.md`.

PHP D port: all 495 tracked Monolog/Symfony Routing files independently verify
2,727 named functions/methods and 480 named types, with zero misses, false entries,
wrong ends or grammar exclusions. Frozen Go also has zero function errors, while
its old type profile misses 2/6 types and has 0/1 false types. Separate module
units and token/bounds maps pass; 21 fixtures cover PHP/HTML, attributes, anonymous
classes, references, DNF returns, interpolation, heredoc/nowdoc, Unicode/CRLF and
long/deep input. Nineteen valid fixtures verify 94 functions/eight types; malformed
and incomplete fixtures remain explicit unscored robustness checks. All 17 prior
D backend hashes are unchanged. Seven paired Go/D parser medians ms:
Monolog 38.611/12.632 (3.06x), Routing 44.612/13.850 (3.22x); process medians
77.852/50.419 and 86.153/52.694. PHP-only D is 929,792 bytes versus generic Go
2,807,296; algorithms/profiles/specialization differ. Twelve of fifteen languages
have finder prototypes; shared CLI/workers/other analyses and complete PHP grammar
coverage remain open. Pins, snapshots, maps, truth, hashes and all raw samples:
`build/php-d-port-final-20261019/`; see `benchmarks/PHP_D_PORT.md`.

Swift D port: the dedicated token finder independently verifies 3,249 functions
and 835 nominal/alias/associated types on 268 accepted Alamofire/Kingfisher files
with zero misses, false entries or wrong ends. All 276 tracked Swift inputs are
retained, audited and timed; eight grammar exclusions remain explicit (5/3).
CRLF normalization in reference memory avoids two false multiline-string grammar
failures. Under the same fresh pins/profile frozen Go misses 4/48 functions and
has 2/2 wrong ends; D closes all 52 accepted misses and four ends. The older Go
Swift TODO remains open; production Go is unchanged. Thirteen fixtures pass map/
token/bounds checks; eleven independently verify 21 functions and seven types.
All 15 earlier backend source hashes are unchanged. Seven paired Go/D parser
medians ms: Alamofire 148.556/17.392 (8.54x), Kingfisher 160.907/16.548 (9.72x);
process 184.337/55.808 and 198.859/54.003. Swift-only D is 936,448 bytes versus
generic Go 2,807,296; these compare different algorithms/specialization/profiles,
not language-only gains or an equivalent complete CLI. Reference exclusions,
full Unicode/regex contexts, semantic owners and common CLI/file workers remain
open. Pins, snapshots, truth/diagnostics, audits and samples:
`build/swift-d-port-20261019/`; see `benchmarks/SWIFT_D_PORT.md`.

Kotlin D port: native Kotlin compiler 2.2.0 PSI now independently verifies all
1,995 named functions and 413 named types on 311 tracked NekoBox/Seal `.kt`/`.kts`
files with zero misses, false entries, wrong ends or parse exclusions. Against
the same fresh pins, frozen Go has function missing/false/wrong-end counts
NekoBox 3/0/1 and Seal 6/0/0; D closes all nine misses and the local `counters`
span issue. These fixes are in the D prototype; the older Go TODOs remain open.
Type discovery intentionally expands aliases and other named declarations.
The tree-sitter Kotlin diagnostic misparsed `finally`; it is superseded by
compiler PSI with explicit UTF-8 and CRLF normalization. Twelve fixture/stress
files pass release/assertion checks, ten independently verify 19 functions and
five types; module unit checks pass and all 13 previous backend hashes remain
unchanged. Seven paired Go/D parser medians ms: NekoBox 48.855/11.846 (4.12x),
Seal 84.858/17.119 (4.96x); process 86.132/46.428 and 120.787/53.108. Kotlin-only
D is 939,520 bytes versus generic Go 2,807,296, not equivalent complete-toolkit
scope. Full CLI/file workers, full Unicode categories and semantic ownership
remain open. Snapshots, PSI oracle/runtime provenance, truth and samples:
`build/kotlin-d-port-20261019/`; see `benchmarks/KOTLIN_D_PORT.md`.

Rust D port: a separate Rust lexer/token finder verifies 1,379 functions and 470
named types (including aliases/associated types) on 57 accepted kanzi-rs/Serde
files with zero misses, false entries or wrong ends. All 58 inputs are retained,
audited and timed. One `text_codec.rs` grammar rejection at line 978 remains an
explicit exclusion; native rustfmt accepts it but does not establish span truth.
Kanzi is a dirty checkout at `3a991f3`, recorded with actual source hashes; Serde
is clean at `0db9b3b`, restricted to its two core source trees. D removes all 63
Serde Go false functions inside macro token trees. Its broader named-type map
intentionally differs from Go's impl-scope class map. Eleven fixtures pass map/
bounds/token checks; nine accepted fixtures independently verify 16 functions
and six types. Seven paired Go/D parser medians ms: kanzi 15.194/12.799 (1.19x),
Serde 14.795/8.088 (1.83x); process 45.653/42.783 and 44.517/37.720. Rust-only
D size is 932,864 bytes versus generic Go 2,807,296, not an equivalent toolkit.
All 11 prior C/C++/C#/Go/Java D source hashes are unchanged. Remaining Rust work:
macro expansion/cfg selection, full Unicode XID, another oracle for the excluded
file and full CLI integration/file workers. Snapshots, truth, diagnostics and
samples: `build/rust-d-port-20261019/`; see `benchmarks/RUST_D_PORT.md`.

C D port: a separate C-only token/declarator probe now verifies all 1,568 Lua
definitions/prototypes across 68 files with zero misses, false entries or wrong
ends. The 135 inactive raw candidates remain separate; all preprocessing/grammar
files are accepted for the recorded C99/GCC configuration. Eleven fixture/stress
files pass release/assertion/normal/raw checks; eight valid fixtures independently
verify 15 declarations. Prior C#/C++/Go/Java D sources remain byte-identical.
Seven paired parser medians Go/D: 15.456/17.150 ms; complete process
47.002/49.323 ms. D is currently 11.0% slower in parsing, with extra local-scope
walking and 68 unscored tag definitions; there is no measured C speed gain.
C-only D size is 946,688 bytes versus generic Go's 2,807,296, not equivalent
toolkit scope. Remaining C work includes variable-first/initialized declaration
lists, typedef ambiguities and macro expansion; unified CLI/file workers remain
open. Full snapshots, checks, independent truth and samples:
`build/c-d-port-20261019/`; see `benchmarks/C_D_PORT.md`.

Go/Java D port: token backends now join C#/C++ in the shared serial probe.
On 458 Gson/gin/installed Go 1.25.7 `encoding/json` files, native Go AST and the
Java grammar verify all 6,059 named functions and 1,679 named types with zero
missing/false/wrong-end entries. Go finds 16 extra generic functions with nested
brackets in constraints; type output now covers all 912 Go TypeSpecs, including
grouped/local/alias declarations. Java retains exact function/signature/scope
and named-type-header parity, while correcting one record end previously closed
inside an annotation array (85 -> 86). No previously correct function is lost.
Seven paired parser medians Go/D ms: Gson 58.036/31.363 (1.85x), gin
14.016/13.675 and JSON 30.392/28.855. Go gains are modest (2.4%/5.1% less time),
with effectively unchanged gin complete-process time; D does broader Go discovery.
The four-language D probe is 1,254,912 bytes (+14,848 versus C#/C++), generic
Go 2,807,296 bytes. Unit/bounds/token/cleaner/native-fixture checks pass; all 980
previous C#/C++ maps and both cleanup modes remain identical. Build-tag filtering,
full CLI/file workers and other toolkit analyses remain outside this probe.
Pins, snapshots, independent truth, samples and final source/binary/input audits:
`build/go-java-d-port-20261018/`; see `benchmarks/GO_JAVA_D_PORT.md`.

C++ D tokenization: normal function/type discovery now uses immutable tokens,
paired brackets and declaration/body contexts, with no regex initialization or
calls in the normal scan. Against the same libclang profile, fmt improves from
36/7/0 to 22/0/0 and tinyxml2 from 1/0/0 to 0/0/0 (missing/false/wrong-end).
No previously correct function is lost. Remaining fmt misses are 12 conversions
needing semantic type resolution and 10 macro-expanded methods. Function-name
tokens anchor matching; declaration starts can include outer template clauses
before clang's extent and have explicit regression tests.
Seven paired parser rounds, Go/regex-D/token-D ms: fmt 35.362/89.927/12.184,
tinyxml2 11.067/28.524/2.632. Token D is 2.90x/4.20x Go and 7.38x/10.84x old D;
full probe times are 24.070/12.219 ms (Go 46.613/18.891 ms). D size is 1,240,064
bytes (+7,168 versus regex D). C# source/output and both normal/raw cleaners
remain unchanged. Bounds/raw-coordinate/pair/no-regex audits, module unit tests,
all 980 corpus files and existing fixture/pattern diagnostics pass. Snapshots,
three-variant samples and final hash/input/quality checks:
`build/cpp-tokens-20261018/`; `benchmarks/CSHARP_CPP_D_PORT.md` has reproduction
and serial-probe/semantic/preprocessing limits. Full CLI/file workers remain open.

C#/C++ D port: the separate serial API prototype now matches frozen Go 1.15.9
(`bdbc81b`) on all 980 Polly/Dapper/fmt/tinyxml2 files: 8,978 ordered raw function
entries, 1,797 raw classes, signatures/scopes and normal/raw cleanup facts.
Roslyn retains all 6,708 C# truth functions with zero misses/false entries/wrong
ends; fmt remains 36/7/0 and tinyxml2 1/0/0 (missing/false/wrong end). The two
fmt grammar exclusions and inactive-source profiles remain explicit.
Seven paired parser rounds (Go/D ms): Polly 115.694/68.649, Dapper 36.484/18.264,
fmt 35.034/90.810, tinyxml2 10.758/28.088. C# takes 40.7%/49.9% less time; the
initial runtime-regex C++ port takes 2.59x/2.61x as long as Go; the token stage
above supersedes that implementation. The initial combined D probe is 1,232,896 bytes versus generic Go's
2,807,296 bytes; neither is an equivalent complete-toolkit comparison.
Bounds/cleaner audits, 261 fixture/stress files, 180,000 pattern-group cases and
all 1,114,112 Unicode code points pass. Go Unicode 15.0.0 is pinned because D's
newer categories otherwise broaden identifiers. Sources, pins, independent
findings, raw samples and final hash checks: `build/cs-cpp-d-port-20261018/`.
See `benchmarks/CSHARP_CPP_D_PORT.md`; full CLI/file workers remain open.

JS/TS missing-function fix: Go and the D prototype now recover all 71 previously
missing named functions in the accepted profiles. Express/lodash remain clean;
zod missing/false/wrong-end = 0/2/1 (was 14/3/1), OmniRoute JS = 0/2/0 (was
37/2/0), OmniRoute TS = 0/45/38 (was 20/49/38). These are 35,999 accepted truth
functions with the same 13 parse-error exclusions, not complete JS/TS accuracy.
Incremental template handling and TS balanced-token recovery have regression
tests for source extraction/selection/offsets. Both APIs/lexical facts agree on
all 10,009 inputs; raw results and the frozen Go base-plus-overlay are under
`build/js-ts-misses-20261018/`. Remaining quality work is false entries and ends.
Historical five paired parser samples quantify the initial recovery cost: OmniRoute TS Go
1710.505 -> 2158.723 ms (+26.2%), D 1593.788 -> 1919.023 ms (+20.4%); zod Go
+35.8%, D +19.4%. The narrower Go 1.15.9 gate is now also ported to D below.
No complete-process speed claim is made for this correctness change.

D TS recovery-gate follow-up: quoted arguments and ordinary split/generic calls
avoid the supplemental token pass; incomplete shapes remain conservative within
the 64-following-line lookahead. Seven paired rounds against frozen Go 1.15.9
(`bdbc81b`) give Go/previous-D/current-D parser medians: OmniRoute TS
1857.160/1881.277/1688.873 ms, zod 155.915/147.077/139.524 ms. D parser time
falls 10.2%/5.1%; full OmniRoute TS probe time falls 2197.515 -> 1980.538 ms
(Go 2247.350 ms). This is the serial API probe, not the production parallel CLI.
All 10,009 complete cleanup/finder maps and independent counts remain unchanged.
Gate unit tests, 20,009 Go/D gate cases, bounds/token audits and 100,000 header
vectors pass. D probe size is 1,296,384 bytes (+1,536). Source snapshots, paired
samples and final source/binary/input checks are under `build/d-ts-gate-20261018/`;
the verified previous D stays in `build/d-ts-gate-20261018-before/`.
See `benchmarks/JS_TS_D_PORT.md`. Remaining D work includes profiling the TS
scanner and integrating the serial finder into a complete CLI with file workers.

JS D-port tokenization follow-up: direct ASCII character checks and immutable
UTF-8 token slices remove the JS slowdown while preserving the full Go map.
Seven paired shuffled rounds compare Go/previous-D/current-D: Express
29.009/23.534/12.552 ms, lodash 145.039/150.737/75.185 ms, OmniRoute JS
108.048/127.191/61.358 ms. Complete OmniRoute JS probe time falls
156.578 -> 91.776 ms (Go 143.798 ms). All 10,009 JS/TS file results and independent
grammar findings remain unchanged; this is performance work, not a quality fix.
Separate bounds/token/header audits, 100,000 TS vectors, all Unicode scalar
word-classifier checks and malformed UTF-8 direct API cases pass.
New D binary: 1,272,832 bytes. Unicode slices can retain joined source storage
through returned names. See `benchmarks/JS_TS_D_PORT.md`; raw paired results and
source snapshots: `build/js-ts-d-port-20261018-js-opt/`. This performance stage
precedes the missing-function fix above; full CLI/file parallelism remains open.

D implementation pilot (2026-10-18): `benchmarks/dlang_port.d` ports the D
finder API to LDC 1.43.0, compared with frozen Go `v1.15.3`. Both runtime-regex
and ctRegex D builds exactly match ordered function/class bounds and scopes on
all 160 tracked D files; the independent accepted profile retains all 12244
functions with zero misses/false entries/wrong ends and six grammar exclusions.
Ten shuffled serial-parser samples: D-YAML Go/runtime-D/ctRegex-D
168.246/260.410/207.502 ms, arsd 1285.069/2982.511/2013.926 ms. Probe sizes are
2.737/1.206/1.562 decimal MB; the Go probe retains generic finder machinery,
while D implements only D. No file parallelism or complete toolkit/CLI parity
is claimed. The initial direct port was slower than Go; follow-up measurements
and optimization results are recorded below. See
`benchmarks/DLANG_PORT.md`; artifacts: `build/dlang-port-20261018-final/`.

Follow-up: Go 1.15.5's negative function hints, fewer rune/string conversions
and attribute gate are now ported to D. Ten shuffled paired samples retain
all previous quality and ordered-map parity. D ctRegex parser time falls
207.342 -> 144.242 ms on D-YAML and 2006.095 -> 1385.388 ms on arsd (about 31%);
Go in the same series falls 171.265 -> 114.027 and 1272.144 -> 824.552 ms.
D binaries keep their sizes; D ctRegex still takes 1.27x/1.68x optimized Go's
time. `AuditHints` verifies every cached result and attribute gate on all
160 files. Go reference: `4ef44059a45aec74502dcf908bb5ece31314767b`;
artifacts: `build/dlang-port-20261018-optimized/`.
Separate D ctRegex phase diagnostics locate the remaining reserve in preparation
(about 92% of arsd time): roughly 580 MB cumulative managed allocations for
9.53 MB input, while actual GC collection takes 11 ms of 1407 ms. The complete
nesting scan takes about 5%.

The next D optimization implements Go `08f3531`'s plain-line lexer/prefix paths,
growing per-file rune buffers with lazy decoding, direct contract-token
comparisons, reusable scan contexts/events and type/statement matcher gates.
All 160 ordered maps and the independent accepted profile remain unchanged.
Bounds-checked `AuditHints`/`AuditFastPaths` builds additionally agree on the
corpora, three fixtures and generated Unicode/128 KiB-line/64-level nesting
stress input (1066 functions). Separate diagnostics reduce cumulative parser
allocations from 579.690 to 42.526 MB on arsd and 31.176 to 3.339 MB on D-YAML.
These are allocation traffic counters, not peak RAM. Ten contemporaneous
shuffled samples reduce D ctRegex parser time 145.608 -> 103.986 ms on D-YAML
and 1397.629 -> 726.832 ms on arsd (28.59%/48.00%). Current Go is
106.000/723.677 ms: approximately equal parser performance, with overlapping
sample ranges. Complete probe processes are Go/Dct 116.551/130.782 ms on
D-YAML and 773.386/776.825 ms on arsd. Current Go/runtime-D/ctRegex-D probe
sizes are 2.742/1.211/1.567 MB; specialization limits still apply. Artifacts:
`build/dlang-port-20261018-buffers/`; remaining measured targets are multiline
header joining and contract/hint preparation. No production Go dispatch or
release is changed by this experimental D work.

After the D function-header scanner was ported, a fresh profile against Go
`b744fa0` found contracts/hints at about 40%, cleaning at 32% and header joins
at 12% of arsd diagnostic parser time. The D pilot now reads ordinary attribute,
type and statement prefixes directly, retaining rare Unicode regex fallbacks,
and builds a header-join candidate incrementally only after `(` appears.
328051 differential predicate cases and the full audit/fixture/stress checks
pass; all 160 maps and independent accepted quality remain unchanged. In ten
paired shuffled rounds, D ctRegex parser time falls 27.555 -> 16.181 ms on
D-YAML and 398.570 -> 253.035 ms on arsd (41.28%/36.51%); current Go is
35.004/384.121 ms. Complete new probe times are Go/Dct 46.074/26.764 ms and
432.862/294.618 ms. Sizes are Go/runtime-D/ctRegex-D 2.768/1.224/1.436 MB,
with the same specialization and serial-API limits. Diagnostic allocation
traffic falls 42.428 -> 37.500 MB on arsd; cleaning/normalization is now the
largest measured phase. Artifacts: `build/dlang-port-20261018-predicates/`;
baseline/profile: `build/dlang-port-20261018-postscanner-baseline/`.

The D cleaner now reads ASCII directly through the same templated state
machine, with the rune fallback and checked state parity retained. Ten paired
rounds reduce ctRegex D parser time another 16.6%/22.7% on D-YAML/arsd:
16.414 -> 13.682 ms and 253.640 -> 195.963 ms. Current Go is 35.131/382.914 ms.
The arsd diagnostic cleaning/normalization phase falls 114.249 -> 57.962 ms;
total allocation traffic falls 37.500 -> 27.178 MB. All previous D quality,
audit and long-line/stress checks pass. Artifacts:
`build/dlang-clean-js-ts-20261018/d-clean/`.

The experimental D backend now also implements complete serial JS/TS named-
function/class discovery and cleanup facts, including JS signature metadata,
TS multiline/inline signatures and the direct header scanner. Exact Go parity
holds on Express, lodash, zod and both OmniRoute languages: 10009 files,
36702 functions, 4159 raw class entries. Checked builds, raw cleanup, malformed
UTF-8, 128 KiB lines and 100000 TS group-vector cases pass. Seven paired rounds
show cleanup 3.29–5.50x faster. Full finder medians Go/D: OmniRoute TS
1743.758/1639.100 ms, OmniRoute JS 110.373/132.488 ms; Express D is faster,
zod approximately equal and lodash D slightly slower. The diagnostic full-
facts JSON process is slower in D, so cleanup speed is not an end-to-end claim.
Probe sizes Go/D: 2.777/1.270 MB, with generic-Go versus JS/TS-only limits.
Independent grammar findings retain the same Go limitations (including 14
zod misses, 20 OmniRoute TS misses and 37 OmniRoute JS misses); see
`benchmarks/JS_TS_D_PORT.md` for false entries, wrong ends, exclusions and the
dirty OmniRoute input snapshot. Artifacts: `build/js-ts-d-port-20261018/`.
Production Go dispatch and releases remain unchanged by these prototypes.

Earlier measurements used `main` at `2022d5b` against syntax-tree ground truth (tree-sitter, one grammar per
language); subsequent runs and their revisions are recorded below. Python, PHP, Scala/D, C, C++, TypeScript and C# evaluators now live in `benchmarks/`. Truth
includes declarations without a body (interface, abstract, prototype members): the design lists
them with their own span, so a missing C# or Java declaration counts as missing.

| Language | Project (truth functions) | Correct | Wrong end | Missing | State |
|---|---|---|---|---|---|
| Go | gin (1342), funcfinder itself (603) | 100% | 0 | 0 | done; re-measured on gin HEAD `0f09c3a` (1554 truth): 4 misses — one-line `struct{}` members of `type (...)` groups and multi-line `[]struct{...}` composites — fixed, now 1554/1554, 100%/100% |
| Rust | kanzi-rs (490) | 100% | 0 | 0 | done |
| Swift | Alamofire (1890), Kingfisher (109) | 99.6%, 100% | 2, 0 | 6, 0 | good |
| Kotlin | NekoBoxForAndroid (1166), Seal (781) | 99.7%, 99.2% | 1, 0 | 3, 6 | good |
| Ruby | huginn (1865), maybe (2284) | 99.0%, 99.9% | 0, 3 | 19, 0 | good |
| TypeScript | hr_gemini (2988), zod (2072) | 100%, 99.3% | 0, 1 | 0, 14 | hr_gemini measured 2026-10-13 after the declarator fix (tree-sitter TypeScript reference, 0 false functions); zod measured before it, 3 false functions |
| JavaScript | lodash (2458), Express (218) | 100%, 100% | 0, 0 | 0, 0 | local JS token fix verified 2026-10-18; 55/55 and 141/141 tracked files, 0 false functions; expanded named-function profile including vendor sources and 4 minified bundles, not a replay of the old 1918-function profile |
| Java | gson (3492) | 100% | 0 | 0 | production linear-token fix verified on `e2292f9`; 264/264 files, 0 false functions; 767/767 named types match header locations; recursive pilot remains separate |
| C# | Polly (5116), Dapper (1592) | 100%, 100% | 0, 0 | 0, 0 | measured 2026-10-16 with Roslyn; no false functions; 801/801 and 157/157 files; default symbols |
| C++ | fmt (1450 truth) | 97.5% | 0 | 36 (19 conversion operators, 11 macro-generated) | measured 2026-10-13; 7 false functions (explicit instantiations) |
| C++ | tinyxml2 (627 truth) | 99.8% | 0 | 1 | measured 2026-10-13; 0 false functions; `8224e427b655b83dae5e2298f1e6919523a78737` |
| C | Lua (1568, including 391 prototypes) | 100% | 0 | 0 | local fix verified; no false functions; one GCC configuration |
| Python | Flask (1462), Requests (711), hr_portal (711) | 100%, 100%, 100% | 0, 0, 0 | 0, 0, 0 | local fix verified; no false functions |
| PHP | Monolog (1602), Symfony Routing (1125) | 100%, 100% | 0, 0 | 0, 0 | local fix verified; no false functions |
| Scala | scopt (380), os-lib (643) | 100%, 100% | 0, 0 | 0, 0 | local fix verified; no false functions |
| D | D-YAML (539), arsd (11705) | 100%, 100% | 0, 0 | 0, 0 | local fix verified; no false functions; oracle accepts 34/35 and 120/125 files |

C# was re-measured on 2026-10-16 on a working-tree fix based on `bf62c23`, using
Python 3.14.6, .NET SDK 9.0.308 / Roslyn 4.14.0.0, Preview syntax and the
`FileBasedProgram` feature. No preprocessor symbols were supplied. All 958 tracked
`.cs` files parse successfully; 20 Polly and 113 Dapper candidates in disabled text
are recorded outside scoring. Truth includes named methods, constructors,
destructors, local functions and operators, including 175/29 bodyless declarations;
anonymous lambdas, implicit primary constructors and property/event accessors are
excluded. Conversion type names are compact syntax tokens. All 6708 measured
declarations have correct names/header locations and exact end lines, with zero
missing or false functions. On the same oracle, the `bf62c23` baseline had 1322/629
missing, 80/14 false functions and 3/1 wrong ends. The finder keeps source ranges
through the last branch of a conditional expression without evaluating symbols;
these results describe the recorded default-symbol profile, not every build variant.
Pins: Polly `ea41f708cfc33ba5ae01738ee8c5a1cf211fb9e9`,
Dapper `eb47546a408bbf6c178bda4f04dc205bc51ffbfe`. Evaluator:
`benchmarks/csharp_roslyn_eval.py` with `csharp_oracle.cs`; raw before/after JSON
and executable/oracle hashes are in ignored `build/csharp-eval-20261016/`.

C# scope traversal was compared on the same pins against the test-only recursive
prototype on 2026-10-16 (base `8e5cf67`, Go 1.25.7, Windows/Ryzen 9 5950X).
Ten serial in-process samples per variant give median parser times 125.152 →
123.609 ms for Polly and 38.101 → 37.240 ms for Dapper, with 0.75%/1.50% fewer
allocated bytes. Twenty paired CLI samples per variant are effectively equal:
146.369/146.373 ms and 70.226/70.254 ms. Traversal is 83 → 69 lines; the whole
effective finder is 447 → 433 lines. Both independently retain 6708/6708 correct
Roslyn functions and complete-result parity in map/extraction/offset/selection
modes. Default production traversal is unchanged. Shared tokenization/signature
rules are identical; this does not compare full grammar parsers. See
`benchmarks/CSHARP_TRAVERSAL_COMPARISON.md`; raw samples and hashes are in
ignored `build/csharp-traversal-20261016/`.

The shared recursive pilot now covers C#, Java and D, with Python and Ruby
excluded. On the same C# and D pins plus Gson `e2292f9`, independent references
find all 22444 measured declarations with correct ends and no false functions:
C# 6708, Java 3492, D 12244. D retains the six existing parse-error exclusions
(120/125 arsd and 34/35 D-YAML files); C# retains its default-symbol profile.
The walker owns nesting/context/extraction; adapters own signature, block and
preparation rules. Default production dispatch is unchanged. See
`benchmarks/BRACE_PILOT.md`; artifacts are in `build/brace-pilot-20261016/`.

D speed of the shared pilot was measured separately on all tracked D files
(125 arsd, 35 D-YAML), including the six reference exclusions. Ten serial parser
samples and twenty paired CLI samples per variant: arsd 2238.230 → 4888.042 ms
parser (2.18x slower), 441.851 → 1957.523 ms complete CLI (4.43x slower), allocated
bytes 231.212 → 1434.969 decimal MB (6.21x). D-YAML 267.529 → 279.766 ms parser;
113.920 → 110.128 ms CLI (overlapping ranges), allocations 12.306 → 23.220 MB.
The current D pilot must not replace production yet. Profiling points to repeated
header reconstruction/normalization and regex matching in the adapter. CLI type
maps also differ (arsd 4451/2857, D-YAML 90/74 entries), outside the named-function
oracle; establish type-map parity before promotion. Raw samples/profiles and
maps: `build/d-traversal-20261016/`; method/results: `benchmarks/BRACE_PILOT.md`.

After the user stopped background work, identical binaries/inputs/settings were
remeasured in `build/d-traversal-20261016-quiet/`: arsd 2234.783/4808.390 ms
serial parsing (2.15x) and 396.890/1809.914 ms CLI (4.56x). D-YAML
263.803/233.988 ms parsing, 89.293/89.196 ms CLI. Arsd's regression persists;
allocation totals are unchanged. Parser benchmarks deliberately use one Go CPU;
directory CLI uses file-level parallelism with the default CPU-count workers.

A bounded within-file scope scheduler was tried separately on the same pins
(`benchmarks/BRACE_PARALLEL.md`). With 32 file workers, adding up to 31 scope
tasks changes arsd CLI median 1804.244 → 1828.307 ms and D-YAML 86.579 →
86.739 ms (three samples/case). Arsd mean total CPU utilization rises only
12.4% → 13.5%. One file worker gives 4965.057 → 4884.765 ms on arsd, too
small a change to establish a gain in this exploratory run. Exact ordered
serial-pilot parity holds on all 1382 tracked C#/Java/D files; scheduler race
checks pass. Keep it test-only: serial preparation and parent-side header
recognition remain, and production type-map parity is still unresolved.

Java/Gson was then measured with the same scope scheduler (ten samples/case,
two warmups; `build/brace-parallel-java-20261016/`). With 32 file workers:
production 47.165 ms, serial pilot 68.369 ms, parallel pilot with scope budget
32: 69.646 ms. With one file worker: 146.582 / 158.297 / 156.413 ms.
Scope parallelism again gives no useful scaling; ordered results on all 264
Java files and timed CLI text match the serial pilot exactly. Tests/vet pass.
The pre-fix production/pilot timing gap includes different recognition: the unchanged
pilot matches all 3492 reference declarations; the old production has 367 misses and
55 false functions. This is not an isolated recursion-cost comparison.

The production Java path is now fixed independently of the recursive pilot:
`java_finder.go` uses a linear token walk and explicit scope stack. All 3492
Gson reference functions have correct ends, zero misses and zero false names;
all 767 named types match names/header locations without duplicates. The type
audit does not score end boundaries. Shared bracket pairing retains C# output
byte-for-byte on Polly/Dapper. Unit tests, complete tests, vet, race checks and
all five tool builds pass. Raw artifacts: `build/java-fix-20261016/`;
method, final timing ranges and limits: `benchmarks/JAVA_FIX.md`. The final
20-sample CLI run is noisy (an old-parser outlier reaches 2.27 s); its medians
are 70.501 → 113.450 ms at 32 file workers and 179.799 → 181.555 ms at one.
The correct parser is slower than the old incomplete path; no precise speed
ratio is established under this load.

Python was measured separately on 2026-10-12 on a working-tree fix based on `7b9131e`, built with `build.ps1`, against
Python 3.14 `ast`. All tracked `.py` files were included (83/37/60 files, no parse errors).
Names are matched one-to-one within the AST declaration/decorator span; correct ends exclude
trailing blank/comment lines. Duplicate method names are retained using plain `--map` output,
since file-mode JSON overwrites repeated names. Recall and precision are 100% for all three
projects. Before the fix, wrong ends were 20/30/97 and Flask had 29 false functions.
Pins: Flask `d086db856be187255b8ec61ef409357393020f32`,
Requests `611c6162cbc4ac2020a2f91c7cfa4f3abf9bbb60`, hr_portal `87a2909d85ee173209afcecdd12f530257e5d2ab`.
The evaluator is `benchmarks/python_ast_eval.py`; raw outputs from this run are in ignored
`build/language-eval-20261012/`.

PHP was measured on 2026-10-12 on a working-tree fix based on `34d7dc8` using
tree-sitter 0.26.0 / tree-sitter-php 0.25.1 under Python 3.14.6. All tracked PHP
files were included (228 Monolog, 267 Symfony Routing; zero parse errors). Truth
includes 16/18 bodyless declarations, excludes anonymous closures, and matches
names one-to-one within the declaration header with exact end lines. Baseline:
27/43 missing, zero wrong ends or false functions. Both projects now have 100%
recall and precision. Pins: Monolog `3bed304b1905fed5c30d172872f4d890cbb03c6a`,
Symfony Routing `57970507cf869e358330cb336772a7dcb31fa67c`. Evaluator:
`benchmarks/php_tree_eval.py`; raw before/after JSON is in the same ignored build
directory. Line numbers are derived from byte positions because the native Point
wrapper crashed on this Windows Python runtime.

Scala was measured on 2026-10-12 on a working-tree fix based on `bd1575e`;
D's follow-up fix is based on `25a1e52`,
using Python 3.14.6 / tree-sitter 0.26.0. Scala grammar: tree-sitter-scala 0.26.2;
D grammar: tree-sitter-language-pack 1.22.0, DLL SHA-256
`acf35033a28e91d15620505ee2b2e06efc5be01c8caa0c6fef5de6925f9588f8`.
Reference ends exclude syntax-tree extras/trailing whitespace. D truth includes
constructors, destructors and postblits; anonymous lambdas and value/type templates
are excluded. All 29 scopt and 59 os-lib files parsed. D excludes
`D-YAML/examples/tojson/source/app.d` and arsd's `cli.d`, `core.d`, `ini.d`,
`script.d`, `uda.d` because the oracle reports syntax errors; their symbols are
not scored. The D grammar mislabels two `final switch` statements and 18
constructor-delegation calls as declarations; the evaluator rejects these by
syntax/context and records every rejection (`oracle_rejected`). Final truth is
11705 functions, all found with correct ends and no false functions. With this
audited reference, `25a1e52` missed 249 functions and produced 93 false functions;
the follow-up fix has zero in both categories. Before the first fixes: Scala
missed 120/203; D-YAML missed 486 with 300 false functions. Evaluator:
`benchmarks/scala_d_tree_eval.py`, raw JSON in the ignored build directory above.
Pins: scopt `fb371a7cf997fe053c9e1496cc6e13fbff3ff9de`,
os-lib `86e5ddb65fe62305afb5885fcc0f8da50aba4b48`,
D-YAML `09aeac155c9d0f85a0e7442dffb442de53528a41`,
arsd `953107183178fccbe8af7340b230c37c2071aef8`.

C was re-measured on 2026-10-12 on a working-tree fix based on `c2cf2fe`,
against Lua `0b29f408433e92953cc72b1d3e06c7ac8139e439`. All 68 tracked `.c`/`.h`
files were included, with no preprocessing/parse failures. Runtime: Python 3.14.6,
tree-sitter 0.26.0, tree-sitter-c 0.24.1, MSYS2 UCRT64 GCC 15.2.0 (Rev8).
The oracle preprocesses C99, with the Lua root on the include path, forced
`lua.h` for standalone headers and `LUA_USE_JUMPTABLE=0`. It maps GCC line markers
back to original source and scores active definitions/prototypes. All 135 finder
candidates in inactive branches are recorded separately, not scored. Thirty-eight
GCC va_arg/offsetof expressions are masked for grammar compatibility, preserving
bytes/newlines and function boundaries. This is one Windows/compiler configuration,
not all conditional variants. Before the fix: 157 missing (156 prototypes, one
definition), 136 false names, zero wrong ends; after: zero in all three categories.
The earlier 99.9% did not measure prototypes. Evaluator: `benchmarks/c_tree_eval.py`;
exact flags, compiler/binary hashes and findings are in the ignored build directory.

TypeScript was measured on 2026-10-13 with `benchmarks/ts_tree_eval.py` (tree-sitter-typescript 0.23.2 as the reference). Truth: function declarations and signatures, class and interface methods (including bodyless overloads and abstract methods), constructors, and functions bound to a variable; anonymous callbacks are not truth. zod `b500a2ee95134f81d74da7911d7378dd2853d042` (511 tracked files, 4 unparseable): on `main` 2007 of 2072 found, recall 96.9%, precision 99.0%; now 2057 found, recall 99.3%, precision 99.85%. hr_gemini `5b69a1eb` (455 tracked files, 9 unparseable): on `main` recall 91.3%, precision 96.2%; now recall 99.9%, precision 100%, 2 wrong ends (down from 12: a closing JSX tag was read as a regular expression, and a typed component React.FC<{...}> lost its body). Still open: the 2 wrong ends (onData, an object-literal method closed by a brace and a semicolon on its line, and loadBonusGrantsForYear); 0 false functions; 2 missing (a second declarator in one const, and OrderCreationForm whose parameter destructuring runs over lines). zod: 3 false functions and 15 missing. Lodash is not in this table: its reference is a minified bundle, and a non-minified JavaScript project is still needed.

C++ was also measured on tinyxml2 `8224e427b655b83dae5e2298f1e6919523a78737` (`tinyxml2.h` and
`tinyxml2.cpp`, 627 truth, libclang 18.1.1, same target): on `main` 332 found, recall 53.0%, precision
100%; now 626 found, recall 99.8%, precision 100%, no wrong ends or false functions. The override and
final specifiers were the largest remaining miss. CLI11 (`07c9f13b15a70c367a406e1cfe2c363aa1b66894`)
is not measured: 22 of its 29 headers fail to parse with the clang reference (a GCC-only x86 builtin in
the mingw headers, and redefinitions from its include structure).

C++ was measured on 2026-10-13 on a working-tree fix based on `edc5f94`, against fmt
`b4ffa39b9994c961418406f47358791d89a5aa48`, with libclang 18.1.1 and MinGW-Builds GCC 15.2.0
(C++17, `x86_64-w64-mingw32`). `include/` and `src/` were measured: 18 files are scored, and
`src/fmt.cc` (a C++20 module unit) and `src/os.cc` (an x86 builtin this target lacks) are unparseable
exclusions. Truth is 1450 functions and declarations, 138 of them bodyless. On `main` at `edc5f94`:
727 missing, 21 false functions, 2 wrong ends (recall 49.9%, precision 97.2%). Now: 36 missing,
7 false functions, 0 wrong ends (recall 97.5%, precision 99.5%). Evaluator:
`benchmarks/cpp_tree_eval.py`; raw JSON is in the ignored build directory.

- [ ] **Verify CI on `main` at `2022d5b`.** PR #77 was merged while four of its checks (Analyze
  (go), Build, Build Windows, Build macOS) were still running. Read the result once; do not poll.

### Open items, in order of measured gap

- [x] **TypeScript, zod: 267 missing, now 31.** Three shapes were missed: a const annotated with
  a function type (`const f: (x) => T = (x) => {`), a method whose name is a string literal
  (`"=== INVALID"() {`, read from the raw line because the sanitizer blanks the literal), and a
  method with a function-typed parameter (`add(fn: (v) => any) {`). Correct 88.8% -> 98.6%, wrong
  ends unchanged. The 31 left: generic or multi-line method headers (`refine(` with its parameters
  below, `extend<...>(` over several lines); methods named `catch` or `with`, which the statement
  words exclude; a `const x =` whose value starts on the next line; a function whose return type
  holds braces (`loadPriorTriage(...): Map<string, { verdict: string }> {` ends at the return type,
  and it is not found at all); a computed name (`get [SYM]() {`).
- [x] **TypeScript, hr_gemini: 46 missing, now 26; lodash: 473 missing, now 357.** A function can
  start after a separator on a line (`let n=0;const f=…`, `cookie='';constructor(){`, `}, m(){`).
  Each such header is read from its own start, and its body closes where its braces do. Flag
  `inline_headers`, TypeScript and JavaScript only. Two closing rules came with it: a body closed
  mid-line is over when a header follows the close (minified `}function a(){…}`), and a depth that
  drops below zero ends the function. The 26 left in hr_gemini: 8 `React.FC<{` headers whose type
  argument runs over several lines, 11 headers whose parameters or destructuring run over several
  lines (`export const X: React.FC<P> = ({` and `async issue(input: {`), and 7 others, including
  `export default function App()`, which may be a cascade from an earlier frame that did not close
  (not checked yet). Also `notification=` after a comma in a `const` list.
- [x] **TypeScript, hr_gemini: 23 wrong ends, now 0 (2026-10-13).** Causes fixed: a template literal's
  `${...}` holds strings and nested templates (the sanitizer closed the outer template at the
  first inner backtick, which hid whole files), an arrow header ending in `=>` is not complete,
  a parenthesized arrow body runs to its closing `);`, a brace inside a type argument
  (`React.FC<{...}>`) does not stop the bracket count, a closing JSX tag (`</h4>`) and a
  self-closing `}/>` are not regular expressions, and a body that opens and closes on its header
  line ends there.
- [ ] **TypeScript, zod: 14 missing, 3 false functions, 1 wrong end (2026-10-13; not re-measured after the declarator fix).**
  zod missing: `catch`, `create`, `args`, `returns`, `$constructor` (generic header over lines), the
  `formatContributor` and `createToJSONSchemaMethod` consts whose `=` sits on the previous line,
  `createStandardJSONSchemaMethod`, `registries.add<` over lines, `async "array-objects"`, and
  `loadPriorTriage` (return type with braces). zod false functions: the methods named `async` and `new`.
  zod wrong end: `validate-abort-sweep.ts` `deepInput` (truth 78-81): funcfinder ends it at 80.
- [x] **C#, Polly/Dapper: declaration context and function ends fixed locally.**
  The C# token scan uses shared cleaned facts and distinguishes typed declarations from calls,
  control statements, properties/indexers/events, primary type headers and object patterns.
  It finds multiline/generic and explicit-interface methods, constructors/destructors, local
  functions, operators and bodyless declarations, retaining original extraction and offsets.
  `callgraph` and `funcstats` exclude declaration tokens using finder metadata; real recursion
  remains, and C# local callers use the innermost scope. All 5116/1592 reference functions match
  exactly in the documented profile. Tests: `internal/csharp_finder_test.go` and funcstats tests.
- [x] **C++, fmt: 727 missing, now 36.** Fixed with header patterns and a few engine rules:
  nested parentheses in parameters, bodies on the header line, signatures that continue on later
  lines, constructors and destructors (in-class and out-of-class), operator symbols, template
  prefixes, export macros in class headers (`class FMT_API file`), braced initializations in
  constructor initializer lists, and one-line classes that held the next class open. Remaining:
  conversion operators (19, which clang names by canonical type), declarations generated by
  `FMT_*` macros (11), template specializations and `template <...>` headers that do not fit
  one line (6), and the 7 false functions in the table. The C++20 module unit and the x86 builtin
  file are not measured.
- [ ] **Promote the shared C#/Java/D recursive pilot after performance and wider-corpus validation.**
  On Gson `e2292f9`, the pilot and the independent production Java fix both retain
  3492 correct functions, including annotation elements. C# and measured D quality are retained.
  The recursive implementation is still test-only. First remove the measured D regression (arsd 2.18x slower
  serial parsing, 4.43x slower CLI, 6.21x allocated bytes) and establish type-map parity;
  optimize repeated header construction/normalization and matching in the D adapter.
- [ ] **Kotlin, NekoBox `ServiceButton.kt`: `counters` ends late.** A local expression body is
  followed by an `if` block, and the `if` is read as the body. It was missing before PR #76, so this
  is not a regression, but it is a wrong span now. A rule for a statement keyword that cannot
  continue an expression (`if`, `for`, `while`, `return`) would cover it; check the previous line
  does not end in an operator first.
- [ ] **Kotlin, Seal: 6 missing, NekoBox: 3 missing.** Not analysed yet (`FormatPage.kt`,
  `DownloaderV2.kt`, `Utils.kt`).
- [ ] **Ruby, huginn: 19 missing.** Methods defined inside another method's body, such as `authors`
  and `_links` in `lib/feedjira_extension.rb` (inside `self.included`). The block-end finder does not
  nest. Also a few in `spec/`. Ruby `maybe`: 3 wrong ends, not analysed.
- [ ] **Swift: Alamofire 2 wrong ends, 6 missing.** Not analysed. Project `Ice` is not measured.
- [x] **JavaScript: expression discovery and minified endpoints fixed locally.**
  The historical 1918-function lodash profile is superseded by a reproducible
  JS evaluator with explicit named-expression/direct-variable binding rules.
  On lodash `2b5e6f7` (55 tracked files, including vendor and four minified
  bundles), the same-profile baseline has 403 misses, 4 false functions and
  4 wrong ends; now all 2458 functions match exactly. Express `0dee2c5`
  (141 files): 78 misses become zero, all 218 match; zero false functions or
  wrong ends. The shared TS line scanner is unchanged; pinned zod's 338 TS
  files with symbols are identical, while `.cjs` discovery adds one JS file
  whose 7 functions independently match. Tests, vet, race and all five builds
  pass. Methods, dollar/private names, statement context, original extraction,
  selected names and offsets have dedicated regressions. See
  `benchmarks/JAVASCRIPT_FIX.md`; raw scores/maps/timings are in ignored
  `build/javascript-eval-20261018/`.
- [x] **Python: wrong spans and docstring false positives fixed locally.** Logical statements
  now account for strings, bracket continuations, explicit backslashes and multiline decorators.
  Function indentation comes from the `def` line; suite colons are read outside brackets, with
  trailing comments ignored. Inline suites close at the end of their logical statement. Triple
  strings match their own delimiter and scanning resumes after the close. Finder class ranges and
  `AnalyzePythonScopes` use the same boundaries. All 2884 real functions now have correct ends;
  the 29 Flask docstring false functions are gone. Regression tests are in `internal/python_bounds_test.go`.
- [x] **PHP: modifiers, references, nested methods and heredoc fixed locally.** The function
  pattern accepts abstract/final/static/visibility modifiers and return-by-reference names.
  Nested scanning finds methods of classes declared inside other functions. PHP-only heredoc
  preprocessing hides literal code/quotes/braces and retains code after the closing label;
  extracted source and slice offsets are preserved. Both reference projects now have zero
  missing functions, false functions and wrong ends. Tests: `internal/php_finder_test.go`.
- [x] **Scala: modifiers, parameterless/operator names and expression ends fixed locally.**
  Inline annotations/members, function-typed returns, multiline signatures, string-only bodies,
  chains after brace blocks and triple-string closing runs are covered. Both projects now match
  all 1023 reference functions exactly. Shared nested-parser implementation was not changed.
- [x] **D-YAML: nested discovery, lexical masking and contract ends fixed locally.** All 539
  oracle functions match. Attribute continuations, templates, return-by-reference/type modifiers,
  nested comments, raw/delimited/token strings and inline classes are covered by dedicated code.
- [x] **D, arsd: declaration prefixes and constructor calls fixed locally.** Conditional and
  parameterized attribute/linkage prefixes, pointer/type forms, inferred properties and multiline
  return types are found. Constructor calls are excluded using function/class context, including
  callback arguments; local constructors and postblits remain visible. All 11705 audited reference
  functions match exactly on 120 accepted files. Tests also cover selected-function extraction and offsets.
- [ ] **D: audit six excluded files with a stronger/native oracle.** One D-YAML and five arsd files
  have grammar errors; the accepted-file results do not establish whole-repository coverage.
- [x] **C, Lua: parenthesized function names and function-pointer typedefs fixed locally.**
  Definitions and prototypes such as `int (name)(...)` now retain their names;
  `typedef void (*Callback)(...)` and function-pointer variables are not functions.
  All 1568 active declarations match exactly on all 68 tracked C/header files in the
  recorded GCC configuration. Tests verify original extraction, offsets and selection.

### Regression sweep for any change to the nested path

The nested path is shared. Before a PR that touches `findFunctionsWithNesting`, `sigScanner`,
`continuesExpression` or the sanitizer, run the same projects and compare with the table above:
TypeScript (`hr_gemini`, `zod`), JavaScript (`lodash`), Go (`gin`, funcfinder itself), Rust
(`kanzi-rs`), Swift (`Alamofire`, `Kingfisher`). A PR that moves any of these numbers down has to
say so and why. The 2026-10-13 change to class bounds (`findClassesWithOffset`, net brace depth) and
to `sigScanner` (braced initializers, used by C++ only) touches the class lookup of every language
with classes. Differential check on 2026-10-13 (`main` binary against this change, same checkouts, no
truth oracle): zod `b500a2ee95134f81d74da7911d7378dd2853d042` 2472 functions, lodash
`2b5e6f7399a7b48005140b5d5c6bc6c0e62919a8` 2120, hr_gemini `5b69a1eb` 3277 — identical function sets.
Class attribution changed only for one-line class declarations (`class Test {}`: 11 in zod, 9 in
hr_gemini); methods after them no longer belong to that class. `abstract class X {` was not a class in the
TS/JS `class_pattern` (the `--struct` pattern already accepted it). Fixed on 2026-10-13: zod gains 11 methods of
its abstract classes (`ZodType`, `Metabench`), lodash and hr_gemini are unchanged. Abstract methods without a body are listed too (zod: `run`, `_parse`). The truth-based numbers for the 2026-10-13 TypeScript fixes are in the table above.
The fixtures in `test_examples/` and `test_files/` show no change outside C++.

### Profile and duplicates (2026-10-13)

Measured with a temporary `BenchmarkProfileRepo` (whole repository, `all` mode; 124 tracked files, 29.7k lines):
about 95 ms and 39 MB per run, 335k allocations. `LoadConfig` costs about 17 ms per CLI run. Profile reports:
`build/profile-cpu-top.txt`, `build/profile-cpu-cum.txt`, `build/profile-mem-top.txt` (ignored by git).

Open speed items, by share of the cumulative CPU profile:
- `Sanitizer.CleanLine` about 36%: every line is converted to `[]rune` and back. A byte path for ASCII lines is the largest single gain.
- Struct finder second pass about 19% (`FindStructures`): the whole file is scanned again for types the class pattern has already found in `all` mode.
- Regex execution about 21%: function patterns run on every line; a cheap pre-check could skip most lines.
- Repeated file reads: finder, struct finder and the Go alias pass each open the file (`os.Open` about 4.5%); the alias pass also sanitizes the file again (about 5% on Go files).
- `LoadConfig` compiles every language's patterns on each run; a lazy compile per language would remove most of its 17 ms.

Duplicates in production code:
- Open and scan into `[]string`, four copies (`finder.go`, `python_struct_finder.go`, `struct_finder_factory.go`, `structfinder.go`); `readAllLines` already exists.
- The type-scan loop is copied between `structfinder.go` and the hybrid finder in `struct_finder_factory.go`: the largest duplicate.
- The shard-key choice (`splitBy == "file"`) is repeated three times in `dirprocessor.go`.
- Rune-slice cleaning is repeated in `d_lines.go`, `python_finder.go` and `scala_lines.go`.

Done in the same pass (2026-10-13), each checked against the reference output on a fixed corpus (725 files, byte
identical in four output modes): each file is read once; regex patterns compile on first use; `CleanLine` has a fast
path for lines with no literal or comment start (checked against the slow path); the four open-and-scan copies are
one helper; the struct-pattern kind is fixed by sorted order. Corpus full run 1.75 s to 1.53 s; a single-file run
150 ms to 129 ms (the process itself is about 70 ms of that).

Left, and why:
- Merging `HybridStructFinder` into `StructFinder` changes TypeScript and JavaScript output: the hybrid closes a
  class or interface header that has no brace on its line, the generic finder waits for the brace. That needs a
  decision on which behaviour is wanted before the merge.
- The `all` mode still scans each file a second time for types (`FindStructures`, about 19% of CPU before these
  changes); the classes of the first pass could be reused if the dedupe rule is kept.
- The shard-key choice in `dirprocessor.go` (three copies) and the rune-slice cleaning in `d_lines.go`,
  `python_finder.go` and `scala_lines.go` are duplicates that are not yet merged.

### Harness

- [ ] **Move the evaluators into the repo** (`tools/eval/`): `lang_eval.py` (all languages but
  Kotlin), `kt_eval.py` (Kotlin). They currently sit in a scratch directory. Run them with
  `python3`, where the tree-sitter grammars are installed.
- [ ] **Pin each project to a commit.** The projects were cloned with `--depth=1` and the commits were
  not recorded, so the numbers cannot be reproduced exactly.
- [ ] **Fix the evaluator's header comment.** It says declarations without a body are not truth, but
  the code counts them, and the table above relies on that.
- [ ] **Record the build command** next to each number. Use
  `GOPROXY=https://goproxy.io GONOSUMDB='*' go build ./cmd/funcfinder/`.

---

## CI: `Test` job failing on every push since 2026-07-01 (2026-07-17)

- [x] **FIXED.** Root cause had nothing to do with `-race` despite every
  failure looking like a bare `exit code 1` right after the last package's
  `ok` line, with no `FAIL`/`panic` anywhere — the actual error, buried in
  the full (not `--log-failed`) log: `go: no such tool "covdata"` for
  `internal/embed`, the sole package with zero test files. Computing its
  coverage entry needs `covdata`, which the CI runner's Go install lacks.
  `-race` was a red herring — the original single `go test -race
  -coverprofile=... ./internal/...` bundled both concerns into one exit
  code, so a coverage-tooling failure looked like a test/race failure.
  Confirmed by bisection: dropping `-coverprofile`/`-covermode` alone made
  every job pass.

  Fix in `.github/workflows/ci.yml`: split into `Run tests (race)` (no
  coverage) and `Run tests (coverage)` (no `-race`, excludes
  `internal/embed` via `go list ./internal/... | grep -v
  '/internal/embed$'`). A real data race still fails CI on its own; a
  missing toolchain component for an untested package no longer masquerades
  as one. Verified: `Test (1.23)`, `Test (1.24)`, and all three `Build`
  jobs green on the next push.

## deps --shards: shardForDir prefix match non-deterministically drops parent-package edges (2026-07-17)

- [x] **FIXED.** Found while mapping this project with its own tools:
  `deps . -l go --shards` showed `cmd_funcfinder.json` depending on
  `internal_knowledge.json` — but `cmd/funcfinder/main.go` only imports
  `github.com/ruslano69/funcfinder/internal`, not `internal/knowledge` at
  all. `internal/importresolver.go`'s `shardForDir` matched by string prefix
  (`rel == dirRel || strings.HasPrefix(rel, dirRel+"/")`), which any
  subpackage's files also satisfy (`internal/knowledge/db.go` starts with
  `internal/`) — so which shard it returned for the plain `internal` import
  depended on Go's unspecified map iteration order over `relToShard`.
  Verified non-deterministic against the real repo: 8 repeated runs of the
  pre-fix binary alternated between `internal.json` (correct) and
  `internal_knowledge.json` (wrong) 5:3.

  Fix: `shardForDir` now matches a file's *immediate* containing directory
  (`filepath.Dir(rel)`) exactly against `dirRel`, instead of a string
  prefix — matching Go's own import semantics (a package is the files in one
  directory, never its subdirectories). The now-unused `splitBy` parameter
  threaded through `shardForDir`/`resolveImportToShard` (it never actually
  affected the returned shard, since `relToShard`'s values already encode
  `splitBy`) was removed. Post-fix, 5 repeated real-repo runs all
  consistently show `internal.json`.

  Test: `TestBuildShardGraph_ParentAndSubpackageBothResolve`
  (`internal/importresolver_test.go`), 20 iterations per run to catch the
  map-iteration-order flakiness; fails deterministically against the pre-fix
  code on iteration 0.

## findFunctionsSimple: single-line function body treated as "no brace yet" (2026-07-17)

- [x] **FIXED.** Found while verifying the `callgraph -l` filter fix (below):
  `internal/finder.go`'s `findFunctionsSimple` — the non-nested path used by
  Rust, C, C++, C#, Java, D, Kotlin, PHP — closes a function on `depth == 0 &&
  prevDepth > 0`. That condition is never satisfied when a function's opening
  AND closing brace land on the *same* line, since the net brace delta for
  that line is 0 and `prevDepth` was also still 0 (nothing open yet) — so it
  read identically to "no brace at all, still waiting for a multiline
  signature's brace". The function stayed open indefinitely, silently
  absorbing every following line (including an entire next function) until an
  unrelated brace-balance event happened to close it. Two real-world shapes:
  - Rust idiomatic single-line bodies: `fn helper() -> i32 { 42 }`.
  - K&R-style C with a one-line body on the brace line: a signature line
    ending bare (C's `func_pattern` requires end-of-line after `)`), then
    `{ return 42; }` on the next line.

  This is exactly why `callgraph --dir` on a Rust-only file set was reporting
  0 functions/0 calls (found while verifying the `-l` filter fix — see
  below), and would affect `funcfinder --map`/`--struct` output quality on
  any of the 8 affected languages whenever a function's body fits on one
  line.

  Fix: both places `findFunctionsSimple` decides "is this function closed"
  now also check whether the line actually contained a `{` (`hasBrace :=
  strings.Contains(cleaned, "{")`), not just the net brace delta — mirroring
  the already-correct pattern in `findFunctionsWithNesting`
  (`braceDelta == 0 && strings.Contains(cleaned, "{")`). Verified the
  Rust `where`-clause multiline-signature case (the reason `prevDepth > 0`
  was required in the first place) still works.

  Tests: `TestFindFunctionsSimple_SingleLineBodyOnSignatureLine`,
  `TestFindFunctionsSimple_SingleLineBodyOnBraceLine`
  (`internal/finder_multiline_test.go`).

## Rust/Scala char literals vs single-quote ambiguity (deferred)

Loop #2 of the tdtp spec-sheet fixed char/rune-literal brace leakage by setting
`"char_delimiters": ["'"]` for Go, C#, Java, D, Kotlin. Rust and Scala were
**intentionally left out**: a lone `'` there is legal and unpaired — Rust
lifetimes (`&'a str`, `fn f<'a>()`) and Scala symbol literals (`'sym`). Treating
`'` as a char delimiter would open `StateCharLiteral` and blank the rest of the
line. If Rust/Scala char literals (`'{'`, `'}'`) ever show up as a measured
recall miss on a reference project, the fix is a *context-aware* char rule
(e.g. require a closing `'` within N chars, or a lookahead that rejects
`'<ident>` lifetime/symbol shapes), not a blanket delimiter.

---

## Code review: hot-path optimizations, bugs, stdlib wheels (2026-06-30)

Findings from reviewing the most heavily-executed code paths. Ordered by the
fix sequence (safe → risky). Checkboxes track progress.

### Stdlib "wheels" — low risk, output-identical

- [x] **`checksum_stdlib.go` hand-rolled hex** (`hexUint64Pair` + `uint64Hex`)
  → `encoding/hex.EncodeToString(h.Sum(nil))`. Byte order identical; verified
  by the incremental split round-trip tests.
- [x] **`config.go` bubble sort** in `GetSupportedLanguages` → `sort.Strings`.
- [x] **`dirprocessor.go` `itoa`** — inlined to `strconv.Itoa`.
- [x] **`callgraph_test.go` `containsSubstr`** → `strings.Contains`.

### Hot-path performance — no output change, benchmark before/after

- [x] **`enhanced_sanitizer.go` `matchesAt`** — rewritten to range over the
  pattern's runes directly (no `[]rune(pattern)`, no `string(runes[pos:...])`).
  **~2.5x faster** on the realistic CleanLine benchmark (16148 → 6592 ns/op).
- [x] **`enhanced_sanitizer.go` repeated `[]rune(s.config.X)`** — replaced with
  allocation-free `runeLen`/`firstRune` helpers; delimiter rune lengths are no
  longer recomputed via `[]rune(...)` slices.

### Bugs

- [x] **`enhanced_sanitizer.go` byte/rune index mismatch** (`CleanLine`) —
  FIXED. Buffer is now sized by rune count, and `handleBlockComment`,
  `handleMultiLineString`, `tryHandleBlockComment`, `tryHandleMultiLineString`
  scan via the rune-based `indexRunesFrom` instead of byte-slicing `line[idx:]`
  at a rune index. Covered by `sanitizer_nonascii_test.go` (6 tests:
  rune-length invariance, code preservation, block/line comment and multiline
  docstring handling across multibyte runes).
- **Unicode identifiers** (NEW, found during smoke test) — DONE across all 15
  languages. funcfinder, findstruct, callgraph and funcstats now detect
  function/type/call names with non-ASCII letters (`func Привет()`, `type Café`).
  - [x] **Single source of truth** — `internal/identifiers.go` defines
    `identClass` (`[\p{L}\p{Nd}_]`, Unicode equivalent of `\w`) and `identStart`
    (`[\p{L}_]`, no leading digit). Language patterns reference it via the
    `{IDENT}` placeholder, expanded at config-load time (`expandIdentPlaceholder`,
    applied in `config.go` to every compiled pattern). callgraph.go builds its
    call-site regex from the same constants — identifier recognition can no
    longer drift between "where is X defined" and "who calls X".
  - [x] **All 15 languages** — every name capture in `func_pattern`,
    `class_pattern`, `call_pattern`, `struct_type_patterns` switched
    `(\w+)` → `({IDENT}+)` (112 patterns). `\w` inside char classes
    (return-type matchers) and keyword groups left untouched.
  - [x] **callgraph** — hardcoded ASCII `callIdentRe` (ASCII-only `\b`) replaced
    with one built from `identStart`/`identClass`, no `\b`. Resolves
    `Привет → Старт`.
  - [x] **funcstats** — picks it up for free via each language's `call_pattern`.
  - Out-of-the-box, no flag: ASCII output unchanged (regression-guarded in
    `finder_unicode_test.go`). Cost: Unicode RE2 classes ~28% slower on the
    affected patterns (microseconds/line), dwarfed by the 2.5x sanitizer speed-up
    and negligible end-to-end. Benchmarks in `identifiers_bench_test.go`.
  - Note (left as-is, minor): regex return-type matchers in C/C++/D
    (`[\w\s\*]+`) and decorator/annotation patterns stay ASCII — a Unicode-named
    *return type* or attribute won't match, though the *name* will.
- [x] **`finder.go` dead branch** (`findFunctionsSimple`) — REMOVED. Confirmed
  dead: `} else if currentFunc != nil ...` sat inside the `else` of
  `if currentFunc != nil`, so `currentFunc` was always nil there. Multiline
  signatures (e.g. Rust where-clauses) are actually continued by the
  `if currentFunc != nil` branch on following lines. Pinned by
  `finder_multiline_test.go` (passes identically before and after removal).

### Resolved

- [x] **`dirprocessor.go` hand-rolled JSON** (`formatDirResultsJSON`,
  `formatManifestJSON`) + **incomplete `escapeJSON`** — MIGRATED to
  `encoding/json` (decision: accept the one-time format change). This fixes the
  control-char escaping bug and removes ~120 lines. Bonus: the manifest format
  is now byte-identical to what `deps --update-manifest` already wrote
  (cmd/deps used `json.MarshalIndent` all along), so the two tools no longer
  disagree on format. `escapeJSON` removed. Covered by
  `TestFormatDirResultsJSON_EscapesSpecialChars` (round-trips special chars
  through `json.Unmarshal`) plus the existing manifest round-trip tests.

---

## Reference-project findings (meetily run, 2026-06-30)

Surfaced while running the toolkit on Zackriya-Solutions/meetily (Tauri app:
TS frontend + Rust `src-tauri` + Python backend) as a candidate golden
reference project. Both are about honesty of output, which matters most for an
enterprise/Pro correctness story.

### `deps` silently returns an empty graph when rooted wrong (trust bug)

- [x] **FIXED (2026-07-17).** Two stderr diagnostics, both in `internal/importresolver.go`:
  - `DetectTSConfigAbove` — the precise signal for "rooted one level too deep,
    below the tsconfig": walks up from the analyzed root (bounded, 8 levels)
    looking for a `tsconfig.json`/`tsconfig.base.json` that `DetectTSAliases`
    (which only looks at the root itself and one level below) would never find.
    If one exists above, warns and names the directory to re-run from.
  - `ShardGraphStats.Warning()` — a general ratio-based fallback: `BuildShardGraph`
    now returns `(ShardGraph, ShardGraphStats)`, tracking how many imports
    *looked* intra-project (relative, or under a known modulePrefix/alias
    prefix) versus how many actually resolved to a shard. Below 20% resolved
    (min sample 3, to avoid noise on tiny dirs) triggers a warning — covers
    Go's `modulePrefix` misrooting too, not just TS/JS aliases.

  Wired into `cmd/deps/main.go`'s `--shards` path. Reproduced the exact
  meetily scenario with a synthetic tsconfig+alias fixture: correctly-rooted
  run is silent, `deps frontend/src --shards` (misrooted) now prints
  `WARNING: found .../frontend/tsconfig.json above .../frontend/src, ...`
  before the (still leaf-only, but now explained) graph. Tests:
  `TestDetectTSConfigAbove`, `TestBuildShardGraph_MisrootedShardsWarns`
  (`internal/importresolver_test.go`).

### `callgraph --dir` `-l` flag is a hint, not a filter — decide & document

- [x] **FIXED (2026-07-17).** Decided (b): `-l` now restricts the scan.
  `runDirMode` in [cmd/callgraph/main.go](cmd/callgraph/main.go) (~line 125)
  used to always pass the full multi-language `config` map to
  `internal.NewDirProcessor`, so the function-extraction pass auto-detected
  and processed every supported language in `dir` regardless of `-l` — only
  the (unused-for-extraction) import-alias collection was actually scoped to
  `-l`. Now passes a single-entry `internal.Config{lang: langConfig}` when
  `-l` is set, so extraction is scoped too. `--help` for `-l` updated to
  document the `--dir` behavior explicitly.

  Verified before/after on a synthetic Go+Python mixed directory: pre-fix,
  `callgraph --dir <mix> -l go` still "Analyzed 2 files" (both `main.go` and
  `app.py`); post-fix, "Analyzed 1 files" (`main.go` only). Consistent with
  `deps -l`'s existing shard-mode behavior, which already fully filters.

  **Found but out of scope, spun off separately:** Rust-only file sets
  return 0 functions/0 calls from `callgraph` (both `--dir` and `--inp`,
  independent of this fix) — Rust extraction only seems to work when mixed
  with another language's files in the same run. Pre-existing, unrelated to
  the `-l` filter change (reproduced identically on the pre-fix binary).

---

## D port: the ffd executable, stage 0/1 (2026-10-19)

`benchmarks/ffd/` is one D executable over the 15 token finders: the Go tool's `--dir [--all|--struct] --json` and text,
the root `.gitignore`, a worker pool (default 8 threads, at most 16), `--timing`. Against the released Go 1.15.9 on 30
trees: every function key but 185 of about 98,000 (111 Rust macro bodies, 71 C++, 3 Kotlin/Ruby); JavaScript/TypeScript
types equal (six trees byte-identical); the walk selects the same files on all 31 trees. Speed: serially 8% to 80% less
time than Go; with 8 threads each ahead on 8 of 14 trees and behind on 5; against Go's default 32 threads ahead on 6 and
behind on 8 (OmniRoute 610 ms against 488): `ffd` does less work per thread but scales worse (see Scaling below). The
first version of these numbers compared with a stale local binary and was wrong. See `benchmarks/ffd/README.md`.

Open, in order:

- [ ] **Types of the other 13 languages.** Go's struct finder (`structfinder.go`, `python_struct_finder.go`) adds kinds the
  D finders do not list (C#, C/C++, D: 86-565 types per tree) and the D finders list named types Go does not (Kotlin,
  Swift, Scala, Rust, Ruby, Python, PHP). Per-language decision: port Go's kinds like `ffd_hybrid.d` does for JS/TS, or
  accept the D set; counts are in `benchmarks/ffd/ffd_deltas.json`. Also Go's class order (structs/interfaces first).
- [ ] **Functions Go finds and D does not:** Rust `macro_rules` bodies (111 in serde), C++ cases in `fmt` (71, e.g.
  `StreamableToString` after a `template` line), 2 in Kotlin `Seal`, 1 in Ruby `huginn`.
- [ ] **Scaling.** The run stops scaling after about 8 threads: the runtime allocator is shared and the finders still
  allocate (6-40 bytes per source byte; JS 40, Ruby 19, C/C++ 17, Rust/Kotlin/Python 15). Reusing blocks is done for the
  line array, facts, cleaned text and the C#/Go/Java tokens (`reuse.d`); the token arrays of the other finders and the
  regex-based passes (D class/attribute/type/statement, TS declaration/declarator) remain. A `Vec`-style buffer instead
  of `~=` is 6x faster per append in a micro-benchmark and scales with threads.
- [ ] **Regexes, in D and in Go.** TypeScript spends 40% of its serial time in regexes: `declaration_pattern` 20%,
  `declarator_pattern` 7%, `func_pattern` 5% (already a scanner), `class_method_open` 3%, `class_pattern` 2%,
  `quoted_method_pattern` 2%. Go runs the same patterns (declaration 4%, struct fields 4% in the Go profile), so a scanner
  for each, proved equal like `tsHeaderSubmatch`, would speed up both implementations.
- [ ] **Stage 2 and later:** `--inp`, `--tree`, `--names`, `--doc`, `--split`, `--extract`, then `funcstats` and
  `callgraph` (need line facts from every finder), `complexity`, `deps`.

---

## New uses of the cleaner and finders (plan, 2026-10-18)

The cleaner knows, per line and for every language, which characters are code, comment or string literal (raw
strings, docstrings, template literals, nested comments included); the finders know function and type bounds; the
other tools add calls, imports and complexity. These are line-based heuristics, not grammars: no expressions, types or
name resolution. Order is by value for agents against cost.

- [ ] **1. Changed functions from a diff.** `--diff <rev>` / `--changed`: map `git diff` hunks to the functions and
  types whose bounds they touch; with `callgraph --reverse`, list affected callers (test selection, review risk,
  changelog draft, "read only these functions" for agents). Everything needed exists; a deleted function needs the
  old revision's map.
- [ ] **2. Code-aware grep.** Search only code, only comments or only string literals, reporting the enclosing
  function: dangerous calls without hits in strings/comments, hard-coded secrets/URLs in strings, i18n string
  extraction, TODO/FIXME per function, comment spell-check. Needs the comment/string ranges the cleaner computes but
  currently discards (it blanks them).
- [ ] **3. Structural lint for CI.** Thresholds with a non-zero exit code: function length, nesting depth, cognitive
  complexity (`complexity` already computes it; add `--max-*`), parameter count, missing doc comment on exported
  functions (`--doc` finds them), commented-out code, duplicate functions (hash of the cleaned, whitespace-normalized
  body), never-called functions (reverse call graph, heuristic). A multi-language baseline, not a replacement for
  language linters.
- [ ] **4. Function-bounded chunks for RAG/embeddings** (direction of `internal/knowledge`).
- [ ] **5. Editor data without grammars:** document symbols and folding ranges (functions, comments, strings).
- [ ] **6. Per-function history metrics:** churn, owners and age from `git log`/`git blame` mapped onto bounds;
  combined with complexity, a hot-spot map.
- [ ] **7. Literal-safe whitespace fixes:** trailing whitespace, tabs/spaces and CRLF outside multi-line, raw and
  template strings. This is the only formatter-like use that fits: a real formatter needs a full token stream and
  grammar.

Out of scope: type, null-safety or unused-variable checks, macro/preprocessor evaluation, refactorings that rewrite
expressions.

---

## Bugs

### `deps -j` lists equal counts and their files in a different order on every run (2026-10-18)

- [ ] **Open.** Two runs of the same binary on the same tree give different JSON: `cmd/deps/main.go` sorts
  dependencies by `Count` only (`sort.Slice`, ties in map order) and builds each dependency's file list from a map.
  Sort ties by module name and the file lists by path, so the output can be compared between runs and versions.

### `--dir --tree-full` prints the same as `--dir --tree` (2026-10-18)

- [ ] **Open.** In directory mode `--tree-full` gives byte-identical output to `--tree`, with no signatures: checked
  on `internal/`, arsd, OmniRoute and tdtp-framework (v1.15.3). `AggregateDirResults` passes `treeFull` to
  `formatDirResultsTree`, but `writeTreeNode` (`internal/dirprocessor.go`) prints only `def name (line N)` and never
  reads it; `DirResult` holds no signature text to print. Single-file `--inp ... --tree-full` is a separate path.
  Either carry signatures into `DirResult` and print them, or reject/document the flag in `--dir` mode.

### `callgraph --dir` walks the tree twice (2026-10-18)

- [ ] **Open.** `runDirMode` (`cmd/callgraph/main.go`) lists files with `internal.CollectSourceFiles` only to read
  import aliases, while `DirProcessor.collectFiles` walks the same tree with its own walker. Since #107 the second walk
  runs alongside the directory job, so it costs ~100 ms on OmniRoute rather than ~160 ms of wall time. Dropping it
  means reading imports for the directory job's results directly, which is safe only after showing that both walkers
  select the same files (hidden files, extensions, `.gitignore`, `-l` scoping); otherwise a file's aliases change.

### `.gitignore`: negation and nested files are not read (2026-10-18)

- [ ] **Open.** Glob syntax, anchoring and Windows separators follow git (`TestIgnoreMatcher_GitSemantics`). Still
  missing: `!` lines are dropped instead of re-including a path, only the
  root `.gitignore` is read (no nested files, no `.git/info/exclude`), and a tracked file that a pattern matches is
  skipped, since the walk does not read git's index.

### HybridStructFinder: brace-less `type_alias` swallows following types (TS/JS)

- [x] **FIXED (2026-07-17).** `findAllTypes` in
  [internal/struct_finder_factory.go](internal/struct_finder_factory.go)
  (around line 212): the `braceCount == 0`, non-`IndentBased` branch used to
  leave a brace-less single-line construct (e.g. a TS `type Handler = (req:
  Request) => void;`) open with `depth = 0`, so it absorbed every following
  line — including a subsequent `class Server {...}` — until an unrelated
  brace-balance event coincidentally closed it. Now closes the type
  immediately at the line that defines it, the same way the `braceCount > 0
  && depth == 0` branch already did.

  Verified live: `funcfinder --struct --map --inp handler.ts --source ts` on
  the exact repro now reports `Handler: 1-1; Server: 3-5;` instead of
  swallowing `Server`. The test that pinned the buggy behavior
  (`TestHybridStructFinder_TypeAliasWithoutBraces_SwallowsFollowingLines`)
  is renamed `..._DoesNotSwallowFollowingLines` and now asserts both types
  are found separately.

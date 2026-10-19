# Go/Java token finder API port to D

## Scope and implementation

`go_port.d` and `java_port.d` extend the shared experimental D probe to four
languages: Go, Java, C# and C++. Production tools remain Go. Normal discovery
uses tokens and paired brackets without regex initialization/calls. Shared
lexical/model/token modules are listed in the work directory's source manifest;
historical C#/C++ binaries and snapshots remain preserved separately.

Go recognizes named functions/methods, generic parameters/receivers, bodyless
declarations and return types containing anonymous structs/interfaces/functions.
It uses newline/semicolon boundaries to avoid absorbing the next declaration.
Type discovery includes all named type specifications: grouped declarations,
local types, aliases and defined scalar/function/composite types. Anonymous
function literals and interface methods do not count as named functions.
This exceeds the old raw Go struct/interface map, so whole-map equality is not
required. Functions retain bare names; this probe does not add receiver-based
class ownership or structural fields/type-kind metadata.

Java ports the existing linear token walk with explicit scopes, annotations,
generic/qualified/array return types, bodyless methods, constructors/compact
records and anonymous/local/enum class contexts. Function order, bounds,
signature metadata and scopes exactly match frozen Go on Gson. Type keyword
starts also match. A separate type-end audit found and fixed one inherited
record-header bug: an array in a component's annotation was taken as the record
body. Balanced header parentheses now skip that array. This changes one type
end while retaining the function/header maps.

The shared cleaner adds opt-in Go backtick strings and Go/Java character-literal
states; Java dollar names are opt-in in tokenization. All output text remains
owned and invalid UTF-8 follows Go's one-byte replacement behavior. C#/C++
results are unchanged after these shared-source changes. Unicode Letter/Digit
tables remain pinned to Go 15.0.0 and retain their earlier scalar audit.

The serial probe does not implement full CLI directory/file workers, selection,
offsets/extraction, structural fields/aliases as separate CLI kinds, call graphs,
dependencies or complexity. Go aliases appear in the raw type array. Regex
recognition overrides are outside these token backends. Binary-size comparisons
are specialized four-language D versus generic Go, not equivalent toolkits.

## Frozen references and quality

Tool reference: Go 1.15.9 `bdbc81bdc9be260782163d62c49e8b6135a0923d`, archived in
`build/go-java-d-port-20261018/reference-src/`. Candidate modules, Python helpers
and native-oracle source are retained in `sources/` and compiled from snapshots.
Go 1.25.7 (`GOAMD64=v1`), Python 3.14.6 and LDC 1.43.0 / LLVM 22.1.8 are recorded
with build flags and SHA-256 hashes.

| Corpus | Files | Function truth/found | Type truth/found | Missing / false / wrong end |
|---|---:|---:|---:|---:|
| Gson Java | 264 | 3492 / 3492 | 767 / 767 | 0 / 0 / 0, functions and types |
| gin Go | 99 | 1342 / 1342 | 212 / 212 | 0 / 0 / 0, functions and types |
| Go `encoding/json` | 95 | 1225 / 1225 | 700 / 700 | 0 / 0 / 0, functions and types |

Totals: **458 files, 6059 named functions and 1679 named types**, no parse
exclusions. Gson is clean at `e2292f9c9f2e07a4791827e8eea34aa7d80545d6`;
gin is clean at `0f09c3a9b4626d9fe9979cf4d0521d6ae32ab646`.
`C:/Program Files/Go/src/encoding/json` is the installed Go 1.25.7 source subtree,
recursively including tests, internal packages and experimental JSON packages.
It has no Git revision; version and every input hash identify this corpus.
Build tags/platforms are not filtered: files are parsed individually in both
implementations and the AST reference.

Go truth uses native `go/parser` + `go/ast` FuncDecl/TypeSpec start/name/end
positions, including bodyless functions and aliases. `go_ast_spans.go.txt` is
copied to the ignored work directory and compiled without external packages.
Java uses tree-sitter 0.26.0 / tree-sitter-java 0.23.5 and the existing function
profile. Named Java type truth covers classes/interfaces/enums/records/annotation
types, keyword start and final closing syntax position; it does not establish
semantic type information or field accuracy.

Frozen Go already finds all gin functions. It misses 16 `encoding/json`
functions whose generic constraints contain nested brackets, such as
`func appendString[Bytes []byte | string](...)`; the token port finds all 16.
The old raw Go type map has 39 missing gin types and 358 missing JSON types;
the new broader TypeSpec profile is exact. Java function accuracy is unchanged;
the record type `RecordWithCustomNames` now ends on line 86 instead of 85 in
`Java17RecordTest.java`. No previously correct function is lost.

## Verification and measurement

Separate bounds/assertion builds audit ASCII/rune cleanup, ordered token lines,
bracket symmetry and C++ raw-coordinate/no-regex invariants. Three module unit
test suites pass. There are 86 Go and 86 Java fixture/stress inputs, including
128 KiB lines, malformed UTF-8, raw/character literals, generic constraints,
bodyless declarations, grouped/local/alias types and Java record/default/anonymous
class shapes. Both normal/raw cleanup modes agree with Go; release/audit maps
agree. Fixed examples are also scored by native Go AST/tree-sitter Java.

Regression checks compare all **980 previous Polly/Dapper/fmt/tinyxml2 files**
against their preserved token-stage full maps and both cleanup modes, byte for
byte after JSON decoding. No shared-source regression was found.

Timings are seven shuffled paired rounds with three preloaded passes per parser
sample, a discarded warmup and per-variant checksum/full-output validation.
Complete-process timing includes startup/read/parse/JSON; Python JSON decoding
is outside it. Correctness/oracle/audit builds run separately from timed children.
Function/type outputs differ for Go, so timing is not an identical-output claim.

| Corpus | Go parser, ms | D parser, ms | Go full process, ms | D full process, ms |
|---|---:|---:|---:|---:|
| Gson Java | 58.036 | **31.363** | 84.166 | **57.150** |
| gin Go | 14.016 | 13.675 | 26.655 | 26.740 |
| `encoding/json` Go | 30.392 | 28.855 | 46.763 | 45.100 |

Java parser throughput is 1.85x Go (46.0% less time), with 1.47x complete-process
throughput. Go parser time falls only 2.4%/5.1%; this is a modest difference on
these corpora, and complete gin process time is effectively unchanged. D also
returns additional Go functions/types, so this stage establishes quality and
coverage rather than a large Go speedup. Full samples/ranges are retained.

The combined four-language D probe is **1,254,912 bytes** (+14,848 versus the
previous two-language C#/C++ token probe); the generic Go probe is 2,807,296
bytes. Sizes include their differing specialization. Final postflight verifies
both current binaries, all source snapshots/inputs, independent counts and the
preserved C#/C++ regression artifacts. No checks/builds/profiles ran concurrently
with timed children.

## Reproduction

Initialize `F:\D\ldcvars64.bat` for the installed LDC/MSVC linker. Java grammar
packages live under `build/eval-runtime/`. The Go oracle requires only the
installed Go toolchain. Preserve another revision in a new work directory.

```powershell
python benchmarks/go_java_port_compare.py --help
python benchmarks/go_java_port_checks.py --help
python benchmarks/go_java_port_compare.py `
  --revision bdbc81b --work-dir build/go-java-d-port-20261018 `
  --project java=build/brace-pilot-20261016/repos/gson `
  --project go=build/go-java-d-port-20261018/repos/gin `
  --project "go=C:/Program Files/Go/src/encoding/json" `
  --samples 7 --repeats 3
```

Use `--samples 0` for correctness only. `--audit --samples 0` with the same
projects builds a separate assertion executable/report, or use the retained
`compile-audit.bat`. Run `compile-unit.bat` and its `d-unit-probe.exe` separately;
the default D unit-test runtime skips main. Run
`python benchmarks/go_java_port_checks.py --work-dir build/go-java-d-port-20261018`
after release/audit builds and before timing. The work directory retains complete
maps/truth, configurations, snapshots, source/compiler/binary/input hashes,
samples and `verification.json`. `postflight.py`/`postflight.json` recheck these
against the pre-timing correctness report and prior C#/C++ artifacts.

The benchmark DOX/README and root TODO are updated. Root and other child DOX
contracts/indexes remain unchanged because production ownership and CLI/directory
boundaries are unchanged.

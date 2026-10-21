# Go-language types-only step in D — 2026-10-21

The D finder for Go source now accepts `findGo(facts, includeFunctions = true)`. Directory `--struct` passes false,
skipping named-function recognition, header/end scans, function bounds and their neutral-model copies. The same
tokenizer, bracket pairs and type scanner still visit the whole input, retaining local/grouped/generic types.
Unlike C++, function recognition does not control this backend's traversal, so it can be omitted entirely.

The existing alias scanner, owned alias strings and removal of aliases from the class list remain unchanged.
Function/all modes and file field/ownership consumers retain full scans. No public flag, second parser or change to
the production Go implementation is introduced; this optimizes the Go-language backend of experimental D `ffd`.

## Measured result

Intel i3-12100, Windows, LDC 1.43.0 / LLVM 22.1.8, `-O3 -release -mcpu=x86-64`. Seven shuffled paired full-CLI
samples, warm filesystem cache and per-case warmups, identical paths, `--dir --no-gitignore --struct --json`.
Median milliseconds:

| Corpus | Workers | Before | After |
|---|---:|---:|---:|
| gin | 1 | 34.51 | 34.08 |
| gin | 8 | 26.58 | 25.99 |
| encoding/json | 1 | 46.65 | 46.57 |
| encoding/json | 8 | 29.93 | 30.14 |

No consistent full-CLI acceleration is established. An initial gin all-mode slowdown above the 3% investigation
budget did not repeat in 15 additional shuffled pairs: one worker 35.72/35.59 ms, eight workers 27.26/26.68 ms.
The same repeat gave gin struct mode 33.97/33.54 ms and 26.37/26.44 ms. All raw samples, including outliers, remain.

Serial parser timing preloads source and warms configurations/buffers. Each of seven shuffled process samples times
five passes; allocation counters are read outside the stopwatch. `FFDReferenceScan` forces full work from the same
current finder. No audits or allocation diagnostics are enabled in timed release CLI builds.

| Corpus | Go files | Parser before / after, ms per pass | Allocated bytes before / after per pass | Allocation reduction |
|---|---:|---:|---:|---:|
| gin | 99 | 6.121 / 5.862 | 414,550 / 79,808 | 80.7% |
| encoding/json | 97 | 14.962 / 14.329 | 746,960 / 334,912 | 55.2% |

Parser medians improve about 4.2%; the firm result is reduced warmed thread allocations, not a guaranteed CLI speed
gain or a peak-memory measurement. Lexing, type/alias discovery, reads, startup and output still run.

## Correctness

- All 2,504 existing CLI checks and 29 module unit suites pass, including full/type-only local/alias type equality.
- `go_types_only_checks.py` adds 42 checks: frozen output in all modes, independent Go AST fixture type names/lines,
  owned type/alias audits, names/trees, file fields/extraction/docs, generics with bracket constraints, grouped/local
  types, bodyless functions, methods/function literals, CRLF/Unicode/invalid UTF-8, raw/long literals and deep blocks.
- Full directory output matches before/after in three modes at one/eight workers on both corpora. A separate
  assertions build with bounds checks, `AuditBraceCleaner`, `AuditBraceTokens` and `FFDTypeAudit` verifies complete
  core type bounds/kinds/order and aliases/targets against the full route, before annotation.
- Independent reference is Go 1.27.1 `go/parser` + `go/ast`, using the repository's retained native span oracle.
  All named `FuncDecl` and all `TypeSpec` entries, including local/grouped/generic declarations and aliases, are
  scored with duplicates and exact name/start/end. Build tags are not filtered; syntax, not semantic type checking,
  defines the reference. The assertion API probe checks full/type-only types and serializes each file before reusable
  token storage can be overwritten.

| Corpus | Accepted / excluded files | Functions | TypeSpecs | Missing / false / wrong spans |
|---|---:|---:|---:|---:|
| gin | 99 / 0 | 1,342 | 212 | 0 / 0 / 0 |
| encoding/json | 97 / 0 | 1,259 | 725 | 0 / 0 / 0 |

These are native finder API counts, not claims that the CLI's inherited line-based alias classification supports
every Go spelling. The CLI still separates aliases through its existing ASCII-name/single-line profile; generic,
Unicode and multiline alias classification limitations are unchanged. Full API types include all aliases regardless
of that separation. No shared tokenizer/cleaner or other language backend was changed.

## Evidence

Base revision: `3aa982a`, with recorded `go_port.d` and `ffd_langs.d` overlays.
gin pin: `0f09c3a9b4626d9fe9979cf4d0521d6ae32ab646`.
Standard-library corpus: `C:\Program Files\Go\src\encoding\json`, Go 1.27.1, actual per-file hashes retained;
this is an installed, non-Git corpus, not a historical pinned Go release tree.
Baseline executable SHA-256: `caa425ad141b58d8281d016888330da89d42f9c8da5646ad82248cada71bafae`.
Candidate SHA-256: `c1cc4b2a6a6ed36c46ab91be2a2f82d65c5663a9818afddca90895225595c10f`.

Ignored `build/go-types-only/` retains frozen binaries, baseline/current sources, oracle source/binary hashes,
build scripts, `verify.py`, `go_api_probe.d`, parser probe, manifests, complete output maps and correctness logs,
168 primary CLI/28 parser samples and 120 gin repeat samples. Input/binary hashes are verified after checks/timing.
Temporary gin clone is removed; the installed GOROOT is read only. The historical Go/Java entrypoint help was checked;
this Go-only change uses the independent Go reference directly, leaving Java and shared modules unchanged.

```powershell
Copy-Item benchmarks/go_ast_spans.go.txt build/go-types-only/go_ast_oracle.go
go build -o build/go-types-only/go-ast-oracle.exe build/go-types-only/go_ast_oracle.go
python benchmarks/ffd/build.py --test --out build/go-types-only/audit --flag=-boundscheck=on --flag=-d-version=AuditBraceCleaner --flag=-d-version=AuditBraceTokens --flag=-d-version=FFDTypeAudit
python benchmarks/ffd/go_types_only_checks.py --before build/go-types-only/before/ffd.exe --after build/go-types-only/after/ffd.exe --audit build/go-types-only/audit/ffd-test.exe --oracle build/go-types-only/go-ast-oracle.exe
```

Next: C#/Java consumer specialization, preserving the type scopes required by each existing traversal.

# D callgraph

Experimental third D tool, alongside `ffd` and `funcstats`. Production release binaries remain Go.
`deps` and `complexity` have not been ported.

## Build and use

```powershell
python benchmarks/ffd/build.py --tool callgraph
build/ffd/callgraph.exe --dir internal -l go --json
build/ffd/callgraph.exe --dir src --reverse --func Read
build/ffd/callgraph.exe --inp source.ts -l ts --func run --depth 2
```

The executable supports `--dir`, `--inp`, `-l`, `--json`/`-j`, `--reverse`/`-r`, `--func`, `--depth`,
`--no-gitignore`, `--version`, `--help`/`-h` and a positional directory. A single file requires its language;
a directory without `-l` scans all supported languages. Forced language applies to the full scan, including
the C/C++ `.h` extension override. File processing uses eight workers, sorted output and existing ignore rules.
Missing inputs, invalid languages/options, negative/noninteger depths and conflicting file/directory inputs fail.

## Shared implementation and graph contract

- `ffd_calls.d` is the call recognizer shared with `funcstats`. It reuses native tokens and signature metadata,
  excludes declaration occurrences rather than whole lines, retains recursion and Go generic/indexed calls, and
  rejects Go control words/interface signatures/composite-type conversions. JS private names and optional/non-null
  receivers are handled here, without changing the discovery/field token adapter.
- `cgraph_core.d` uses native function bounds and declaration-token locations to select named callers. Token pairs
  refine brace-body ends and same-line siblings; Python defaults/decorators belong to their enclosing execution
  scope rather than the function being declared. Named inner functions own their bodies; anonymous bodies inherit
  the surrounding named function. This is a syntax profile, not a complete model of runtime execution.
- Qualified calls are retained; bare calls must name a declaration somewhere in the scanned scope. A file run uses
  only that file's known names. Receiver chains use their immediate lexical identifier (`cfg.reader.Read` becomes
  `reader.Read`); computed receivers retain the terminal method as a bare name only if it is known. Go import aliases
  normalize to the imported path's basename, not a compiler-resolved package name.
- Each file retains the first source line for each caller/callee pair. Files and edges are sorted. Reverse queries
  deduplicate caller names across files; a bare `Read` also matches `reader.Read`, while a qualified query is exact.
- Depth filtering uses shortest-path distance, with zero meaning unlimited. Tree rendering uses explicit frames,
  marks cycles on the active path separately from previously shown branches, and needs no recursive graph walk.
- Result strings own storage before the next file overwrites thread-local lexer/finder buffers. Reading/parsing and
  pending call collection happen once per file job; global known-name filtering follows collection.

## Deliberate differences from Go

- File-mode `total_functions` contains the number of distinct discovered names (Go currently emits zero).
- Filtered JSON preserves original files and call-site lines; Go builds a synthetic `(filtered)` file with zero
  lines. The D known-name count describes the full scanned scope, including when the edge view is filtered.
- A depth limit follows shortest paths. Go's current DFS can first reach a shared node through a longer route and
  suppress edges that are within the requested distance through another route.
- D rejects malformed options instead of silently ignoring/coercing them. JSON key order/spacing need not match Go.
- All 15 native finder profiles are available, but inherited differences in function discovery and language syntax
  remain. Explicit edges/owners on small fixtures are stronger evidence than agreement with another implementation;
  the full native AST scoring below covers Go only.

## Limits

Names are not resolved by package, receiver type, overload, dynamic dispatch or call-site data flow. Same-named
functions/methods merge, and mixed-language directory scope also merges bare names. Unknown bare external calls,
dynamic/computed property names and Ruby/Scala calls without parentheses are outside this initial graph profile.
Non-dot namespace syntax is retained only through known terminal names; Rust macros are not expanded. Go import
path basenames may differ from declared package names; complex commented/semicolon-packed import spellings are
not independently covered. Native parser literal/macro/conditional-code exclusions still apply. Full semantic
call coverage and independent 15-language corpus accuracy are not claimed.

Caller attribution uses native line bounds plus token refinement, not compiler-provided source offsets for every
language. Dense expression/signature combinations outside the explicit tests need independent checking. The
current pipeline still tokenizes for discovery and again for call recognition; sharing those immutable token
results is a future performance step. Caller selection also scans native regions per candidate; profile large
files before choosing an index.

## Verification, 2026-10-21

```powershell
python benchmarks/ffd/build.py --tool callgraph --test
python benchmarks/ffd/callgraph_checks.py --d build/ffd/callgraph.exe --go callgraph.exe
python benchmarks/ffd/funcstats_checks.py --d build/ffd/funcstats.exe --go funcstats.exe
go test ./benchmarks/ffd/... ./cmd/funcstats
```

- 21 imported D modules pass unit tests; the normal `-unittest` executable runs tests only. To exercise its CLI too,
  pass `--DRT-testmode=run-main` directly to that executable.
- 66 callgraph checks cover explicit edges across 15 languages, duplicate calls, same-line JS neighbors/inner
  functions/arrows, private/optional calls, TS return-type braces, C++ constructor initializer braces, Python nested/default-expression owners,
  recursion/revisits, shortest-path diamond graphs, reverse matching, Go aliases/generics/Unicode/CRLF/literal
  guards, cross-file names, forced-language filtering, `.h`, ignore rules, many-file buffer reuse, deterministic
  output and error paths. The simple 15-language fixture edges also agree with Go.
- 189 funcstats checks cover its previous 15 profiles plus private JS calls/uses; shared recognizer extraction
  preserves its other existing controlled results.
- `callgraph_go_ast.go.txt` independently derives the defined syntax profile from Go `ast.CallExpr` inside named
  declarations, including function literals within those bodies. It unwraps generic/indexed callees, normalizes
  immediate/computed receivers explicitly and deduplicates by caller/callee. It parses every supplied file, ignores
  build tags and does not perform type checking. Parse failures are errors, not silent exclusions.
- On this repository's Go source subtrees, both implementations match all 3907 AST edges in 137 `internal/` files
  and all 778 edges in 30 `cmd/` files, including caller names and first-call lines: **4685 edges, 0 missing, 0 extra**.
  These are two separate scan scopes; cross-scope calls are not combined into a single whole-project truth set.

Base revision is `5c989f70cefb96b04a8ce1aecf1c4dd0e99ea2c5` plus the working-tree Go keyword fix and D additions.
This is not an untouched release comparison. Source hashes/snapshots, exact binary hashes/copies, AST version/hash,
maps and raw samples are retained under ignored `build/d-callgraph-port-20261021/{internal,cmd}-final/`.

Reproduce scoring and complete CLI measurements:

```powershell
python benchmarks/ffd/callgraph_compare.py --root internal --d build/ffd/callgraph.exe --go callgraph.exe --out build/callgraph-internal --time 9
python benchmarks/ffd/callgraph_compare.py --root cmd --d build/ffd/callgraph.exe --go callgraph.exe --out build/callgraph-cmd --time 9
```

The evaluator takes Git-visible Go sources as AST inputs and the normal directory walk as CLI inputs. Differences
in scope remain visible in the edge deltas; the measured subtrees have matching scopes. It warms each variant,
shuffles nine paired rounds, verifies each variant's output every round, and rechecks saved source/binary bytes.
These are process startup + traversal + parsing + JSON timings, not parser-only timings or a language comparison.
Host: Windows amd64, 32 logical CPUs; Go 1.25.7, LDC 1.43.0, default D eight workers vs Go runtime CPU default.
Release D flags are `-O3 -release -mcpu=x86-64`.

The D executable is 1,769,984 bytes vs Go's 3,111,424 bytes (about 43% smaller), while supporting a different
lexical/parser implementation and the deliberate CLI differences above. Final medians, nine paired rounds:

| Scope | Files | AST edges | Go ms | D ms |
|---|---:|---:|---:|---:|
| internal | 137 | 3907 | 40.50 | 43.40 |
| cmd | 30 | 778 | 18.45 | 19.19 |

There is **no speed gain** on these small, startup-sensitive inputs. Raw samples in the saved reports are
preferable to generalizing to larger projects. The assertions-enabled callgraph CLI additionally produces the
same 3907 internal edges as the release build; D funcstats passes its 22-module unit build after integration.

DOX ownership/contracts and the parent child index are updated. Root instructions and other domains retain their
existing structure and responsibilities; they need no changes for this addition under `benchmarks/ffd/`.

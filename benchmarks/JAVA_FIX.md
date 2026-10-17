# Production Java parser fix — 2026-10-17

The production Java path now uses a linear token walk with an explicit brace
stack. This is independent of the test-only recursive pilot. The fix started
on 2026-10-16; artifacts retain that date in `build/java-fix-20261016/`.

## Behavior

The old line scan skipped methods of anonymous/local classes, missed annotated
or generic multiline declarations and sometimes treated expressions as headers.
`internal/java_finder.go` distinguishes type/member bodies from executable
blocks, validates return types and retains bodyless members, constructors,
compact record constructors, enum constant bodies and annotation defaults.
Java-specific identifier/keyword rules retain dollar names and methods named
`delegate`. Annotation argument braces and default-value arrays do not close a
method body. Extraction retains original lines and leading annotations; type
starts use the keyword line so directory type discovery deduplicates entries.

Bracket pairing and source coordinates share `braceTokens` with C#. C# calls
the original lexical mode; Java explicitly enables dollar identifiers. Both
finders provide declaration coordinates to `callgraph` and `funcstats` so split
signatures do not create self-calls, while actual recursion still counts.
Nested Java calls belong to the innermost named method.

## Independent quality

Base revision `8e5cf6757ba4c96946121e13285c9f7241b9e9a0` plus the working-tree
fix. Gson pin `e2292f9c9f2e07a4791827e8eea34aa7d80545d6`, all 264 tracked Java
files parse successfully. Python 3.14.6, tree-sitter 0.26.0,
tree-sitter-java 0.23.5. The reference includes methods, explicit/compact
constructors and annotation elements, including 52 bodyless declarations.
Implicit constructors and anonymous lambdas are outside named-function truth.

| Function metric | Before | After |
|---|---:|---:|
| Reference declarations | 3492 | 3492 |
| Matched declarations | 3125 | 3492 |
| Missing | 367 | 0 |
| False functions | 55 | 0 |
| Wrong end lines among matches | 0 | 0 |

The current directory map also matches all 767 named classes/interfaces/enums/
records/annotation types by name and header location with zero missing, false
or duplicate entries. `check_types.py` and `types.json` retain that separate
audit in the artifact directory. It does not score type end boundaries. The
old map had 252 type entries; the frozen recursive pilot has 798, including
duplicates. Whole-map equivalence across variants is not assumed.

These results describe this pinned syntax corpus, not all Java projects or
build configurations.

## Timing

Ryzen 9 5950X, 16 cores / 32 logical CPUs, Windows amd64, Go 1.25.7. Compare
the frozen pre-fix production and recursive-pilot binaries with the final fix,
all built using `-trimpath -buildvcs=false`. Twenty shuffled sample rounds,
seed 20261016, two warmups per case, `GOMAXPROCS=32`, warm caches. Commands use
`--dir <gson> --all --json --no-gitignore --workers <count>`. Times include
startup, walking/reads, complete type/function discovery and captured JSON.

| File workers | Variant | Median, ms | Observed min–max, ms |
|---:|---|---:|---:|
| 1 | Before | 179.799 | 172.57–272.57 |
| 1 | Fixed production | 181.555 | 174.22–403.41 |
| 1 | Frozen recursive pilot | 184.339 | 179.51–308.57 |
| 32 | Before | 70.501 | 65.56–2267.45 |
| 32 | Fixed production | 113.450 | 89.50–167.22 |
| 32 | Frozen recursive pilot | 92.504 | 87.92–207.44 |

This final run is noisy, including a 2.27-second outlier in the old parser.
It does not establish a precise performance ratio or a reliable advantage
over the recursive pilot. The complete correct path costs more than the old
incomplete parser, and its type/function output also differs. These numbers
do not isolate the cost of traversal. All samples and outliers are preserved;
output is deterministic within each variant. JSON omits files with no symbols,
so its reported file count is not the number of tracked Java inputs.

The timing binary SHA-256 is
`fb29ac268e1b27a26b138eb94ed593b608bd52428251937672f8723432b1bef7`.
The same binary passed the final independent function score. Source/binary
hashes, exact commands, map counts, CPU readings and samples are in
`comparison.json`; CPU readings average the whole run and are not peak usage.

## Reproduction and verification

```bash
go build -trimpath -buildvcs=false -o build/java-fix-20261016/funcfinder.exe ./cmd/funcfinder
python benchmarks/scala_d_tree_eval.py --language java \
  --root build/brace-pilot-20261016/repos/gson --label gson-java-fix \
  --binary build/java-fix-20261016/funcfinder.exe \
  --module-dir build/eval-runtime --output build/java-fix-20261016/gson.json
python benchmarks/java_finder_compare.py \
  --root build/brace-pilot-20261016/repos/gson \
  --before build/brace-pilot-20261016/funcfinder-baseline.exe \
  --after build/java-fix-20261016/funcfinder.exe \
  --pilot build/brace-pilot-20261016/funcfinder-pilot.exe \
  --work-dir build/java-fix-20261016 --samples 20 --warmups 2
```

Use the pinned binaries from `build/brace-pilot-20261016/build.json` as the
before/pilot controls. Do not rebuild the before control from the fixed tree.
Reference runtime installation is documented in `README.md`.

Passed: `go test ./... -count=1`, `go vet ./...`,
`go test -race ./internal ./cmd/funcstats -count=1`, and `build.ps1` for all five
shipped tools. Tests pin original extraction, offsets, selected-name mode,
anonymous/local/enum contexts, annotations, Java identifiers and nested caller
attribution. All five root executables were rebuilt with the normal versioned
build script. Race tests used the working MSYS2 UCRT64 GCC/cgo toolchain outside
the restricted Windows sandbox.

On the same pinned Polly/Dapper checkouts, complete directory JSON is byte
identical before and after the shared tokenizer refactor. This regression
check does not re-score disabled C# branches or broaden the earlier Roslyn
coverage claim. `csharp-regression.json` records the pins and output hashes.
The verification logs are `tests.txt`, `vet.txt`, `race.txt` and `build.txt`.

DOX: updated `internal/AGENTS.md`, `cmd/AGENTS.md`, `benchmarks/AGENTS.md`,
`TODO.md`, user measurement documentation and `CHANGELOG.md`. Root and other
child contracts and all child indexes remain unchanged because the hierarchy,
public API and CLI flags are unchanged. The recursive experiments remain
test-only; only the independent production Java parser is enabled.

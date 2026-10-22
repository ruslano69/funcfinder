# cmd/

## Purpose

CLI entrypoints for the funcfinder toolkit. Each subdirectory contains a single `main.go` that wires flags to the `internal` package and writes output to stdout. Five binaries ship via `build.sh`; the rest are internal dev/benchmark tools.

## Ownership

- Each `cmd/<tool>/main.go` owns its own flag definitions, usage text, and exit codes.
- Business logic lives in `internal/`; `cmd/` must not duplicate it.

## Local Contracts

- All tools accept `--json` for machine-readable output.
- `--dir` mode processes a directory tree; `--inp` mode processes a single file and requires `--source <lang>`.
- Exit code 0 = success, non-zero = error. Legacy diagnostics use stderr; JSON object modes return structured errors on stdout.
- Binary names match directory names: `cmd/funcfinder` → binary `funcfinder`.

## Work Guidance

- When adding a flag, add it to the tool's `main.go` only; propagate to `internal` via an existing config struct.
- Type edit commands forward `--line` and `--old` to the shared edit request. `--old` is valid for replacement only; insertion/removal reject it. Type selection/validation remains in `internal`, restricted to Go.
- `funcfinder --read-object NAME` / `--edit-object FILE|-` are Go/Rust/C#/D/JS/TS JSON object modes (`object.go`). Dispatch precedes legacy input validation. `object_decode.go` strictly distinguishes direct objects, operation wrappers and single-file batches; direct replacement keeps its response shape. Strict JSON, disjoint flag sets and stable error envelopes support automation. Preview is default; apply stages/syncs a same-directory file, rechecks bytes and renames. Validation labels distinguish Go syntax from Rust/C#/D/JS/TS token/item checks. Regular files only, no cross-process lock. `object_test.go` checks stale-write refusal and staging cleanup.
- Keep `main.go` files thin: parse flags → call internal → handle error → exit.
- Do not add business logic to `main.go`.
- `funcstats` consumes the finder's signature coordinates/token for C#, Java and JavaScript, excluding only the declaration occurrence from call counts; the configured-pattern fallback applies to other languages. A real recursive call must still count, including on a split header's name line or the declaration's own line. JS anonymous variable function/arrow values have no name/parenthesis declaration token and must not lose recursive calls on their header line.

## Verification

```bash
./build.sh          # compiles all five shipped binaries
./funcfinder --help # spot-check flag registration
```

On sandboxed Windows, if the system temporary directory denies replacement renames, run tests with `TMP` and `TEMP` pointing to an existing writable directory under ignored `build/`. The object write test must still run.

## Child DOX Index

- `cmd/funcfinder/` — primary tool: function/type mapping, extraction, split-shard output
  (also `--replace-func NAME --source LANG --with FILE|- [--line N] [--old FILE] [--apply]`: one function replaced by file, without a shell; preview unless `--apply`; `--add-after NAME` inserts the functions from `--with` after it; `--remove-func NAME` deletes one)
- `cmd/funcstats/` — call counts and file statistics; `--uncalled` lists candidates to delete. See [docs/FUNCSTATS.md](../docs/FUNCSTATS.md)
- `cmd/deps/` — import dependency and inter-shard graph
- `cmd/callgraph/` — forward/reverse call graph traversal
- `cmd/complexity/` — cognitive complexity scoring per function, breakdown flags and their limits — see [cmd/complexity/AGENTS.md](complexity/AGENTS.md)
- `cmd/benchmark/` — internal throughput benchmark, not a user-facing tool
- `cmd/astoracle/` — Go-only ground-truth symbol oracle (go/ast) for benchmarking funcfinder's regex output; not shipped

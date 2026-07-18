# cmd/

## Purpose

CLI entrypoints for the funcfinder toolkit. Each subdirectory contains a single `main.go` that wires flags to the `internal` package and writes output to stdout. Five binaries ship via `build.sh`; the rest are internal dev/benchmark tools.

## Ownership

- Each `cmd/<tool>/main.go` owns its own flag definitions, usage text, and exit codes.
- Business logic lives in `internal/`; `cmd/` must not duplicate it.

## Local Contracts

- All tools accept `--json` for machine-readable output.
- `--dir` mode processes a directory tree; `--inp` mode processes a single file and requires `--source <lang>`.
- Exit code 0 = success, non-zero = error (printed to stderr).
- Binary names match directory names: `cmd/funcfinder` → binary `funcfinder`.

## Work Guidance

- When adding a flag, add it to the tool's `main.go` only; propagate to `internal` via an existing config struct.
- Keep `main.go` files thin: parse flags → call internal → handle error → exit.
- Do not add business logic to `main.go`.

## Verification

```bash
./build.sh          # compiles all five shipped binaries
./funcfinder --help # spot-check flag registration
```

## Child DOX Index

- `cmd/funcfinder/` — primary tool: function/type mapping, extraction, split-shard output
- `cmd/stat/` — call frequency analysis for a single file
- `cmd/deps/` — import dependency and inter-shard graph
- `cmd/callgraph/` — forward/reverse call graph traversal
- `cmd/complexity/` — cognitive complexity scoring per function
- `cmd/benchmark/` — internal throughput benchmark, not a user-facing tool
- `cmd/astoracle/` — Go-only ground-truth symbol oracle (go/ast) for benchmarking funcfinder's regex output; not shipped

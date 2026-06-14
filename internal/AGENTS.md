# internal/

## Purpose

Core parsing engine. Implements function/struct discovery, body extraction, call-graph analysis, dependency resolution, shard splitting, output formatting, and incremental update logic for all 15+ supported languages.

## Ownership

- All language-specific parsing logic lives here.
- `cmd/` packages are consumers only; they must not bypass the public API of this package.
- Test files (`*_test.go`) are co-located with the code they test.

## Local Contracts

- Public API surface: `FindFunctions`, `FindStructs`, `ProcessDirectory`, `BuildCallGraph`, `WriteSplitOutput`, `WriteSplitOutputIncremental`.
- Language dispatch is handled by `finder_factory.go` (functions) and `struct_finder_factory.go` (structs); new languages must register here.
- `languages.json` is the canonical list of supported file extensions → language identifiers.
- Shard output goes to `.codemap/` by default; manifest is `.codemap/manifest.json`.
- Incremental mode (`--inc`) uses xxh3 checksums (`checksum_xxh3.go`) with stdlib fallback (`checksum_stdlib.go`).

## Work Guidance

- Adding a new language: implement the `Finder` interface, register in `finder_factory.go` and `struct_finder_factory.go`, add the extension in `languages.json`, add test fixtures in `test_examples/`.
- `enhanced_sanitizer.go` strips string literals and comments before parsing to prevent false positives; touch it carefully and run its bench test after changes.
- `python_finder.go` and `python_lines_processor.go` handle Python's indent-based scope; keep them in sync.
- Call graph logic (`callgraph.go`) operates on the output of the finder layer, not on raw source.
- `shardutil.go` owns all `.codemap/` path conventions; do not hardcode shard paths elsewhere.

## Verification

```bash
go test ./internal/...          # run all unit tests
go test -bench=. ./internal/... # run benchmarks (includes enhanced_sanitizer)
```

## Child DOX Index

No child AGENTS.md files. All internal sub-concerns are documented in this file.

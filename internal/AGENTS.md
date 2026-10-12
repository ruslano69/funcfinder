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
- Both Python paths share logical-statement boundaries: continuation lines and string contents do not dedent a scope, suite colons are read outside brackets, and inline suites end with their logical statement. Finder class ranges use the same rules; extraction retains original source and trailing trivia.
- PHP uses the nested finder so methods of classes inside functions are visible. `php_heredoc.go` masks heredoc/nowdoc text before function/class discovery, preserves trailing code after closing labels and restores original source for function extraction, including offset scans.
- D preparation in `d_lines.go` masks nested comments/literals and contracts, rejects statement/type headers as functions, and preserves original extraction. D uses nested discovery; class ranges must also close correctly for inline bodies. Reserved words and types are excluded as names.
- Scala preparation and endpoint checks in `scala_lines.go` handle braceless expressions, bodyless declarations, multiline signatures/literals, symbol-vs-char quotes and inline members. Function recognition stays configured in `languages.json`; shared nested-parser and sanitizer implementations are unchanged.
- Call graph logic (`callgraph.go`) operates on the output of the finder layer, not on raw source.
- Reverse lookup uses `ReverseCallers`: bare names include qualified callees with the same final name; qualified queries match exactly. Results are sorted, deduplicated name-based candidates, not type-resolved method identities. The full reverse graph preserves qualified keys.
- `shardutil.go` owns all `.codemap/` path conventions; do not hardcode shard paths elsewhere.

## Verification

```bash
go test ./internal/...          # run all unit tests
go test -bench=. ./internal/... # run benchmarks (includes enhanced_sanitizer)
```

## Child DOX Index

- `internal/knowledge/` — SQLite knowledge base: FTS5 + vector hybrid search for agent memory — see [internal/knowledge/AGENTS.md](knowledge/AGENTS.md)

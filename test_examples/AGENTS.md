# test_examples/

## Purpose

Multi-language source fixtures used by parser unit tests in `internal/`. Each file exercises the language-specific finder and struct-finder logic for its extension.

## Ownership

- One fixture file per language: `test_example.<ext>`.
- Stress-test fixtures (`test_stress_*.{cpp,java,py}`) exercise parser performance and correctness at scale.
- Struct fixtures (`test_structs.*`) exercise struct/class/type extraction.
- JS-specific fixtures (`test_generators_arrows.js`, `test_edge_cases.cpp`) cover tricky syntax edge cases.

## Local Contracts

- Fixture files are consumed by `internal/*_test.go` tests via relative paths; do not rename or move them without updating the corresponding test.
- When adding a new language, add `test_example.<ext>` here and reference it from `internal/finder_test.go` or `internal/struct_finder_test.go`.
- Fixtures must contain enough representative syntax (functions, methods, classes/structs, comments, string literals) to exercise the sanitizer and finder together.

## Work Guidance

- Add edge-case fixtures when a parser bug is found; the fixture becomes the regression test.
- Keep fixtures minimal but representative — do not add dead code unrelated to the parser under test.

## Verification

```bash
go test ./internal/...
```

## Child DOX Index

No child AGENTS.md files.

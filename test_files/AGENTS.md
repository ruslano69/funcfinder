# test_files/

## Purpose

Edge-case source fixtures that specifically target tricky literal and comment forms: Go raw strings, Python docstrings, and C# verbatim string literals. Used to verify the enhanced sanitizer handles them correctly.

## Ownership

- `test_go_raw_strings.go` — Go backtick raw string edge cases.
- `test_python_docstrings.py` — Python triple-quoted docstring edge cases.
- `test_csharp_verbatim.cs` — C# `@"..."` verbatim literal edge cases.

## Local Contracts

- These fixtures target `internal/enhanced_sanitizer.go` specifically.
- Referenced by `internal/enhanced_sanitizer_test.go`; do not rename without updating that test.
- When a new sanitizer edge case is discovered, add a minimal fixture here before fixing the sanitizer.

## Work Guidance

- Each fixture should contain the minimal code needed to reproduce the edge case — no unrelated functions.
- Pair every new fixture with a test case in `enhanced_sanitizer_test.go`.

## Verification

```bash
go test ./internal/... -run TestEnhancedSanitizer
```

## Child DOX Index

No child AGENTS.md files.

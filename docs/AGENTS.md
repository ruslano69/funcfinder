# docs/

## Purpose

User-facing documentation: use cases, CI/CD integration guide, Windows setup, enhanced sanitizer explanation, and shell script examples.

## Ownership

- Markdown files here describe stable user-facing behavior.
- `archive/` holds historical analysis reports and completed investigation notes — do not update them; they are read-only records.
- `examples/` holds runnable shell scripts demonstrating tool combinations.

## Local Contracts

- Docs must stay consistent with current flag names and output formats in `cmd/` and `internal/`.
- When a flag is renamed or removed, update affected docs before closing the task.
- Shell scripts in `examples/` must use the binary names produced by `build.sh`.

## Work Guidance

- Add new docs for features that have stable, user-facing contracts.
- Keep `archive/` untouched; add a note in the relevant live doc instead of editing archived files.

## Verification

No automated check. Manually verify that commands in shell scripts match current binary interfaces after any CLI change.

## Child DOX Index

- `docs/archive/` — read-only historical reports; no edits expected
- `docs/examples/` — runnable shell script demos; keep in sync with current CLI flags

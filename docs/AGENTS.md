# docs/

## Purpose

User-facing documentation: CI/CD integration guide (`CI_CD.md`), Windows setup guide (`WINDOWS.md`), the docsearch knowledge-base guide (`DOCSEARCH.md`), and the `docsearch-server/` subtree — the full spec, differentiation, and usage guide for the versioned truth server.

## Ownership

- Markdown files here describe stable user-facing behavior.
- `examples/` holds runnable shell scripts demonstrating tool combinations.

## Local Contracts

- Docs must stay consistent with current flag names and output formats in `cmd/` and `internal/`.
- When a flag is renamed or removed, update affected docs before closing the task.
- Shell scripts in `examples/` must use the binary names produced by `build.sh`.

## Work Guidance

- Add new docs only for features with stable, user-facing contracts.
- Do not duplicate content that already lives in the root `AGENTS.md`.

## Verification

No automated check. Manually verify that commands in shell scripts match current binary interfaces after any CLI change.

## Child DOX Index

- `docs/examples/` — runnable shell script demos; keep in sync with current CLI flags
- `docs/docsearch-server/` — versioned truth server docs: `TZ.md` (spec, FR-1..FR-22 all implemented), `DIFFERENTIATION.md`, `HOW_TO_USE.md` (agent-facing grounding guide)

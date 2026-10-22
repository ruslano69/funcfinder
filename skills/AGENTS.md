# skills/

## Purpose

Portable Codex/Claude Code skill definitions that teach the agent how to use the funcfinder toolkit's binaries during coding sessions.

## Ownership

- `funcfinder.md` defines the invocation pattern and workflow steps for code navigation (map/extract/callgraph/deps/complexity) and guarded source editing (JSON objects and legacy text commands).

- `funcfinder/` is the installable package: `SKILL.md` is the entrypoint; `references/edit.md` mirrors root `docs/EDIT.md` for standalone use.

## Local Contracts

- Keep `funcfinder/references/edit.md` synchronized with `docs/EDIT.md` whenever the editing contract changes. Validate installable frontmatter with the system skill-creator validator and verify referenced files exist before deployment.
- Each skill file must reflect the current CLI flags and output format of its tool.
- When a tool's API changes (new flags, renamed flags, JSON schema changes), update the matching skill file in the same commit.

## Work Guidance

- Keep the skill file concise and action-oriented; agents execute it, they do not read it for background.
- Mirror the "Investigation Workflow" structure from the root AGENTS.md so behavior is consistent.

## Verification

Run the system skill-creator `scripts/quick_validate.py` against `skills/funcfinder/`. Verify its reference links and editing reference match `docs/EDIT.md`; manually check commands against current binary help after CLI changes.

## Child DOX Index

No child AGENTS.md files.

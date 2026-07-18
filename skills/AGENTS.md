# skills/

## Purpose

Claude Code skill definitions that teach the agent how to use the funcfinder toolkit's binaries during coding sessions.

## Ownership

- `funcfinder.md` defines the invocation pattern and workflow steps for code navigation (map/extract/callgraph/deps/complexity).

## Local Contracts

- Each skill file must reflect the current CLI flags and output format of its tool.
- When a tool's API changes (new flags, renamed flags, JSON schema changes), update the matching skill file in the same commit.

## Work Guidance

- Keep the skill file concise and action-oriented; agents execute it, they do not read it for background.
- Mirror the "Investigation Workflow" structure from the root AGENTS.md so behavior is consistent.

## Verification

No automated check. Manually verify the skill commands match current binary output after CLI changes.

## Child DOX Index

No child AGENTS.md files.

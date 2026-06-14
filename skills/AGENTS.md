# skills/

## Purpose

Claude Code skill definition that teaches the agent how to use funcfinder during coding sessions.

## Ownership

- `funcfinder.md` is the single skill file; it defines the invocation pattern and workflow steps for the `/funcfinder` slash command inside Claude Code.

## Local Contracts

- The skill file must reflect the current CLI flags and output format.
- When the funcfinder API changes (new flags, renamed flags, JSON schema changes), update `funcfinder.md` in the same commit.

## Work Guidance

- Keep the skill file concise and action-oriented; agents execute it, they do not read it for background.
- Mirror the "Investigation Workflow" structure from the root AGENTS.md so behavior is consistent.

## Verification

No automated check. Manually verify the skill commands match current binary output after CLI changes.

## Child DOX Index

No child AGENTS.md files.

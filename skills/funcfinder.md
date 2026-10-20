---
name: funcfinder
description: "Use this skill whenever the user wants to analyze a GitHub repository or local project: explore code structure, find functions or methods, understand architecture, detect dependencies, measure complexity, or navigate large codebases efficiently. Triggers on: analyze this repo, what functions are in, show me the structure, find all methods, explore this codebase, any GitHub URL + code analysis, refactor this project, where is this logic, how does X work in this codebase. Use funcfinder tools to get 95%+ token reduction vs reading raw files. ALWAYS use this skill before opening any source file in Go, C, C++, C#, Java, D, JavaScript, TypeScript, or Python - map first, read only what you need. Also covers doc comments and Python docstrings without bodies, call graphs, complexity limits and summaries, unused functions, and the function behind each grep hit."
---

# Skill: funcfinder — Codebase Investigation

Use this skill when you need to navigate, analyze, or understand an unfamiliar codebase without wasting tokens reading source files directly.

---

## When to invoke

- "find the function that handles X"
- "what calls Y / what does Y call"
- "map the codebase" / "explore the project"
- "which files are responsible for Z"
- "impact analysis: what breaks if I change W"
- any task requiring you to locate code before reading it

---

## Decision tree

```
Is the project already indexed (.codemap/ exists)?
  YES → cat .codemap/manifest.json → load relevant shard → done
  NO  → how many files?
          < 50  → funcfinder --dir . --all --json > map.json
          50+   → funcfinder --dir . --all --json --split
```

---

## Phase 1 — Orient (always do this first)

### Small project
```bash
funcfinder --dir . --all --json > map.json
grep -i "<keyword>" map.json
```

### Large project
```bash
funcfinder --dir . --all --json --split
cat .codemap/manifest.json
```

Manifest tells you: shard names, file counts, function counts, `depends_on` links.
**Read manifest before loading any shard.** It costs ~50 tokens, saves you from loading 100KB+.

---

## Phase 2 — Locate

```bash
# From full map
grep -i "auth" map.json
# → auth/handler.go:42: AuthenticateUser

# From shard (large project)
cat .codemap/internal_auth.json
grep -i "auth" .codemap/internal_auth.json
```

Names only (one line each: kind, name, file:line) — when you need the list, not the bodies:
```bash
funcfinder --dir . --names | grep -i "auth"
funcfinder --dir . --names --all      # adds types and Go aliases
```

Use `jq` when grep isn't precise enough:
```bash
jq '.files[] | .path as $p | .functions[] | select(.name | contains("Auth")) | {path: $p, name, line}' map.json
```

---

## Phase 3 — Extract

```bash
# Extract function body (the only time you need to read actual code)
funcfinder --inp auth/handler.go --source go --func AuthenticateUser --extract

# Extract multiple functions
funcfinder --inp auth/handler.go --source go --func "AuthenticateUser,ValidateToken" --extract

# Extract a type/struct
funcfinder --inp auth/handler.go --source go --struct "User,Session" --extract

# Doc comment and first line of a declaration, without the body
funcfinder --inp auth/handler.go --source go --doc AuthenticateUser
```

**Doc first**: `--doc Name` gives the docstring (Python `def`/`class`) or the comment above a declaration, and its first line, without the body. Use it to learn what a function is for; read the body with `--extract` only when the code itself is needed.

**Rule**: extract only the specific function you need, not the whole file.

---

## Phase 4 — Trace calls (when you need to understand dependencies)

```bash
# What does AuthenticateUser call?
callgraph --inp auth/handler.go -l go --func AuthenticateUser

# Full call tree with depth limit
callgraph --dir . -l go --func AuthenticateUser --depth 3

# Who calls AuthenticateUser? (impact analysis)
callgraph --dir . -l go --reverse --func AuthenticateUser
```

Combine with deps for import-level context:
```bash
deps auth/ -l go --json
```

---

## Phase 5 — Deep architecture (optional, once per project)

Run once, reuse across sessions:

```bash
# Build shard map
funcfinder --dir . --all --json --split --no-gitignore

# Add inter-shard dependency graph
deps . -l go --shards --no-gitignore --update-manifest .codemap/manifest.json

# Now manifest.json has depends_on per shard:
# cmd_funcfinder.json → depends_on: [internal.json]
# → agent knows: to understand cmd/funcfinder, load internal shard too
```

For TypeScript/Next.js projects (auto-detects @/ alias from tsconfig.json):
```bash
deps frontend -l ts --shards --update-manifest .codemap/manifest.json
```

For Rust workspaces (crate names come from each `Cargo.toml`), run from the workspace root:
```bash
deps . -l rust --shards --update-manifest .codemap/manifest.json
```

**Trust check** — if `deps --shards` prints `WARNING: only N/M (X%) intra-project
imports resolved` (or flags a `tsconfig.json` found *above* the analyzed root),
the graph is likely wrong, not just sparse: the directory was rooted below
`go.mod`, `tsconfig.json` or `Cargo.toml`, so relative, alias or crate imports can't resolve. Re-run from
the actual project root. A shard graph that comes back all-leaves (no
`depends_on`) with no warning means the project genuinely has no internal deps.

---

## Supporting tools

```bash
# Which functions are called most often (hotspots)
funcstats internal/finder.go -l go
# Candidates to delete: functions no code line in the tree names (fixtures excluded)
funcstats --dir . --uncalled --exclude "test_examples/*"

# Cognitive complexity — find the hard functions
complexity internal/dirprocessor.go -l go --nosimple
# Figures behind each score and the flags it passes: a probe for where to read first, not a verdict
complexity internal/dirprocessor.go -l go --breakdown
# Set a limit or turn its flag off (0): -len lines, -if decisions, -deep branch depth, -loop loop depth
complexity internal/dirprocessor.go -l go --breakdown -len 200 -loop 0

# Tracking a goal: counts per level and per limit (keep the limits fixed between runs)
complexity . -l go -summary -exclude "*_test.go"

# Boolean chains: lines with more than N operators (&&, ||, and, or)
complexity internal/dirprocessor.go -l go --breakdown -bool 2

# Which function holds each grep hit (experimental): reads grep -n output on stdin
grep -rn "<name>" . --include=*.go | funcfinder --enclose

# Import graph for one file
deps internal/finder.go -l go --json
```

---

## Token budget reference

| Action | Tokens spent | When to use |
|--------|-------------|-------------|
| `cat manifest.json` | ~50-200 | Always — free orientation |
| `grep map.json` | ~10 | Faster than jq for simple lookups |
| `cat shard.json` | ~1000-4000 | When you know which module |
| `--names` | ~10-15 per name | List what exists in a file or project |
| `--doc` one name | ~20-100 (depends on the comment) | Learn what a function is for |
| `--extract` one function | ~50-500 | When you know which function |
| `callgraph --reverse` | ~100 | Impact analysis before editing |
| `grep ... \| funcfinder --enclose` | ~20 per hit | Name the function behind each grep hit (experimental) |
| Read full source file | ~500-5000 | Almost never needed |

---

## Workflow example: "find and fix the bug in authentication"

```bash
# 1. Locate (~50 tokens)
funcfinder --dir . --all --json --split
cat .codemap/manifest.json
# → see internal_auth.json shard

# 2. Inspect shard (~1000 tokens)
cat .codemap/internal_auth.json
# → AuthenticateUser at auth/handler.go:42

# 3. Extract the function (~200 tokens)
funcfinder --inp auth/handler.go --source go --func AuthenticateUser --extract

# 4. Check impact before fixing (~100 tokens)
callgraph --dir . -l go --reverse --func AuthenticateUser
# → called by: LoginHandler, RefreshHandler, AdminMiddleware

# Total: ~1350 tokens vs ~15000 reading files directly
```

---

## Common pitfalls

| Mistake | Fix |
|---------|-----|
| Reading source files to find functions | Use `funcfinder --dir` first |
| Loading full JSON for large project | Use `--split`, read manifest first |
| `deps`/`callgraph` misses `cmd/` | Add `--no-gitignore` |
| `--split` without `--json` | Always pair: `--json --split` |
| Calling `--extract` without knowing the line | Use `--names` or `--map` first, then `--func Name --extract` |
| Wanting the purpose of a function from its body | Use `--doc Name`: it shows the docstring or the comment above the declaration |
| Expecting Go methods as `method` in `--names`/`--doc` | They show as `function` for now: the receiver is not recorded |
| A grep hit in a comment, and you need its function | `grep ... \| funcfinder --enclose`: a comment above a function names that function and prints its doc |
| `deps --shards` returns an empty/all-leaf graph | You rooted below `go.mod`/`tsconfig.json` — heed the resolution-% WARNING and re-run from the real root |

---

## Quick reference card

```bash
# Orient
funcfinder --dir . --all --json --split && cat .codemap/manifest.json

# Find
grep -i "<name>" .codemap/<shard>.json
funcfinder --dir . --names | grep -i "<name>"    # kind, name, file:line (add --all for types and aliases)

# Extract
funcfinder --inp <file> --source <lang> --func <Name> --extract
funcfinder --inp <file> --source <lang> --doc <Name>    # docstring or comment + first line, no body

# Impact
callgraph --dir . -l <lang> --reverse --func <Name>

# Architecture
deps . -l <lang> --shards --no-gitignore --update-manifest .codemap/manifest.json
```

---

## See also

The `.codemap/` this skill builds is ephemeral, per-session scratch. For a
**persistent, versioned** structural map — one compiled into an immutable
release and shared across a team/agents — the
[distill-docs](https://github.com/ruslano69/distill-docs) project's
`distill-server publish --code-dir` bakes the same funcfinder function/type map
(via funcfinder's public `analyze` API) into a truth release, tagged with its
git commit. Use this skill to investigate now; use that to freeze what you
learned as durable, queryable truth.

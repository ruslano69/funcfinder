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
```

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

---

## Supporting tools

```bash
# Which functions are called most often (hotspots)
stat internal/finder.go -l go

# Cognitive complexity — find the hard functions
complexity internal/dirprocessor.go -l go --nosimple

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
| `--extract` one function | ~50-500 | When you know which function |
| `callgraph --reverse` | ~100 | Impact analysis before editing |
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
| Calling `--extract` without knowing the line | Use `--map` first, then `--func Name --extract` |

---

## Quick reference card

```bash
# Orient
funcfinder --dir . --all --json --split && cat .codemap/manifest.json

# Find
grep -i "<name>" .codemap/<shard>.json

# Extract
funcfinder --inp <file> --source <lang> --func <Name> --extract

# Impact
callgraph --dir . -l <lang> --reverse --func <Name>

# Architecture
deps . -l <lang> --shards --no-gitignore --update-manifest .codemap/manifest.json
```

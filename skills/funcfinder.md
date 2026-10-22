# Skill: funcfinder — Codebase Investigation

Installable skill for Codex and other sessions: [funcfinder/SKILL.md](funcfinder/SKILL.md). This document retains the expanded navigation recipes.

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

**Doc first**: `--doc Name` gives the comment above a declaration and its first line, without the body. Use it to learn what a function is for; read the body with `--extract` only when the code itself is needed.

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

## Phase 6 — Edit one function

For Go/Rust/C# replacement, prefer code objects (use `--source rust` or `--source cs`): `funcfinder --inp FILE --source go --read-object NAME`
(`--object-kind type` for types). Modify only the returned JSON `source`, then send the object to
`funcfinder --edit-object -` for preview and to `funcfinder --edit-object - --apply` to write.
Keep metadata unchanged; on `stale_revision`, read again and reconsider the edit. Ambiguity returns editable
`error.candidates`. Apply the original request after preview; only the applied response's `object` is reusable
for subsequent edits. Validation checks syntax/identity, so still run project tests. See [protocol](../docs/EDIT.md).

Go/Rust/C# JSON also accepts operation wrappers (`replace`, `insert_after`, `remove`) and one-file `funcfinder.edits/v1` batches. Keep all anchors at one original revision, preview/apply the whole request once, and consume final objects from `results`. Rust/C# checks are structural; run the language compiler/project tests. See [installable skill](funcfinder/SKILL.md) for envelope details and backend scope.

For other languages or legacy workflows:

Edit a function with `--replace-func`, not with `sed` or a text patch. The span comes from the same finder, so the edit stays inside the function, and the tool checks the result before it writes anything.

```bash
# 1. The function's current text: this is the --old the edit must still find.
#    --extract prints a "// name: start-end" header first; drop it with tail -n +2
funcfinder --inp internal/edit.go --source go --func ReplaceFunction --extract | tail -n +2 > old.txt
# 2. Write the new function to new.txt without that header (or pipe it with --with -)
# 3. Preview: validates and writes nothing
funcfinder --inp internal/edit.go --source go --replace-func ReplaceFunction --with new.txt --old old.txt
# 4. Apply
funcfinder --inp internal/edit.go --source go --replace-func ReplaceFunction --with new.txt --old old.txt --apply
```

To add a new function after an existing one, use `--add-after` with the same flags minus `--old`. The new text is inserted after the target with one blank line in between:

```bash
funcfinder --inp internal/edit.go --source go --add-after ReplaceFunction --with new.txt --apply
```

To delete a function, run `callgraph --dir . -l <lang> --reverse --func <Name>` first, then `--remove-func <Name> --apply`. Callers are not checked by the tool.

The tool enforces these rules. Do not work around them:
- Several functions share the name: add `--line N`, the start line from `--map`.
- The replacement keeps every function and type the finder reports, in the same order. A rename, an extra function or a missing one is refused. A nested helper inside the function belongs in the replacement.
- Decorators and annotations above the function (`@x`, `#[x]`) are kept when the replacement omits them. A different set is refused.
- Brace languages must balance their brackets in the replacement. Python has no bracket check; instead the replacement must start at the function's indentation, its def must have an indented body, and no line may leave the block (lines inside triple-quoted strings excepted).
- Comments above the `func` line are not replaced.
- "is not the expected text" means the file changed since you read it. Re-run `--extract` and rebuild the replacement. Do not drop `--old`.

Why this is cheaper than grep and read: `--extract` of one function was 3466 bytes against 9001 for the whole file, and `callgraph --reverse` answers "who calls this" in one call, where `grep` returns every textual mention.

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
| Patching a function with `sed` or a text replace | `--replace-func` with `--old`; preview first, then `--apply` |
| `--replace-func` refused: "not the expected text" | Re-run `--extract | tail -n +2`, rebuild the replacement; do not drop `--old` |
| `callgraph --inp` lists `fmt.Errorf`, `strings.Count` and other library calls | Use `callgraph --reverse` for impact; the library names are noise there |
| Calling `--extract` without knowing the line | Use `--names` or `--map` first, then `--func Name --extract` |
| Wanting the purpose of a function from its body | Use `--doc Name`: it shows the comment above the declaration |
| Expecting Go methods as `method` in `--names`/`--doc` | They show as `function` for now: the receiver is not recorded |
| Expecting a Python docstring from `--doc` | Not read yet: only the comment above `def`/`class` is shown |
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
funcfinder --inp <file> --source <lang> --doc <Name>    # comment + first line, no body

# Impact
callgraph --dir . -l <lang> --reverse --func <Name>

# Edit one function (preview, then --apply)
funcfinder --inp <file> --source <lang> --replace-func <Name> --with <new-file|-> [--old <old-file>] [--line N] [--apply]

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

# funcfinder for AI Agents

**Quick Reference**: Map codebases, extract functions, trace calls, save 99% tokens.

---

## Build First!

```bash
./build.sh   # Required before first use (~5 sec)
```

Builds 5 binaries: `funcfinder`, `stat`, `deps`, `complexity`, `callgraph`.

---

## The Toolkit at a Glance

| Tool | Purpose | Input |
|------|---------|-------|
| `funcfinder` | Map functions & types, extract bodies | file or dir |
| `deps` | Import dependencies + shard graph | dir |
| `callgraph` | Who calls whom | file or dir |
| `stat` | Call frequency & hotspots | file |
| `complexity` | Cognitive complexity per function | file |
| `docsearch` | Knowledge base: FTS5 + vector hybrid search | SQLite file |

---

## Investigation Workflow

### Small project (< 50 files)

```bash
# 1. Orient — full map in one shot (~30ms)
funcfinder --dir . --all --json > map.json

# 2. Find the target
grep -i "auth" map.json
# → auth/handler.go:42: AuthenticateUser

# 3. Extract the body
funcfinder --inp auth/handler.go --source go --func AuthenticateUser --extract

# 4. Trace who it calls
callgraph --inp auth/handler.go -l go --func AuthenticateUser

# 5. Trace who calls it (impact)
callgraph --dir . -l go --reverse --func AuthenticateUser
```

### Large project (50+ files)

```bash
# 1. Split into shards (one-time, ~100ms)
funcfinder --dir . --all --json --split

# 2. Read manifest — 2KB overview of entire codebase
cat .codemap/manifest.json
# → see shards, function counts, depends_on links

# 3. Load the relevant shard
cat .codemap/internal_auth.json

# 4. Extract the function
funcfinder --inp internal/auth.go --source go --func Authenticate --extract

# 5. Check call graph for impact
callgraph --dir . -l go --reverse --func Authenticate
```

### Incremental update (repeat sessions)

```bash
funcfinder --dir . --all --json --split --inc
# INFO: Incremental: 1 shards changed, 32 unchanged
```

### Full architecture index (once per project)

```bash
# Build shard map
funcfinder --dir . --all --json --split --no-gitignore

# Add inter-shard dependency graph to manifest
deps . -l go --shards --no-gitignore --update-manifest .codemap/manifest.json

# manifest.json now contains:
# {"path": "cmd_funcfinder.json", "depends_on": ["internal.json"], ...}
```

---

## docsearch — Knowledge Base

```bash
# Initialize (creates .knowledge/docs.sqlite by default)
docsearch init
docsearch --db /path/to/custom.sqlite init

# Add a document (plain text, no embedding)
docsearch add --title "Title" --content "..." --type general

# Add with embedding (float32 comma-separated, e.g. from Ollama)
docsearch add --title "Error: connection refused" --content "..." --type error \
  --embedding "0.12,0.34,..." --meta '{"scenario":"db-setup"}'

# FTS keyword search
docsearch search --query "candidate storage" --mode fts

# Semantic vector search
docsearch search --embedding "0.12,0.34,..." --mode vec

# Hybrid (default) — FTS + vector, Reciprocal Rank Fusion
docsearch search --query "connection error" --embedding "0.12,0.34,..." --limit 5 --json

# Count total documents
docsearch count
```

**Document types**: `general`, `tool_usage`, `error`, `scenario` (or any custom string).

**Hybrid mode**: RRF combines BM25 ranks and cosine ranks. Pass both `--query` and `--embedding` for best results. Without an embedding, falls back to FTS only; without a query, falls back to vector only.

**Embedding source**: generate externally (Ollama, OpenAI, local model) and pass as `--embedding`. The tool stores and retrieves — it does not generate.

---

## callgraph — New Tool

```bash
# Call tree from a function (with depth)
callgraph --dir . -l go --func ProcessDirectory
callgraph --dir . -l go --func ProcessDirectory --depth 2

# Who calls a function (impact analysis)
callgraph --dir . -l go --reverse --func computeShardChecksum

# Full call graph, JSON
callgraph --dir . -l go --json

# Single file
callgraph --inp internal/finder.go -l go

# Include gitignore-excluded files
callgraph --dir . -l go --no-gitignore
```

**Output examples:**

```
# Forward graph (plain text)
internal/dirprocessor.go
  ProcessDirectory → collectFiles
  ProcessDirectory → processFilesParallel
  collectFiles → NewIgnoreMatcher
  collectFiles → filepath.Walk

# Reverse (who calls computeShardChecksum)
computeShardChecksum is called by:
  ProcessDirectoryIncremental
  WriteSplitOutput
  WriteSplitOutputIncremental
```

**JSON format:**
```json
{
  "files": [
    {
      "path": "internal/dirprocessor.go",
      "calls": [
        {"caller": "ProcessDirectory", "callee": "collectFiles", "line": 66},
        {"caller": "ProcessDirectory", "callee": "processFilesParallel", "line": 76}
      ]
    }
  ],
  "total_calls": 956,
  "total_functions": 256
}
```

---

## deps — Shard Dependency Graph

```bash
# Inter-shard graph (plain text)
deps . -l go --shards --no-gitignore

# Write depends_on into manifest.json
deps . -l go --shards --no-gitignore --update-manifest .codemap/manifest.json

# TypeScript with @/ alias auto-detection
deps frontend -l ts --shards --update-manifest .codemap/manifest.json
```

---

## funcfinder Flag Reference

| Task | Command |
|------|---------|
| Map entire codebase | `funcfinder --dir . --all --json` |
| Map only functions | `funcfinder --dir . --json` |
| Map only types/classes | `funcfinder --dir . --struct --json` |
| **Split large codebase** | `funcfinder --dir . --all --json --split` |
| **Incremental update** | `funcfinder --dir . --all --json --split --inc` |
| Split by file | `funcfinder --dir . --all --json --split --split-by file` |
| Custom output dir | `funcfinder --dir . --json --split --out ./analysis` |
| Map single file | `funcfinder --inp file.go --source go --map` |
| Find specific function | `funcfinder --inp file.go --source go --func Name` |
| Extract function body | `funcfinder --inp file.go --source go --func Name --extract` |
| Extract named structs | `funcfinder --inp file.go --source go --struct "TypeA,TypeB" --extract` |
| Extract all structs | `funcfinder --inp file.go --source go --struct --extract` |
| Tree view | `funcfinder --dir . --tree` |

**Key Rules**:
- `--dir` mode: `--map` is DEFAULT
- `--inp` mode: requires `--source <lang>` AND (`--map` or `--func` or `--tree` or `--extract`)
- `--struct "TypeA,TypeB"` — shorthand for `--struct --type "TypeA,TypeB"`
- Languages: `go`, `py`, `js`, `ts`, `java`, `cs`, `cpp`, `c`, `rust`, `swift`, `kotlin`, `php`, `ruby`, `scala`, `d`

---

## Common Mistakes

### 1. Forgot to build
```bash
./build.sh
```

### 2. Missing --map in file mode
```bash
funcfinder --inp file.go --source go --map
```

### 3. Missing --source in file mode
```bash
funcfinder --inp file.go --source go --func Main
```

### 4. Using --struct to extract named types
```bash
# Both forms work:
funcfinder --inp file.go --source go --struct "TypeA,TypeB" --extract
funcfinder --inp file.go --source go --struct --type "TypeA,TypeB" --extract
```

### 5. Using --split without --json
```bash
funcfinder --dir . --json --split
```

### 6. Using --inc without existing .codemap/
```bash
# First run = full scan, creates manifest
funcfinder --dir . --json --split --inc
# Second run = incremental
funcfinder --dir . --json --split --inc
```

### 7. cmd/ excluded by gitignore in deps/callgraph
```bash
# Bare names like "deps", "stat" in .gitignore match directories.
# Use --no-gitignore to include cmd/ packages:
deps . -l go --shards --no-gitignore
callgraph --dir . -l go --no-gitignore
```

---

## JSON Output Structure

### Standard (`--json`)
```json
{
  "files": [
    {
      "path": "internal/finder.go",
      "functions": [{"name": "FindFunctions", "line": 89}],
      "classes": [{"name": "Finder", "line": 45, "kind": "struct"}]
    }
  ],
  "total_files": 25,
  "total_functions": 228,
  "total_classes": 69
}
```

**Important**: `path` is at file level only, not inside function objects!

### Split manifest (`.codemap/manifest.json`)
```json
{
  "version": "1.0",
  "root_dir": ".",
  "split_by": "dir",
  "shards": [
    {
      "path": "internal.json",
      "files": 27,
      "total_functions": 248,
      "total_classes": 40,
      "checksum": "5583a4ba...",
      "depends_on": ["internal_db.json", "internal_config.json"]
    }
  ],
  "total_files": 57,
  "total_functions": 610,
  "total_classes": 378
}
```

Each shard file uses the same format as standard `--json` output.

### jq Recipes

```bash
# Functions with paths
jq -r '.files[] | .path as $p | .functions[] | "\($p):\(.line): \(.name)"' map.json

# Find by name
jq '.files[] | .path as $p | .functions[] | select(.name | contains("Auth")) | {path: $p, name, line}' map.json

# Shards that depend on internal.json
jq '.shards[] | select(.depends_on[]? == "internal.json") | .path' .codemap/manifest.json

# Simple grep often faster
grep -i "functionname" map.json
```

---

## Performance

- **Speed**: 763K lines/sec
- **Parallel**: Auto-uses all CPU cores
- **Token savings**: 99%+ vs reading full files

| Project size | Full JSON | With `--split` (1 shard) | Savings |
|-------------|-----------|--------------------------|---------|
| 60 files    | 44KB / 11K tok | 15KB / 4K tok | **3x** |
| 500 files   | 375KB / 93K tok | 15KB / 4K tok | **~25x** |
| 5000 files  | 3.7MB / 930K tok* | 15KB / 4K tok | **~230x** |

*without `--split` exceeds context window

**Incremental (`--inc`)**: change 1 file in a 500-file project → reprocess 1/33 shards, skip 32.

---

## Golden Rule

> On small projects: `--dir . --all --json`, then grep.
> On large projects (50+ files): `--split` → read manifest → load one shard.
> To understand impact of a change: `callgraph --reverse --func Name`.
> Never read source files to find functions — that's what funcfinder is for.

---

*Full documentation: [docs/](docs/)*

---

# DOX framework

- DOX is highly performant AGENTS.md hierarchy installed here
- Agent must follow DOX instructions across any edits

## Core Contract

- AGENTS.md files are binding work contracts for their subtrees
- Work products, source materials, instructions, records, assets, and durable docs must stay understandable from the nearest applicable AGENTS.md plus every parent AGENTS.md above it

## Read Before Editing

1. Read the root AGENTS.md
2. Identify every file or folder you expect to touch
3. Walk from the repository root to each target path
4. Read every AGENTS.md found along each route
5. If a parent AGENTS.md lists a child AGENTS.md whose scope contains the path, read that child and continue from there
6. Use the nearest AGENTS.md as the local contract and parent docs for repo-wide rules
7. If docs conflict, the closer doc controls local work details, but no child doc may weaken DOX

Do not rely on memory. Re-read the applicable DOX chain in the current session before editing.

## Update After Editing

Every meaningful change requires a DOX pass before the task is done.

Update the closest owning AGENTS.md when a change affects:

- purpose, scope, ownership, or responsibilities
- durable structure, contracts, workflows, or operating rules
- required inputs, outputs, permissions, constraints, side effects, or artifacts
- user preferences about behavior, communication, process, organization, or quality
- AGENTS.md creation, deletion, move, rename, or index contents

Update parent docs when parent-level structure, ownership, workflow, or child index changes. Update child docs when parent changes alter local rules. Remove stale or contradictory text immediately. Small edits that do not change behavior or contracts may leave docs unchanged, but the DOX pass still must happen.

## Hierarchy

- Root AGENTS.md is the DOX rail: project-wide instructions, global preferences, durable workflow rules, and the top-level Child DOX Index
- Child AGENTS.md files own domain-specific instructions and their own Child DOX Index
- Each parent explains what its direct children cover and what stays owned by the parent
- The closer a doc is to the work, the more specific and practical it must be

## Child Doc Shape

- Create a child AGENTS.md when a folder becomes a durable boundary with its own purpose, rules, responsibilities, workflow, materials, or quality standards
- Work Guidance must reflect the current standards of the project or user instructions; if there are no specific standards or instructions yet, leave it empty
- Verification must reflect an existing check; if no verification framework exists yet, leave it empty and update it when one exists

Default section order:
- Purpose
- Ownership
- Local Contracts
- Work Guidance
- Verification
- Child DOX Index

## Style

- Keep docs concise, current, and operational
- Document stable contracts, not diary entries
- Put broad rules in parent docs and concrete details in child docs
- Prefer direct bullets with explicit names
- Do not duplicate rules across many files unless each scope needs a local version
- Delete stale notes instead of explaining history
- Trim obvious statements, repeated rules, misplaced detail, and warnings for risks that no longer exist

## Closeout

1. Re-check changed paths against the DOX chain
2. Update nearest owning docs and any affected parents or children
3. Refresh every affected Child DOX Index
4. Remove stale or contradictory text
5. Run existing verification when relevant
6. Report any docs intentionally left unchanged and why

## User Preferences

When the user requests a durable behavior change, record it here or in the relevant child AGENTS.md

## Child DOX Index

- `cmd/` → CLI entrypoints for all 6 tools (funcfinder, stat, deps, callgraph, complexity, findstruct) — see [cmd/AGENTS.md](cmd/AGENTS.md)
- `internal/` → Core parsing engine: language finders, formatters, call graph, shard logic — see [internal/AGENTS.md](internal/AGENTS.md)
- `docs/` → User-facing documentation and usage examples — see [docs/AGENTS.md](docs/AGENTS.md)
- `examples/` → Shell script usage examples and swe-agent integration workflows — see [examples/AGENTS.md](examples/AGENTS.md)
- `skills/` → Claude Code skill definition for funcfinder — see [skills/AGENTS.md](skills/AGENTS.md)
- `internal/knowledge/` → SQLite knowledge base package (FTS5 + vector hybrid) — see [internal/knowledge/AGENTS.md](internal/knowledge/AGENTS.md)
- `test_examples/` → Multi-language source fixtures used by parser tests — see [test_examples/AGENTS.md](test_examples/AGENTS.md)
- `test_files/` → Edge-case source fixtures (raw strings, docstrings, verbatim literals) — see [test_files/AGENTS.md](test_files/AGENTS.md)

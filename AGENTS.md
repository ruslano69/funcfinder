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

---

## Investigation Workflow

### Small project (< 50 files)

```bash
# 1. Orient — full map in one shot (~30ms)
./funcfinder --dir . --all --json > map.json

# 2. Find the target
grep -i "auth" map.json
# → auth/handler.go:42: AuthenticateUser

# 3. Extract the body
./funcfinder --inp auth/handler.go --source go --func AuthenticateUser --extract

# 4. Trace who it calls
./callgraph --inp auth/handler.go -l go --func AuthenticateUser

# 5. Trace who calls it (impact)
./callgraph --dir . -l go --reverse --func AuthenticateUser
```

### Large project (50+ files)

```bash
# 1. Split into shards (one-time, ~100ms)
./funcfinder --dir . --all --json --split

# 2. Read manifest — 2KB overview of entire codebase
cat .codemap/manifest.json
# → see shards, function counts, depends_on links

# 3. Load the relevant shard
cat .codemap/internal_auth.json

# 4. Extract the function
./funcfinder --inp internal/auth.go --source go --func Authenticate --extract

# 5. Check call graph for impact
./callgraph --dir . -l go --reverse --func Authenticate
```

### Incremental update (repeat sessions)

```bash
./funcfinder --dir . --all --json --split --inc
# INFO: Incremental: 1 shards changed, 32 unchanged
```

### Full architecture index (once per project)

```bash
# Build shard map
./funcfinder --dir . --all --json --split --no-gitignore

# Add inter-shard dependency graph to manifest
./deps . -l go --shards --no-gitignore --update-manifest .codemap/manifest.json

# manifest.json now contains:
# {"path": "cmd_funcfinder.json", "depends_on": ["internal.json"], ...}
```

---

## callgraph — New Tool

```bash
# Call tree from a function (with depth)
./callgraph --dir . -l go --func ProcessDirectory
./callgraph --dir . -l go --func ProcessDirectory --depth 2

# Who calls a function (impact analysis)
./callgraph --dir . -l go --reverse --func computeShardChecksum

# Full call graph, JSON
./callgraph --dir . -l go --json

# Single file
./callgraph --inp internal/finder.go -l go

# Include gitignore-excluded files
./callgraph --dir . -l go --no-gitignore
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
./deps . -l go --shards --no-gitignore

# Write depends_on into manifest.json
./deps . -l go --shards --no-gitignore --update-manifest .codemap/manifest.json

# TypeScript with @/ alias auto-detection
./deps frontend -l ts --shards --update-manifest .codemap/manifest.json
```

---

## funcfinder Flag Reference

| Task | Command |
|------|---------|
| Map entire codebase | `./funcfinder --dir . --all --json` |
| Map only functions | `./funcfinder --dir . --json` |
| Map only types/classes | `./funcfinder --dir . --struct --json` |
| **Split large codebase** | `./funcfinder --dir . --all --json --split` |
| **Incremental update** | `./funcfinder --dir . --all --json --split --inc` |
| Split by file | `./funcfinder --dir . --all --json --split --split-by file` |
| Custom output dir | `./funcfinder --dir . --json --split --out ./analysis` |
| Map single file | `./funcfinder --inp file.go --source go --map` |
| Find specific function | `./funcfinder --inp file.go --source go --func Name` |
| Extract function body | `./funcfinder --inp file.go --source go --func Name --extract` |
| Extract named structs | `./funcfinder --inp file.go --source go --struct "TypeA,TypeB" --extract` |
| Extract all structs | `./funcfinder --inp file.go --source go --struct --extract` |
| Tree view | `./funcfinder --dir . --tree` |

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
./funcfinder --inp file.go --source go --map
```

### 3. Missing --source in file mode
```bash
./funcfinder --inp file.go --source go --func Main
```

### 4. Using --struct to extract named types
```bash
# Both forms work:
./funcfinder --inp file.go --source go --struct "TypeA,TypeB" --extract
./funcfinder --inp file.go --source go --struct --type "TypeA,TypeB" --extract
```

### 5. Using --split without --json
```bash
./funcfinder --dir . --json --split
```

### 6. Using --inc without existing .codemap/
```bash
# First run = full scan, creates manifest
./funcfinder --dir . --json --split --inc
# Second run = incremental
./funcfinder --dir . --json --split --inc
```

### 7. cmd/ excluded by gitignore in deps/callgraph
```bash
# Bare names like "deps", "stat" in .gitignore match directories.
# Use --no-gitignore to include cmd/ packages:
./deps . -l go --shards --no-gitignore
./callgraph --dir . -l go --no-gitignore
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

# funcfinder for AI Agents

**Quick Reference**: Map codebases, extract functions, save 99% tokens.

---

## Build First!

```bash
./build.sh   # Required before first use (~5 sec)
```

---

## Three Essential Commands

```bash
# 1. MAP codebase — small project (your starting point)
./funcfinder --dir . --all --json > map.json

# 2. SEARCH the map
grep -i "auth" map.json

# 3. EXTRACT specific function
./funcfinder --inp path/to/file.go --source go --func AuthHandler --extract
```

### Large Codebase? Use --split instead of step 1

```bash
# 1. SPLIT into shards (500+ files, saves 25-230x tokens)
./funcfinder --dir . --all --json --split

# 2. READ manifest to orient (2-10KB, not 100KB+)
cat .codemap/manifest.json

# 3. LOAD only the relevant shard
cat .codemap/internal_auth.json

# Subsequent runs: only reprocess changed directories
./funcfinder --dir . --all --json --split --inc
# INFO: Incremental: 1 shards changed, 32 unchanged
```

---

## Flag Reference

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
- `--struct "TypeA,TypeB"` — activates struct mode and filters by names (shorthand for `--struct --type "TypeA,TypeB"`)
- Languages: `go`, `py`, `js`, `ts`, `java`, `cs`, `cpp`, `c`, `rust`, `swift`, `kotlin`, `php`, `ruby`, `scala`, `d`

---

## Common Mistakes

### 1. Forgot to build
```bash
# Error: command not found
./funcfinder --dir .

# Fix:
./build.sh
```

### 2. Missing --map in file mode
```bash
# Error: either --func, --map, or --tree must be specified
./funcfinder --inp file.go --source go

# Fix:
./funcfinder --inp file.go --source go --map
```

### 3. Missing --source in file mode
```bash
# Error: --source parameter is required
./funcfinder --inp file.go --func Main

# Fix:
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
# Error: --split requires --json output mode
./funcfinder --dir . --split

# Fix:
./funcfinder --dir . --json --split
```

### 6. Using --inc without existing .codemap/
```bash
# --inc with no prior manifest silently falls back to full scan
./funcfinder --dir . --json --split --inc  # first run = full scan, creates manifest
./funcfinder --dir . --json --split --inc  # second run = incremental
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
    {"path": "internal.json", "files": 27, "total_functions": 248, "total_classes": 40, "checksum": "5583a4ba..."}
  ],
  "total_files": 57,
  "total_functions": 610,
  "total_classes": 378
}
```

Each shard file uses the same format as standard `--json` output.

### jq Recipes

```bash
# Functions with paths (most common!)
jq -r '.files[] | .path as $p | .functions[] | "\($p):\(.line): \(.name)"' map.json

# Find by name
jq '.files[] | .path as $p | .functions[] | select(.name | contains("Auth")) | {path: $p, name, line}' map.json

# Count per file
jq -r '.files[] | "\(.path): \(.functions | length)"' map.json

# Simple grep often faster
grep -i "functionname" map.json
```

---

## Additional Tools

After `./build.sh`:

```bash
# Function call statistics
./stat file.go -l go

# Import/dependency analysis
./deps file.go -l go -json

# Cognitive complexity (skip trivial with --nosimple)
./complexity file.go -l go --nosimple
```

**Note**: These tools work on single files only. Use `funcfinder --dir` for directories.

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

## Workflow Example

```bash
# Task: Find and modify user authentication (small project)

# 1. Map (30ms)
./funcfinder --dir . --all --json > map.json

# 2. Search (instant)
grep -i "auth" map.json
# Found: auth/handler.go:42: AuthenticateUser

# 3. Extract (instant)
./funcfinder --inp auth/handler.go --source go --func AuthenticateUser --extract
```

```bash
# Task: Explore large unknown codebase

# 1. Split (one-time, ~100ms)
./funcfinder --dir . --all --json --split

# 2. Orient via manifest (2KB, instant)
cat .codemap/manifest.json
# → see all modules, function counts, find what's relevant

# 3. Load one shard (15KB instead of 375KB)
cat .codemap/internal_auth.json

# 4. On next session — incremental update only
./funcfinder --dir . --all --json --split --inc
```

---

## Golden Rule

> On small projects use `--dir . --all --json`.
> On large projects (50+ files) use `--split`: read manifest first, then load one shard.
> Never read source files to find functions — that's what funcfinder is for.

---

*Full documentation: [docs/](docs/) | Advanced Python --lines: [docs/UTILITIES.md](docs/UTILITIES.md)*

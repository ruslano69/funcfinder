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
# 1. MAP codebase (your starting point)
./funcfinder --dir . --all --json > map.json

# 2. SEARCH the map
grep -i "auth" map.json

# 3. EXTRACT specific function
./funcfinder --inp path/to/file.go --source go --func AuthHandler --extract
```

---

## Flag Reference

| Task | Command |
|------|---------|
| Map entire codebase | `./funcfinder --dir . --all --json` |
| Map only functions | `./funcfinder --dir . --json` |
| Map only types/classes | `./funcfinder --dir . --struct --json` |
| Map single file | `./funcfinder --inp file.go --source go --map` |
| Find specific function | `./funcfinder --inp file.go --source go --func Name` |
| Extract function body | `./funcfinder --inp file.go --source go --func Name --extract` |
| Tree view | `./funcfinder --dir . --tree` |

**Key Rules**:
- `--dir` mode: `--map` is DEFAULT
- `--inp` mode: requires `--source <lang>` AND (`--map` or `--func` or `--tree`)
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

### 4. Zero files from root scan
```bash
# Result: 0 files (gitignore hiding sources)
./funcfinder --dir . --all

# Fix: scan specific directory
./funcfinder --dir internal --all --json
```

---

## JSON Output Structure

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

**Example**: 25 files, 228 functions → ~30ms

---

## Workflow Example

```bash
# Task: Find and modify user authentication

# 1. Map (30ms)
./funcfinder --dir . --all --json > map.json

# 2. Search (instant)
grep -i "auth" map.json
# Found: auth/handler.go:42: AuthenticateUser

# 3. Extract (instant)
./funcfinder --inp auth/handler.go --source go --func AuthenticateUser --extract

# Total: 2 operations vs reading 50 files
```

---

## Golden Rule

> If you're about to read a file to find a function, use `./funcfinder --dir . --all --json` first.
> The map tells you exactly where to look. Save 99% tokens.

---

*Full documentation: [docs/](docs/) | Advanced Python --lines: [docs/UTILITIES.md](docs/UTILITIES.md)*

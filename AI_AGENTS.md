# Code Analysis Workflow with funcfinder for AI Agents

**For AI Agents**: This document describes the optimal workflow for analyzing codebases using `funcfinder`. Following these patterns will dramatically reduce token consumption while maintaining accuracy.

---

## 🔨 Build First!

**CRITICAL**: Before using funcfinder, you MUST build it first!

```bash
# Step 0: Build all utilities (required before first use)
./build.sh

# Output:
# ✓ funcfinder
# ✓ stat
# ✓ deps
# ✓ complexity
# ✅ All binaries built successfully!
```

**Why build first?**
- funcfinder is a Go binary that must be compiled
- Takes ~5 seconds to build all utilities
- Only needed once per session
- If you see "command not found", run `./build.sh` first

---

## 🚀 Quick Start for AI Agents

**Copy-paste this workflow for immediate results:**

```bash
# 1. BUILD (only needed once)
./build.sh

# 2. MAP entire codebase (your starting point)
# --map is now DEFAULT for directories!
./funcfinder --dir . --all --json > codebase_map.json

# 3. SEARCH the map (no file reads!)
grep -i "YourFunction" codebase_map.json

# 4. EXTRACT specific function once found
./funcfinder --inp path/to/file.go --source go --extract "YourFunction"
```

**Result**: 99%+ token savings, instant navigation, zero guesswork.

---

## Core Principle: Map Before Reading

**ALWAYS** use `funcfinder` to map the codebase structure **BEFORE** reading any files. This is the single most important optimization for token efficiency.

```bash
# ✅ CORRECT: Map entire codebase first (763K lines/sec)
# --map is now DEFAULT for directories!
./funcfinder --dir . --all --json

# ❌ WRONG: Reading files blindly
cat src/file1.go src/file2.go src/file3.go
grep -r "function" src/
```

## Performance Context

- **Speed**: 763,000 lines/second parsing
- **Parallel**: Worker pools (default: CPU cores)
- **Token Savings**: 99%+ reduction vs reading full files
- **Accuracy**: State-machine parser (not regex) handles edge cases correctly

**Example**: Analyzing 25 files with 228 functions takes ~30ms vs potentially 25× separate file reads.

## Decision Tree: When to Use What

### Scenario 1: Understanding a New Codebase

```bash
# Step 1: Get complete map (--map is now default!)
./funcfinder --dir . --all --json > codebase_map.json

# Result: Full inventory of all functions, classes, types
# - Auto-detects all languages
# - Respects .gitignore (skips node_modules/, vendor/, etc.)
# - One JSON read = entire codebase structure
```

**Now you have**:
- Every function name and line number
- Every class/struct/type and location
- File-by-file organization
- Zero file reads yet

**Use this map to**:
- Answer "where is X defined?" without reading files
- Identify entry points (main, init functions)
- Understand module structure
- Plan which files to read

### Scenario 2: Finding Specific Functionality

```bash
# Search the map (grep on funcfinder output)
./funcfinder --dir . --all --json | jq -r '.files[].functions[] | select(.name | contains("Auth"))'

# Or use grep-style output directly
./funcfinder --dir . --all | grep "Auth"

# Result: authentication.go:42: AuthenticateUser
#         auth_handler.go:18: AuthMiddleware
```

**Only NOW read the specific files** at the identified line numbers.

### Scenario 3: Architecture Analysis

```bash
# Map type hierarchy without implementation details
./funcfinder --dir . --struct --json

# Result: All classes, interfaces, structs, types
# - Perfect for understanding data models
# - No function bodies = fewer tokens
# - Shows relationships between types
```

### Scenario 4: Single File Deep Dive

```bash
# After identifying the target file from directory scan
./funcfinder --inp target.go --source go --map

# Result: Function map with line ranges
# Now use --extract to read only relevant functions:
./funcfinder --inp target.go --source go --extract "ProcessRequest"
```

## Complete Workflow Example

**Task**: "Find and modify the user authentication logic"

```bash
# 1. Map the codebase (30ms for 25 files)
./funcfinder --dir . --all --json > map.json

# 2. Search the map (no file reads)
grep -i "auth" map.json
# Found: authentication.go:42: AuthenticateUser

# 3. Read ONLY the relevant function
./funcfinder --inp authentication.go --source go --extract "AuthenticateUser"

# 4. Understand dependencies by checking what it calls
# (Already in the map.json from step 1)

# 5. Make changes to authentication.go

# Total file reads: 2 (map.json + 1 function extraction)
# vs reading entire src/ directory: potentially 50+ files
```

## Common Patterns

### Pattern 1: Multi-Language Projects

```bash
# Auto-detects Go, Python, JavaScript, TypeScript, etc.
./funcfinder --dir . --all --json

# No need to specify language per file
# Result includes language metadata per file
```

### Pattern 2: Large Repositories

```bash
# Use parallel processing (automatic)
./funcfinder --dir large_repo --all --json --workers 8

# Result: Linear scaling with CPU cores
# Example: 100K lines in ~130ms
```

### Pattern 3: Incremental Analysis

```bash
# Scan only changed directory
./funcfinder --dir src/modified_module --all --json

# Compare with previous map to see what changed
# No need to re-scan entire repo
```

### Pattern 4: Tree View for Humans

```bash
# Get hierarchical visualization
./funcfinder --dir src --tree

# Result: Directory tree with functions nested under files
# src/
#   ├── auth.go
#   │   ├── def AuthenticateUser (line 42)
#   │   └── def ValidateToken (line 67)
#   └── handler.go
#       └── def HandleRequest (line 23)
```

---

## 📋 Flag Reference Table (Critical for AI Agents!)

**NEW**: `--map` is now **DEFAULT** for directory mode! File mode still requires `--map`.

| Task | Correct Command | ❌ Common Mistake |
|------|----------------|-------------------|
| **Map entire codebase** | `./funcfinder --dir . --all --json` | `./funcfinder --dir . --all --json` (missing `./`) |
| **Map only functions** | `./funcfinder --dir . --json` | `./funcfinder --dir .` (missing --json) |
| **Map only types** | `./funcfinder --dir . --struct --json` | `./funcfinder --dir . --struct` (missing --json) |
| **Map single file** | `./funcfinder --inp file.go --source go --map` | `./funcfinder --inp file.go --source go` (missing --map!) |
| **Find specific function** | `./funcfinder --inp file.go --source go --func "MyFunc"` | `./funcfinder --inp file.go --func "MyFunc"` (missing --source) |
| **Extract function body** | `./funcfinder --inp file.go --source go --extract "MyFunc"` | `./funcfinder --inp file.go --extract "MyFunc"` (missing --source) |
| **Tree view** | `./funcfinder --dir . --tree` | `./funcfinder --tree` (missing --dir) |

**Key Rules**:
- ✅ Always use `./funcfinder` (not `funcfinder`) until added to PATH
- ✅ **Directory mode**: `--map` is DEFAULT (no need to specify!)
- ✅ **File mode**: `--map` is REQUIRED for mapping (or use `--func`, `--extract`, `--tree`)
- ✅ File mode **always requires** `--source <lang>` flag
- ✅ Use `--all` to get both functions AND types

---

## ⚠️ Common AI Agent Mistakes

### Mistake #1: Forgetting to Build

```bash
# ❌ ERROR: Command not found
./funcfinder --dir . --all --json

# ✅ SOLUTION: Build first!
./build.sh
./funcfinder --dir . --all --map --json
```

**Why it happens**: funcfinder is a Go binary, not a system command.

### Mistake #2: Forgetting --map in File Mode

```bash
# ❌ ERROR: either --func, --map, or --tree must be specified
./funcfinder --inp file.go --source go

# ✅ CORRECT: Add --map flag for file mode
./funcfinder --inp file.go --source go --map
```

**Why it happens**: File mode needs to know what you want: `--map` (all functions), `--func` (specific), or `--extract`.
**Note**: Directory mode has `--map` as DEFAULT, file mode does NOT!

### Mistake #3: Scanning Root Gets 0 Files

```bash
# ❌ RESULT: 0 files (because .gitignore hides *.go files)
./funcfinder --dir . --all --json

# ✅ SOLUTION: Scan specific directory OR use --no-gitignore
./funcfinder --dir internal --all --json
./funcfinder --dir . --all --json --no-gitignore
```

**Why it happens**: `.gitignore` might exclude source files at root level.

### Mistake #4: Missing --source Flag in File Mode

```bash
# ❌ ERROR: --source parameter is required
./funcfinder --inp file.go --func "ProcessRequest"

# ✅ CORRECT: Add --source flag
./funcfinder --inp file.go --source go --func "ProcessRequest"
```

**Why it happens**: funcfinder can't auto-detect language in file mode.

### Mistake #5: Using Wrong Path Separator

```bash
# ❌ ERROR: Uses `-inp` (single dash) instead of `--inp`
./funcfinder -inp file.go -source go --map

# ✅ CORRECT: Use double dash for long flags
./funcfinder --inp file.go --source go --map
```

**Why it happens**: funcfinder uses GNU-style `--long-flags`, not `-short` flags.

---

## Output Format Reference

### JSON Format (Machine-Readable)

```json
{
  "files": [
    {
      "path": "internal/finder.go",
      "functions": [
        {"name": "FindFunctions", "line": 89},
        {"name": "parseFunction", "line": 156}
      ],
      "classes": [
        {"name": "Finder", "line": 45}
      ]
    }
  ],
  "total_files": 25,
  "total_functions": 228,
  "total_classes": 69
}
```

**Use jq to query**:
```bash
# Get all function names
jq -r '.files[].functions[].name' map.json

# Find functions in specific file
jq -r '.files[] | select(.path | contains("auth")) | .functions[]' map.json

# Count functions per file
jq -r '.files[] | "\(.path): \(.functions | length)"' map.json
```

### Grep Format (Search-Friendly)

```bash
./funcfinder --dir . --all

# Output:
# internal/finder.go:89: FindFunctions
# internal/finder.go:156: parseFunction
# internal/config.go:23: LoadConfig
```

**Use standard grep/awk**:
```bash
# Filter by filename
./funcfinder --dir . --all | grep "finder.go"

# Extract line numbers
./funcfinder --dir . --all | awk -F: '{print $2}'
```

### Tree Format (Human-Readable)

```bash
./funcfinder --dir . --tree

# Output: Hierarchical structure
# Perfect for initial exploration
# Use --tree-full for complete tree including empty directories
```

## Edge Cases funcfinder Handles Correctly

**Why use funcfinder instead of regex?** Because simple regex breaks on:

### C# Verbatim Strings
```csharp
string path = @"C:\Users\Documents\file.txt";  // NOT a comment!
string sql = @"SELECT * FROM users WHERE id = 1 // not a comment";
```

### Python Docstrings
```python
"""
This is a docstring, not code.
Should not be counted as function definitions.
"""
def real_function():
    pass
```

### Go Raw Strings
```go
query := `SELECT * FROM users WHERE name = "John" // not a comment`
```

### Nested Comments
```javascript
/* Outer comment
   /* This would break simple regex */
   Still in comment
*/
```

**funcfinder uses a state-machine parser** that correctly handles all these cases, unlike naive `grep "^func"` or regex approaches.

## Performance Comparison

| Approach | Speed | Accuracy | Token Cost |
|----------|-------|----------|------------|
| **./funcfinder --dir . --all --json** | 763K lines/sec | 100% | 1 read |
| Reading each file individually | Depends on filesystem | 100% | N reads |
| `grep -r "^func"` | Fast | ~60% | N scans |
| `cat **/*.go \| grep` | Fast | ~40% | Huge tokens |

**Bottom line**: funcfinder is both faster AND more accurate while using 99% fewer tokens.

## Integration Examples

### Example 1: Find Entry Point

```bash
# Find main function across entire project
./funcfinder --dir . --all | grep ":main$"

# Result: cmd/app/main.go:15: main
# Now read only that file
```

### Example 2: Analyze Test Coverage

```bash
# Get all test functions
./funcfinder --dir . --all | grep "Test"

# Compare with implementation functions to identify untested code
./funcfinder --dir src --all | grep -v "Test" > implementations.txt
```

### Example 3: Refactoring Impact Analysis

```bash
# Before refactoring: map current structure
./funcfinder --dir . --all --json > before.json

# After refactoring: map new structure
./funcfinder --dir . --all --json > after.json

# Compare to see what changed
diff <(jq -r '.files[].functions[].name' before.json | sort) \
     <(jq -r '.files[].functions[].name' after.json | sort)
```

## Command Reference for AI Agents

### Directory Analysis
```bash
# Full scan with all features
./funcfinder --dir <path> --all --json [--recursive] [--workers N]

# Only functions (faster if types not needed)
./funcfinder --dir <path> --map

# Only types/classes/structs
./funcfinder --dir <path> --struct --json

# Ignore .gitignore (analyze dependencies)
./funcfinder --dir <path> --all --no-gitignore
```

### Single File Analysis
```bash
# Map functions in file
./funcfinder --inp <file> --source <lang> --map

# Extract specific function
./funcfinder --inp <file> --source <lang> --extract "<FuncName>"

# Get line range
./funcfinder --inp <file> --source <lang> --func "<FuncName>" --lines
```

### Output Formats
```bash
--json      # Machine-readable (best for parsing)
--map       # Grep-style (best for search)
--tree      # Hierarchical (best for humans)
--tree-full # Complete tree with empty dirs
```

## Best Practices Checklist

- ✅ **Always start with directory scan**: `./funcfinder --dir . --all --json`
- ✅ **Use JSON for structured queries**: Pipe to `jq` for precise extraction
- ✅ **Use grep format for simple searches**: Fast text filtering
- ✅ **Extract functions, don't read full files**: Use `--extract` after identifying targets
- ✅ **Leverage .gitignore**: Automatically skips vendor/, node_modules/, etc.
- ✅ **Use parallel processing**: Default workers = CPU cores (optimal)
- ✅ **Cache the directory map**: Save JSON output, reuse for multiple queries

- ❌ **Don't read files blindly**: Always map first
- ❌ **Don't use cat/grep for parsing**: Use funcfinder's state machine
- ❌ **Don't scan same directory repeatedly**: Cache and reuse map
- ❌ **Don't ignore the map**: It contains 99% of navigation info

## Token Efficiency Example

**Scenario**: Find and read the `ProcessRequest` function in a 50-file codebase.

**Naive approach**:
```bash
# Read all 50 files to find ProcessRequest
cat file1.go file2.go ... file50.go
# Token cost: ~500K tokens (assuming 10K tokens per file)
# Time: 50 file reads
# Accuracy: Depends on not missing the function
```

**funcfinder approach**:
```bash
# 1. Map codebase (30ms)
./funcfinder --dir . --all --json > map.json
# Token cost: ~5K tokens (just the map)

# 2. Search map (instant)
grep "ProcessRequest" map.json
# Found: handler.go:142

# 3. Extract function (5ms)
./funcfinder --inp handler.go --source go --extract "ProcessRequest"
# Token cost: ~500 tokens (just the function)

# Total: ~5.5K tokens vs 500K = 99% reduction
```

## Summary

**For AI Agents**: `funcfinder` is your X-ray vision into codebases. Use it to:

1. **Navigate efficiently**: Map first, read second
2. **Save tokens**: 99% reduction vs naive approaches
3. **Maintain accuracy**: State-machine parser handles edge cases
4. **Scale effortlessly**: Parallel processing + multi-language support

**Golden Rule**: If you're about to read a file to find a function, use `./funcfinder --dir . --all --json` instead. The map will tell you exactly where to look, and you'll save 99% of your token budget.

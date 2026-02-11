# funcfinder Use Cases

## AI-Driven Development

**Problem:** AI reading 10,000 lines when it needs 250

```bash
# 1. Get file structure (minimal tokens)
funcfinder --inp large_file.go --source go --map --json

# 2. AI selects function from map

# 3. Extract only that function (97.5% token savings!)
funcfinder --inp large_file.go --source go --func ProcessData --extract
```

## Git Hooks Integration

Automatically update code map on every commit:

```bash
# .git/hooks/post-commit
#!/bin/bash
funcfinder --dir . --all --json > .codemap.json 2>/dev/null
git add .codemap.json 2>/dev/null
```

```bash
chmod +x .git/hooks/post-commit
```

**Result:** Code map always up-to-date, instant search via jq.

## Struct/Class Analysis

```bash
# Map all types
funcfinder --dir ./models --struct --json

# Tree view with fields
funcfinder --inp models.py --source py --struct --tree

# Output:
# class User (10-25)
#   ├── field id: int (line 11)
#   ├── field name: str (line 12)
#   └── field email: str (line 13)
```

## Multi-Language Projects

```bash
# Auto-detects Go, Python, JavaScript, etc.
funcfinder --dir . --all --json

# Result includes language metadata per file
```

## Line Range Filtering

```bash
# Extract specific lines (works on any file)
funcfinder --inp app.log --lines 1000:1100

# Filter functions to specific area
funcfinder --inp large_file.go --source go --map --lines 500:1000
```

## jq Recipes

```bash
# Functions with paths
jq -r '.files[] | .path as $p | .functions[] | "\($p):\(.line): \(.name)"' map.json

# Find by name
jq '.files[] | .path as $p | .functions[] | select(.name | contains("Auth"))' map.json

# Count per file
jq -r '.files[] | "\(.path): \(.functions | length)"' map.json

# Top 10 files by function count
jq '[.files[] | {path, count: (.functions | length)}] | sort_by(-.count) | .[:10]' map.json
```

## Complexity Analysis

```bash
# Find complex functions
complexity . -l go -n 10

# Skip trivial functions
complexity file.go -l go --nosimple
```

**Levels:**
- SIMPLE (depth ≤ 2) - Flat code
- MODERATE (depth = 3) - One nesting level
- HIGH (depth ≥ 4) - Needs attention
- CRITICAL (depth ≥ 6) - Needs refactoring

## Dependency Analysis

```bash
# Analyze imports
deps file.go -l go -json

# Output:
{
  "file": "config.go",
  "dependencies": [
    {"name": "encoding/json", "count": 5},
    {"name": "regexp", "count": 3}
  ]
}
```

## Integration with Other Tools

```bash
# Combine with grep
grep -i "auth" map.json
funcfinder --inp auth.go --source go --func AuthHandler --extract

# Get line number in scripts
START=$(funcfinder --inp api.go --source go --func Handler --json | jq '.Handler.start')
```

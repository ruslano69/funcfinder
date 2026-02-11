# mini-SWE-agent Integration Guide

## Overview

funcfinder toolkit provides perfect CLI tools for [mini-SWE-agent](https://github.com/SWE-agent/mini-SWE-agent) - a minimalist AI coding agent that uses **only bash commands** to solve development tasks.

**Why perfect match:**
- ✅ Pure bash interface (no special tool-calling needed)
- ✅ Stateless execution (each command independent)
- ✅ JSON output for LLM parsing (`--json` everywhere)
- ✅ Zero dependencies (static binaries)
- ✅ Fast execution (280K lines/sec)

## Installation for mini-SWE-agent

```bash
# Install all funcfinder tools
go install github.com/ruslano69/funcfinder/cmd/funcfinder@latest
go install github.com/ruslano69/funcfinder/cmd/stat@latest
go install github.com/ruslano69/funcfinder/cmd/deps@latest
go install github.com/ruslano69/funcfinder/cmd/complexity@latest

# Or use pre-built binaries
wget https://github.com/ruslano69/funcfinder/releases/latest/download/funcfinder-linux-amd64.tar.gz
tar -xzf funcfinder-linux-amd64.tar.gz
sudo mv funcfinder stat deps complexity /usr/local/bin/
```

## Use Case 1: Understanding Codebase Before Fixing Bug

**Scenario:** mini-SWE-agent needs to fix a bug in `server.go`

**Problem:** LLM shouldn't read 5000 lines when bug is in 1 function

**Solution:**

```bash
# Step 1: Get file structure (minimal tokens)
funcfinder --inp server.go --source go --map --json

# LLM receives:
# {
#   "StartServer": {"start": 10, "end": 45},
#   "HandleRequest": {"start": 47, "end": 120},
#   "ProcessData": {"start": 122, "end": 350}
# }

# Step 2: Extract only the buggy function
funcfinder --inp server.go --source go --func HandleRequest --extract

# LLM receives 73 lines instead of 5000 (98.5% token savings!)
```

## Use Case 2: Finding High Complexity Functions to Refactor

**Scenario:** Agent asked to "improve code quality in auth module"

**Solution:**

```bash
# Find complex functions needing refactoring
complexity auth/ -t 4 -j

# LLM receives JSON:
# {
#   "functions": [
#     {
#       "name": "ValidateToken",
#       "file": "auth/validate.go",
#       "complexity": 256,
#       "max_nesting_depth": 9,
#       "level": "CRITICAL"
#     }
#   ]
# }

# Then extract for refactoring
funcfinder --inp auth/validate.go --source go --func ValidateToken --extract
```

## Use Case 3: Analyzing Dependencies Before Adding Import

**Scenario:** Agent needs to add a feature, wants to check existing imports

**Solution:**

```bash
# Check what's already imported
deps --inp server.go --source go --json

# LLM sees:
# {
#   "imports": ["net/http", "encoding/json", "database/sql"],
#   "stdlib": 3,
#   "external": 0
# }

# Analyze entire project dependencies
deps project/ --source go --summary
```

## Use Case 4: Counting Function Calls Before Optimization

**Scenario:** Agent optimizing hot path in performance-critical code

**Solution:**

```bash
# Find most-called functions (candidates for inlining)
stat --inp hot_path.go --source go --json

# LLM receives:
# {
#   "QueryDatabase": 47,
#   "ValidateInput": 35,
#   "FormatResponse": 28
# }

# Focus optimization on QueryDatabase (called 47 times)
```

## Use Case 5: Tree Navigation for Class-Based Refactoring

**Scenario:** Agent refactoring OOP code, needs class structure

**Solution:**

```bash
# Get class hierarchy
funcfinder --inp controller.py --source py --tree --json

# LLM sees structure:
# {
#   "classes": [
#     {
#       "name": "UserController",
#       "start": 10,
#       "end": 150,
#       "methods": [
#         {"name": "get_user", "start": 15, "end": 25},
#         {"name": "create_user", "start": 27, "end": 45}
#       ]
#     }
#   ]
# }
```

## Integration Pattern: Multi-Step Reasoning

mini-SWE-agent can chain these tools for complex tasks:

```bash
# Step 1: Find high complexity
COMPLEX=$(complexity src/ -t 5 -j | jq -r '.functions[0].name')

# Step 2: Extract function
funcfinder --inp src/main.go --source go --func $COMPLEX --extract > /tmp/func.go

# Step 3: Analyze dependencies
deps --inp /tmp/func.go --source go --json

# Step 4: Check call frequency
stat --inp src/ --source go --json | jq ".$COMPLEX"

# LLM now has:
# - Function body
# - Complexity metrics
# - Dependencies
# - Call frequency
# = Complete context for refactoring!
```

## Prompt Engineering for mini-SWE-agent

**Instruct the agent to use funcfinder toolkit:**

```yaml
# .miniswe.yaml
system_message: |
  You have access to funcfinder toolkit for efficient codebase navigation:

  - funcfinder: Find function boundaries, extract code
    Usage: funcfinder --inp FILE --source LANG --map/--tree/--func NAME --extract --json

  - complexity: Find complex functions needing refactoring
    Usage: complexity PATH -t THRESHOLD -j -n TOP_N

  - deps: Analyze imports and dependencies
    Usage: deps --inp FILE --source LANG --json

  - stat: Count function call frequency
    Usage: stat --inp FILE --source LANG --json

  IMPORTANT: Always use funcfinder to extract specific functions instead of reading
  entire files. This saves tokens and focuses analysis.

  WORKFLOW for bug fixes:
  1. Use complexity to find problem areas
  2. Use funcfinder --map --json to get file structure
  3. Use funcfinder --extract to get only relevant code
  4. Make changes
  5. Test with existing test suite
```

## Performance Benefits

| Traditional Approach | With funcfinder |
|---------------------|-----------------|
| Read 10K line file | Get function map (50 lines JSON) |
| Parse entire AST | Regex-based (280K lines/sec) |
| Load IDE/LSP | Zero dependencies |
| Token cost: 10K | Token cost: 50 (99.5% reduction) |

## Advanced: Container Integration

Since mini-SWE-agent uses subprocess.run, funcfinder works seamlessly in containers:

```bash
# Docker execution
docker run --rm -v $(pwd):/code funcfinder:latest \
  funcfinder --inp /code/main.go --source go --map --json

# Podman execution
podman run --rm -v $(pwd):/code:Z funcfinder:latest \
  complexity /code -t 4 -j
```

## Real-World Example: Fix Authentication Bug

**Task:** "Fix the token validation bug in auth/middleware.go"

**Agent execution:**

```bash
# 1. Understand structure (10 tokens)
$ funcfinder --inp auth/middleware.go --source go --map --json
{
  "ValidateToken": {"start": 45, "end": 120},
  "RefreshToken": {"start": 122, "end": 180}
}

# 2. Extract buggy function (75 lines instead of 500)
$ funcfinder --inp auth/middleware.go --source go --func ValidateToken --extract
// ValidateToken: 45-120
func ValidateToken(token string) (*Claims, error) {
    // ... 75 lines of code
}

# 3. Check complexity (is it too complex?)
$ complexity auth/middleware.go -j | jq '.functions[] | select(.name=="ValidateToken")'
{
  "name": "ValidateToken",
  "complexity": 64,
  "max_nesting_depth": 7,
  "level": "VERY_HIGH"
}

# 4. Check dependencies
$ deps --inp auth/middleware.go --source go --json
{
  "imports": ["github.com/golang-jwt/jwt", "crypto/rsa"],
  "stdlib": 1,
  "external": 1
}

# Agent now has full context with 99% token savings!
# Makes targeted fix and commits
```

## Supported Languages (15)

All tools work with:
- Go, C, C++, C#, Java, D
- JavaScript, TypeScript
- Python
- Rust, Swift
- Kotlin, PHP, Ruby, Scala

## API Responses Format

All tools support `--json` for structured LLM consumption:

```json
// funcfinder --map --json
{
  "FunctionName": {"start": 10, "end": 50}
}

// complexity -j
{
  "language": "go",
  "total_functions": 25,
  "functions": [
    {
      "name": "ComplexFunc",
      "complexity": 512,
      "max_nesting_depth": 10,
      "level": "CRITICAL"
    }
  ]
}

// deps --json
{
  "imports": ["fmt", "os"],
  "stdlib": 2,
  "external": 0
}

// stat --json
{
  "FuncA": 15,
  "FuncB": 8
}
```

## Troubleshooting

**Q: Agent reads whole files instead of using funcfinder**

A: Add explicit instruction in system prompt:
```
RULE: Before reading any file >100 lines, ALWAYS use:
funcfinder --inp FILE --source LANG --map --json
Then extract only needed functions with --extract
```

**Q: How to make agent use complexity for refactoring?**

A: Include in task prompt:
```
Before refactoring, run: complexity PROJECT/ -t 4 -n 10 -j
Focus on functions with level >= "HIGH"
```

**Q: Agent not using --json flag**

A: Modify system message:
```
ALWAYS use --json flag with funcfinder/complexity/deps/stat
for structured output that you can parse reliably.
```

## Contributing

See [SWE-agent integration examples](https://github.com/ruslano69/funcfinder/tree/main/examples/swe-agent) for more patterns.

## See Also

- [mini-SWE-agent GitHub](https://github.com/SWE-agent/mini-SWE-agent)
- [funcfinder Documentation](../README.md)
- [Complexity Analysis Guide](COMPLEXITY.md)

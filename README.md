# funcfinder

[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Platform](https://img.shields.io/badge/platform-Linux%20%7C%20macOS%20%7C%20Windows-lightgrey)](https://github.com/ruslano69/funcfinder)

**Production-grade code analysis factory for multi-language codebases**

`funcfinder` is not just a parser—it's a **universal code analysis factory** that automatically detects languages, extracts functions/classes/types, and scales from single files to entire repositories. Built on a state-machine sanitizer (not regex), it handles C# verbatim strings, Python docstrings, and nested comments correctly where simple regex fails.

**⚡ Performance**: 763,000 lines/sec parsing · Parallel processing with worker pools · Processes 100K lines in ~130ms

## ✨ What Makes It Different

### 🏭 Factory Architecture, Not Simple Regex
- **Language Factory**: Auto-detects 15+ languages by extension
- **Parser Factory**: Switches between brace-based (Go/Java) and indent-based (Python) parsers
- **Enhanced Sanitizer**: State machine that correctly handles edge cases regex can't (C# `@"..."`, Python `"""`, nested comments)
- **Multiple Finders**: Function finder, struct finder, combined mode—all through unified API

### 🚀 Production Features

- 📁 **Directory Mode** ⭐ NEW: `--dir ./project` scans entire repositories with automatic language detection
- ⚡ **Parallel Processing**: `--workers 8` for 4-8x speedup on large codebases
- 🎯 **Smart Filtering**: Automatic `.gitignore` support, skip `node_modules`, `vendor`, etc.
- 🔍 **Three Analysis Modes**:
  - `--map` (default): Find all functions
  - `--struct`: Find only classes/structs/types
  - `--all`: Both functions and types
- 📊 **Output Formats**: grep-style, JSON, tree, tree-full, extract bodies
- 🎨 **Multi-language**: Single command analyzes mixed Go/Python/C++/Java projects
- 🪟 **Cross-platform**: Linux, macOS, Windows—zero dependencies

### 📈 Real-World Performance

```bash
# Small project (21 files, mixed languages)
funcfinder -dir test_examples --all --map
→ 273 functions + 438 types in ~30-45ms

# Medium project (25 Go files)
funcfinder -dir internal --all --json
→ 228 functions + 69 classes in ~40ms

# Parallel speedup
--workers 1: 45ms  →  --workers 4: 29ms (1.55x faster)
```

## 🌐 Core Capabilities

- 🔍 **Single File Analysis**: `--inp file.go --map` for targeted extraction
- 📁 **Directory Scanning**: `--dir ./src --all` for entire codebases
- 🏗️ **Type Extraction**: `--struct` finds classes, structs, interfaces, enums
- 🔄 **Combined Analysis**: `--all` gets both functions and types in one pass
- 🗺️ **Codebase Mapping**: `--tree` shows hierarchical structure
- 📏 **Precise Extraction**: `--lines 50:100` for specific code ranges
- 📤 **Body Extraction**: `--extract` pulls complete function bodies
- 📊 **JSON Export**: `--json` for programmatic processing
- ⚡ **Performance**: 763K lines/sec sanitizer, parallel worker pools
- 🎯 **Zero Dependencies**: Single static binary

## 🌐 Supported Languages (15)

- Go
- C
- C++
- C#
- Java
- D
- **JavaScript** (including async functions, generator functions, arrow functions)
- **TypeScript** (including async functions, generator functions, arrow functions, generics)
- **Python** (including async/await, decorators, generators, class methods)
- **Rust** (including pub/async functions, structs, traits, enums, impl blocks)
- **Swift** (including classes, structs, protocols, enums, static functions)
- **Kotlin** ⭐ NEW (including suspend functions, data classes, sealed classes, objects)
- **PHP** ⭐ NEW (including classes, traits, interfaces, visibility modifiers)
- **Ruby** ⭐ NEW (including modules, class methods, methods with ? and !)
- **Scala** ⭐ NEW (including case classes, traits, objects, pattern matching)

## 📦 Installation

### Via Go Install (Recommended)

```bash
go install github.com/ruslano69/funcfinder@latest
```

### Pre-built Binaries

Download from [Releases](https://github.com/ruslano69/funcfinder/releases):

```bash
# Linux
wget https://github.com/ruslano69/funcfinder/releases/download/v1.6.0/funcfinder-linux-amd64.tar.gz
tar -xzf funcfinder-linux-amd64.tar.gz
sudo mv funcfinder /usr/local/bin/

# macOS
wget https://github.com/ruslano69/funcfinder/releases/download/v1.6.0/funcfinder-darwin-amd64.tar.gz
tar -xzf funcfinder-darwin-amd64.tar.gz
sudo mv funcfinder /usr/local/bin/

# Windows
# Download funcfinder-windows-amd64.zip and add to PATH
```

### From Source

```bash
git clone https://github.com/ruslano69/funcfinder.git
cd funcfinder

# Linux/macOS: Build all utilities (funcfinder, stat, deps, complexity)
./build.sh

# Windows (PowerShell): Build all utilities
.\build.ps1

# Or build funcfinder only
go build  # Now works! ✅
```

**✅ Fixed:** `go build` now works without errors! Other utilities use build tags and are built via `build.sh`/`build.ps1`.

For Windows-specific instructions, see [docs/WINDOWS.md](docs/WINDOWS.md).


## 🚀 Quick Start

### Directory Mode (⭐ Most Powerful)

```bash
# Scan entire project (auto-detects languages)
funcfinder --dir ./myproject --map --tree

# Find all functions + classes in repository
funcfinder --dir ./src --all --json > codebase.json

# Fast parallel scan with 8 workers
funcfinder --dir . --map --workers 8

# Only structs/classes (skip functions)
funcfinder --dir ./models --struct --map

# Output:
# INFO: Scanning directory: ./src (mode=all, workers=8, gitignore=true)
# └── src
# ├── main.go
# │   ├── def main (line 10)
# │   └── class Server (line 25)
# ├── handler.py
# │   ├── def process (line 5)
# │   └── class Handler (line 20)
# INFO: Processed 15 files, found 45 functions, 12 classes/types
```

### Single File Mode

#### Check version

```bash
funcfinder --version
# Output: funcfinder version 1.6.0
```

#### Map all functions in a file

```bash
funcfinder --inp main.go --source go --map
# Output: main: 10-25; Handler: 45-78; helper: 65-72;
```

### Find specific functions

```bash
funcfinder --inp api.go --source go --func Handler,Middleware
# Output: Handler: 45-78; Middleware: 80-95;
```

### JSON output for AI

```bash
funcfinder --inp api.go --source go --map --json
```

```json
{
  "Handler": {"start": 45, "end": 78},
  "Middleware": {"start": 80, "end": 95}
}
```

### Extract function body

```bash
funcfinder --inp api.go --source go --func Handler --extract
```

```go
// Handler: 45-78
func Handler(w http.ResponseWriter, r *http.Request) {
    // function body...
}
```

### Find structs/classes/types (NEW in v1.5.0)

```bash
# Map all types in a Go file
funcfinder --inp models.go --source go --struct --map
# Output: User: 10-15; fields: ID, Name, Email Address: 20-25; fields: Street, City, Zip

# Find specific types
funcfinder --inp models.go --source go --struct --type User,Address
# Output: User: 10-15; fields: ID, Name, Email Address: 20-25; fields: Street, City, Zip

# JSON output for types
funcfinder --inp models.py --source py --struct --map --json
```

```json
{
  "filename": "models.py",
  "types": [
    {
      "name": "User",
      "kind": "class",
      "start": 5,
      "end": 12,
      "fields": [
        {"name": "id", "type": "int", "line": 6},
        {"name": "name", "type": "str", "line": 7}
      ]
    }
  ]
}
```

### Combined mode: functions + structs (NEW in v1.5.0)

```bash
# Get complete file structure in one call
funcfinder --inp service.go --source go --all --map

# Output:
# === FUNCTIONS ===
# NewService: 30-35; Process: 40-55; Validate: 60-70;
#
# === TYPES ===
# Service: 10-15; fields: db, cache Config: 20-25; fields: Host, Port

# JSON output with both functions and types
funcfinder --inp api.go --source go --all --json
```

```json
{
  "filename": "api.go",
  "functions": [
    {"name": "NewService", "start": 30, "end": 35},
    {"name": "Process", "start": 40, "end": 55}
  ],
  "types": [
    {
      "name": "Service",
      "kind": "struct",
      "start": 10,
      "end": 15,
      "fields": [
        {"name": "db", "type": "*sql.DB", "line": 11},
        {"name": "cache", "type": "Cache", "line": 12}
      ]
    }
  ]
}
```

## 🏗️ Architecture: Why Not Just Regex?

### The Problem with Simple Regex Parsers

Most code parsers use naive regex patterns and fail on edge cases:

```csharp
// ❌ Simple regex breaks here:
string path = @"C:\Users\Test";  // C# verbatim string
string msg = @"He said ""Hello""";  // Escaped quotes in verbatim

// ❌ Regex truncates this:
query := `SELECT * FROM users // not a comment`  // Go raw string

// ❌ Regex sees 6 quotes, not 1 docstring:
"""This is a Python docstring"""
```

### funcfinder's Solution: Factory + State Machine

```
Input File → Language Factory → Parser Factory → Enhanced Sanitizer → Pattern Matching
     │              │                  │                    │                  │
  file.go      GoConfig          Finder           State Machine         Find Functions
  file.py    PythonConfig    PythonFinder     (763K lines/sec)         Extract Bodies
  file.cs      CSharpConfig       Finder       Handles verbatim            Output
```

**Key Components:**

1. **Language Factory** (`languages.json`)
   - Auto-detects language by extension
   - Loads correct parser config (brace-based vs indent-based)
   - Supports 15+ languages with language-specific features

2. **Enhanced Sanitizer** (State Machine, not regex)
   - **7 states**: Normal, LineComment, BlockComment, String, RawString, CharLiteral, MultiLineString
   - Correctly handles: C# `@"..."`, Python `"""`, Go `` `...` ``, nested comments
   - Performance: 763,000 lines/sec
   - [Technical deep-dive](docs/COMPLEXITY_ANALYSIS.md)

3. **Parser Factory**
   - `CreateFinder()` → Brace-based (Go, Java, C++) or Indent-based (Python)
   - `CreateStructFinder()` → Type extraction with field parsing
   - Unified API regardless of language

4. **Directory Processor**
   - Worker pool for parallel processing
   - `.gitignore` pattern matching
   - Language detection per file
   - Result aggregation

**Result:** Handles production code that breaks simple regex parsers.

### Proof: Test Results

```bash
# C# verbatim strings (broken in most tools)
funcfinder --inp test.cs --source cs --map
✅ Correctly handles @"C:\Users" and @"He said ""Hello"""

# Python docstrings (often counted as 6 separate strings)
funcfinder --inp test.py --source py --map
✅ Treats """...""" as single multiline string

# Go raw strings with comment-like content
funcfinder --inp test.go --source go --map
✅ `SELECT // not comment` parsed correctly
```

See [STAT_FIX_COMPLETED.md](docs/STAT_FIX_COMPLETED.md) for detailed comparison.

## 🤖 AI Agent Integration

### mini-SWE-agent Support

funcfinder provides **perfect CLI tools** for [mini-SWE-agent](https://github.com/SWE-agent/mini-SWE-agent) - a minimalist AI coding agent that uses only bash commands.

**Why perfect match:**
- ✅ Pure bash interface (no special tool-calling)
- ✅ Stateless execution (each command independent)
- ✅ JSON output everywhere (`--json` flag)
- ✅ 99% token reduction vs reading full files

**Quick Example:**
```bash
# Agent workflow: Fix bug in auth/middleware.go

# 1. Get COMPLETE structure - functions + types (50 tokens vs 5000) ⭐ NEW
funcfinder --inp auth/middleware.go --source go --all --json

# 2. Extract buggy function (150 tokens vs 5000)
funcfinder --inp auth/middleware.go --source go --func ValidateToken --extract

# 3. Check related types (understand data structures)
funcfinder --inp auth/middleware.go --source go --struct --type TokenData --extract

# 4. Check complexity
complexity auth/middleware.go -j | jq '.functions[] | select(.name=="ValidateToken")'

# 5. Make targeted fix with 99% token savings! 🎉
```

**Why --all is perfect for AI agents:**
- ✅ One call = complete context (functions + data structures)
- ✅ Understand both behavior (functions) and state (types)
- ✅ Minimal tokens for maximum insight
- ✅ Perfect for code understanding and refactoring tasks

**See:** [Complete Integration Guide](docs/MINI_SWE_AGENT_INTEGRATION.md) | [Example Workflows](examples/swe-agent/)

## 💡 Use Cases

### AI-Driven Development

**Problem:** AI reading 10,000 lines when it needs 250

**Solution:** 
```bash
# 1. Get file structure (minimal tokens)
funcfinder --inp large_file.go --source go --map --json

# 2. AI selects needed function from map

# 3. Extract only that function (97.5% token savings!)
funcfinder --inp large_file.go --source go --func ProcessData --extract
```

### Code Navigation

```bash
# Find all methods in a C# file
funcfinder --inp Controller.cs --source cs --map --json > functions.json

# Extract specific method for review
funcfinder --inp Controller.cs --source cs --func CreateUser --extract
```

### JavaScript/TypeScript Support

```bash
# Find all functions in a JavaScript file
funcfinder --inp app.js --source js --map --json

# Extract async function from TypeScript
funcfinder --inp api.ts --source ts --func fetchUser --extract

# Find generator functions
funcfinder --inp generators.js --source js --func simpleGenerator --extract

# Extract arrow functions
funcfinder --inp utils.js --source js --func arrowFunc,asyncArrow --extract

# Find React component methods
funcfinder --inp Component.jsx --source js --func render,componentDidMount
```

### Python Support with Decorators

```bash
# Map all functions in Python file
funcfinder --inp api.py --source py --map

# Extract function with decorators
funcfinder --inp api.py --source py --func cached_function --extract

# JSON output includes decorators
funcfinder --inp api.py --source py --func get_user --json
{
  "get_user": {
    "decorators": [
      "@require_auth",
      "@validate_input"
    ],
    "end": 42,
    "start": 35
  }
}

# Find async functions and generators
funcfinder --inp utils.py --source py --func async_generator,fibonacci --extract
```

### Struct/Type Finding (NEW in v1.5.0)

```bash
# Find all types in a Go file
funcfinder --inp models.go --source go --struct --map
# Output: User: 10-20; fields: ID, Name, Email Config: 25-30; fields: Host, Port

# Find specific structs/classes
funcfinder --inp models.py --source py --struct --type User,Product --extract

# Get complete file structure (functions + types)
funcfinder --inp service.go --source go --all --json
{
  "functions": [{"name": "NewService", "start": 35, "end": 45}],
  "types": [
    {
      "name": "Service",
      "kind": "struct",
      "fields": [{"name": "db", "type": "*sql.DB", "line": 12}]
    }
  ]
}

# Find C++ classes and structs
funcfinder --inp widget.cpp --source cpp --struct --map

# Find Java classes and interfaces
funcfinder --inp api.java --source java --struct --map

# Find Python classes with fields
funcfinder --inp models.py --source py --struct --tree
# Output:
# class User (10-25)
#   ├── field id: int (line 11)
#   ├── field name: str (line 12)
#   └── field email: str (line 13)
```

**Why struct finding is useful:**
- 🏗️ **Understand data structures** before modifying code
- 🔍 **Find type definitions** across large codebases
- 📊 **JSON output** for AI-powered refactoring
- 🔄 **Combined with functions** for complete context

### Tree Visualization for Classes

```bash
# Display class hierarchy in tree format
funcfinder --inp Calculator.java --source java --tree

# Output:
# class Calculator (1-20)
# ├── method add (5-7)
# ├── method subtract (9-11)
# └── method multiply (13-15)
# class Helper (22-30)
# ├── method assist (23-25)
# └── method process (27-29)

# Tree with full signatures
funcfinder --inp api.ts --source ts --tree-full

# Visualize Python classes (with decorators!)
funcfinder --inp models.py --source py --tree
```

### Line Range Filtering (v1.4.0+)

```bash
# Standalone mode: Fast file slicing (works on ANY file, no --source needed)
funcfinder --inp app.log --lines 1000:1100
# Output: Lines 1000-1100 with line numbers

# JSON output for line ranges
funcfinder --inp config.yaml --lines :50 --json

# Filter mode: Narrow function search to specific lines
funcfinder --inp large_file.go --source go --map --lines 500:1000
# Only shows functions within lines 500-1000

# Find function in specific area (much faster for large files)
funcfinder --inp server.js --source js --func handleAPI --lines 100:500 --extract

# Tree view of limited scope
funcfinder --inp Calculator.java --source java --tree --lines 1:100

# Windows-compatible sed alternative (10-50x faster than PowerShell)
funcfinder --inp server.log --lines 5000:   # From line 5000 to EOF
funcfinder --inp debug.txt --lines :1000    # First 1000 lines
funcfinder --inp trace.log --lines 500      # Single line 500
```

**Why --lines is useful:**
- 🪟 **Cross-platform**: Works on Windows without sed
- ⚡ **Performance**: 10-50x faster than PowerShell alternatives
- 🎯 **Precision**: Combine with --map/--func/--tree to narrow search scope
- 📏 **Any file**: Standalone mode works on logs, configs, any text file

### Integration with Other Tools

```bash
# Combine with grep/mgrep for comprehensive analysis
mgrep "authentication" api.go
funcfinder --inp api.go --source go --func AuthHandler --extract

# Get function start line in scripts
START=$(funcfinder --inp api.go --source go --func Handler --json | jq '.Handler.start')
```

## 🔄 Usage Scenarios

### Git Hooks Integration: "Commit Once, Search Instantly Forever"

Automatically update your code map on every commit. Set up once — instant search forever.

**1. Create post-commit hook:**

```bash
# .git/hooks/post-commit
#!/bin/bash
funcfinder --dir . --all --json > .codemap.json 2>/dev/null
git add .codemap.json 2>/dev/null
```

```bash
chmod +x .git/hooks/post-commit
```

**2. Add project configuration (optional):**

```bash
# .funcfinder.config
EXCLUDE_DIRS="node_modules,vendor,.git,dist,build"
WORKERS=8
OUTPUT_FORMAT="json"
```

**3. Result:**

| Approach | Search Time | Context |
|----------|-------------|---------|
| `grep -r "func"` | ~2-5 sec | Text only |
| `funcfinder + hooks` | **instant** | Structure + boundaries + types |

**Benefits:**
- 📍 Code map always up-to-date
- ⚡ Instant search via `.codemap.json`
- 🔍 AI agents get structure without parsing
- 💾 Minimal overhead (JSON ~50KB for medium project)

**Using the map:**

```bash
# Find functions in auth module
jq '.files[] | select(.path | contains("auth")) | .functions[]' .codemap.json

# List all classes with paths
jq '.files[] | .path as $p | .types[] | "\($p):\(.name)"' .codemap.json

# Project statistics
jq '{files: (.files | length), functions: [.files[].functions[]] | length}' .codemap.json
```

## 📖 Usage

```
funcfinder --inp <file> [--source <lang>] [OPTIONS]

Required:
  --inp <file>       Source file to analyze
  --source <lang>    Language: go/c/cpp/cs/java/d/js/ts/py/rust/swift/kotlin/php/ruby/scala
                     (optional when using --lines alone)

Work modes (choose one):
  (default)          Find functions (default behavior)
  --struct           Find structs/classes/types instead of functions ⭐ NEW
  --all              Find both functions and structs ⭐ NEW

Search modes (choose one):
  --func <names>     Find specific functions (comma-separated)
  --type <names>     Find specific types (comma-separated, requires --struct) ⭐ NEW
  --map              Map all functions/types in file
  --tree             Display in tree format (shows class hierarchy)
  --tree-full        Display in tree format with signatures

Filtering:
  --lines <range>    Extract/filter by line range (standalone or with --source)
                     Formats: 100:150, :50, 100:, 100

Output formats:
  (default)          grep-style: name: n1-n2;
  --json             JSON format
  --extract          Extract function/type bodies

Options:
  --raw              Don't ignore raw strings in brace counting
  --version          Print version and exit
```

### Examples of flag combinations:

```bash
# Functions (default mode)
funcfinder --inp file.go --source go --map                    # Map all functions
funcfinder --inp file.go --source go --func Handler          # Find specific function

# Structs mode
funcfinder --inp file.go --source go --struct --map          # Map all types
funcfinder --inp file.go --source go --struct --type User    # Find specific type

# Combined mode (--all requires --map, --tree, or --json)
funcfinder --inp file.go --source go --all --map             # Map functions + types
funcfinder --inp file.go --source go --all --json            # JSON with both
funcfinder --inp file.go --source go --all --extract         # Extract both

# Invalid combinations (will error)
funcfinder --inp file.go --source go --struct --all          # Mutually exclusive
funcfinder --inp file.go --source go --func foo --struct     # Can't mix modes
funcfinder --inp file.go --source go --type User             # Need --struct or --all
```

## 🎯 Token Reduction Examples

### Example 1: Large Codebase Analysis

**Traditional approach:**
- AI reads entire codebase: 100,000 lines = 150,000 tokens
- Cost: $0.45 (at $0.003/1K tokens)
- Time: Multiple AI requests, ~5-10 seconds

**With funcfinder:**
```bash
# 1. Get structure (280,000 lines/sec)
funcfinder --inp . --source go --map --json
```
- Analysis time: 0.36 seconds for 100K lines
- Tokens sent to AI: ~500 tokens (JSON structure)
- Cost: $0.0015
- **Token savings: 99.67% | Cost savings: 300x | Time: faster than 1 AI request!**

### Example 2: Targeted Function Extraction

**Traditional approach:**
- AI reads entire file: 5,000 lines = 7,500 tokens

**With funcfinder:**
```bash
# 1. Map functions
funcfinder --inp api.go --source go --map --json

# 2. AI selects function from structure (50 tokens)

# 3. Extract only that function
funcfinder --inp api.go --source go --func ProcessData --extract
```
- Tokens used: 50 (structure) + 375 (function body) = 425 tokens
- **Token savings: 94%**

## 🏗️ Architecture

```
funcfinder/
├── cmd/                        # CLI entry points
│   ├── funcfinder/main.go      # Main tool
│   ├── stat/main.go            # Call counter
│   ├── deps/main.go            # Dependency analyzer
│   └── complexity/main.go      # Complexity analyzer
├── internal/                   # Core logic
│   ├── config.go               # Language configuration
│   ├── languages.json          # 15 language patterns (embedded)
│   ├── finder.go               # Function boundary detection
│   ├── structfinder.go         # Class/struct detection
│   ├── enhanced_sanitizer.go   # State-machine parser
│   ├── dirprocessor.go         # Directory scanning
│   ├── python_finder.go        # Python-specific logic
│   ├── formatter.go            # Output formatting
│   └── tree.go                 # Tree visualization
├── examples/
│   └── analyze.sh              # Full project analysis
├── docs/                       # Documentation
└── test_examples/              # Test files (15 languages)
```

## 🔧 Configuration

Language patterns are defined in `languages.json` (embedded in binary):

```json
{
  "go": {
    "func_pattern": "^\\s*func\\s+(\\([^)]*\\)\\s+)?(\\w+)\\s*\\(",
    "line_comment": "//",
    "block_comment_start": "/*",
    "block_comment_end": "*/",
    "string_chars": ["\""],
    "raw_string_chars": ["`"],
    "escape_char": "\\"
  }
}
```

## 🧪 Testing

Tested on:
- Go standard library (`fmt/print.go`)
- Production C# code (TELB project)
- Real-world codebases with complex nesting

```bash
# Run tests
go test ./...

# Test on sample file
funcfinder --inp config.go --source go --map
```

## 🛠️ Additional Utilities

funcfinder ships with additional utilities for comprehensive code analysis. All utilities share a **unified architecture** with common configuration and error handling modules.

### Quick Start

```bash
# Build all utilities
./build.sh

# Full project analysis in one command
./examples/analyze.sh

# AI agent workflow
funcfinder --inp api.go --source go --map  # Code structure
stat api.go -l go -n 10                    # Hotspots
deps . -l go -j                            # Dependency graph
complexity api.go -l go                    # Cognitive complexity
```

### Utilities

| Utility | Purpose | Languages | Output |
|---------|---------|-----------|--------|
| **funcfinder** | Code structure (functions, classes, boundaries) | 15 | grep/JSON/extract |
| **stat** | Function call analysis + file metrics | 15 | text |
| **deps** | Module dependency analysis (stdlib/external/internal) | 15 | text/JSON |
| **complexity** | Cognitive complexity analyzer (nesting depth) | 15 | colored text |

### 🧠 complexity - Cognitive Complexity Analyzer

**Philosophy:** Deep nesting (not branch count) is the real complexity.

```bash
# Analyze single file
complexity main.go -l go

# Analyze directory
complexity . -l go

# JSON output for automation
complexity api.py -l py --json

# Top N most complex functions
complexity . -l go -n 10
```

**Complexity Levels:**
- ✅ **SIMPLE** (depth ≤ 2) - Flat code, easy to understand
- ⚠️ **MODERATE** (depth = 3) - One nesting level
- 🔶 **HIGH** (depth ≥ 4) - Two+ nesting levels
- 🔴 **CRITICAL** (depth ≥ 6) - Needs refactoring

**Formula:** `NDC = 2^(maxDepth - 1)`

### 📊 Full Analysis with analyze.sh

```bash
./examples/analyze.sh
```

**Report includes:**
- 📈 File statistics (lines, size, code/comments/blank ratio)
- 🔍 Function inventory
- 🔥 Call hotspots (top functions by frequency)
- 📦 Dependency graph (stdlib vs external vs internal)
- 🧠 Complexity distribution (SIMPLE/MODERATE/HIGH/CRITICAL)

### 🏗️ Unified Architecture (v1.4.0)

All utilities share **common modules** (DRY principle):

```
funcfinder/
├── internal/
│   ├── config.go          # Unified language configuration
│   ├── errors.go          # Standard error handling
│   └── languages.json     # Single source of patterns (embedded)
├── cmd/
│   ├── funcfinder/        # Main CLI
│   ├── stat/              # Call counter + metrics
│   ├── deps/              # Dependency analyzer
│   └── complexity/        # Complexity analyzer
└── examples/
    └── analyze.sh         # Full project analysis
```

**Architecture Benefits:**
- ✅ **Zero duplication** - single config for all utilities
- ✅ **Consistency** - same error messages everywhere
- ✅ **Easy extension** - add language = update JSON
- ✅ **Zero dependencies** - all utilities are static binaries

**Common Use Cases:**
- 📊 Initial analysis of unfamiliar code
- 🔍 Finding optimization bottlenecks
- 🔄 Refactoring and duplication detection
- 📈 Code review and PR analysis
- 🤖 AI agent navigation with minimal tokens
- 🧠 Complexity assessment before refactoring

## 🤝 Contributing

Contributions welcome! Please follow these guidelines:

**How to contribute:**
- Fork the repository and create a feature branch
- Write tests for new functionality
- Follow existing code style and conventions
- Submit PR with clear description

**Areas for contribution:**
- Additional language support
- Improved regex patterns
- Performance optimizations
- Test coverage

## 📊 Performance

### Verified Benchmarks

**Parsing throughput:** **280,000 lines/sec** (3.6 μs per line)

```bash
# Real-world performance (verified with benchmark tool)
100,000 lines → 0.36 seconds
280,000 lines → 1.00 second

# This means funcfinder analyzes 100K lines FASTER than a single AI API request! (~500ms)
```

### ⚡ The X-Ray and Microscope for AI

**What makes funcfinder unique:**
- 🔬 **X-ray vision**: Scan entire codebase structure in milliseconds
- 🔍 **Microscope precision**: Extract exact functions with zero noise
- 🚀 **Faster than AI requests**: 100K lines in 360ms vs AI request ~500ms
- 💰 **99.67% token savings**: 150,000 tokens → 500 tokens for structure

**Why this matters for AI workflows:**
```bash
Traditional approach (without funcfinder):
├── Read entire file: 150,000 tokens @ $0.003/1K = $0.45
├── Wait for AI processing: ~500-1000ms
└── Get answer with full file context

With funcfinder:
├── Get file structure: 0.36 seconds for 100K lines
├── Send structure to AI: 500 tokens @ $0.003/1K = $0.0015
├── AI selects function: instant
└── Extract function: <1ms
   Total: 0.36s + minimal cost (300x cheaper!)
```

### Technical Details

- **Complexity:** O(n) linear with respect to file size
- **Memory:** Minimal (streaming line-by-line processing)
- **Binary size:** 3MB (static, no external dependencies)
- **Platform:** Cross-platform (Linux, macOS, Windows)

## 🗺️ Roadmap

### v1.1.0 ✅
- [x] JavaScript/TypeScript support
- [x] `--version` flag
- [x] Arrow function support for JS/TS
- [x] Generator function support

### v1.2.0 ✅
- [x] Python support with decorator detection
- [x] Async/await function support
- [x] Improved function detection across all languages

### v1.3.0 ✅
- [x] Tree visualization (`--tree` and `--tree-full`)
- [x] Class hierarchy detection
- [x] Method-class association for all OOP languages
- [x] **Rust support** (structs, traits, enums, impl blocks)
- [x] **Swift support** (classes, structs, protocols, enums)
- [x] **Kotlin, PHP, Ruby, Scala support** ⭐ NEW
- [x] **15 languages total** (added without Go code changes!)

### v1.4.0 ✅
- [x] **--lines flag** for line range filtering
- [x] Cross-platform file slicing (sed alternative)
- [x] Standalone and filter modes
- [x] **stat utility** - function call counter + file metrics (15 languages)
- [x] **deps utility** - dependency analyzer (15 languages)
- [x] **complexity utility** ⭐ NEW - cognitive complexity analyzer (15 languages)
- [x] **Unified architecture** - shared config.go and errors.go (DRY principle)
- [x] **analyze.sh** - comprehensive project analysis script
- [x] Complete code analysis toolkit with zero dependencies

### v1.5.0 (Current) ✅
- [x] **--struct flag** - Find structs/classes/types/interfaces ⭐ NEW
- [x] **--type flag** - Find specific types by name ⭐ NEW
- [x] **--all flag** - Combined mode (functions + structs) ⭐ NEW
- [x] **EnhancedSanitizer** - Multiline strings, char literals, nested comments
- [x] **Struct pattern support** for all 15 languages
- [x] **Field extraction** - Extract type fields with names and types
- [x] **Combined JSON output** - Functions and types in single response
- [x] **Nested function support** - Python, JS, TS, Go, Ruby, Scala
- [x] Complete code structure analysis (behavior + state)

### v1.6.0
- [ ] Configuration file support (.funcfinderrc)
- [ ] Custom patterns via CLI
- [ ] Improved C# regex patterns
- [ ] Function type filters (public/private)
- [ ] Cyclomatic complexity (as alternative to nesting depth)
- [ ] HTML reports for analyze.sh

### v2.0.0
- [ ] Tree-sitter integration for precise parsing
- [ ] 30+ language support
- [ ] API server mode
- [ ] IDE integrations

## 📚 Documentation

- **[docs/WINDOWS.md](docs/WINDOWS.md)** - Complete Windows build and usage guide
- **[docs/UTILITIES.md](docs/UTILITIES.md)** - Documentation for stat, deps, complexity utilities
- **[docs/COMPLEXITY.md](docs/COMPLEXITY.md)** - Cognitive complexity analyzer guide
- **[docs/examples/](docs/examples/)** - Example scripts and demonstrations
- **[CHANGELOG.md](CHANGELOG.md)** - Version history and release notes
- **[docs/archive/](docs/archive/)** - Analysis reports and benchmarks

## 📄 License

MIT License - see [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

Built for AI-driven development workflows. Inspired by the need to minimize token usage in large codebases.

## 📞 Support

- 🐛 [Report Issues](https://github.com/ruslano69/funcfinder/issues)
- 💡 [Feature Requests](https://github.com/ruslano69/funcfinder/issues)
- 📖 [Documentation](https://github.com/ruslano69/funcfinder/wiki)

---

**funcfinder** - Navigate code efficiently, save tokens intelligently 🚀

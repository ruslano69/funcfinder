# funcfinder

[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Platform](https://img.shields.io/badge/platform-Linux%20%7C%20macOS%20%7C%20Windows-lightgrey)](https://github.com/ruslano69/funcfinder)

**AI-optimized CLI tool for finding function boundaries in source code with 99.67% token reduction**

`funcfinder` provides X-ray vision and microscope precision for AI workflows - scan entire codebases in milliseconds, extract exact functions with zero noise. **100K lines analyzed faster than a single AI request!**

## ✨ Features

- 🔍 **Find function boundaries** by name in source files
- 🗺️ **Map all functions** in a file with `--map`
- 🌳 **Tree visualization** with `--tree` for classes and methods
- 📏 **Line range filtering** with `--lines` for precise scope control ⭐ NEW
- 📤 **Extract function bodies** with `--extract`
- 📊 **JSON output** for AI integration with `--json`
- 🪟 **Windows-compatible file slicing** - native sed alternative
- 🚀 **99.67% token reduction** for code navigation
- ⚡ **Blazing fast**: 280,000 lines/sec (100K lines in 0.36s)
- 🎯 **Zero dependencies**: static binary

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
wget https://github.com/ruslano69/funcfinder/releases/download/v1.4.0/funcfinder-linux-amd64.tar.gz
tar -xzf funcfinder-linux-amd64.tar.gz
sudo mv funcfinder /usr/local/bin/

# macOS
wget https://github.com/ruslano69/funcfinder/releases/download/v1.4.0/funcfinder-darwin-amd64.tar.gz
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

### Check version

```bash
funcfinder --version
# Output: funcfinder version 1.4.0
```

### Map all functions in a file

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

# 1. Get structure (50 tokens vs 5000)
funcfinder --inp auth/middleware.go --source go --map --json

# 2. Extract buggy function (150 tokens vs 5000)
funcfinder --inp auth/middleware.go --source go --func ValidateToken --extract

# 3. Check complexity
complexity auth/middleware.go -j | jq '.functions[] | select(.name=="ValidateToken")'

# 4. Make targeted fix with 99% token savings! 🎉
```

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

## 📖 Usage

```
funcfinder --inp <file> [--source <lang>] [OPTIONS]

Required:
  --inp <file>       Source file to analyze
  --source <lang>    Language: go/c/cpp/cs/java/d/js/ts/py/rust/swift
                     (optional when using --lines alone)

Modes (choose one):
  --func <names>     Find specific functions (comma-separated)
  --map              Map all functions in file
  --tree             Display functions in tree format (shows class hierarchy)
  --tree-full        Display functions in tree format with signatures

Filtering:
  --lines <range>    Extract/filter by line range (standalone or with --source)
                     Formats: 100:150, :50, 100:, 100

Output formats:
  (default)          grep-style: funcname: n1-n2;
  --json             JSON format
  --extract          Extract function bodies

Options:
  --raw              Don't ignore raw strings in brace counting
  --version          Print version and exit
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

### funcfinder Core

```
funcfinder/
├── main.go             # CLI and coordination
├── config.go           # Unified language configuration (shared)
├── errors.go           # Standard error handling (shared)
├── sanitizer.go        # Comment/string literal handler
├── finder.go           # Function boundary detection
├── python_finder.go    # Python-specific indentation logic
├── finder_factory.go   # Language-specific finder selection
├── formatter.go        # Output formatting (grep/json/extract)
├── tree.go             # Tree visualization for classes
├── decorator.go        # Python decorator detection
└── lines.go            # Line range filtering
```

### Shared Modules

```
config.go           # Loads languages.json, provides regex cache
errors.go           # FatalError, WarnError, InfoMessage, PrintVersion
languages.json      # Unified patterns for ALL utilities (embedded)
```

### Additional Utilities

```
stat.go             # Uses config.go + errors.go
deps.go             # Uses config.go + errors.go
complexity.go       # Uses config.go + errors.go + finder.go
analyze.sh          # Orchestrates all utilities for full analysis
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

funcfinder поставляется с дополнительными утилитами для полного анализа кода. Все утилиты используют **единую архитектуру** с общими модулями конфигурации и обработки ошибок.

### Quick Start

```bash
# Собрать все утилиты
./build.sh

# Полный анализ проекта одной командой
./analyze.sh

# Workflow для AI-агентов
funcfinder --inp api.go --source go --map  # Структура кода
stat api.go -l go -n 10                    # Горячие точки
deps . -l go -j                            # Граф зависимостей
complexity api.go -l go                    # Когнитивная сложность
```

### Утилиты

| Утилита | Назначение | Языки | Выход |
|---------|------------|-------|-------|
| **funcfinder** | Структура кода (функции, классы, границы) | 11 | grep/JSON/extract |
| **stat** | Анализ вызовов функций + метрики файлов | 11 | текст |
| **deps** | Анализ зависимостей модулей (stdlib/external/internal) | 11 | текст/JSON |
| **complexity** ⭐ NEW | Анализ когнитивной сложности (nesting depth) | 11 | текст с цветами |

### 🧠 complexity - Анализатор когнитивной сложности

**Философия:** Глубокая вложенность (nesting depth), а не количество веток — настоящая сложность кода.

⚠️ **ВАЖНО:** Различайте вложенные циклы (критично для производительности) и вложенные if (читаемость). См. [PERFORMANCE.md](PERFORMANCE.md) для деталей.

```bash
# Анализ одного файла
complexity main.go -l go

# Анализ директории
complexity . -l go

# JSON выход для автоматизации
complexity api.py -l py --json

# Топ N самых сложных функций
complexity . -l go -n 10
```

**Примеры вывода:**

```
Average max complexity: 8.00
============================================================
Philosophy: Deep nesting (not branch count) is the real complexity
============================================================
#1 finder.go:238 findClassesWithOffset() depth=5 complexity=16 level=VERY_HIGH
  Lines: 44, File: finder.go

#2 finder.go:83 FindFunctionsInLines() depth=4 complexity=8 level=HIGH
  Lines: 104, File: finder.go

#3 config.go:142 GetLanguageConfig() depth=2 complexity=2 level=SIMPLE
  Lines: 7, File: config.go

============================================================
Complexity distribution (by nesting depth):
SIMPLE: 8 ██████████████ (depth ≤ 2)
MODERATE: 2 ████ (depth = 3)
HIGH: 1 ██ (depth ≥ 4)
```

**Уровни сложности:**
- ✅ **SIMPLE** (depth ≤ 2) - Плоский код, легко понять
- ⚠️ **MODERATE** (depth = 3) - Один уровень вложенности
- 🔶 **HIGH** (depth ≥ 4) - Два+ уровня вложенности
- 🔴 **CRITICAL** (depth ≥ 6) - Требуется рефакторинг

**Формула:** `NDC = 2^(maxDepth - 1)`

### 📊 Комплексный анализ с analyze.sh

Автоматический скрипт для полного анализа проекта:

```bash
./analyze.sh
```

**Отчет включает:**
- 📈 Статистику по файлам (строки, размер, code/comments/blank ratio)
- 🔍 Инвентаризацию функций (всего 85 функций в funcfinder)
- 🔥 Горячие точки вызовов (топ функций по частоте)
- 📦 Граф зависимостей (stdlib vs external vs internal)
- 🧠 Распределение сложности (SIMPLE/MODERATE/HIGH/CRITICAL)
- 💡 Рекомендации по улучшению кода

**Пример отчета:**
```
📊 Code Metrics:
  • Total files:      14
  • Total lines:      3,090
  • Total size:       84.9 KB
  • Total functions:  85
  • Avg func/file:    6.0

🎯 Code Quality:
  ✅ Excellent - Low complexity, well-structured code

═══════════════════════════════════════
Overall Complexity Distribution:
═══════════════════════════════════════
✅ SIMPLE:    13 functions (depth ≤ 2)
⚠️  MODERATE:  2 functions (depth = 3)
🔶 HIGH:      1 functions (depth ≥ 4)
🔴 CRITICAL:  0 functions (depth ≥ 6)
```

### 🏗️ Унифицированная архитектура (v1.4.0)

Все утилиты используют **единые модули** (DRY принцип):

```
funcfinder/
├── config.go          # Унифицированная конфигурация языков
├── errors.go          # Стандартная обработка ошибок
├── languages.json     # Единый источник паттернов (embedded)
├── main.go            # funcfinder CLI
├── stat.go            # Счётчик вызовов + метрики
├── deps.go            # Анализатор зависимостей
├── complexity.go      # Анализатор когнитивной сложности
├── analyze.sh         # Комплексный анализ проекта
└── build.sh           # Сборка всех утилит
```

**Преимущества архитектуры:**
- ✅ **Нулевые дубликаты** - единая конфигурация для всех утилит
- ✅ **Консистентность** - одинаковые сообщения об ошибках
- ✅ **Простота расширения** - добавить язык = обновить JSON
- ✅ **Нулевые зависимости** - все утилиты статические бинарники

**Типичные сценарии:**
- 📊 Первичный анализ незнакомого кода
- 🔍 Поиск узких мест для оптимизации
- 🔄 Рефакторинг и поиск дублирования
- 📈 Code review и анализ PR
- 🤖 AI-агент навигация с минимальными токенами
- 🧠 Оценка когнитивной сложности перед рефакторингом

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

### v1.4.0 (Current) ✅
- [x] **--lines flag** for line range filtering
- [x] Cross-platform file slicing (sed alternative)
- [x] Standalone and filter modes
- [x] **stat utility** - function call counter + file metrics (15 languages)
- [x] **deps utility** - dependency analyzer (15 languages)
- [x] **complexity utility** ⭐ NEW - cognitive complexity analyzer (15 languages)
- [x] **Unified architecture** - shared config.go and errors.go (DRY principle)
- [x] **analyze.sh** - comprehensive project analysis script
- [x] Complete code analysis toolkit with zero dependencies

### v1.5.0
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

# funcfinder

[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Platform](https://img.shields.io/badge/platform-Linux%20%7C%20macOS%20%7C%20Windows-lightgrey)](https://github.com/yourusername/funcfinder)

**AI-optimized CLI tool for finding function boundaries in source code with 95%+ token reduction**

`funcfinder` helps AI models and developers navigate large codebases efficiently by extracting function boundaries and structure without reading entire files.

## ✨ Features

- 🔍 **Find function boundaries** by name in source files
- 🗺️ **Map all functions** in a file with `--map`
- 🌳 **Tree visualization** with `--tree` for classes and methods
- 📏 **Line range filtering** with `--lines` for precise scope control ⭐ NEW
- 📤 **Extract function bodies** with `--extract`
- 📊 **JSON output** for AI integration with `--json`
- 🪟 **Windows-compatible file slicing** - native sed alternative
- 🚀 **95%+ token reduction** for code navigation
- ⚡ **Fast**: ~50ms per 5000 lines
- 🎯 **Zero dependencies**: static binary

## 🌐 Supported Languages

- Go
- C
- C++
- C#
- Java
- D
- **JavaScript** (including async functions, generator functions, arrow functions)
- **TypeScript** (including async functions, generator functions, arrow functions, generics)
- **Python** (including async/await, decorators, generators, class methods)
- **Rust** ⭐ NEW (including pub/async functions, structs, traits, enums, impl blocks)
- **Swift** ⭐ NEW (including classes, structs, protocols, enums, static functions)

## 📦 Installation

### Via Go Install (Recommended)

```bash
go install github.com/yourusername/funcfinder@latest
```

### Pre-built Binaries

Download from [Releases](https://github.com/yourusername/funcfinder/releases):

```bash
# Linux
wget https://github.com/yourusername/funcfinder/releases/download/v1.4.0/funcfinder-linux-amd64.tar.gz
tar -xzf funcfinder-linux-amd64.tar.gz
sudo mv funcfinder /usr/local/bin/

# macOS
wget https://github.com/yourusername/funcfinder/releases/download/v1.4.0/funcfinder-darwin-amd64.tar.gz
tar -xzf funcfinder-darwin-amd64.tar.gz
sudo mv funcfinder /usr/local/bin/

# Windows
# Download funcfinder-windows-amd64.zip and add to PATH
```

### From Source

```bash
git clone https://github.com/yourusername/funcfinder.git
cd funcfinder

# Build all utilities (funcfinder, stat, deps)
./build.sh

# Or build individually
go build  # funcfinder only
```

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

### Example 1: Single Function

**Traditional approach:**
- AI reads entire file: 357 lines

**With funcfinder:**
```bash
funcfinder --inp file.cs --source cs --func ValidateConversion --extract
```
- AI reads only function: 57 lines
- **Token savings: 84%**

### Example 2: File Navigation

**Traditional approach:**
- AI reads entire file to understand structure: 10,000 lines

**With funcfinder:**
```bash
funcfinder --inp file.go --source go --map --json
```
- AI reads JSON map: ~100 tokens
- **Token savings: 95%+**

## 🏗️ Architecture

```
funcfinder/
├── main.go          # CLI and coordination
├── config.go        # Language configuration loader
├── sanitizer.go     # Comment/string literal handler
├── finder.go        # Function boundary detection
├── formatter.go     # Output formatting (grep/json/extract)
└── languages.json   # Language patterns (embedded)
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

funcfinder поставляется с дополнительными утилитами для полного анализа кода. См. [UTILITIES.md](UTILITIES.md) для детальной документации.

### Quick Start

```bash
# Собрать все утилиты
./build.sh

# Workflow для AI-агентов
funcfinder --inp api.go --source go --map  # Структура кода
stat api.go -l go -n 10                    # Горячие точки
deps . -l go -j                            # Граф зависимостей
```

### Утилиты

| Утилита | Назначение | Языки |
|---------|------------|-------|
| **funcfinder** | Структура кода (функции, классы, границы) | 11 |
| **stat** | Анализ вызовов функций (hotspots) | 9 |
| **deps** | Анализ зависимостей модулей | 9 |

**Типичные сценарии:**
- 📊 Первичный анализ незнакомого кода
- 🔍 Поиск узких мест для оптимизации
- 🔄 Рефакторинг и поиск дублирования
- 📈 Code review и анализ PR
- 🤖 AI-агент навигация с минимальными токенами

См. [UTILITIES.md](UTILITIES.md) для примеров и best practices.

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

- **Speed:** ~50ms per 5000 lines (linear O(n))
- **Memory:** Minimal (streaming line-by-line)
- **Binary size:** 3MB (static, no dependencies)

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
- [x] **11 languages total** (added without Go code changes!)

### v1.4.0 (Current) ✅
- [x] **--lines flag** for line range filtering
- [x] Cross-platform file slicing (sed alternative)
- [x] Standalone and filter modes
- [x] **stat utility** - function call counter (9 languages)
- [x] **deps utility** - dependency analyzer (9 languages)
- [x] Complete code analysis toolkit

### v1.5.0
- [ ] Configuration file support
- [ ] Custom patterns via CLI
- [ ] Improved C# regex patterns
- [ ] Function type filters (public/private)

### v2.0.0
- [ ] Tree-sitter integration for precise parsing
- [ ] 30+ language support
- [ ] API server mode
- [ ] IDE integrations

## 📄 License

MIT License - see [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

Built for AI-driven development workflows. Inspired by the need to minimize token usage in large codebases.

## 📞 Support

- 🐛 [Report Issues](https://github.com/yourusername/funcfinder/issues)
- 💡 [Feature Requests](https://github.com/yourusername/funcfinder/issues)
- 📖 [Documentation](https://github.com/yourusername/funcfinder/wiki)

---

**funcfinder** - Navigate code efficiently, save tokens intelligently 🚀

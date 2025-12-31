# funcfinder

[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Platform](https://img.shields.io/badge/platform-Linux%20%7C%20macOS%20%7C%20Windows-lightgrey)](https://github.com/yourusername/funcfinder)

**AI-optimized CLI tool for finding function boundaries in source code with 95%+ token reduction**

`funcfinder` helps AI models and developers navigate large codebases efficiently by extracting function boundaries and structure without reading entire files.

## ✨ Features

- 🔍 **Find function boundaries** by name in source files
- 🗺️ **Map all functions** in a file with `--map`
- 📤 **Extract function bodies** with `--extract`
- 📊 **JSON output** for AI integration with `--json`
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

## 📦 Installation

### Via Go Install (Recommended)

```bash
go install github.com/yourusername/funcfinder@latest
```

### Pre-built Binaries

Download from [Releases](https://github.com/yourusername/funcfinder/releases):

```bash
# Linux
wget https://github.com/yourusername/funcfinder/releases/download/v1.0.0/funcfinder-linux-amd64.tar.gz
tar -xzf funcfinder-linux-amd64.tar.gz
sudo mv funcfinder /usr/local/bin/

# macOS
wget https://github.com/yourusername/funcfinder/releases/download/v1.0.0/funcfinder-darwin-amd64.tar.gz
tar -xzf funcfinder-darwin-amd64.tar.gz
sudo mv funcfinder /usr/local/bin/

# Windows
# Download funcfinder-windows-amd64.zip and add to PATH
```

### From Source

```bash
git clone https://github.com/yourusername/funcfinder.git
cd funcfinder
go build -o funcfinder
```

## 🚀 Quick Start

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
funcfinder --inp <file> --source <lang> [OPTIONS]

Required:
  --inp <file>       Source file to analyze
  --source <lang>    Language: go/c/cpp/cs/java/d

Modes (choose one):
  --func <names>     Find specific functions (comma-separated)
  --map              Map all functions in file

Output formats:
  (default)          grep-style: funcname: n1-n2;
  --json             JSON format
  --extract          Extract function bodies

Options:
  --raw              Don't ignore raw strings in brace counting
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

## 🤝 Contributing

Contributions welcome! See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

**Areas for contribution:**
- Additional language support (Python, JavaScript, Rust, etc.)
- Improved regex patterns
- Preprocessor support (C/C++ #ifdef)
- Performance optimizations
- Test coverage

## 📊 Performance

- **Speed:** ~50ms per 5000 lines (linear O(n))
- **Memory:** Minimal (streaming line-by-line)
- **Binary size:** 3MB (static, no dependencies)

## 🗺️ Roadmap

### v1.1.0
- [ ] Python support
- [ ] JavaScript/TypeScript support
- [ ] `--version` flag
- [ ] Improved C# regex patterns

### v1.2.0
- [ ] Configuration file support
- [ ] Custom patterns via CLI
- [ ] Function type filters (public/private)
- [ ] Code statistics

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

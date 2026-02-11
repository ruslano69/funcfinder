# funcfinder

[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Platform](https://img.shields.io/badge/platform-Linux%20%7C%20macOS%20%7C%20Windows-lightgrey)](https://github.com/ruslano69/funcfinder)

**Stop mastering grep. Run one script. Get the full picture.**

```bash
./build.sh && ./funcfinder --dir . --all --json > map.json
```

## Why?

| Without funcfinder | With funcfinder |
|-------------------|-----------------|
| AI reads README, avoids code (too expensive) | AI sees full structure, reads only what matters |
| Hours browsing files: "what depends on what?" | `map.json` + one `jq` query = instant answer |
| 80% API budget on exploration | 99% reduction — map once, extract targeted |
| ctags + LSP + grep + manual work | One binary, 15 languages, zero setup |
| Learn tools, then teach model | Drop `map.json` in context — model just works |

## Supported Languages

| Category | Languages |
|----------|-----------|
| **Systems** | C, C++, Rust, Go, D |
| **JVM** | Java, Kotlin, Scala |
| **Web** | JavaScript, TypeScript, PHP |
| **Scripting** | Python, Ruby |
| **Mobile** | Swift, C# |

## Installation

```bash
# Option 1: Go install
go install github.com/ruslano69/funcfinder@latest

# Option 2: Build from source
git clone https://github.com/ruslano69/funcfinder.git
cd funcfinder && ./build.sh
```

## Quick Start

```bash
# Map entire codebase (functions + classes)
./funcfinder --dir . --all --json > map.json

# Search the map
jq '.files[] | select(.path | contains("auth"))' map.json

# Extract specific function
./funcfinder --inp api.go --source go --func Handler --extract

# Tree view
./funcfinder --dir ./src --tree
```

## Usage Reference

```
funcfinder --dir <path> [OPTIONS]    # Directory mode
funcfinder --inp <file> [OPTIONS]    # File mode (requires --source)

Mode flags (pick one):
  --map          List all functions (default for --dir)
  --struct       List all classes/structs/types
  --all          Both functions and types
  --tree         Hierarchical view

File mode flags:
  --source <lang>   Language: go/py/js/ts/java/cs/cpp/c/rust/swift/kotlin/php/ruby/scala/d
  --func <name>     Find specific function
  --type <name>     Find specific type (with --struct)
  --extract         Output function body
  --lines <range>   Line range (e.g., 100:200)

Output:
  --json         JSON format
  --workers N    Parallel workers (default: CPU cores)
```

## Additional Tools

```bash
./stat file.go -l go           # Function call hotspots
./deps file.go -l go -json     # Import analysis
./complexity file.go -l go     # Nesting depth analysis
```

## Documentation

- **[AGENTS.md](AGENTS.md)** — AI agent quick reference
- **[docs/WINDOWS.md](docs/WINDOWS.md)** — Windows build guide
- **[docs/USE_CASES.md](docs/USE_CASES.md)** — Detailed examples
- **[CHANGELOG.md](CHANGELOG.md)** — Version history

## License

MIT — see [LICENSE](LICENSE)

---

*Built by a developer who couldn't afford to waste tokens on exploration.*

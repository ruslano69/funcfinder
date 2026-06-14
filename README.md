# funcfinder

[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Platform](https://img.shields.io/badge/platform-Linux%20%7C%20macOS%20%7C%20Windows-lightgrey)](https://github.com/ruslano69/funcfinder)

Map codebases, extract functions, trace calls — 99% fewer tokens than reading files.

```bash
./build.sh && ./funcfinder --dir . --all --json > map.json
```

## Why

| Without funcfinder | With funcfinder |
|---|---|
| AI reads full files (expensive) | Map once, extract targeted |
| grep + ctags + manual work | One binary, 15 languages, zero setup |
| 80% budget on exploration | 99% token reduction |

## Installation

```bash
# Build from source
git clone https://github.com/ruslano69/funcfinder.git
cd funcfinder && ./build.sh
```

Produces 6 binaries: `funcfinder`, `stat`, `deps`, `callgraph`, `complexity`, `findstruct`.

## Tools

| Tool | Purpose |
|------|---------|
| `funcfinder` | Map functions & types, extract bodies, shard large codebases |
| `callgraph` | Forward/reverse call graph |
| `deps` | Import dependencies + inter-shard graph |
| `stat` | Call frequency & hotspots |
| `complexity` | Cognitive complexity per function |

## Languages

C, C++, Go, Rust, D, Java, Kotlin, Scala, JavaScript, TypeScript, PHP, Python, Ruby, Swift, C#

## Quick Start

```bash
# Map entire codebase
./funcfinder --dir . --all --json > map.json

# Find function
./funcfinder --inp internal/finder.go --source go --func FindFunctions --extract

# Call graph
./callgraph --dir . -l go --reverse --func ProcessDirectory

# Large project: split into shards
./funcfinder --dir . --all --json --split        # creates .codemap/
cat .codemap/manifest.json                        # 2KB overview
./funcfinder --dir . --all --json --split --inc   # incremental update
```

## Architecture

```
cmd/            CLI entrypoints (thin wrappers)
internal/       Parser engine: finders, sanitizer, formatter, shard logic
languages.json  15 language patterns (embedded)
```

Parser uses a state-machine sanitizer (not regex) to correctly handle Go raw strings, Python docstrings, C# verbatim literals, nested comments — at 763K lines/sec.

## DOX — Agent Documentation

This project uses the [DOX framework](https://github.com/agent0ai/dox): a hierarchy of `AGENTS.md` files that gives AI agents precise, token-efficient context without loading the full repository.

### How it works

```
AGENTS.md               ← root: tools overview, workflows, DOX rules
├── cmd/AGENTS.md       ← CLI entrypoints, flag conventions
├── internal/AGENTS.md  ← parser API, how to add languages
├── docs/AGENTS.md      ← documentation rules
├── examples/AGENTS.md  ← example scripts
├── skills/AGENTS.md    ← Claude Code skill definition
├── test_examples/AGENTS.md  ← test fixture rules
└── test_files/AGENTS.md     ← edge-case sanitizer fixtures
```

### Rules for agents

Before editing any file, read the chain from root to the target directory:

```
AGENTS.md → <subdirectory>/AGENTS.md
```

After any meaningful change, update the nearest `AGENTS.md` that owns the changed path. Keep the Child DOX Index in each file accurate.

### Adding documentation

When a folder gains its own stable purpose, contracts, or workflow — create `<folder>/AGENTS.md` with sections: Purpose · Ownership · Local Contracts · Work Guidance · Verification · Child DOX Index. Then add it to the parent's index.

## Documentation

- **[AGENTS.md](AGENTS.md)** — agent quick reference + DOX framework
- **[docs/CI_CD.md](docs/CI_CD.md)** — CI/CD pipeline
- **[docs/WINDOWS.md](docs/WINDOWS.md)** — Windows build guide
- **[CHANGELOG.md](CHANGELOG.md)** — version history

## License

MIT — see [LICENSE](LICENSE)

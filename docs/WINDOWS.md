# Windows Build Instructions

Quick guide for building and using funcfinder on Windows.

## Prerequisites

- Go 1.22+ installed ([download](https://go.dev/dl/))
- PowerShell (built into Windows)

## Building

### Option 1: Build All Utilities (Recommended)

Open PowerShell in the project directory and run:

```powershell
.\build.ps1
```

This builds all four utilities:
- `funcfinder.exe` - Main function finder
- `stat.exe` - Function call counter
- `deps.exe` - Dependency analyzer
- `complexity.exe` - Cognitive complexity analyzer

### Option 2: Build Individual Utilities

**IMPORTANT:** Do NOT use `go build` without specifying files, as the project has multiple `main()` functions (one per utility). You must either use `build.ps1` or specify exact files:

```powershell
# Build only funcfinder
go build -o funcfinder.exe main.go config.go sanitizer.go finder.go formatter.go tree.go decorator.go python_finder.go finder_factory.go lines.go errors.go

# Build only stat
go build -o stat.exe stat.go config.go errors.go

# Build only deps
go build -o deps.exe deps.go config.go errors.go

# Build only complexity
go build -o complexity.exe complexity.go config.go errors.go sanitizer.go finder.go python_finder.go finder_factory.go decorator.go
```

**Common Error:**
```powershell
# ❌ WRONG - Will fail with "main redeclared"
go build -o funcfinder.exe

# ✅ CORRECT - Use build.ps1 or specify files
.\build.ps1
# OR
go build -o funcfinder.exe main.go config.go sanitizer.go ...
```

## Usage Examples

### funcfinder

```powershell
# Map all functions in a file
.\funcfinder.exe --inp .\myfile.go --source go --map

# Extract specific function
.\funcfinder.exe --inp .\myfile.go --source go --func MyFunction --extract

# JSON output
.\funcfinder.exe --inp .\myfile.go --source go --map --json

# Line range filtering
.\funcfinder.exe --inp .\myfile.go --lines 100:200
```

### stat

```powershell
# Analyze function calls
.\stat.exe .\myfile.go -l go -n 10
```

### deps

```powershell
# Analyze dependencies
.\deps.exe . -l go

# JSON output
.\deps.exe . -l go -j
```

### complexity

```powershell
# Analyze cognitive complexity
.\complexity.exe .\myfile.go -l go

# Top 10 most complex functions
.\complexity.exe . -l go -n 10

# JSON output
.\complexity.exe .\myfile.go -l go --json
```

## Execution Policy

If you get an error about execution policy when running `.\build.ps1`, run this command first:

```powershell
Set-ExecutionPolicy -ExecutionPolicy RemoteSigned -Scope CurrentUser
```

This allows running local PowerShell scripts.

## Adding to PATH

To use the utilities from anywhere, add the funcfinder directory to your PATH:

1. Open System Properties → Environment Variables
2. Edit the PATH variable for your user
3. Add the full path to the funcfinder directory
4. Restart your terminal

Alternatively, copy the `.exe` files to a directory already in your PATH (e.g., `C:\Windows\System32`).

## Performance on Windows

funcfinder performance on Windows is comparable to Linux/macOS:
- **~280,000 lines/sec** parsing throughput
- **O(n) linear** complexity
- **Zero dependencies** - static binaries

## Cross-Platform File Paths

funcfinder handles both Unix-style (`/`) and Windows-style (`\`) paths automatically.

```powershell
# Both work on Windows
.\funcfinder.exe --inp .\src\main.go --source go --map
.\funcfinder.exe --inp ./src/main.go --source go --map
```

## Troubleshooting

### "go is not recognized"

Install Go from https://go.dev/dl/ and restart PowerShell.

### "cannot run .ps1 script"

See [Execution Policy](#execution-policy) section above.

### Unicode/Emoji not displaying

Use Windows Terminal (recommended) or enable UTF-8 in PowerShell:

```powershell
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
```

## Building for Different Platforms

```powershell
# Build for Windows (default)
go build -o funcfinder.exe

# Build for Linux
$env:GOOS="linux"; go build -o funcfinder

# Build for macOS
$env:GOOS="darwin"; go build -o funcfinder
```

## See Also

- [README.md](README.md) - Main documentation
- [UTILITIES.md](UTILITIES.md) - Detailed utility documentation
- [COMPLEXITY.md](COMPLEXITY.md) - Complexity analyzer guide

---

**funcfinder v1.4.0** - Windows-compatible AI code analysis toolkit

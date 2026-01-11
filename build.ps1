# Build script for funcfinder toolkit (PowerShell)
# Builds: funcfinder, stat, deps, complexity
# Usage: .\build.ps1

$ErrorActionPreference = "Stop"

Write-Host "Building funcfinder toolkit v1.4.0..." -ForegroundColor Cyan
Write-Host ""

# Build main funcfinder
Write-Host "→ Building funcfinder..." -ForegroundColor Yellow
go build -o funcfinder.exe `
  main.go config.go sanitizer.go finder.go `
  formatter.go tree.go decorator.go python_finder.go `
  finder_factory.go lines.go errors.go

if ($LASTEXITCODE -eq 0) {
    Write-Host "  ✓ funcfinder.exe" -ForegroundColor Green
} else {
    Write-Host "  ✗ funcfinder.exe failed" -ForegroundColor Red
    exit 1
}

# Build stat utility
Write-Host "→ Building stat..." -ForegroundColor Yellow
go build -o stat.exe stat.go config.go errors.go

if ($LASTEXITCODE -eq 0) {
    Write-Host "  ✓ stat.exe" -ForegroundColor Green
} else {
    Write-Host "  ✗ stat.exe failed" -ForegroundColor Red
    exit 1
}

# Build deps utility
Write-Host "→ Building deps..." -ForegroundColor Yellow
go build -o deps.exe deps.go config.go errors.go

if ($LASTEXITCODE -eq 0) {
    Write-Host "  ✓ deps.exe" -ForegroundColor Green
} else {
    Write-Host "  ✗ deps.exe failed" -ForegroundColor Red
    exit 1
}

# Build complexity utility
Write-Host "→ Building complexity..." -ForegroundColor Yellow
go build -o complexity.exe `
  complexity.go config.go errors.go `
  sanitizer.go finder.go python_finder.go finder_factory.go decorator.go

if ($LASTEXITCODE -eq 0) {
    Write-Host "  ✓ complexity.exe" -ForegroundColor Green
} else {
    Write-Host "  ✗ complexity.exe failed" -ForegroundColor Red
    exit 1
}

Write-Host ""
Write-Host "✅ All binaries built successfully!" -ForegroundColor Green
Write-Host ""
Write-Host "Usage:" -ForegroundColor Cyan
Write-Host "  .\funcfinder.exe --inp file.go --source go --map"
Write-Host "  .\stat.exe file.go -l go -n 10"
Write-Host "  .\deps.exe . -l go -j"
Write-Host "  .\complexity.exe file.go -l go"
Write-Host ""

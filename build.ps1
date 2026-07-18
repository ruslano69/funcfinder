# Build script for funcfinder toolkit (PowerShell)
# Builds: funcfinder, stat, deps, complexity, callgraph
# Usage: .\build.ps1

$ErrorActionPreference = "Stop"

$VersionBase = "1.10"
$Patch = (git rev-list --count HEAD 2>$null)
if (-not $Patch) { $Patch = "0" }
$Version = "$VersionBase.$Patch"
$LdFlags = "-s -w -X github.com/ruslano69/funcfinder/internal.Version=$Version"

Write-Host "Building funcfinder toolkit v$Version..." -ForegroundColor Cyan
Write-Host ""

$Binaries = @(
    @{ Name = "funcfinder"; Cmd = ".\cmd\funcfinder" },
    @{ Name = "stat";       Cmd = ".\cmd\stat" },
    @{ Name = "deps";       Cmd = ".\cmd\deps" },
    @{ Name = "complexity"; Cmd = ".\cmd\complexity" },
    @{ Name = "callgraph";  Cmd = ".\cmd\callgraph" }
)

foreach ($b in $Binaries) {
    Write-Host "-> Building $($b.Name)..." -ForegroundColor Yellow
    go build -ldflags $LdFlags -o "$($b.Name).exe" $b.Cmd
    if ($LASTEXITCODE -eq 0) {
        Write-Host "   OK $($b.Name).exe" -ForegroundColor Green
    } else {
        Write-Host "   FAIL $($b.Name).exe" -ForegroundColor Red
        exit 1
    }
}

Write-Host ""
Write-Host "All binaries built successfully!" -ForegroundColor Green
Write-Host ""
Write-Host "Usage:" -ForegroundColor Cyan
Write-Host "  .\funcfinder.exe --inp file.go --source go --map"
Write-Host "  .\stat.exe file.go -l go -n 10"
Write-Host "  .\deps.exe . -l go -j"
Write-Host "  .\complexity.exe file.go -l go"
Write-Host "  .\callgraph.exe --dir . -l go"
Write-Host ""

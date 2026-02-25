#!/bin/bash
# Build script for funcfinder toolkit
# Builds: funcfinder, stat, deps, complexity

set -e

VERSION_BASE="1.6"
PATCH=$(git rev-list --count HEAD 2>/dev/null || echo "0")
VERSION="${VERSION_BASE}.${PATCH}"
LDFLAGS="-s -w -X github.com/ruslano69/funcfinder/internal.Version=${VERSION}"

echo "Building funcfinder toolkit v${VERSION}..."
echo ""

# Build funcfinder
echo "→ Building funcfinder..."
go build -ldflags "${LDFLAGS}" -o funcfinder ./cmd/funcfinder
echo "  ✓ funcfinder"

# Build stat
echo "→ Building stat..."
go build -ldflags "${LDFLAGS}" -o stat ./cmd/stat
echo "  ✓ stat"

# Build deps
echo "→ Building deps..."
go build -ldflags "${LDFLAGS}" -o deps ./cmd/deps
echo "  ✓ deps"

# Build complexity
echo "→ Building complexity..."
go build -ldflags "${LDFLAGS}" -o complexity ./cmd/complexity
echo "  ✓ complexity"

echo ""
echo "✅ All binaries built successfully!"
echo ""
echo "Usage:"
echo "  ./funcfinder --inp file.go --source go --map"
echo "  ./stat file.go -l go -n 10"
echo "  ./deps . -l go -j"
echo "  ./complexity file.go -l go"

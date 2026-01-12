#!/bin/bash
# Build script for funcfinder toolkit
# Builds: funcfinder, stat, deps, complexity

set -e

echo "Building funcfinder toolkit v1.4.0..."
echo ""

# Build funcfinder
echo "→ Building funcfinder..."
go build -o funcfinder ./cmd/funcfinder
echo "  ✓ funcfinder"

# Build stat
echo "→ Building stat..."
go build -o stat ./cmd/stat
echo "  ✓ stat"

# Build deps
echo "→ Building deps..."
go build -o deps ./cmd/deps
echo "  ✓ deps"

# Build complexity
echo "→ Building complexity..."
go build -o complexity ./cmd/complexity
echo "  ✓ complexity"

echo ""
echo "✅ All binaries built successfully!"
echo ""
echo "Usage:"
echo "  ./funcfinder --inp file.go --source go --map"
echo "  ./stat file.go -l go -n 10"
echo "  ./deps . -l go -j"
echo "  ./complexity file.go -l go"

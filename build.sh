#!/bin/bash
# Build script for funcfinder toolkit
# Builds: funcfinder, stat, deps, complexity

set -e

echo "Building funcfinder toolkit v1.4.0..."
echo ""

# Build main funcfinder
echo "→ Building funcfinder..."
go build -o funcfinder \
  main.go config.go sanitizer.go finder.go \
  formatter.go tree.go decorator.go python_finder.go \
  finder_factory.go lines.go errors.go
echo "  ✓ funcfinder"

# Build stat utility
echo "→ Building stat..."
go build -o stat stat.go config.go errors.go
echo "  ✓ stat"

# Build deps utility
echo "→ Building deps..."
go build -o deps deps.go config.go errors.go
echo "  ✓ deps"

# Build complexity utility
echo "→ Building complexity..."
go build -o complexity \
  complexity.go config.go errors.go \
  sanitizer.go finder.go python_finder.go finder_factory.go decorator.go
echo "  ✓ complexity"

echo ""
echo "✅ All binaries built successfully!"
echo ""
echo "Usage:"
echo "  ./funcfinder --inp file.go --source go --map"
echo "  ./stat file.go -l go -n 10"
echo "  ./deps . -l go -j"
echo "  ./complexity file.go -l go"

#!/bin/bash
echo "========================================="
echo "Testing funcfinder on all languages"
echo "========================================="
echo ""

echo "1. Go:"
./funcfinder --inp test_examples/test_example.go --source go --map | head -c 100
echo "..."
echo ""

echo "2. C:"
./funcfinder --inp test_examples/test_example.c --source c --map | head -c 100
echo "..."
echo ""

echo "3. C++:"
./funcfinder --inp test_examples/test_example.cpp --source cpp --map | head -c 100
echo "..."
echo ""

echo "4. C#:"
./funcfinder --inp test_examples/test_example.cs --source cs --map | head -c 100
echo "..."
echo ""

echo "5. Java:"
./funcfinder --inp test_examples/test_example.java --source java --map | head -c 100
echo "..."
echo ""

echo "6. D:"
./funcfinder --inp test_examples/test_example.d --source d --map | head -c 100
echo "..."
echo ""

echo "7. JavaScript:"
./funcfinder --inp test_examples/test_example.js --source js --map | head -c 100
echo "..."
echo ""

echo "8. TypeScript:"
./funcfinder --inp test_examples/test_example.ts --source ts --map | head -c 100
echo "..."
echo ""

echo "9. Python (with decorators):"
./funcfinder --inp test_examples/test_example.py --source py --func cached_function --json
echo ""

echo "========================================="
echo "All tests completed!"
echo "========================================="

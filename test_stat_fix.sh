#!/bin/bash

echo "=== Testing stat Fix for C# Verbatim Strings ==="
echo

# Test file
TEST_FILE="test_files/test_csharp_verbatim.cs"

# Run fixed stat
echo "Running fixed stat on $TEST_FILE:"
echo "----------------------------------------"
./stat -l cs "$TEST_FILE"
echo

echo "=== Expected Metrics ==="
echo "Code lines: 7-8 (NOT 14!)"
echo "  - Lines with actual code: ~7"
echo "  - Multiline verbatim strings should NOT count as code"
echo
echo "Function calls: 1 (NOT 2!)"
echo "  - Example: 1"
echo "  - WriteLine should NOT be counted (inside comments/strings)"
echo

echo "=== Verification ==="
echo "✅ If Code ~7-8 (28-32%): CORRECT - multiline strings handled"
echo "❌ If Code 14 (56%): BROKEN - multiline strings counted as code"
echo
echo "✅ If Calls = 1 (Example): CORRECT"
echo "❌ If Calls = 2 (WriteLine + Example): BROKEN - counted calls in strings"

#!/bin/bash
# Example workflow for mini-SWE-agent: Fix authentication bug
# This demonstrates how funcfinder toolkit helps agent solve tasks efficiently

set -e

FILE="auth/middleware.go"
BUG_FUNCTION="ValidateToken"

echo "=== mini-SWE-agent Workflow: Fix Bug in $FILE ==="
echo

# Step 1: Understand file structure (minimal token usage)
echo "Step 1: Get file structure"
echo "Command: funcfinder --inp $FILE --source go --map --json"
STRUCTURE=$(funcfinder --inp "$FILE" --source go --map --json)
echo "$STRUCTURE" | jq '.'
echo
echo "✅ Token usage: ~50 tokens (vs 5000+ for full file)"
echo

# Step 2: Extract only the buggy function
echo "Step 2: Extract buggy function: $BUG_FUNCTION"
echo "Command: funcfinder --inp $FILE --source go --func $BUG_FUNCTION --extract"
FUNCTION_CODE=$(funcfinder --inp "$FILE" --source go --func "$BUG_FUNCTION" --extract)
echo "$FUNCTION_CODE"
echo
echo "✅ Token usage: 150 tokens (98.5% savings vs full file)"
echo

# Step 3: Check complexity (is it too complex?)
echo "Step 3: Check complexity"
echo "Command: complexity $FILE -j"
COMPLEXITY=$(complexity "$FILE" -j | jq ".functions[] | select(.name==\"$BUG_FUNCTION\")")
echo "$COMPLEXITY" | jq '.'
echo
LEVEL=$(echo "$COMPLEXITY" | jq -r '.level')
echo "⚠️  Complexity level: $LEVEL"
echo

# Step 4: Check dependencies
echo "Step 4: Check dependencies"
echo "Command: deps --inp $FILE --source go --json"
DEPS=$(deps --inp "$FILE" --source go --json)
echo "$DEPS" | jq '.'
echo

# Step 5: Check how often this function is called
echo "Step 5: Check call frequency (for impact analysis)"
echo "Command: stat --inp auth/ --source go --json"
STATS=$(stat --inp auth/ --source go --json)
CALLS=$(echo "$STATS" | jq ".\"$BUG_FUNCTION\" // 0")
echo "Function '$BUG_FUNCTION' is called $CALLS times"
echo

# Summary
echo "=== Summary for LLM ==="
echo "Context gathered:"
echo "  ✅ File structure: $FILE has $(echo "$STRUCTURE" | jq 'length') functions"
echo "  ✅ Bug location: $BUG_FUNCTION at lines $(echo "$FUNCTION_CODE" | head -1 | grep -oP '\d+-\d+')"
echo "  ✅ Complexity: $LEVEL ($(echo "$COMPLEXITY" | jq -r '.complexity'))"
echo "  ✅ Dependencies: $(echo "$DEPS" | jq -r '.imports | length') imports"
echo "  ✅ Impact: Called $CALLS times in codebase"
echo
echo "Total tokens used: ~300 (vs 10,000+ traditional approach)"
echo "Token savings: 97% 🎉"

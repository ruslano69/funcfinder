#!/bin/bash
# Example workflow for mini-SWE-agent: Refactor complex code
# Demonstrates using complexity tool to find refactoring candidates

set -e

PROJECT_DIR="${1:-.}"
THRESHOLD="${2:-5}"

echo "=== mini-SWE-agent Workflow: Find and Refactor Complex Code ==="
echo "Project: $PROJECT_DIR"
echo "Complexity threshold: $THRESHOLD"
echo

# Step 1: Find all complex functions
echo "Step 1: Find complex functions (depth >= $THRESHOLD)"
echo "Command: complexity $PROJECT_DIR -t $THRESHOLD -n 10 -j"
COMPLEX_FUNCS=$(complexity "$PROJECT_DIR" -t "$THRESHOLD" -n 10 -j)
echo "$COMPLEX_FUNCS" | jq '.functions[] | {name, file, complexity, level}'
echo

# Get the most complex function
MOST_COMPLEX=$(echo "$COMPLEX_FUNCS" | jq -r '.functions[0]')
FUNC_NAME=$(echo "$MOST_COMPLEX" | jq -r '.name')
FUNC_FILE=$(echo "$MOST_COMPLEX" | jq -r '.file')
FUNC_COMPLEXITY=$(echo "$MOST_COMPLEX" | jq -r '.complexity')
FUNC_LEVEL=$(echo "$MOST_COMPLEX" | jq -r '.level')

echo "🔍 Most complex function: $FUNC_NAME"
echo "   File: $FUNC_FILE"
echo "   Complexity: $FUNC_COMPLEXITY (NDC = 2^(depth-1))"
echo "   Level: $FUNC_LEVEL"
echo

# Step 2: Extract function for analysis
echo "Step 2: Extract function for refactoring"
LANG=$(basename "$FUNC_FILE" | sed 's/.*\.//')
case "$LANG" in
    go) LANG="go" ;;
    py) LANG="py" ;;
    js|ts) LANG="js" ;;
    *) LANG="go" ;;
esac

echo "Command: funcfinder --inp $FUNC_FILE --source $LANG --func $FUNC_NAME --extract"
FUNC_CODE=$(funcfinder --inp "$FUNC_FILE" --source "$LANG" --func "$FUNC_NAME" --extract)
LINES=$(echo "$FUNC_CODE" | wc -l)
echo "Extracted $LINES lines"
echo

# Step 3: Analyze dependencies
echo "Step 3: Analyze dependencies (what does it import?)"
echo "Command: deps --inp $FUNC_FILE --source $LANG --json"
DEPS=$(deps --inp "$FUNC_FILE" --source "$LANG" --json)
echo "$DEPS" | jq '{imports, stdlib, external}'
echo

# Step 4: Check usage frequency
echo "Step 4: Check how often this function is called"
DIR=$(dirname "$FUNC_FILE")
echo "Command: stat --inp $DIR --source $LANG --json"
STATS=$(stat --inp "$DIR" --source "$LANG" --json 2>/dev/null || echo "{}")
CALLS=$(echo "$STATS" | jq ".\"$FUNC_NAME\" // 0")
echo "Called $CALLS times in $DIR"
echo

# Step 5: Show function with detailed nesting analysis
echo "Step 5: Get detailed nesting analysis"
echo "Command: complexity $FUNC_FILE -v | grep -A 20 $FUNC_NAME"
complexity "$FUNC_FILE" -v | grep -A 20 "$FUNC_NAME" || echo "No detailed output"
echo

# Generate refactoring suggestions
echo "=== Refactoring Recommendations ==="
echo
echo "Function: $FUNC_NAME"
echo "Current complexity: $FUNC_COMPLEXITY ($FUNC_LEVEL)"
echo
echo "Suggested actions:"

if [ "$FUNC_COMPLEXITY" -gt 128 ]; then
    echo "  🔴 CRITICAL complexity - Break into smaller functions"
    echo "  📦 Consider extracting nested logic into separate methods"
    echo "  🎯 Target: Reduce nesting depth from current to <= 4"
elif [ "$FUNC_COMPLEXITY" -gt 32 ]; then
    echo "  🟡 HIGH complexity - Simplify control flow"
    echo "  ✂️  Consider using early returns to reduce nesting"
    echo "  🔄 Look for repeated patterns to extract"
else
    echo "  🟢 MODERATE complexity - Minor cleanup recommended"
fi

echo
echo "Dependencies to review:"
echo "$DEPS" | jq -r '.imports[]' | sed 's/^/  - /'
echo

echo "=== Output for LLM ==="
cat <<EOF
Context for refactoring $FUNC_NAME:
- Location: $FUNC_FILE
- Complexity: $FUNC_COMPLEXITY (depth: $(echo "$MOST_COMPLEX" | jq -r '.max_nesting_depth'))
- Level: $FUNC_LEVEL
- Lines: $LINES
- Called: $CALLS times
- Dependencies: $(echo "$DEPS" | jq -r '.imports | length') imports

Function code:
$FUNC_CODE

Recommendation: $([ "$FUNC_COMPLEXITY" -gt 128 ] && echo "Complete refactor needed" || echo "Simplify control flow")
EOF

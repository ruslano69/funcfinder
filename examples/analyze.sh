#!/bin/bash
# analyze.sh - Comprehensive codebase analysis using funcfinder toolkit
# Analyzes the funcfinder project itself using all 4 utilities

set -e

echo "╔════════════════════════════════════════════════════════════════╗"
echo "║  FUNCFINDER TOOLKIT - CODEBASE ANALYSIS REPORT                ║"
echo "╚════════════════════════════════════════════════════════════════╝"
echo ""

# Ensure all binaries are built
if [ ! -f ./funcfinder ] || [ ! -f ./funcstats ] || [ ! -f ./deps ] || [ ! -f ./complexity ]; then
    echo "⚠️  Building utilities..."
    ./build.sh > /dev/null 2>&1
fi

# Find all Go source files (excluding test files and test_examples)
GO_FILES=$(find ./internal ./cmd -name "*.go" ! -name "*_test.go" 2>/dev/null | sort)
TOTAL_FILES=$(echo "$GO_FILES" | wc -l)

echo "📁 Project: funcfinder v1.5.0"
echo "📊 Go source files found: $TOTAL_FILES"
echo ""

# ============================================================================
# SECTION 1: PROJECT STATISTICS
# ============================================================================
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "📈 SECTION 1: FILE STATISTICS"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

TOTAL_LINES=0
TOTAL_CODE=0
TOTAL_COMMENTS=0
TOTAL_BLANK=0
TOTAL_SIZE=0

for file in $GO_FILES; do
    if [ -f "$file" ]; then
        # Get line counts
        lines=$(wc -l < "$file")
        size=$(stat -f%z "$file" 2>/dev/null || stat -c%s "$file" 2>/dev/null)

        TOTAL_LINES=$((TOTAL_LINES + lines))
        TOTAL_SIZE=$((TOTAL_SIZE + size))
    fi
done

echo "Total Lines:    $TOTAL_LINES"
echo "Total Size:     $(echo "scale=1; $TOTAL_SIZE / 1024" | bc) KB"
echo ""

# Show detailed stats for key files
echo "Key Files Analysis:"
echo "-----------------------------------"
for file in cmd/funcfinder/main.go internal/config.go internal/finder.go cmd/complexity/main.go cmd/funcstats/main.go cmd/deps/main.go; do
    if [ -f "$file" ]; then
        ./funcstats "$file" -l go 2>/dev/null | head -8 | tail -5
        echo ""
    fi
done

# ============================================================================
# SECTION 2: FUNCTION MAPPING
# ============================================================================
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "🔍 SECTION 2: FUNCTION INVENTORY"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

TOTAL_FUNCTIONS=0
for file in $GO_FILES; do
    if [ -f "$file" ]; then
        func_count=$(./funcfinder --inp "$file" --source go --map 2>/dev/null | grep -o ';' | wc -l || echo "0")
        TOTAL_FUNCTIONS=$((TOTAL_FUNCTIONS + func_count))

        funcs=$(./funcfinder --inp "$file" --source go --map 2>/dev/null || echo "")
        if [ ! -z "$funcs" ]; then
            basename_file=$(basename "$file")
            count=$(echo "$funcs" | grep -o ';' | wc -l)
            echo "📄 $basename_file ($count functions)"
            echo "   $funcs" | head -c 100
            if [ ${#funcs} -gt 100 ]; then
                echo "..."
            else
                echo ""
            fi
            echo ""
        fi
    fi
done

echo "═══════════════════════════════════════"
echo "Total Functions: $TOTAL_FUNCTIONS"
echo "═══════════════════════════════════════"
echo ""

# ============================================================================
# SECTION 3: FUNCTION CALL HOTSPOTS
# ============================================================================
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "🔥 SECTION 3: FUNCTION CALL HOTSPOTS"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

echo "Top function calls across the project:"
echo ""

# Analyze main modules
for file in main.go finder.go config.go; do
    if [ -f "$file" ]; then
        echo "📄 $(basename $file):"
        ./funcstats "$file" -n 8 2>/dev/null | tail -8
        echo ""
    fi
done

# ============================================================================
# SECTION 4: DEPENDENCY ANALYSIS
# ============================================================================
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "📦 SECTION 4: DEPENDENCY ANALYSIS"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

./deps . -l go -n 15 2>/dev/null
echo ""

# ============================================================================
# SECTION 5: COMPLEXITY ANALYSIS
# ============================================================================
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "🧠 SECTION 5: COMPLEXITY ANALYSIS (Nesting Depth)"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

echo "Philosophy: Deep nesting indicates cognitive complexity"
echo ""

# Analyze most complex files
HIGH_COMPLEXITY_FILES="finder.go python_finder.go complexity.go main.go"

for file in $HIGH_COMPLEXITY_FILES; do
    if [ -f "$file" ]; then
        echo "📄 Analyzing $file..."
        ./complexity -l go "$file" 2>/dev/null | head -20
        echo ""
    fi
done

# Get overall complexity stats
echo "═══════════════════════════════════════"
echo "Overall Complexity Distribution:"
echo "═══════════════════════════════════════"

SIMPLE_COUNT=0
MODERATE_COUNT=0
HIGH_COUNT=0
CRITICAL_COUNT=0

for file in $GO_FILES; do
    if [ -f "$file" ]; then
        output=$(./complexity -l go "$file" 2>/dev/null || echo "")
        if [ ! -z "$output" ]; then
            simple=$(echo "$output" | grep -c "level=SIMPLE" || echo "0")
            moderate=$(echo "$output" | grep -c "level=MODERATE" || echo "0")
            high=$(echo "$output" | grep -c "level=HIGH" || echo "0")
            critical=$(echo "$output" | grep -c "level=CRITICAL" || echo "0")

            SIMPLE_COUNT=$((SIMPLE_COUNT + simple))
            MODERATE_COUNT=$((MODERATE_COUNT + moderate))
            HIGH_COUNT=$((HIGH_COUNT + high))
            CRITICAL_COUNT=$((CRITICAL_COUNT + critical))
        fi
    fi
done

echo "✅ SIMPLE:    $SIMPLE_COUNT functions (depth ≤ 2)"
echo "⚠️  MODERATE:  $MODERATE_COUNT functions (depth = 3)"
echo "🔶 HIGH:      $HIGH_COUNT functions (depth ≥ 4)"
echo "🔴 CRITICAL:  $CRITICAL_COUNT functions (depth ≥ 6)"
echo ""

# ============================================================================
# SECTION 6: SUMMARY & RECOMMENDATIONS
# ============================================================================
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "📋 SECTION 6: SUMMARY & RECOMMENDATIONS"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

echo "📊 Code Metrics:"
echo "  • Total files:      $TOTAL_FILES"
echo "  • Total lines:      $TOTAL_LINES"
echo "  • Total size:       $(echo "scale=1; $TOTAL_SIZE / 1024" | bc) KB"
echo "  • Total functions:  $TOTAL_FUNCTIONS"
echo "  • Avg func/file:    $(echo "scale=1; $TOTAL_FUNCTIONS / $TOTAL_FILES" | bc)"
echo ""

echo "🎯 Code Quality:"
if [ $CRITICAL_COUNT -eq 0 ] && [ $HIGH_COUNT -lt 5 ]; then
    echo "  ✅ Excellent - Low complexity, well-structured code"
elif [ $CRITICAL_COUNT -eq 0 ]; then
    echo "  ✅ Good - Manageable complexity with some nested functions"
elif [ $CRITICAL_COUNT -lt 3 ]; then
    echo "  ⚠️  Fair - Some critical complexity functions need review"
else
    echo "  🔴 Needs attention - Multiple high complexity functions"
fi
echo ""

echo "💡 Recommendations:"
if [ $HIGH_COUNT -gt 10 ]; then
    echo "  • Consider refactoring functions with depth ≥ 4"
fi
if [ $CRITICAL_COUNT -gt 0 ]; then
    echo "  • Priority: Review $CRITICAL_COUNT critical complexity functions"
fi
if [ $(echo "$TOTAL_FUNCTIONS > 100" | bc) -eq 1 ]; then
    echo "  • Consider splitting large files into modules"
fi
echo "  • Maintain current test coverage"
echo "  • Document complex algorithms"
echo ""

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "✅ Analysis Complete!"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "Generated by: funcfinder toolkit v1.5.0"
echo "Timestamp: $(date '+%Y-%m-%d %H:%M:%S')"

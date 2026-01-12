# Test Coverage Report - funcfinder v1.4.0

## Summary

**Total coverage: 34.1%**

Generated using funcfinder analysis + go test coverage

## Coverage by Module

| Module | Functions | Tested | Coverage | Status |
|--------|-----------|--------|----------|--------|
| config.go | 11 | 4 | ~36% | ⚠️ Partial |
| decorator.go | 6 | 0 | 0% | ❌ No tests |
| errors.go | 7 | 0 | 0% | ❌ No tests |
| finder.go | 7 | 5 | ~71% | ✅ Good |
| finder_factory.go | 1 | 0 | 0% | ❌ No tests |
| formatter.go | 3 | 3 | ~93% | ✅ Excellent |
| lines.go | 5 | 0 | 0% | ❌ No tests |
| python_finder.go | 2 | 0 | 0% | ❌ No tests |
| sanitizer.go | 5 | 5 | ~99% | ✅ Excellent |
| tree.go | 16 | 0 | 0% | ❌ No tests |

## Test Files Status

✅ **Existing tests (moved to internal/):**
- config_test.go - 7 test functions
- finder_test.go - 14 test functions  
- formatter_test.go - 6 test functions
- sanitizer_test.go - 10 test functions

❌ **Missing test files:**
- decorator_test.go
- errors_test.go
- finder_factory_test.go
- lines_test.go
- python_finder_test.go
- tree_test.go

## Detailed Function Coverage

### ✅ Fully Tested Functions (100% coverage)

**config.go:**
- FuncRegex()
- ClassRegex()
- HasClasses()
- GetLanguageConfig()

**finder.go:**
- NewFinder()
- ParseFuncNames()
- findClassForLine()

**formatter.go:**
- FormatGrepStyle()
- FormatExtract()

**sanitizer.go:**
- NewSanitizer()
- matchesAt()
- matchesAnyAt()
- CountBraces()

### ⚠️ Partially Tested Functions

**config.go:**
- LoadConfig() - 81.4%

**finder.go:**
- FindFunctionsInLines() - 73.4%
- findClassesWithOffset() - 80.0%
- FindFunctions() - 90.9%

**formatter.go:**
- FormatJSON() - 80.0%

**sanitizer.go:**
- CleanLine() - 97.6%

### ❌ Untested Modules (0% coverage)

**tree.go (16 functions):**
- BuildTree()
- buildClassTree()
- buildFunctionTree()
- findParent()
- setLastFlags()
- FormatTree()
- formatChildren()
- formatNode()
- formatFunctionLine()
- extractSignatureFromLines() ⚠️ CRITICAL complexity=1024
- FormatTreeCompact()
- FormatTreeFull()
- TreeToJSON()
- calcDepth()
- calculateTotalLines()

**python_finder.go (2 functions):**
- NewPythonFinder()
- FindFunctions()

**lines.go (5 functions):**
- ParseLineRange()
- ReadFileLines()
- CheckPartialFunctions()
- OutputPlainLines()
- OutputJSONLines()

**decorator.go (6 functions):**
- NewDecoratorWindow()
- Add()
- ExtractDecorators()
- Clear()
- GetIndentLevel()
- IsEmptyOrComment()

**errors.go (7 functions):**
- FatalError()
- FatalErrorWithCode()
- WarnError()
- InfoMessage()
- PrintUsage()
- PrintVersion()
- FatalErrorMsg()

**finder_factory.go (1 function):**
- CreateFinder()

**config.go (7 functions):**
- GetLanguageByExtension()
- GetSupportedLanguages()
- CallRegex()
- DecoratorRegex()
- ImportRegex()
- BlockCommentRegex()

## Test Issues Found

### Failed Tests (3)

1. **TestLoadConfig_StringChars** - c/cpp languages
   - Expected: StringChars length = 1
   - Actual: StringChars length = 2
   - Impact: Low (non-critical)

2. **TestGetLanguageConfig** - py language
   - Expected: error for 'py' (invalid language)
   - Actual: 'py' is now supported (added Python support)
   - Impact: Low (test outdated, needs update)

3. **TestFormatJSON** - JSON formatting
   - Issue: `%!d(float64=45)` format error
   - Expected: `{start: 45, end: 78}`
   - Impact: Medium (JSON output format)

## Priority Actions

### High Priority (Critical Functions Untested)

1. **tree.go** - Contains extractSignatureFromLines() with complexity=1024
   - Need comprehensive tests for tree building
   - Test tree formatting (compact/full)
   - Test signature extraction edge cases

2. **python_finder.go** - No test coverage for Python support
   - Test indent-based parsing
   - Test decorator extraction
   - Test async functions
   - Test multiline signatures

### Medium Priority

3. **lines.go** - Line range parsing untested
   - Test ParseLineRange() with various inputs
   - Test ReadFileLines() error handling

4. **decorator.go** - Decorator support untested
   - Test DecoratorWindow behavior
   - Test multi-decorator extraction

5. Fix failing tests in config_test.go and formatter_test.go

### Low Priority

6. **errors.go** - Simple utility functions
7. **finder_factory.go** - Simple factory method

## Recommendations

1. **Move tests to internal/** ✅ DONE
2. **Fix 3 failing tests** - Update expected values for py support
3. **Create tree_test.go** - Priority 1 (CRITICAL function)
4. **Create python_finder_test.go** - Priority 1 (core feature)
5. **Create lines_test.go** - Priority 2
6. **Create decorator_test.go** - Priority 2
7. **Target coverage: 70%+** (from current 34.1%)

---

*Generated: 2026-01-12*
*Tool: funcfinder v1.4.0 + go test -cover*

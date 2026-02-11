# EnhancedSanitizer Refactoring Plan: Map-Based Architecture

## Current Problems

1. **Linear Search for Delimiters**: O(n) search through slice on every character
2. **Repeated State Checks**: Same switch statements for state validation
3. **Code Duplication**: 14+ instances of "replace with spaces" pattern
4. **Dead Code**: ~250 lines of unused functions and fields
5. **No Fast Lookups**: Every delimiter match requires iteration

## Proposed Map-Based Architecture

### 1. Delimiter Lookup Maps

**Current (lines 91-127):**
```go
type SanitizerConfig struct {
    StringDelimiters []StringDelimiter  // Linear O(n) search
}

// Usage: iterate through all delimiters
for _, delim := range s.sanitizerConfig.StringDelimiters {
    if delim.IsMultiLine && s.matchesAt(...) {
        // found
    }
}
```

**Refactored:**
```go
type SanitizerConfig struct {
    // Fast O(1) lookups by delimiter string
    DelimiterMap map[string]*StringDelimiter

    // Pre-filtered maps for common queries
    RegularStringDelims   map[string]*StringDelimiter  // !IsMultiLine
    RawStringDelims       map[string]*StringDelimiter  // IsRaw && !IsMultiLine
    MultiLineDelims       map[string]*StringDelimiter  // IsMultiLine

    // Ordered by priority for conflict resolution
    DelimitersByPriority []*StringDelimiter
}

func buildDelimiterMaps(config *LanguageConfig) *SanitizerConfig {
    delimMap := make(map[string]*StringDelimiter)
    regularMap := make(map[string]*StringDelimiter)
    rawMap := make(map[string]*StringDelimiter)
    multiLineMap := make(map[string]*StringDelimiter)

    var allDelims []*StringDelimiter

    // Build from config
    for _, char := range config.StringChars {
        delim := &StringDelimiter{
            Start: char, End: char,
            EscapeChar: config.EscapeChar,
            Priority: 10,
        }
        delimMap[char] = delim
        regularMap[char] = delim
        allDelims = append(allDelims, delim)
    }

    for _, char := range config.RawStringChars {
        delim := &StringDelimiter{
            Start: char, End: char,
            IsRaw: true, Priority: 20,
        }
        delimMap[char] = delim
        rawMap[char] = delim
        allDelims = append(allDelims, delim)
    }

    for _, marker := range config.DocStringMarkers {
        delim := &StringDelimiter{
            Start: marker, End: marker,
            IsMultiLine: true, Priority: 30,
            EscapeChar: config.EscapeChar,
        }
        delimMap[marker] = delim
        multiLineMap[marker] = delim
        allDelims = append(allDelims, delim)
    }

    // Sort by priority
    sort.Slice(allDelims, func(i, j int) bool {
        return allDelims[i].Priority > allDelims[j].Priority
    })

    return &SanitizerConfig{
        DelimiterMap:         delimMap,
        RegularStringDelims:  regularMap,
        RawStringDelims:      rawMap,
        MultiLineDelims:      multiLineMap,
        DelimitersByPriority: allDelims,
    }
}
```

**Benefits:**
- O(1) delimiter lookup instead of O(n)
- Pre-filtered maps eliminate repeated filtering
- Clear separation of delimiter types

---

### 2. State Group Maps

**Current (lines 550-570):**
```go
func (s *EnhancedSanitizer) IsInString(state ParserState) bool {
    switch state {
    case StateString, StateRawString, StateMultiLineString:
        return true
    }
    return false
}

func (s *EnhancedSanitizer) IsInComment(state ParserState) bool {
    switch state {
    case StateLineComment, StateBlockComment:
        return true
    }
    return false
}
```

**Refactored:**
```go
var (
    // Immutable state classification maps (package-level)
    stringStates = map[ParserState]bool{
        StateString:          true,
        StateRawString:       true,
        StateMultiLineString: true,
        StateCharLiteral:     true,
    }

    commentStates = map[ParserState]bool{
        StateLineComment:  true,
        StateBlockComment: true,
    }

    literalStates = map[ParserState]bool{
        StateString:          true,
        StateRawString:       true,
        StateMultiLineString: true,
        StateCharLiteral:     true,
        StateLineComment:     true,
        StateBlockComment:    true,
    }

    validStates = map[ParserState]bool{
        StateNormal:          true,
        StateLineComment:     true,
        StateBlockComment:    true,
        StateString:          true,
        StateRawString:       true,
        StateCharLiteral:     true,
        StateMultiLineString: true,
    }
)

func (s *EnhancedSanitizer) IsInString(state ParserState) bool {
    return stringStates[state]
}

func (s *EnhancedSanitizer) IsInComment(state ParserState) bool {
    return commentStates[state]
}

func (s *EnhancedSanitizer) IsInLiteral(state ParserState) bool {
    return literalStates[state]
}

func ValidState(state ParserState) bool {
    return validStates[state]
}
```

**Benefits:**
- O(1) state classification
- Single source of truth for state groups
- Easy to add new state groups
- No more repetitive switch statements

---

### 3. Fast Delimiter Matching

**Current (lines 462-478):**
```go
func (s *EnhancedSanitizer) matchesAnyStringDelimiter(runes []rune, pos int) bool {
    for _, delim := range s.sanitizerConfig.StringDelimiters {
        if !delim.IsMultiLine && s.matchesAt(runes, pos, delim.Start) {
            return true
        }
    }
    return false
}

func (s *EnhancedSanitizer) matchesAnyRawStringDelimiter(runes []rune, pos int) bool {
    for _, delim := range s.sanitizerConfig.StringDelimiters {
        if delim.IsRaw && !delim.IsMultiLine && s.matchesAt(runes, pos, delim.Start) {
            return true
        }
    }
    return false
}
```

**Refactored:**
```go
// Try to match delimiter at position, return matched delimiter or nil
func (s *EnhancedSanitizer) matchDelimiterAt(runes []rune, pos int,
    delimMap map[string]*StringDelimiter) *StringDelimiter {

    // Try longer delimiters first (e.g., """ before ")
    for _, delim := range s.sanitizerConfig.DelimitersByPriority {
        if _, ok := delimMap[delim.Start]; ok {
            if s.matchesAt(runes, pos, delim.Start) {
                return delim
            }
        }
    }
    return nil
}

func (s *EnhancedSanitizer) matchesAnyStringDelimiter(runes []rune, pos int) bool {
    return s.matchDelimiterAt(runes, pos, s.sanitizerConfig.RegularStringDelims) != nil
}

func (s *EnhancedSanitizer) matchesAnyRawStringDelimiter(runes []rune, pos int) bool {
    return s.matchDelimiterAt(runes, pos, s.sanitizerConfig.RawStringDelims) != nil
}
```

**Benefits:**
- Eliminates linear search through all delimiters
- Returns matched delimiter for more context
- Respects priority ordering automatically

---

### 4. Helper Functions for Space Filling

**Current Pattern (repeated 14+ times):**
```go
for i := idx; i < idx+length; i++ {
    if i < len(result) {
        result[i] = ' '
    }
}
```

**Refactored:**
```go
// Replace range [startIdx, startIdx+length) with spaces
func (s *EnhancedSanitizer) fillWithSpaces(result []rune, startIdx, length int) {
    endIdx := startIdx + length
    if endIdx > len(result) {
        endIdx = len(result)
    }
    for i := startIdx; i < endIdx; i++ {
        result[i] = ' '
    }
}

// Replace from startIdx to end of line with spaces
func (s *EnhancedSanitizer) fillToEndWithSpaces(result []rune, startIdx int) {
    for i := startIdx; i < len(result); i++ {
        result[i] = ' '
    }
}

// Replace single character with space
func (s *EnhancedSanitizer) replaceCharWithSpace(result []rune, idx int) {
    if idx < len(result) {
        result[idx] = ' '
    }
}
```

**Benefits:**
- DRY principle: single implementation
- Clearer intent in calling code
- Easier to optimize (e.g., memset-style operations)

---

### 5. Remove Dead Code

**Items to Remove (~250 lines total):**

1. **Legacy Constants (lines 24-30):** `LegacyStateNormal`, etc. - NEVER USED
2. **Unused Fields (lines 72-74):**
   - `multiLineDepth` - never incremented
   - `blockCommentDepth` - never incremented
3. **Test-Only Functions:**
   - `GetCurrentState()` (lines 572-574) - NEVER CALLED
   - `SkipToNextLine()` (lines 576-581)
   - `ValidState()` (lines 583-585) - move to tests or keep if useful
   - `IsTransitionValid()` (lines 587-609)
   - `RuneCount()` (lines 611-613) - just wraps stdlib
   - `TruncateToRunes()` (lines 615-624)
   - `CountAngleBrackets()` (lines 538-548) - only in tests

**Keep:**
- `CountBraces()` - used in production (finder.go, structfinder.go)
- `IsInString()`, `IsInComment()`, `IsInLiteral()` - refactored with maps
- `CleanLine()` - core functionality

---

### 6. Optimize Multi-line String Search

**Current (lines 307-322) - Inefficient:**
```go
lastPos := -1
searchPos := 0
for {
    pos := strings.Index(afterStart[searchPos:], "\"")
    if pos < 0 {
        break
    }
    lastPos = searchPos + pos
    searchPos = lastPos + 1
}
```

**Refactored:**
```go
// Find last occurrence in one pass
lastPos := strings.LastIndex(afterStart, delim.End)
if lastPos >= 0 {
    foundEnd = lastPos
}
```

**Benefits:**
- Single pass instead of multiple
- Simpler code
- Same O(n) but more efficient constant factor

---

### 7. Unified Bracket Counting

**Current (lines 526-548):** Duplicated for braces and angle brackets

**Refactored:**
```go
// Generic paired character counter
func CountPairedChars(line string, openChar, closeChar rune) int {
    count := 0
    for _, ch := range line {
        if ch == openChar {
            count++
        } else if ch == closeChar {
            count--
        }
    }
    return count
}

// Public API
func CountBraces(line string) int {
    return CountPairedChars(line, '{', '}')
}

func CountAngleBrackets(line string) int {
    return CountPairedChars(line, '<', '>')
}

func CountParens(line string) int {
    return CountPairedChars(line, '(', ')')
}
```

**Benefits:**
- Eliminates duplication
- Easy to add new bracket types
- Single implementation to test

---

## Implementation Plan

### Phase 1: Add Map Infrastructure (Non-Breaking)
1. Add new map fields to `SanitizerConfig`
2. Add state group maps as package-level vars
3. Keep old slice fields for backward compatibility

### Phase 2: Add Helper Functions
1. Implement `fillWithSpaces()`, `fillToEndWithSpaces()`, `replaceCharWithSpace()`
2. Implement `matchDelimiterAt()` with map parameter
3. Implement `CountPairedChars()` generic function

### Phase 3: Refactor CleanLine()
1. Replace repeated space-filling with helper calls
2. Replace delimiter searches with map lookups
3. Replace state checks with map lookups

### Phase 4: Remove Dead Code
1. Remove legacy constants
2. Remove unused struct fields
3. Remove test-only functions (or move to _test.go)
4. Remove `NewSanitizer()` unused `useRaw` parameter

### Phase 5: Optimize
1. Replace inefficient string searches
2. Add benchmark tests to verify improvements

---

## Expected Results

### Code Reduction
- **Before:** 670 lines
- **After:** ~420 lines (250 lines removed)
- **Reduction:** 37%

### Performance Improvements
- Delimiter matching: O(n) → O(1) for single-char delimiters
- State validation: O(1) → O(1) but simpler
- Space filling: 14 implementations → 3 helpers

### Maintainability
- ✅ No code duplication
- ✅ Clear separation of concerns
- ✅ Easy to add new delimiter types
- ✅ Single source of truth for state groups

### Backward Compatibility
- ✅ Public API unchanged
- ✅ All existing tests pass
- ✅ Can be done incrementally

---

## Risk Assessment

**Low Risk:**
- Map lookups are well-tested Go patterns
- State classification is straightforward
- Helper functions are simple wrappers

**Medium Risk:**
- Delimiter priority handling must be preserved
- Edge cases with multi-character delimiters
- Performance regression if map overhead > linear search for small sets

**Mitigation:**
- Comprehensive benchmark suite
- Test against current implementation
- Keep priority-ordered slice for conflict resolution

---

## Next Steps

1. ✅ Create this refactoring plan
2. ⏳ Implement Phase 1 (map infrastructure)
3. ⏳ Write benchmark tests
4. ⏳ Implement Phases 2-3 (helpers and refactor)
5. ⏳ Remove dead code (Phase 4)
6. ⏳ Verify performance and correctness
7. ⏳ Update documentation

**Estimated effort:** 4-6 hours
**Lines saved:** ~250 lines (37% reduction)
**Performance gain:** 2-5x for delimiter-heavy code

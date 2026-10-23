// structfinder.go - Find complex data types and their fields
// Part of the funcfinder toolkit
package internal

import (
	"regexp"
	"sort"
	"strings"
)

// TypeBounds contains information about a type definition
type TypeBounds struct {
	Name            string        // Type name
	Kind            string        // class, struct, interface, enum, union
	Start           int           // Start line (1-based)
	End             int           // End line (1-based)
	Fields          []FieldBounds // Fields/members
	ParentType      string        // Parent type if nested
	ParentLine      int           // Line of parent type definition
	StartLineIndent int           // Indentation level of type start (for indent-based)
}

// FieldBounds contains information about a field/member in a type
type FieldBounds struct {
	Name string // Field name
	Type string // Field type
	Line int    // Line number
}

// StructFindResult contains the result of type search
type StructFindResult struct {
	Types    []TypeBounds
	Filename string
}

// StructFinder finds complex data types in source code
type StructFinder struct {
	config    *LanguageConfig
	sanitizer *Sanitizer
	typeNames map[string]bool
	mapMode   bool
	// noFields skips the fields of each type: a directory job reports only the types (prepareDirJob).
	noFields bool
}

// NewStructFinder creates a new struct finder
func NewStructFinder(config *LanguageConfig, typeNamesStr string, mapMode bool) *StructFinder {
	nameMap := make(map[string]bool)
	for _, name := range ParseFuncNames(typeNamesStr) {
		nameMap[name] = true
	}

	return &StructFinder{
		config:    config,
		sanitizer: NewSanitizer(config, false),
		typeNames: nameMap,
		mapMode:   mapMode,
	}
}

// FindStructures finds all types in the file
func (f *StructFinder) FindStructures(filename string) (*StructFindResult, error) {
	lines, err := readFileLines(filename)
	if err != nil {
		return nil, err
	}
	return f.FindStructuresInLines(lines, 1, filename)
}

// FindStructuresInLines finds types in pre-read lines
func (f *StructFinder) FindStructuresInLines(lines []string, startLine int, filename string) (*StructFindResult, error) {
	if f.config.LangKey == "goose" {
		return f.findGooseStructures(lines, startLine, filename)
	}
	lineOffset := startLine - 1

	result := &StructFindResult{
		Filename: filename,
		Types:    []TypeBounds{},
	}

	// The cleaned lines and the indentation of each line come from one pass over the file (LineFacts).
	facts := f.sanitizer.CleanLinesFacts(lines)

	// Find all types using new struct patterns
	types := f.findAllTypes(facts, lineOffset)

	// For each type, find its fields
	for i := range types {
		typeBounds := &types[i]
		if !f.noFields {
			typeBounds.Fields = f.findFieldsForType(facts, typeBounds, lineOffset)
		}
		result.Types = append(result.Types, *typeBounds)
	}

	return result, nil
}

// findAllTypes finds all type definitions in the file
func (f *StructFinder) findAllTypes(facts []LineFacts, lineOffset int) []TypeBounds {
	var types []TypeBounds
	var currentType *TypeBounds
	depth := 0
	inGoTypeGroup := false

	// Use struct patterns from config if available, otherwise fall back to classRegex
	structPatterns := f.config.GetStructPatterns()
	patternKinds := structPatternOrder(structPatterns)
	hasStructPatterns := len(structPatterns) > 0

	for lineNum, fact := range facts {
		cleaned := fact.Cleaned

		if currentType != nil {
			// We're inside a type definition - find end based on language type
			if f.config.BlockEndKeyword != "" {
				// Ruby-like: find 'end' at same indent level as type start
				if rubyBlockEndAt(cleaned, fact.Indent, f.config.BlockEndKeyword, currentType.StartLineIndent) {
					currentType.End = lineNum + 1 + lineOffset
					types = append(types, *currentType)
					currentType = nil
				}
			} else if f.config.IndentBased {
				// For indent-based languages like Python, use indentation
				indent := fact.Indent
				if indent <= currentType.StartLineIndent {
					currentType.End = lineNum + 1 + lineOffset
					types = append(types, *currentType)
					currentType = nil
				}
			} else if f.config.HasClasses() {
				// Brace-based languages: count braces to find the end
				prevDepth := depth
				depth += CountBraces(cleaned)

				// Type ends when we exit the outermost braces
				if depth == 0 && prevDepth > 0 {
					currentType.End = lineNum + 1 + lineOffset
					types = append(types, *currentType)
					currentType = nil
				}
			}
		} else {
			// Look for new type definition using struct patterns
			found := false

			// Go `type ( ... )` group members carry no `type` keyword of their own: read them with a
			// virtual `type ` prefix so the same patterns match them. Alias members (`A = S`) match no
			// struct pattern and stay aliases; the group open/close lines match nothing either.
			scanLine := cleaned
			if f.config.LangKey == "go" {
				scanLine = goGroupMemberLine(cleaned, &inGoTypeGroup)
			}

			if hasStructPatterns && f.config.MayStartType(scanLine) {
				// Use new struct patterns for type detection
				for _, typeKind := range patternKinds {
					pattern := structPatterns[typeKind]
					matches := pattern.FindStringSubmatch(scanLine)
					if matches != nil {
						// Extract type name (last non-empty group)
						typeName := lastNonEmptyGroup(matches)

						if typeName != "" && (f.mapMode || f.typeNames[typeName]) {
							// Determine opening brace position
							braceCount := CountBraces(cleaned)

							startIndent := fact.Indent

							currentType = &TypeBounds{
								Name:            typeName,
								Kind:            typeKind,
								Start:           lineNum + 1 + lineOffset,
								StartLineIndent: startIndent,
								Fields:          []FieldBounds{},
							}

							if braceCount > 0 {
								// Opening brace(s) unmatched on this line — the body
								// continues below; count down to find the end.
								depth = braceCount
							} else if f.config.BlockEndKeyword != "" {
								// Ruby-like: will find end by matching 'end' keyword
								depth = 1 // Mark as inside
							} else if f.config.IndentBased {
								// Indent-based type, will find end by indentation
								depth = 1 // Mark as inside
							} else if typeKind == "named" || typeKind == "type_alias" {
								// Single-line type definitions (Go `type X string`,
								// `type X func(...)`; TypeScript `type X = Y`) complete
								// on their own line. Close immediately so they don't stay "open"
								// and swallow following declarations while waiting for
								// a brace that never comes.
								currentType.End = lineNum + 1 + lineOffset
								types = append(types, *currentType)
								currentType = nil
							} else if strings.ContainsRune(cleaned, '{') {
								// Balanced braces on the same line (`type Foo struct{}`,
								// `struct{ io.Writer }`, inline interface): the type is
								// complete here. braceCount nets to 0, but the braces
								// *were* present — distinguish this from a genuine
								// multi-line signature (no brace yet) so we don't stay
								// open and swallow the next declaration.
								currentType.End = lineNum + 1 + lineOffset
								types = append(types, *currentType)
								currentType = nil
							} else {
								// Multi-line signature (e.g. C++ `class Foo` with the
								// opening brace on the next line) — wait for the brace.
								depth = 0
							}
							found = true
							break
						}
					}
				}
			}

			if !found && !hasStructPatterns {
				// Fall back to classRegex for backward compatibility
				classRegex := f.config.ClassRegex()
				var matches []string
				if f.config.MayStartType(cleaned) {
					matches = classRegex.FindStringSubmatch(cleaned)
				}
				if matches != nil {
					// Extract type name (last non-empty group)
					typeName := ""
					typeKind := "type"
					for i := len(matches) - 1; i >= 1; i-- {
						if matches[i] != "" {
							// Determine kind based on pattern
							kind := determineTypeKind(matches[0])
							if kind != "" {
								typeKind = kind
							}
							typeName = matches[i]
							break
						}
					}

					if typeName != "" && (f.mapMode || f.typeNames[typeName]) {
						// Determine opening brace position
						braceCount := CountBraces(cleaned)

						startIndent := fact.Indent

						currentType = &TypeBounds{
							Name:            typeName,
							Kind:            typeKind,
							Start:           lineNum + 1 + lineOffset,
							StartLineIndent: startIndent,
							Fields:          []FieldBounds{},
						}

						if braceCount > 0 {
							// Opening brace(s) unmatched on this line — body continues.
							depth = braceCount
						} else if f.config.BlockEndKeyword != "" {
							// Ruby-like: will find end by matching 'end' keyword
							depth = 1 // Mark as inside
						} else if f.config.IndentBased {
							// Indent-based type, will find end by indentation
							depth = 1 // Mark as inside
						} else if strings.ContainsRune(cleaned, '{') {
							// Balanced braces on the same line — complete inline type.
							currentType.End = lineNum + 1 + lineOffset
							types = append(types, *currentType)
							currentType = nil
						} else {
							// Multi-line signature, waiting for brace
							depth = 0
						}
					}
				} else if currentType != nil && depth == 0 && !f.config.IndentBased && f.config.BlockEndKeyword == "" {
					// Continue multi-line signature
					braceCount := CountBraces(cleaned)
					if braceCount > 0 {
						depth = braceCount
						if depth == 0 {
							currentType.End = lineNum + 1 + lineOffset
							types = append(types, *currentType)
							currentType = nil
						}
					}
				}
			}
		}
	}

	// Handle types that extend to EOF
	if currentType != nil && currentType.End == 0 {
		currentType.End = len(facts) + lineOffset
		types = append(types, *currentType)
	}

	return types
}

// determineTypeKind determines the type kind from the matched pattern
// lastNonEmptyGroup returns the last non-empty capture group of a struct pattern's match: the type's name.
func lastNonEmptyGroup(matches []string) string {
	for i := len(matches) - 1; i >= 1; i-- {
		if matches[i] != "" {
			return matches[i]
		}
	}
	return ""
}

// structPatternOrder returns the pattern kinds in sorted order. When two patterns match the same line, the first
// kind in this order names the type, so the kind does not depend on map iteration. The catch-all "named" pattern
// (Go's `type X Y`) also matches a one-line `type X struct{}`, so it comes last and the specific kinds win.
func structPatternOrder(patterns map[string]*regexp.Regexp) []string {
	kinds := make([]string, 0, len(patterns))
	for kind := range patterns {
		if kind != "named" {
			kinds = append(kinds, kind)
		}
	}
	sort.Strings(kinds)
	if _, ok := patterns["named"]; ok {
		kinds = append(kinds, "named")
	}
	return kinds
}

func determineTypeKind(pattern string) string {
	patternLower := strings.ToLower(pattern)
	if strings.Contains(patternLower, "class") {
		return "class"
	}
	if strings.Contains(patternLower, "struct") {
		return "struct"
	}
	if strings.Contains(patternLower, "interface") {
		return "interface"
	}
	if strings.Contains(patternLower, "enum") {
		return "enum"
	}
	if strings.Contains(patternLower, "union") {
		return "union"
	}
	if strings.Contains(patternLower, "type") {
		return "type"
	}
	if strings.Contains(patternLower, "trait") {
		return "trait"
	}
	return "type"
}

// findFieldsForType finds all fields/members in a type definition
func (f *StructFinder) findFieldsForType(facts []LineFacts, typeBounds *TypeBounds, lineOffset int) []FieldBounds {
	var fields []FieldBounds

	// Get the field pattern from config
	fieldRegex := f.config.GetFieldPattern()

	if fieldRegex == nil {
		return fields // No pattern configured
	}

	for lineNum := typeBounds.Start - 1 - lineOffset; lineNum < len(facts) && lineNum < typeBounds.End-1-lineOffset; lineNum++ {
		fact := facts[lineNum]
		cleaned := fact.Cleaned

		// Skip empty lines and comments
		if IsEmptyOrComment(cleaned, f.config.LineComment) {
			continue
		}

		// Check if we exited the type (for indent-based)
		if f.config.IndentBased {
			indent := fact.Indent
			if indent <= typeBounds.StartLineIndent {
				break
			}
		}

		// Find field declarations
		matches := fieldRegex.FindStringSubmatch(cleaned)
		if matches != nil && len(matches) >= 2 {
			fieldName := matches[1]
			fieldType := ""
			if len(matches) >= 3 {
				fieldType = strings.TrimSpace(matches[2])
			}

			// Skip if field name is empty, looks like a method, or is an excluded word
			if fieldName != "" && !isLikelyMethod(fieldName, cleaned) && !isExcludedWord(fieldName, f.config.ExcludeWords) {
				fields = append(fields, FieldBounds{
					Name: fieldName,
					Type: fieldType,
					Line: lineNum + 1 + lineOffset,
				})
			}
		}
	}

	return fields
}

// isLikelyMethod checks if the declaration looks like a method/function
func isLikelyMethod(name string, line string) bool {
	// Functions have parentheses, fields don't
	return strings.Contains(line, "(") || strings.Contains(line, ")")
}

// isExcludedWord checks if the name is in the exclude words list
func isExcludedWord(name string, excludeWords []string) bool {
	for _, word := range excludeWords {
		if name == word {
			return true
		}
	}
	return false
}

// rubyBlockEndAt is isRubyBlockEnd for a cleaned line: the keyword is read from the cleaned text, so a trailing
// comment after `end` does not hide it, and the indentation is the one LineFacts gives (of the raw line).
func rubyBlockEndAt(cleaned string, indent int, keyword string, expectedIndent int) bool {
	return strings.TrimSpace(cleaned) == keyword && indent == expectedIndent
}

// structfinder.go - Find complex data types and their fields
// Part of the funcfinder toolkit
package internal

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// TypeBounds contains information about a type definition
type TypeBounds struct {
	Name           string        // Type name
	Kind           string        // class, struct, interface, enum, union
	Start          int           // Start line (1-based)
	End            int           // End line (1-based)
	Fields         []FieldBounds // Fields/members
	ParentType     string        // Parent type if nested
	ParentLine     int           // Line of parent type definition
	StartLineIndent int          // Indentation level of type start (for indent-based)
}

// FieldBounds contains information about a field/member in a type
type FieldBounds struct {
	Name string // Field name
	Type string // Field type
	Line int    // Line number
}

// StructFindResult contains the result of type search
type StructFindResult struct {
	Types   []TypeBounds
	Filename string
}

// StructFinder finds complex data types in source code
type StructFinder struct {
	config    *LanguageConfig
	sanitizer *Sanitizer
	typeNames map[string]bool
	mapMode   bool
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
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	return f.FindStructuresInLines(lines, 1, filename)
}

// FindStructuresInLines finds types in pre-read lines
func (f *StructFinder) FindStructuresInLines(lines []string, startLine int, filename string) (*StructFindResult, error) {
	lineOffset := startLine - 1

	result := &StructFindResult{
		Filename: filename,
		Types:    []TypeBounds{},
	}

	// Find all types using new struct patterns
	types := f.findAllTypes(lines, lineOffset)

	// For each type, find its fields
	for i := range types {
		typeBounds := &types[i]
		fields := f.findFieldsForType(lines, typeBounds, lineOffset)
		typeBounds.Fields = fields
		result.Types = append(result.Types, *typeBounds)
	}

	return result, nil
}

// findAllTypes finds all type definitions in the file
func (f *StructFinder) findAllTypes(lines []string, lineOffset int) []TypeBounds {
	state := StateNormal
	var types []TypeBounds
	var currentType *TypeBounds
	depth := 0

	// Use struct patterns from config if available, otherwise fall back to classRegex
	structPatterns := f.config.GetStructPatterns()
	hasStructPatterns := len(structPatterns) > 0

	for lineNum, line := range lines {
		// Clean line from comments and literals
		cleaned, newState := f.sanitizer.CleanLine(line, state)
		state = newState

		if currentType != nil {
			// We're inside a type definition
			// Count braces to find the end
			if f.config.HasClasses() {
				prevDepth := depth
				depth += CountBraces(cleaned)

				// Type ends when we exit the outermost braces
				// (prevDepth > 0 && depth == 0), not just depth == 0
				if depth == 0 && prevDepth > 0 {
					currentType.End = lineNum + 1 + lineOffset
					types = append(types, *currentType)
					currentType = nil
				}
			} else {
				// For indent-based languages like Python, use indentation
				// For simple brace-less types, end at next type or EOF
				if f.config.IndentBased {
					indent := GetIndentLevel(line)
					if indent <= currentType.StartLineIndent {
						currentType.End = lineNum + 1 + lineOffset
						types = append(types, *currentType)
						currentType = nil
					}
				}
			}
		} else {
			// Look for new type definition using struct patterns
			found := false
			
			if hasStructPatterns {
				// Use new struct patterns for type detection
				for typeKind, pattern := range structPatterns {
					matches := pattern.FindStringSubmatch(cleaned)
					if matches != nil {
						// Extract type name (last non-empty group)
						typeName := ""
						for i := len(matches) - 1; i >= 1; i-- {
							if matches[i] != "" {
								typeName = matches[i]
								break
							}
						}

						if typeName != "" && (f.mapMode || f.typeNames[typeName]) {
							// Determine opening brace position
							braceCount := CountBraces(cleaned)

							startIndent := GetIndentLevel(line)

							currentType = &TypeBounds{
								Name:           typeName,
								Kind:           typeKind,
								Start:          lineNum + 1 + lineOffset,
								StartLineIndent: startIndent,
								Fields:         []FieldBounds{},
							}

							if braceCount > 0 {
								// Opening brace on same line
								depth = braceCount
								if depth == 0 {
									// Single-line type definition
									currentType.End = lineNum + 1 + lineOffset
									types = append(types, *currentType)
									currentType = nil
								}
							} else if f.config.IndentBased {
								// Indent-based type, will find end by indentation
								depth = 1 // Mark as inside
							} else {
								// Multi-line signature, waiting for brace
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
				matches := classRegex.FindStringSubmatch(cleaned)
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

						startIndent := GetIndentLevel(line)

						currentType = &TypeBounds{
							Name:           typeName,
							Kind:           typeKind,
							Start:          lineNum + 1 + lineOffset,
							StartLineIndent: startIndent,
							Fields:         []FieldBounds{},
						}

						if braceCount > 0 {
							// Opening brace on same line
							depth = braceCount
							if depth == 0 {
								// Single-line type definition
								currentType.End = lineNum + 1 + lineOffset
								types = append(types, *currentType)
								currentType = nil
							}
						} else if f.config.IndentBased {
							// Indent-based type, will find end by indentation
							depth = 1 // Mark as inside
						} else {
							// Multi-line signature, waiting for brace
							depth = 0
						}
					}
				} else if currentType != nil && depth == 0 && !f.config.IndentBased {
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
		currentType.End = len(lines) + lineOffset
		types = append(types, *currentType)
	}

	return types
}

// determineTypeKind determines the type kind from the matched pattern
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
func (f *StructFinder) findFieldsForType(lines []string, typeBounds *TypeBounds, lineOffset int) []FieldBounds {
	var fields []FieldBounds

	// Get the field pattern from config
	fieldRegex := f.config.GetFieldPattern()

	if fieldRegex == nil {
		return fields // No pattern configured
	}

	state := StateNormal

	for lineNum := typeBounds.Start - 1 - lineOffset; lineNum < len(lines) && lineNum < typeBounds.End-1-lineOffset; lineNum++ {
		line := lines[lineNum]

		// Clean line from comments and strings
		cleaned, newState := f.sanitizer.CleanLine(line, state)
		state = newState

		// Skip empty lines and comments
		if IsEmptyOrComment(cleaned, f.config.LineComment) {
			continue
		}

		// Check if we exited the type (for indent-based)
		if f.config.IndentBased {
			indent := GetIndentLevel(line)
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

			// Skip if field name is empty or looks like a method
			if fieldName != "" && !isLikelyMethod(fieldName, cleaned) {
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

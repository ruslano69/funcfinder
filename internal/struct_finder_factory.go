// struct_finder_factory.go - Factory for creating struct/type finders
// Supports multiple finder types: brace-based, indent-based, and hybrid languages
package internal

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// StructFinderFactory creates appropriate struct finder based on language
type StructFinderFactory struct{}

// NewStructFinderFactory creates a new factory
func NewStructFinderFactory() *StructFinderFactory {
	return &StructFinderFactory{}
}

// FinderType represents the type of struct finder to create
type FinderType int

const (
	// FinderTypeBrace-based languages (C++, C#, Java, D, Go, Rust, etc.)
	FinderTypeBrace FinderType = iota
	// FinderTypeIndent-based languages (Python, Ruby)
	FinderTypeIndent
	// FinderTypeHybrid languages (JavaScript, TypeScript with type aliases)
	FinderTypeHybrid
)

// CreateStructFinder creates appropriate struct finder for the language
func (f *StructFinderFactory) CreateStructFinder(config *LanguageConfig, typeNamesStr string, mapMode, extractMode bool) StructFinderInterface {
	// Determine the finder type based on language configuration
	finderType := f.determineFinderType(config)

	switch finderType {
	case FinderTypeIndent:
		// For indent-based languages (Python, Ruby), use special finder
		return NewPythonStructFinder(*config, typeNamesStr, mapMode, extractMode)

	case FinderTypeHybrid:
		// For hybrid languages (JavaScript, TypeScript with type aliases), use hybrid finder
		return NewHybridStructFinder(config, typeNamesStr, mapMode, extractMode)

	case FinderTypeBrace:
		fallthrough
	default:
		// For brace-based languages, use standard finder
		return NewStructFinder(config, typeNamesStr, mapMode)
	}
}

// determineFinderType determines the appropriate finder type for the language
func (f *StructFinderFactory) determineFinderType(config *LanguageConfig) FinderType {
	// Indent-based languages
	if config.IndentBased {
		return FinderTypeIndent
	}

	// Check if language has struct patterns configured
	if config.HasStructSupport() {
		// Check for hybrid language patterns (type aliases, interfaces in JS/TS)
		_, hasTypeAlias := config.structPatterns["type_alias"]
		_, hasInterface := config.structPatterns["interface"]

		// JavaScript and TypeScript have both classes and type aliases
		if hasTypeAlias || hasInterface {
			// For JS/TS, we use hybrid finder to handle both brace-based and type alias patterns
			if config.LangKey == "js" || config.LangKey == "ts" {
				return FinderTypeHybrid
			}
		}
	}

	return FinderTypeBrace
}

// StructFinderInterface defines the contract for struct finders
type StructFinderInterface interface {
	FindStructures(filename string) (*StructFindResult, error)
	FindStructuresInLines(lines []string, startLine int, filename string) (*StructFindResult, error)
}

// HybridStructFinder handles languages with mixed syntax (JS/TS)
type HybridStructFinder struct {
	config      *LanguageConfig
	sanitizer   *Sanitizer
	typeNames   map[string]bool
	mapMode     bool
	extractMode bool
}

// NewHybridStructFinder creates a new hybrid struct finder for JS/TS
func NewHybridStructFinder(config *LanguageConfig, typeNamesStr string, mapMode, extractMode bool) *HybridStructFinder {
	nameMap := make(map[string]bool)
	for _, name := range ParseFuncNames(typeNamesStr) {
		nameMap[name] = true
	}

	return &HybridStructFinder{
		config:      config,
		sanitizer:   NewSanitizer(config, false),
		typeNames:   nameMap,
		mapMode:     mapMode,
		extractMode: extractMode,
	}
}

// FindStructures finds all types in the file
func (f *HybridStructFinder) FindStructures(filename string) (*StructFindResult, error) {
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
func (f *HybridStructFinder) FindStructuresInLines(lines []string, startLine int, filename string) (*StructFindResult, error) {
	lineOffset := startLine - 1

	result := &StructFindResult{
		Filename: filename,
		Types:    []TypeBounds{},
	}

	// Find all types
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

// findAllTypes finds all type definitions in hybrid languages
func (f *HybridStructFinder) findAllTypes(lines []string, lineOffset int) []TypeBounds {
	state := StateNormal
	var types []TypeBounds
	var currentType *TypeBounds
	depth := 0

	structPatterns := f.config.GetStructPatterns()

	for lineNum, line := range lines {
		cleaned, newState := f.sanitizer.CleanLine(line, state)
		state = newState

		if currentType != nil {
			prevDepth := depth
			depth += CountBraces(cleaned)

			if depth == 0 && prevDepth > 0 {
				currentType.End = lineNum + 1 + lineOffset
				types = append(types, *currentType)
				currentType = nil
			}
		} else {
			// Look for new type definition
			for typeKind, pattern := range structPatterns {
				matches := pattern.FindStringSubmatch(cleaned)
				if matches != nil {
					typeName := ""
					for i := len(matches) - 1; i >= 1; i-- {
						if matches[i] != "" {
							typeName = matches[i]
							break
						}
					}

					if typeName != "" && (f.mapMode || f.typeNames[typeName]) {
						braceCount := CountBraces(cleaned)
						startIndent := GetIndentLevel(line)

						currentType = &TypeBounds{
							Name:            typeName,
							Kind:            typeKind,
							Start:           lineNum + 1 + lineOffset,
							StartLineIndent: startIndent,
							Fields:          []FieldBounds{},
						}

						if braceCount > 0 {
							depth = braceCount
							if depth == 0 {
								currentType.End = lineNum + 1 + lineOffset
								types = append(types, *currentType)
								currentType = nil
							}
						} else if f.config.IndentBased {
							depth = 1
						} else {
							depth = 0
						}
						break
					}
				}
			}
		}
	}

	if currentType != nil && currentType.End == 0 {
		currentType.End = len(lines) + lineOffset
		types = append(types, *currentType)
	}

	return types
}

// findFieldsForType finds all fields/members in a type definition
func (f *HybridStructFinder) findFieldsForType(lines []string, typeBounds *TypeBounds, lineOffset int) []FieldBounds {
	var fields []FieldBounds

	fieldRegex := f.config.GetFieldPattern()
	if fieldRegex == nil {
		return fields
	}

	state := StateNormal

	for lineNum := typeBounds.Start - 1 - lineOffset; lineNum < len(lines) && lineNum < typeBounds.End-1-lineOffset; lineNum++ {
		line := lines[lineNum]
		cleaned, newState := f.sanitizer.CleanLine(line, state)
		state = newState

		if IsEmptyOrComment(cleaned, f.config.LineComment) {
			continue
		}

		if f.config.IndentBased {
			indent := GetIndentLevel(line)
			if indent <= typeBounds.StartLineIndent {
				break
			}
		}

		matches := fieldRegex.FindStringSubmatch(cleaned)
		if matches != nil && len(matches) >= 2 {
			fieldName := matches[1]
			fieldType := ""
			if len(matches) >= 3 {
				fieldType = strings.TrimSpace(matches[2])
			}

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

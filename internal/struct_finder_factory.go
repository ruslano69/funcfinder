// struct_finder_factory.go - Factory for creating struct/type finders
// Supports multiple finder types: brace-based, indent-based, and hybrid languages
package internal

import (
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
		return NewPythonStructFinder(config, typeNamesStr, mapMode, extractMode)

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
	// Block-end-keyword languages (Ruby) use standard brace finder with special end detection
	if config.BlockEndKeyword != "" {
		return FinderTypeBrace
	}

	// Indent-based languages (Python)
	if config.IndentBased {
		return FinderTypeIndent
	}

	// Check if language has struct patterns configured
	if config.HasStructSupport() {
		// Check for hybrid language patterns (type aliases, interfaces in JS/TS)
		hasTypeAlias := config.GetStructPattern("type_alias") != nil
		hasInterface := config.GetStructPattern("interface") != nil

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
	// noFields skips the fields of each type: a directory job reports only the types (prepareDirJob).
	noFields bool
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
	lines, err := readFileLines(filename)
	if err != nil {
		return nil, err
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
	facts := f.sanitizer.CleanLinesFacts(lines)
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

// findAllTypes finds all type definitions in hybrid languages
func (f *HybridStructFinder) findAllTypes(facts []LineFacts, lineOffset int) []TypeBounds {
	var types []TypeBounds
	var currentType *TypeBounds
	depth := 0

	structPatterns := f.config.GetStructPatterns()

	patternKinds := structPatternOrder(structPatterns)

	for lineNum, fact := range facts {
		cleaned := fact.Cleaned

		if currentType != nil {
			prevDepth := depth
			depth += CountBraces(cleaned)

			if depth == 0 && prevDepth > 0 {
				currentType.End = lineNum + 1 + lineOffset
				types = append(types, *currentType)
				currentType = nil
			}
		} else if f.config.MayStartType(cleaned) {
			// Look for new type definition
			for _, typeKind := range patternKinds {
				pattern := structPatterns[typeKind]
				matches := pattern.FindStringSubmatch(cleaned)
				if matches != nil {
					typeName := lastNonEmptyGroup(matches)

					if typeName != "" && (f.mapMode || f.typeNames[typeName]) {
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
							depth = braceCount
							if depth == 0 {
								currentType.End = lineNum + 1 + lineOffset
								types = append(types, *currentType)
								currentType = nil
							}
						} else if f.config.IndentBased {
							depth = 1
						} else {
							// No brace on this line and not indent-based: by
							// construction this is a single-line construct (e.g. a
							// TS type alias `type Handler = (req: Request) => void;`)
							// with no depth>0 -> 0 transition ever coming to close
							// it. Close it immediately instead of leaving it open
							// with depth=0, where it would silently swallow every
							// subsequent line until an unrelated brace-balance event
							// coincidentally drove depth back to 0.
							currentType.End = lineNum + 1 + lineOffset
							types = append(types, *currentType)
							currentType = nil
						}
						break
					}
				}
			}
		}
	}

	if currentType != nil && currentType.End == 0 {
		currentType.End = len(facts) + lineOffset
		types = append(types, *currentType)
	}

	return types
}

// findFieldsForType finds all fields/members in a type definition
func (f *HybridStructFinder) findFieldsForType(facts []LineFacts, typeBounds *TypeBounds, lineOffset int) []FieldBounds {
	var fields []FieldBounds

	fieldRegex := f.config.GetFieldPattern()
	if fieldRegex == nil {
		return fields
	}

	for lineNum := typeBounds.Start - 1 - lineOffset; lineNum < len(facts) && lineNum < typeBounds.End-1-lineOffset; lineNum++ {
		fact := facts[lineNum]
		cleaned := fact.Cleaned

		if IsEmptyOrComment(cleaned, f.config.LineComment) {
			continue
		}

		if f.config.IndentBased {
			indent := fact.Indent
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

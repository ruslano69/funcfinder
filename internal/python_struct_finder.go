// python_struct_finder.go - Find Python data types and classes
// Supports: class, dataclass, NamedTuple, TypedDict, Enum, attrs
package internal

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// PythonStructFinder finds data types in Python source code
type PythonStructFinder struct {
	config      LanguageConfig
	typeNames   map[string]bool
	mapMode     bool
	extractMode bool
}

// NewPythonStructFinder creates a new Python struct finder
func NewPythonStructFinder(config LanguageConfig, typeNamesStr string, mapMode, extractMode bool) *PythonStructFinder {
	nameMap := make(map[string]bool)
	for _, name := range ParseFuncNames(typeNamesStr) {
		nameMap[name] = true
	}

	return &PythonStructFinder{
		config:      config,
		typeNames:   nameMap,
		mapMode:     mapMode,
		extractMode: extractMode,
	}
}

// FindStructures finds all types in Python file
func (f *PythonStructFinder) FindStructures(filename string) (*StructFindResult, error) {
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
func (f *PythonStructFinder) FindStructuresInLines(lines []string, startLine int, filename string) (*StructFindResult, error) {
	lineOffset := startLine - 1

	result := &StructFindResult{
		Filename: filename,
		Types:    []TypeBounds{},
	}

	// Find all class/type definitions
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

// Python-specific class patterns
var (
	// Standard class: class Name:
	classPattern = regexp.MustCompile(`^\s*class\s+(\w+)\s*(\(\s*([\w,\s\.\[\]]*)\s*\))?\s*:`)

	// Dataclass: @dataclass\nclass Name:
	dataclassPattern = regexp.MustCompile(`^\s*@dataclass\s*\nclass\s+(\w+)\s*(\(\s*([\w,\s\.\[\]]*)\s*\))?\s*:`)

	// NamedTuple: class Name(NamedTuple):
	namedTuplePattern = regexp.MustCompile(`^\s*class\s+(\w+)\s*\(\s*NamedTuple\s*\)\s*:`)

	// TypedDict: class Name(TypedDict):
	typedDictPattern = regexp.MustCompile(`^\s*class\s+(\w+)\s*\(\s*TypedDict\s*\)\s*:`)

	// Enum: class Name(Enum):
	enumPattern = regexp.MustCompile(`^\s*class\s+(\w+)\s*\(\s*Enum\s*\)\s*:`)

	// attrs: @attr.s\nclass Name:
	attrsPattern = regexp.MustCompile(`^\s*@attr\.s\s*\nclass\s+(\w+)\s*(\(\s*([\w,\s\.\[\]]*)\s*\))?\s*:`)

	// ABC: class Name(ABC):
	abcPattern = regexp.MustCompile(`^\s*class\s+(\w+)\s*\(\s*ABC\s*\)\s*:`)

	// Protocol: class Name(Protocol):
	protocolPattern = regexp.MustCompile(`^\s*class\s+(\w+)\s*\(\s*Protocol\s*\)\s*:`)
)

// findAllTypes finds all type definitions in Python file
func (f *PythonStructFinder) findAllTypes(lines []string, lineOffset int) []TypeBounds {
	var types []TypeBounds

	for lineNum, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Check for decorators before class
		if strings.HasPrefix(trimmed, "@") {
			// Check if next line is a dataclass or attrs class
			if lineNum+1 < len(lines) {
				nextLine := strings.TrimSpace(lines[lineNum+1])

				// Dataclass
				if matches := dataclassPattern.FindStringSubmatch(nextLine); matches != nil {
					typeName := matches[1]
					if f.mapMode || f.typeNames[typeName] {
						endLine := f.findTypeEnd(lines, lineNum+1, lineOffset)
						types = append(types, TypeBounds{
							Name:   typeName,
							Kind:   "dataclass",
							Start:  lineNum + 1 + 1 + lineOffset,
							End:    endLine,
							Fields: []FieldBounds{},
						})
					}
					continue
				}

				// attrs
				if matches := attrsPattern.FindStringSubmatch(nextLine); matches != nil {
					typeName := matches[1]
					if f.mapMode || f.typeNames[typeName] {
						endLine := f.findTypeEnd(lines, lineNum+1, lineOffset)
						types = append(types, TypeBounds{
							Name:   typeName,
							Kind:   "attrs",
							Start:  lineNum + 1 + 1 + lineOffset,
							End:    endLine,
							Fields: []FieldBounds{},
						})
					}
					continue
				}
			}
		}

		// Check for standard class definitions
		if matches := classPattern.FindStringSubmatch(trimmed); matches != nil {
			typeName := matches[1]
			bases := matches[3] // Base classes if any

			// Determine type kind from bases
			kind := "class"
			switch {
			case strings.Contains(bases, "NamedTuple"):
				kind = "NamedTuple"
			case strings.Contains(bases, "TypedDict"):
				kind = "TypedDict"
			case strings.Contains(bases, "Enum"):
				kind = "enum"
			case strings.Contains(bases, "ABC"):
				kind = "abstract"
			case strings.Contains(bases, "Protocol"):
				kind = "Protocol"
			}

			if f.mapMode || f.typeNames[typeName] {
				endLine := f.findTypeEnd(lines, lineNum, lineOffset)
				types = append(types, TypeBounds{
					Name:   typeName,
					Kind:   kind,
					Start:  lineNum + 1 + lineOffset,
					End:    endLine,
					Fields: []FieldBounds{},
				})
			}
		}
	}

	return types
}

// findTypeEnd finds the end line of a type definition using indentation
func (f *PythonStructFinder) findTypeEnd(lines []string, startLine int, lineOffset int) int {
	if startLine >= len(lines) {
		return startLine + 1 + lineOffset
	}

	// Get the indentation level of the class definition
	startIndent := GetIndentLevel(lines[startLine])

	// Find the first line with indentation <= startIndent
	for i := startLine + 1; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue // Skip empty and comment lines
		}

		currentIndent := GetIndentLevel(lines[i])
		if currentIndent <= startIndent {
			return i + lineOffset
		}
	}

	return len(lines) + lineOffset
}

// findFieldsForType finds all fields/members in a Python type
func (f *PythonStructFinder) findFieldsForType(lines []string, typeBounds *TypeBounds, lineOffset int) []FieldBounds {
	var fields []FieldBounds

	// Field patterns for Python
	// Field patterns: name: type, name: type = value, name = value
	fieldPattern := regexp.MustCompile(`^\s*(\w+)\s*:\s*([\w\[\],\s\.\*]*)\s*(=|$)`)

	// For dataclass: fields are often defined with type hints
	// For regular class: look for instance variables in __init__ or class body

	startIdx := typeBounds.Start - 1 - lineOffset
	endIdx := typeBounds.End - 1 - lineOffset

	if startIdx < 0 {
		startIdx = 0
	}
	if endIdx > len(lines) {
		endIdx = len(lines)
	}

	for lineNum := startIdx; lineNum < endIdx; lineNum++ {
		line := lines[lineNum]
		trimmed := strings.TrimSpace(line)

		// Skip comments and decorators
		if strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "@") {
			continue
		}

		// Skip method definitions (def keyword)
		if strings.HasPrefix(trimmed, "def ") {
			continue
		}

		// Skip special methods
		if strings.HasPrefix(trimmed, "__") && strings.HasSuffix(trimmed, "__():") {
			continue
		}

		// Look for field declarations
		if matches := fieldPattern.FindStringSubmatch(trimmed); matches != nil {
			fieldName := matches[1]
			fieldType := strings.TrimSpace(matches[2])

			// Skip if field name looks like a method
			if fieldName == "class" || fieldName == "def" {
				continue
			}

			// Skip properties and methods that might match pattern
			if strings.Contains(trimmed, "->") && strings.Contains(trimmed, "def ") {
				continue
			}

			fields = append(fields, FieldBounds{
				Name: fieldName,
				Type: fieldType,
				Line: lineNum + 1 + lineOffset,
			})
		}
	}

	return fields
}

// Pattern to detect special class types from bases
func detectPythonClassKind(bases string) string {
	basesLower := strings.ToLower(bases)
	if strings.Contains(basesLower, "namedtuple") {
		return "NamedTuple"
	}
	if strings.Contains(basesLower, "typeddict") {
		return "TypedDict"
	}
	if strings.Contains(basesLower, "enum") {
		return "enum"
	}
	if strings.Contains(basesLower, "abc") {
		return "abstract"
	}
	if strings.Contains(basesLower, "protocol") {
		return "Protocol"
	}
	return "class"
}

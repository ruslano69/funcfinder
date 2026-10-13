// python_struct_finder.go - Find Python data types and classes
// Supports: class, dataclass, NamedTuple, TypedDict, Enum, attrs
package internal

import (
	"regexp"
	"strings"
)

// PythonStructFinder finds data types in Python source code
type PythonStructFinder struct {
	config      *LanguageConfig
	sanitizer   *Sanitizer
	typeNames   map[string]bool
	mapMode     bool
	extractMode bool
}

// NewPythonStructFinder creates a new Python struct finder
func NewPythonStructFinder(config *LanguageConfig, typeNamesStr string, mapMode, extractMode bool) *PythonStructFinder {
	nameMap := make(map[string]bool)
	for _, name := range ParseFuncNames(typeNamesStr) {
		nameMap[name] = true
	}

	return &PythonStructFinder{
		config:      config,
		sanitizer:   NewSanitizer(config, false),
		typeNames:   nameMap,
		mapMode:     mapMode,
		extractMode: extractMode,
	}
}

// FindStructures finds all types in Python file
func (f *PythonStructFinder) FindStructures(filename string) (*StructFindResult, error) {
	lines, err := readFileLines(filename)
	if err != nil {
		return nil, err
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

	// One pass gives the cleaned text and the indentation of every line; the finders below read only these facts.
	facts := f.sanitizer.CleanLinesFacts(lines)

	// Find all class/type definitions
	types := f.findAllTypes(facts, lineOffset)

	// For each type, find its fields
	for i := range types {
		typeBounds := &types[i]
		fields := f.findFieldsForType(facts, typeBounds, lineOffset)
		typeBounds.Fields = fields
		result.Types = append(result.Types, *typeBounds)
	}

	return result, nil
}

// Python-specific class patterns
var (
	// Standard class: class Name: — base-class list may include keyword
	// args such as "metaclass=Foo" or string/number literals, so the
	// character class allows =, quotes and digits in addition to
	// identifiers, dots and brackets.
	classPattern = regexp.MustCompile(`^\s*class\s+(\w+)\s*(\(\s*([\w,\s\.\[\]='"]*)\s*\))?\s*:`)

	// Dataclass decorator: @dataclass, @dataclass(...), @dataclasses.dataclass
	dataclassDecoratorPattern = regexp.MustCompile(`^@(dataclasses\.)?dataclass(\(.*\))?$`)

	// attrs decorator: @attr.s, @attr.s(...)
	attrsDecoratorPattern = regexp.MustCompile(`^@attr\.s(\(.*\))?$`)

	// NamedTuple: class Name(NamedTuple):
	namedTuplePattern = regexp.MustCompile(`^\s*class\s+(\w+)\s*\(\s*NamedTuple\s*\)\s*:`)

	// TypedDict: class Name(TypedDict):
	typedDictPattern = regexp.MustCompile(`^\s*class\s+(\w+)\s*\(\s*TypedDict\s*\)\s*:`)

	// Enum: class Name(Enum):
	enumPattern = regexp.MustCompile(`^\s*class\s+(\w+)\s*\(\s*Enum\s*\)\s*:`)

	// ABC: class Name(ABC):
	abcPattern = regexp.MustCompile(`^\s*class\s+(\w+)\s*\(\s*ABC\s*\)\s*:`)

	// Protocol: class Name(Protocol):
	protocolPattern = regexp.MustCompile(`^\s*class\s+(\w+)\s*\(\s*Protocol\s*\)\s*:`)

	// Field patterns: name: type, name: type = value, name = value
	pythonFieldPattern = regexp.MustCompile(`^\s*(\w+)\s*:\s*([\w\[\],\s\.\*]*)\s*(=|$)`)
)

// pythonCodeLine reports whether a line is code: it starts outside a string and holds code text. A blank line, a comment,
// and a line of a docstring are not code.
func pythonCodeLine(fact LineFacts) bool {
	return fact.StartState == StateNormal && fact.CodeText
}

// findAllTypes finds all type definitions in Python file. Headers are matched on the cleaned text, so a class header
// written in a docstring or a comment is not a class. A decorator on the line above a class names its kind.
func (f *PythonStructFinder) findAllTypes(facts []LineFacts, lineOffset int) []TypeBounds {
	var types []TypeBounds

	// Kind named by the decorators above the next class header. A code line that is neither a decorator nor a class
	// header clears it, so a decorator above a def does not reach a class after the def.
	decoratedKind := ""

	for lineNum := range facts {
		if !pythonCodeLine(facts[lineNum]) {
			continue
		}
		trimmed := strings.TrimSpace(facts[lineNum].Cleaned)

		// A decorator names the kind of the class below it; other decorators in between keep the name
		if strings.HasPrefix(trimmed, "@") {
			switch {
			case dataclassDecoratorPattern.MatchString(trimmed):
				decoratedKind = "dataclass"
			case attrsDecoratorPattern.MatchString(trimmed):
				decoratedKind = "attrs"
			}
			continue
		}

		headerKind := decoratedKind
		decoratedKind = ""

		// Check for class definitions
		matches := classPattern.FindStringSubmatch(trimmed)
		if matches == nil {
			continue
		}
		typeName := matches[1]
		bases := matches[3] // Base classes if any

		// A dataclass or attrs decorator decides the kind; without one, the base classes do
		kind := headerKind
		if kind == "" {
			kind = "class"
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
		}

		if f.mapMode || f.typeNames[typeName] {
			endLine := f.findTypeEnd(facts, lineNum, lineOffset)
			types = append(types, TypeBounds{
				Name:   typeName,
				Kind:   kind,
				Start:  lineNum + 1 + lineOffset,
				End:    endLine,
				Fields: []FieldBounds{},
			})
		}
	}

	return types
}

// findTypeEnd finds the end line of a type definition using indentation. Only code lines end a block: the text of a
// docstring does not, whatever its indentation.
func (f *PythonStructFinder) findTypeEnd(facts []LineFacts, startLine int, lineOffset int) int {
	if startLine >= len(facts) {
		return startLine + 1 + lineOffset
	}

	// Get the indentation level of the class definition
	startIndent := facts[startLine].Indent

	// Find the first code line with indentation <= startIndent
	for i := startLine + 1; i < len(facts); i++ {
		if !pythonCodeLine(facts[i]) {
			continue // Skip empty and comment lines, and lines of a docstring
		}

		if facts[i].Indent <= startIndent {
			return i + lineOffset
		}
	}

	return len(facts) + lineOffset
}

// findFieldsForType finds all fields/members in a Python type, read from the cleaned lines: a line of a docstring is
// not a field.
func (f *PythonStructFinder) findFieldsForType(facts []LineFacts, typeBounds *TypeBounds, lineOffset int) []FieldBounds {
	var fields []FieldBounds

	// For dataclass: fields are often defined with type hints
	// For regular class: look for instance variables in __init__ or class body

	startIdx := typeBounds.Start - 1 - lineOffset
	endIdx := typeBounds.End - 1 - lineOffset

	if startIdx < 0 {
		startIdx = 0
	}
	if endIdx > len(facts) {
		endIdx = len(facts)
	}

	for lineNum := startIdx; lineNum < endIdx; lineNum++ {
		if !pythonCodeLine(facts[lineNum]) {
			continue // Skip comments, blank lines, and lines of a docstring
		}
		trimmed := strings.TrimSpace(facts[lineNum].Cleaned)

		// Skip decorators
		if strings.HasPrefix(trimmed, "@") {
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
		if matches := pythonFieldPattern.FindStringSubmatch(trimmed); matches != nil {
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

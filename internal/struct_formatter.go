// struct_formatter.go - Format struct/type finding results
package internal

import (
	"encoding/json"
	"fmt"
	"strings"
)

// FormatStructMap formats struct results in grep-style map format
func FormatStructMap(result *StructFindResult) string {
	if len(result.Types) == 0 {
		return ""
	}

	var parts []string
	for _, t := range result.Types {
		part := fmt.Sprintf("%s: %d-%d", t.Name, t.Start, t.End)
		if len(t.Fields) > 0 {
			fieldNames := make([]string, len(t.Fields))
			for i, f := range t.Fields {
				fieldNames[i] = f.Name
			}
			part += fmt.Sprintf("; fields: %s", strings.Join(fieldNames, ", "))
		} else {
			part += ";"
		}
		parts = append(parts, part)
	}

	return strings.Join(parts, " ")
}

// FormatStructTree formats struct results in tree format
func FormatStructTree(result *StructFindResult) string {
	if len(result.Types) == 0 {
		return ""
	}

	var lines []string
	for _, t := range result.Types {
		lines = append(lines, formatStructTreeLine(t, 0))
	}

	return strings.Join(lines, "\n")
}

// formatStructTreeLine formats a single type in tree format
func formatStructTreeLine(t TypeBounds, depth int) string {
	indent := strings.Repeat("│   ", depth)
	prefix := "├── "
	if depth == 0 {
		prefix = ""
	}

	line := fmt.Sprintf("%s%s%s (%d-%d) [%s]", indent, prefix, t.Name, t.Start, t.End, t.Kind)

	// Add fields
	for i, f := range t.Fields {
		fieldIndent := strings.Repeat("│   ", depth+1)
		fieldPrefix := "├── "
		if i == len(t.Fields)-1 {
			fieldPrefix = "└── "
		}
		line += fmt.Sprintf("\n%s%s%s %s: %d", fieldIndent, fieldPrefix, f.Name, f.Type, f.Line)
	}

	return line
}

// FormatStructJSON formats struct results in JSON format
func FormatStructJSON(result *StructFindResult) (string, error) {
	type JSONField struct {
		Name string `json:"name"`
		Type string `json:"type"`
		Line int    `json:"line"`
	}

	type JSONType struct {
		Name    string     `json:"name"`
		Kind    string     `json:"kind"`
		Start   int        `json:"start"`
		End     int        `json:"end"`
		Fields  []JSONField `json:"fields,omitempty"`
	}

	types := make([]JSONType, len(result.Types))
	for i, t := range result.Types {
		fields := make([]JSONField, len(t.Fields))
		for j, f := range t.Fields {
			fields[j] = JSONField{
				Name: f.Name,
				Type: f.Type,
				Line: f.Line,
			}
		}
		types[i] = JSONType{
			Name:   t.Name,
			Kind:   t.Kind,
			Start:  t.Start,
			End:    t.End,
			Fields: fields,
		}
	}

	output := struct {
		Filename string    `json:"filename"`
		Types    []JSONType `json:"types"`
	}{
		Filename: result.Filename,
		Types:    types,
	}

	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return "", err
	}

	return string(data), nil
}

// FormatStructExtract extracts the full body of each type
func FormatStructExtract(result *StructFindResult, lines []string) string {
	if len(result.Types) == 0 {
		return ""
	}

	var parts []string
	for _, t := range result.Types {
		var typeLines []string
		for i := t.Start - 1; i < t.End && i < len(lines); i++ {
			typeLines = append(typeLines, lines[i])
		}
		parts = append(parts, fmt.Sprintf("=== %s (%s) ===\n%s", t.Name, t.Kind, strings.Join(typeLines, "\n")))
	}

	return strings.Join(parts, "\n\n")
}

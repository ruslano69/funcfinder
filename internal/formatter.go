package internal

import (
	"encoding/json"
	"fmt"
	"strings"
)

// FormatGrepStyle форматирует результат в grep-style
// Пример: Handler: 45-78; Parse: 120-145;
func FormatGrepStyle(result *FindResult) string {
	var parts []string
	for _, fn := range result.Functions {
		parts = append(parts, fmt.Sprintf("%s: %d-%d", fn.Name, fn.Start, fn.End))
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, "; ") + ";"
}

// JSONOutput представляет JSON-вывод
type JSONOutput map[string]map[string]interface{}

// FormatJSON форматирует результат в JSON
// Пример: {"Handler": {"start": 45, "end": 78, "decorators": ["@decorator"]}}
func FormatJSON(result *FindResult) (string, error) {
	output := make(JSONOutput)
	for _, fn := range result.Functions {
		fnData := map[string]interface{}{
			"start": fn.Start,
			"end":   fn.End,
		}
		// Добавляем декораторы, если они есть
		if len(fn.Decorators) > 0 {
			fnData["decorators"] = fn.Decorators
		}
		output[fn.Name] = fnData
	}

	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal JSON: %w", err)
	}
	return string(data), nil
}

// FormatExtract форматирует результат с телами функций
// Пример:
// // Handler: 45-78
// func Handler(...) {
//   ...
// }
func FormatExtract(result *FindResult) string {
	var parts []string
	for _, fn := range result.Functions {
		header := fmt.Sprintf("// %s: %d-%d", fn.Name, fn.Start, fn.End)
		body := strings.Join(fn.Lines, "\n")
		parts = append(parts, header+"\n"+body)
	}
	return strings.Join(parts, "\n\n")
}

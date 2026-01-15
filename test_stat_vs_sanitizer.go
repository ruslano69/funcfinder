package main

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/ruslano69/funcfinder/internal"
)

// stat's cleanLine implementation (simplified)
func statCleanLine(line string, config *internal.LanguageConfig) string {
	// Handle line comments
	if config.LineComment != "" {
		commentIdx := strings.Index(line, config.LineComment)
		if commentIdx >= 0 {
			line = line[:commentIdx]
		}
	}

	// Remove string literals using simple regex
	for _, sc := range config.StringChars {
		pattern := regexp.QuoteMeta(sc) + `[^` + regexp.QuoteMeta(sc) + `\\]*(?:\\.[^` + regexp.QuoteMeta(sc) + `\\]*)*` + regexp.QuoteMeta(sc)
		re := regexp.MustCompile(pattern)
		line = re.ReplaceAllString(line, `""`)
	}

	return line
}

func main() {
	// C# config
	csharpConfig := &internal.LanguageConfig{
		Name:              "C#",
		LineComment:       "//",
		BlockCommentStart: "/*",
		BlockCommentEnd:   "*/",
		StringChars:       []string{"\"", "'"},
		CharDelimiters:    []string{"'"},
		EscapeChar:        "\\",
		DocStringMarkers:  []string{`@"`}, // C# verbatim strings
	}

	// Test cases
	testCases := []struct {
		name string
		line string
	}{
		{
			name: "C# verbatim string with path",
			line: `string path = @"C:\Users\Test";`,
		},
		{
			name: "C# verbatim string with escaped quotes",
			line: `string msg = @"He said ""Hello""";`,
		},
		{
			name: "C# verbatim string with backslash",
			line: `string regex = @"\d+\.\d+";`,
		},
		{
			name: "Regular string",
			line: `string msg = "Hello World";`,
		},
	}

	fmt.Println("=== Comparing stat vs enhanced_sanitizer ===\n")

	sanitizer := internal.NewSanitizer(csharpConfig, false)

	for _, tc := range testCases {
		fmt.Printf("Test: %s\n", tc.name)
		fmt.Printf("Original:  %s\n", tc.line)

		// stat's method
		statResult := statCleanLine(tc.line, csharpConfig)
		fmt.Printf("stat:      %s\n", statResult)

		// enhanced_sanitizer's method
		sanitizerResult, _ := sanitizer.CleanLine(tc.line, internal.StateNormal)
		fmt.Printf("sanitizer: %s\n", sanitizerResult)

		// Check if different
		if statResult != sanitizerResult {
			fmt.Println("❌ DIFFERENT!")
		} else {
			fmt.Println("✅ Same")
		}
		fmt.Println()
	}

	// Additional edge cases
	fmt.Println("\n=== Edge Cases ===\n")

	edgeCases := []struct {
		name   string
		line   string
		config *internal.LanguageConfig
	}{
		{
			name: "Python docstring",
			line: `"""This is a docstring with 'quotes' """ `,
			config: &internal.LanguageConfig{
				Name:             "Python",
				LineComment:      "#",
				StringChars:      []string{"\"", "'"},
				EscapeChar:       "\\",
				DocStringMarkers: []string{`"""`, `'''`},
			},
		},
		{
			name: "Go raw string with comment-like text",
			line: "query := `SELECT * FROM users // not a comment`",
			config: &internal.LanguageConfig{
				Name:           "Go",
				LineComment:    "//",
				StringChars:    []string{"\""},
				RawStringChars: []string{"`"},
				EscapeChar:     "\\",
			},
		},
		{
			name: "String with escaped quote",
			line: `msg := "He said \"Hello\""`,
			config: &internal.LanguageConfig{
				Name:        "Go",
				LineComment: "//",
				StringChars: []string{"\""},
				EscapeChar:  "\\",
			},
		},
	}

	for _, tc := range edgeCases {
		fmt.Printf("Test: %s\n", tc.name)
		fmt.Printf("Original:  %s\n", tc.line)

		// stat's method
		statResult := statCleanLine(tc.line, tc.config)
		fmt.Printf("stat:      %s\n", statResult)

		// enhanced_sanitizer's method
		sanitizer := internal.NewSanitizer(tc.config, false)
		sanitizerResult, _ := sanitizer.CleanLine(tc.line, internal.StateNormal)
		fmt.Printf("sanitizer: %s\n", sanitizerResult)

		if statResult != sanitizerResult {
			fmt.Println("❌ DIFFERENT!")
		} else {
			fmt.Println("✅ Same")
		}
		fmt.Println()
	}
}

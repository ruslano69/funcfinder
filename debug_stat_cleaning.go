package main

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/ruslano69/funcfinder/internal"
)

// stat's cleanLine (copied from cmd/stat/main.go)
func statCleanLine(line string, config *internal.LanguageConfig) (string, bool) {
	if strings.HasPrefix(line, "#!") {
		return "", true
	}

	// Handle block comments
	if config.BlockCommentRegex() != nil {
		blockRe := config.BlockCommentRegex()
		if blockRe.MatchString(line) {
			line = blockRe.ReplaceAllString(line, "")
			if strings.TrimSpace(line) == "" {
				return "", true
			}
		}
	}

	// Handle line comments
	if config.LineComment != "" {
		commentIdx := strings.Index(line, config.LineComment)
		if commentIdx >= 0 {
			before := line[:commentIdx]
			if strings.TrimSpace(before) == "" {
				return "", true
			}
			line = before
		}
	}

	// Remove string literals
	for _, sc := range config.StringChars {
		pattern := regexp.QuoteMeta(sc) + `[^` + regexp.QuoteMeta(sc) + `\\]*(?:\\.[^` + regexp.QuoteMeta(sc) + `\\]*)*` + regexp.QuoteMeta(sc)
		re := regexp.MustCompile(pattern)
		line = re.ReplaceAllString(line, `""`)
	}

	return line, false
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: debug_stat_cleaning <file.cs>")
		os.Exit(1)
	}

	filename := os.Args[1]

	// Load config
	config, err := internal.LoadConfig()
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		os.Exit(1)
	}

	langConfig := config.GetLanguageByExtension(filename)
	if langConfig == nil {
		fmt.Println("Cannot detect language")
		os.Exit(1)
	}

	fmt.Printf("Language: %s\n", langConfig.Name)
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println()

	// Create sanitizer
	sanitizer := internal.NewSanitizer(langConfig, false)
	sanitizerState := internal.StateNormal

	file, err := os.Open(filename)
	if err != nil {
		fmt.Printf("Error opening file: %v\n", err)
		os.Exit(1)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		// Skip empty lines
		if strings.TrimSpace(line) == "" {
			continue
		}

		// Process with stat
		statResult, _ := statCleanLine(line, langConfig)

		// Process with sanitizer
		sanitizerResult, newState := sanitizer.CleanLine(line, sanitizerState)
		sanitizerState = newState

		// Compare
		different := statResult != sanitizerResult

		if different || strings.Contains(line, "@\"") || strings.Contains(line, "\"\"\"") {
			fmt.Printf("Line %d:\n", lineNum)
			fmt.Printf("  Original:   %s\n", line)
			fmt.Printf("  stat:       %s\n", statResult)
			fmt.Printf("  sanitizer:  %s\n", sanitizerResult)

			if different {
				fmt.Printf("  Status:     ❌ DIFFERENT\n")

				// Analyze the difference
				if len(statResult) < len(sanitizerResult) && strings.Contains(statResult, "@") {
					fmt.Printf("  Issue:      stat left '@' prefix (C# verbatim not handled)\n")
				} else if strings.Contains(statResult, "\"\"") {
					fmt.Printf("  Issue:      stat replaced with \"\" instead of spaces\n")
				}
			} else {
				fmt.Printf("  Status:     ✅ Same\n")
			}
			fmt.Println()
		}
	}
}

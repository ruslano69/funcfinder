package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
)

//go:embed languages.json
var languagesFS embed.FS

// LanguageConfig содержит паттерны и настройки для конкретного языка
// Объединенная структура для всех утилит: funcfinder, stat, deps
type LanguageConfig struct {
	// Metadata
	Name       string   `json:"name"`       // Human-readable name (e.g., "Go", "Python")
	Extensions []string `json:"extensions"` // File extensions (e.g., [".go", ".mod"])

	// funcfinder patterns
	FuncPattern  string `json:"func_pattern"`  // Pattern to find function declarations
	ClassPattern string `json:"class_pattern"` // Pattern to find class declarations

	// stat patterns
	CallPattern      string   `json:"call_pattern"`      // Pattern to find function calls
	ImportPattern    string   `json:"import_pattern"`    // Pattern to find imports
	DecoratorPattern string   `json:"decorator_pattern"` // Pattern for decorators (Python, Java, C#)
	CommentPatterns  []string `json:"comment_patterns"`  // Patterns for line comments
	ExcludeWords     []string `json:"exclude_words"`     // Words to exclude from function call counting

	// deps patterns
	ExcludePatterns []string `json:"exclude_patterns"` // Import patterns to exclude (e.g., relative imports)
	MultiLineImport string   `json:"multi_line_import"` // Multi-line import block start (e.g., "import (" in Go)

	// Comment and string handling
	LineComment       string   `json:"line_comment"`
	BlockCommentStart string   `json:"block_comment_start"`
	BlockCommentEnd   string   `json:"block_comment_end"`
	StringChars       []string `json:"string_chars"`
	RawStringChars    []string `json:"raw_string_chars"`
	EscapeChar        string   `json:"escape_char"`
	IndentBased       bool     `json:"indent_based"` // true для языков с отступами (Python)

	// Compiled regex (populated during loading)
	funcRegex      *regexp.Regexp
	classRegex     *regexp.Regexp
	callRegex      *regexp.Regexp
	importRegex    *regexp.Regexp
	decoratorRegex *regexp.Regexp
}

// Config - карта языков и их конфигураций
type Config map[string]*LanguageConfig

// LoadConfig загружает конфигурацию языков из встроенного JSON
func LoadConfig() (Config, error) {
	data, err := languagesFS.ReadFile("languages.json")
	if err != nil {
		return nil, fmt.Errorf("failed to read languages.json: %w", err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse languages.json: %w", err)
	}

	// Компилируем регулярные выражения
	for lang, langConf := range config {
		// Function pattern (required)
		if langConf.FuncPattern != "" {
			re, err := regexp.Compile(langConf.FuncPattern)
			if err != nil {
				return nil, fmt.Errorf("invalid func_pattern for %s: %w", lang, err)
			}
			langConf.funcRegex = re
		}

		// Class pattern (optional)
		if langConf.ClassPattern != "" {
			classRe, err := regexp.Compile(langConf.ClassPattern)
			if err != nil {
				return nil, fmt.Errorf("invalid class_pattern for %s: %w", lang, err)
			}
			langConf.classRegex = classRe
		}

		// Call pattern (for stat)
		if langConf.CallPattern != "" {
			callRe, err := regexp.Compile(langConf.CallPattern)
			if err != nil {
				return nil, fmt.Errorf("invalid call_pattern for %s: %w", lang, err)
			}
			langConf.callRegex = callRe
		}

		// Import pattern (for stat and deps)
		if langConf.ImportPattern != "" {
			importRe, err := regexp.Compile(langConf.ImportPattern)
			if err != nil {
				return nil, fmt.Errorf("invalid import_pattern for %s: %w", lang, err)
			}
			langConf.importRegex = importRe
		}

		// Decorator pattern (for stat - Python, Java, C#)
		if langConf.DecoratorPattern != "" {
			decorRe, err := regexp.Compile(langConf.DecoratorPattern)
			if err != nil {
				return nil, fmt.Errorf("invalid decorator_pattern for %s: %w", lang, err)
			}
			langConf.decoratorRegex = decorRe
		}
	}

	return config, nil
}

// GetLanguageConfig возвращает конфигурацию языка по его ключу
func GetLanguageConfig(config Config, lang string) (*LanguageConfig, error) {
	langConf, ok := config[lang]
	if !ok {
		return nil, fmt.Errorf("unsupported language: %s", lang)
	}
	return langConf, nil
}

// GetLanguageByExtension определяет язык по расширению файла
func GetLanguageByExtension(config Config, filename string) (*LanguageConfig, string, error) {
	ext := filepath.Ext(filename)
	for langKey, langConf := range config {
		for _, e := range langConf.Extensions {
			if ext == e {
				return langConf, langKey, nil
			}
		}
	}
	return nil, "", fmt.Errorf("unsupported file extension: %s", ext)
}

// FuncRegex возвращает скомпилированный regex для функций
func (c *LanguageConfig) FuncRegex() *regexp.Regexp {
	return c.funcRegex
}

// ClassRegex возвращает скомпилированный regex для классов
func (c *LanguageConfig) ClassRegex() *regexp.Regexp {
	return c.classRegex
}

// CallRegex возвращает скомпилированный regex для вызовов функций
func (c *LanguageConfig) CallRegex() *regexp.Regexp {
	return c.callRegex
}

// ImportRegex возвращает скомпилированный regex для импортов
func (c *LanguageConfig) ImportRegex() *regexp.Regexp {
	return c.importRegex
}

// DecoratorRegex возвращает скомпилированный regex для декораторов
func (c *LanguageConfig) DecoratorRegex() *regexp.Regexp {
	return c.decoratorRegex
}

// HasClasses проверяет, поддерживает ли язык классы
func (c *LanguageConfig) HasClasses() bool {
	return c.ClassPattern != ""
}

// GetSupportedLanguages возвращает список поддерживаемых языков
func GetSupportedLanguages(config Config) []string {
	langs := make([]string, 0, len(config))
	for lang := range config {
		langs = append(langs, lang)
	}
	return langs
}

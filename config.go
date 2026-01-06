package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"regexp"
)

//go:embed languages.json
var languagesFS embed.FS

// LanguageConfig содержит паттерны и настройки для конкретного языка
type LanguageConfig struct {
	FuncPattern       string   `json:"func_pattern"`
	ClassPattern      string   `json:"class_pattern"`      // паттерн для поиска классов
	LineComment       string   `json:"line_comment"`
	BlockCommentStart string   `json:"block_comment_start"`
	BlockCommentEnd   string   `json:"block_comment_end"`
	StringChars       []string `json:"string_chars"`
	RawStringChars    []string `json:"raw_string_chars"`
	EscapeChar        string   `json:"escape_char"`
	IndentBased       bool     `json:"indent_based"` // true для языков с отступами (Python)

	// Компилированные regex (заполняется при загрузке)
	funcRegex  *regexp.Regexp
	classRegex *regexp.Regexp
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
		re, err := regexp.Compile(langConf.FuncPattern)
		if err != nil {
			return nil, fmt.Errorf("invalid regex for %s: %w", lang, err)
		}
		langConf.funcRegex = re

		// Компилируем regex для классов, если указан
		if langConf.ClassPattern != "" {
			classRe, err := regexp.Compile(langConf.ClassPattern)
			if err != nil {
				return nil, fmt.Errorf("invalid class regex for %s: %w", lang, err)
			}
			langConf.classRegex = classRe
		}
	}
	
	return config, nil
}

// GetLanguageConfig возвращает конфигурацию для указанного языка
func (c Config) GetLanguageConfig(lang string) (*LanguageConfig, error) {
	conf, ok := c[lang]
	if !ok {
		return nil, fmt.Errorf("unsupported language: %s", lang)
	}
	return conf, nil
}

// FuncRegex возвращает компилированное регулярное выражение для поиска функций
func (lc *LanguageConfig) FuncRegex() *regexp.Regexp {
	return lc.funcRegex
}
// ClassRegex возвращает компилированное регулярное выражение для поиска классов
func (lc *LanguageConfig) ClassRegex() *regexp.Regexp {
	return lc.classRegex
}

// HasClasses возвращает true, если язык поддерживает классы
func (lc *LanguageConfig) HasClasses() bool {
	return lc.ClassPattern != ""
}

// GetSupportedLanguages возвращает отсортированный список всех поддерживаемых языков
func (c Config) GetSupportedLanguages() []string {
	languages := make([]string, 0, len(c))
	for lang := range c {
		languages = append(languages, lang)
	}

	// Сортируем для консистентного вывода
	// Простая сортировка без импорта sort
	for i := 0; i < len(languages)-1; i++ {
		for j := i + 1; j < len(languages); j++ {
			if languages[i] > languages[j] {
				languages[i], languages[j] = languages[j], languages[i]
			}
		}
	}

	return languages
}

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
	LineComment       string   `json:"line_comment"`
	BlockCommentStart string   `json:"block_comment_start"`
	BlockCommentEnd   string   `json:"block_comment_end"`
	StringChars       []string `json:"string_chars"`
	RawStringChars    []string `json:"raw_string_chars"`
	EscapeChar        string   `json:"escape_char"`
	
	// Компилированный regex (заполняется при загрузке)
	funcRegex *regexp.Regexp
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

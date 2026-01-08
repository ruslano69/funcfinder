// config.go - Unified language configuration
// Loads and manages language patterns for funcfinder, stat, deps, and complexity
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

// LanguageConfig contains patterns and settings for a specific language
type LanguageConfig struct {
	// Basic info
	Name       string   `json:"name"`
	Extensions []string `json:"extensions"`

	// Function/Class patterns (for funcfinder)
	FuncPattern  string `json:"func_pattern"`
	ClassPattern string `json:"class_pattern"`

	// Call patterns (for stat.go)
	CallPattern      string   `json:"call_pattern"`
	ExcludeWords     []string `json:"exclude_words"`
	DecoratorPattern string   `json:"decorator_pattern"`

	// Import patterns (for deps.go)
	ImportPattern    string   `json:"import_pattern"`
	MultiLineBlock   string   `json:"multi_line_block"`
	ExcludePatterns  []string `json:"exclude_patterns"`

	// Comment/String handling
	LineComment       string   `json:"line_comment"`
	BlockCommentStart string   `json:"block_comment_start"`
	BlockCommentEnd   string   `json:"block_comment_end"`
	StringChars       []string `json:"string_chars"`
	RawStringChars    []string `json:"raw_string_chars"`
	EscapeChar        string   `json:"escape_char"`
	IndentBased       bool     `json:"indent_based"`

	// Language key for stdlib detection (e.g., "py", "go", "rs")
	LangKey string `json:"lang_key"`

	// Compiled regex cache
	funcRegex     *regexp.Regexp
	classRegex    *regexp.Regexp
	callRegex     *regexp.Regexp
	importRegex   *regexp.Regexp
	decoratorRe   *regexp.Regexp
	blockCommentRe *regexp.Regexp
}

// Config is a map of language keys to their configurations
type Config map[string]*LanguageConfig

// LoadConfig loads language configurations from embedded JSON
func LoadConfig() (Config, error) {
	data, err := languagesFS.ReadFile("languages.json")
	if err != nil {
		return nil, fmt.Errorf("failed to read languages.json: %w", err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse languages.json: %w", err)
	}

	// Compile regex patterns
	for lang, langConf := range config {
		// Set LangKey if not provided
		if langConf.LangKey == "" {
			langConf.LangKey = lang
		}

		// Compile function regex
		if langConf.FuncPattern != "" {
			re, err := regexp.Compile(langConf.FuncPattern)
			if err != nil {
				return nil, fmt.Errorf("invalid func regex for %s: %w", lang, err)
			}
			langConf.funcRegex = re
		}

		// Compile class regex if specified
		if langConf.ClassPattern != "" {
			classRe, err := regexp.Compile(langConf.ClassPattern)
			if err != nil {
				return nil, fmt.Errorf("invalid class regex for %s: %w", lang, err)
			}
			langConf.classRegex = classRe
		}

		// Compile call regex if specified
		if langConf.CallPattern != "" {
			callRe, err := regexp.Compile(langConf.CallPattern)
			if err != nil {
				return nil, fmt.Errorf("invalid call regex for %s: %w", lang, err)
			}
			langConf.callRegex = callRe
		}

		// Compile import regex if specified
		if langConf.ImportPattern != "" {
			importRe, err := regexp.Compile(langConf.ImportPattern)
			if err != nil {
				return nil, fmt.Errorf("invalid import regex for %s: %w", lang, err)
			}
			langConf.importRegex = importRe
		}

		// Compile decorator regex if specified
		if langConf.DecoratorPattern != "" {
			decoratorRe, err := regexp.Compile(langConf.DecoratorPattern)
			if err != nil {
				return nil, fmt.Errorf("invalid decorator regex for %s: %w", lang, err)
			}
			langConf.decoratorRe = decoratorRe
		}

		// Compile block comment regex if specified
		if langConf.BlockCommentStart != "" && langConf.BlockCommentEnd != "" {
			// Use regexp.QuoteMeta to escape special regex characters like /* and */
			start := regexp.QuoteMeta(langConf.BlockCommentStart)
			end := regexp.QuoteMeta(langConf.BlockCommentEnd)
			pattern := fmt.Sprintf(`%s[\s\S]*?%s`, start, end)
			blockRe, err := regexp.Compile(pattern)
			if err != nil {
				return nil, fmt.Errorf("invalid block comment regex for %s: %w", lang, err)
			}
			langConf.blockCommentRe = blockRe
		}
	}

	return config, nil
}

// GetLanguageConfig returns the configuration for the specified language
func (c Config) GetLanguageConfig(lang string) (*LanguageConfig, error) {
	conf, ok := c[lang]
	if !ok {
		return nil, fmt.Errorf("unsupported language: %s", lang)
	}
	return conf, nil
}

// GetLanguageByExtension returns the configuration based on file extension
func (c Config) GetLanguageByExtension(filename string) *LanguageConfig {
	ext := filepath.Ext(filename)
	for _, langConf := range c {
		for _, e := range langConf.Extensions {
			if ext == e {
				return langConf
			}
		}
	}
	return nil
}

// GetSupportedLanguages returns a sorted list of all supported language keys
func (c Config) GetSupportedLanguages() []string {
	languages := make([]string, 0, len(c))
	for lang := range c {
		languages = append(languages, lang)
	}

	// Simple bubble sort for consistency
	for i := 0; i < len(languages)-1; i++ {
		for j := i + 1; j < len(languages); j++ {
			if languages[i] > languages[j] {
				languages[i], languages[j] = languages[j], languages[i]
			}
		}
	}

	return languages
}

// Regex getters for funcfinder
func (lc *LanguageConfig) FuncRegex() *regexp.Regexp {
	return lc.funcRegex
}

func (lc *LanguageConfig) ClassRegex() *regexp.Regexp {
	return lc.classRegex
}

func (lc *LanguageConfig) HasClasses() bool {
	return lc.ClassPattern != ""
}

// Regex getters for stat.go
func (lc *LanguageConfig) CallRegex() *regexp.Regexp {
	return lc.callRegex
}

func (lc *LanguageConfig) DecoratorRegex() *regexp.Regexp {
	return lc.decoratorRe
}

// Regex getters for deps.go
func (lc *LanguageConfig) ImportRegex() *regexp.Regexp {
	return lc.importRegex
}

func (lc *LanguageConfig) BlockCommentRegex() *regexp.Regexp {
	return lc.blockCommentRe
}

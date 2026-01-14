// config.go - Unified language configuration
// Loads and manages language patterns for funcfinder, stat, deps, complexity, and findstruct
package internal

import (
	"embed"
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
)

//go:embed languages.json
var languagesFS embed.FS

// StructTypePattern represents a pattern for a specific type kind
type StructTypePattern struct {
	Type    string `json:"type"`   // Type kind: class, struct, interface, enum, etc.
	Pattern string `json:"pattern"` // Regex pattern for this type
}

// LanguageConfig contains patterns and settings for a specific language
type LanguageConfig struct {
	// Basic info
	Name       string   `json:"name"`
	Extensions []string `json:"extensions"`

	// Function/Class patterns (for funcfinder)
	FuncPattern  string `json:"func_pattern"`
	ClassPattern string `json:"class_pattern"`

	// Struct/type patterns (for findstruct) - stored as map for flexible access
	StructTypePatterns []StructTypePattern `json:"struct_type_patterns,omitempty"`
	FieldPattern       string              `json:"field_pattern,omitempty"`

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
	CharDelimiters    []string `json:"char_delimiters,omitempty"`
	DocStringMarkers  []string `json:"doc_string_markers,omitempty"`
	IndentBased       bool     `json:"indent_based"`

	// Nested function support
	SupportsNested bool `json:"supports_nested"`

	// Language key for stdlib detection (e.g., "py", "go", "rs")
	LangKey string `json:"lang_key"`

	// Extra patterns for specialized finders (structfinder, etc.)
	ExtraPatterns map[string]string `json:"extra_patterns,omitempty"`

	// Compiled regex cache
	funcRegex       *regexp.Regexp
	classRegex      *regexp.Regexp
	structPatterns  map[string]*regexp.Regexp
	fieldRegex      *regexp.Regexp
	callRegex       *regexp.Regexp
	importRegex     *regexp.Regexp
	decoratorRe     *regexp.Regexp
	blockCommentRe  *regexp.Regexp
}

// Config is a map of language keys to their configurations
type Config map[string]*LanguageConfig

// LanguageConfigWithMap is an intermediate struct for JSON unmarshalling
// with struct_type_patterns as a map instead of slice
type LanguageConfigWithMap struct {
	LanguageConfig
	StructTypePatternsMap map[string]string `json:"struct_type_patterns,omitempty"`
}

// LoadConfig loads language configurations from embedded JSON
func LoadConfig() (Config, error) {
	data, err := languagesFS.ReadFile("languages.json")
	if err != nil {
		return nil, fmt.Errorf("failed to read languages.json: %w", err)
	}

	// First, unmarshal into a map to handle struct_type_patterns as object
	var rawConfig map[string]*LanguageConfigWithMap
	if err := json.Unmarshal(data, &rawConfig); err != nil {
		return nil, fmt.Errorf("failed to parse languages.json: %w", err)
	}

	// Convert to final Config
	config := make(Config)
	for lang, langConf := range rawConfig {
		// Convert LanguageConfigWithMap to LanguageConfig
		conf := langConf.LanguageConfig

		// Convert struct_type_patterns map to compiled regexes
		conf.structPatterns = make(map[string]*regexp.Regexp)
		for typeKind, pattern := range langConf.StructTypePatternsMap {
			re, err := regexp.Compile(pattern)
			if err != nil {
				return nil, fmt.Errorf("invalid struct pattern for %s (%s): %w", lang, typeKind, err)
			}
			conf.structPatterns[typeKind] = re
		}

		// Set LangKey if not provided
		if conf.LangKey == "" {
			conf.LangKey = lang
		}

		// Compile function regex
		if conf.FuncPattern != "" {
			re, err := regexp.Compile(conf.FuncPattern)
			if err != nil {
				return nil, fmt.Errorf("invalid func regex for %s: %w", lang, err)
			}
			conf.funcRegex = re
		}

		// Compile class regex if specified
		if conf.ClassPattern != "" {
			classRe, err := regexp.Compile(conf.ClassPattern)
			if err != nil {
				return nil, fmt.Errorf("invalid class regex for %s: %w", lang, err)
			}
			conf.classRegex = classRe
		}

		// Compile field pattern if specified
		if conf.FieldPattern != "" {
			fieldRe, err := regexp.Compile(conf.FieldPattern)
			if err != nil {
				return nil, fmt.Errorf("invalid field pattern for %s: %w", lang, err)
			}
			conf.fieldRegex = fieldRe
		}

		// Compile call regex if specified
		if conf.CallPattern != "" {
			callRe, err := regexp.Compile(conf.CallPattern)
			if err != nil {
				return nil, fmt.Errorf("invalid call regex for %s: %w", lang, err)
			}
			conf.callRegex = callRe
		}

		// Compile import regex if specified
		if conf.ImportPattern != "" {
			importRe, err := regexp.Compile(conf.ImportPattern)
			if err != nil {
				return nil, fmt.Errorf("invalid import regex for %s: %w", lang, err)
			}
			conf.importRegex = importRe
		}

		// Compile decorator regex if specified
		if conf.DecoratorPattern != "" {
			decoratorRe, err := regexp.Compile(conf.DecoratorPattern)
			if err != nil {
				return nil, fmt.Errorf("invalid decorator regex for %s: %w", lang, err)
			}
			conf.decoratorRe = decoratorRe
		}

		// Compile block comment regex if specified
		if conf.BlockCommentStart != "" && conf.BlockCommentEnd != "" {
			// Use regexp.QuoteMeta to escape special regex characters like /* and */
			start := regexp.QuoteMeta(conf.BlockCommentStart)
			end := regexp.QuoteMeta(conf.BlockCommentEnd)
			pattern := fmt.Sprintf(`%s[\s\S]*?%s`, start, end)
			blockRe, err := regexp.Compile(pattern)
			if err != nil {
				return nil, fmt.Errorf("invalid block comment regex for %s: %w", lang, err)
			}
			conf.blockCommentRe = blockRe
		}

		config[lang] = &conf
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

// GetExtraPattern returns an extra pattern by key
func (lc *LanguageConfig) GetExtraPattern(key string) string {
	if lc.ExtraPatterns != nil {
		return lc.ExtraPatterns[key]
	}
	return ""
}

func (lc *LanguageConfig) HasClasses() bool {
	return lc.ClassPattern != ""
}

// Struct pattern getters for findstruct

// GetStructPatterns returns all compiled struct type patterns
func (lc *LanguageConfig) GetStructPatterns() map[string]*regexp.Regexp {
	return lc.structPatterns
}

// GetStructPattern returns a compiled pattern for a specific struct type
func (lc *LanguageConfig) GetStructPattern(typeKind string) *regexp.Regexp {
	if lc.structPatterns != nil {
		return lc.structPatterns[typeKind]
	}
	return nil
}

// GetFieldPattern returns the compiled field pattern regex
func (lc *LanguageConfig) GetFieldPattern() *regexp.Regexp {
	return lc.fieldRegex
}

// HasStructSupport returns true if the language has struct type patterns configured
func (lc *LanguageConfig) HasStructSupport() bool {
	return len(lc.structPatterns) > 0
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

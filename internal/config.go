// config.go - Unified language configuration
// Loads and manages language patterns for funcfinder, stat, deps, complexity, and findstruct
package internal

import (
	"embed"
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
)

//go:embed languages.json
var languagesFS embed.FS

// StructTypePattern represents a pattern for a specific type kind
type StructTypePattern struct {
	Type    string `json:"type"`    // Type kind: class, struct, interface, enum, etc.
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
	CallPattern  string   `json:"call_pattern"`
	ExcludeWords []string `json:"exclude_words"`
	// ClassMethodOpen matches a class member's first line when its signature continues
	// on the next lines; it is read only at the top of a class body.
	ClassMethodOpen string `json:"class_method_open,omitempty"`
	// HeaderOpenPattern matches the first line of a signature that continues on the next lines
	// while its parenthesis is still open (`auto f(int a,`). It is read when FuncPattern does not
	// match and the line leaves a parenthesis open, at any scope (C++).
	HeaderOpenPattern string `json:"header_open_pattern,omitempty"`
	// ConstructorPattern matches a constructor or destructor header, which has no return type.
	// It is read only inside a class body, and only when the name is that class's name or ~ it (C++).
	ConstructorPattern string `json:"constructor_pattern,omitempty"`
	// DeclarationPattern matches a bodyless declaration that ends in ';' (`void f(int);`) at any
	// scope. A line that starts with a reserved word (return, throw) is a statement, not one (C++).
	DeclarationPattern string `json:"declaration_pattern,omitempty"`
	// InitializerLists: a constructor's member-initializer list (`: a{1}, b(2) {`) may hold braced
	// initializations, which are not the body. Set for C++ only.
	InitializerLists bool `json:"initializer_lists,omitempty"`
	// ConversionOperatorPattern matches a conversion operator's header (`operator bool() const`).
	// Its name is a type that the compilers spell canonically, so the finder does not name it;
	// the header is not taken as a function. C++ only.
	ConversionOperatorPattern string `json:"conversion_operator_pattern,omitempty"`
	// QuotedMethodPattern matches a method whose name is a string literal ("=== X"() {). It is
	// read from the raw line, because the sanitizer blanks string literals in the cleaned one.
	QuotedMethodPattern string `json:"quoted_method_pattern,omitempty"`
	// KeywordsNotNames: the exclude_words are reserved words of the language, so a
	// match on one of them (if (x) at the end of a line) is a statement, not a function.
	KeywordsNotNames bool `json:"keywords_not_names,omitempty"`
	// StatementWords are control words that can be followed by a parenthesis and a brace
	// (if (x) {, catch (e) {): a match on one is a statement, never a method.
	StatementWords   []string `json:"statement_words,omitempty"`
	DecoratorPattern string   `json:"decorator_pattern"`

	// Nesting patterns (for complexity.go) — NestingPattern matches lines that
	// open a new level of control-flow branching (if/for/while/switch/...);
	// FlatPattern matches a sibling continuation of the same construct
	// (else/elif/case/default/...) that should NOT add another nesting level.
	// LoopPattern is the loop-only subset of NestingPattern (for/while/
	// foreach/do/loop/repeat/until — never if/switch/case/catch), used to
	// track loop-in-loop nesting as a signal distinct from general
	// branching depth: iterating code compounds cost per iteration in a way
	// a conditional doesn't, so it's tracked and reported separately rather
	// than folded into the same score.
	NestingPattern string `json:"nesting_pattern,omitempty"`
	FlatPattern    string `json:"flat_pattern,omitempty"`
	LoopPattern    string `json:"loop_pattern,omitempty"`

	// Import patterns (for deps.go)
	ImportPattern   string   `json:"import_pattern"`
	MultiLineBlock  string   `json:"multi_line_block"`
	ExcludePatterns []string `json:"exclude_patterns"`

	// Comment/String handling
	LineComment       string   `json:"line_comment"`
	BlockCommentStart string   `json:"block_comment_start"`
	BlockCommentEnd   string   `json:"block_comment_end"`
	StringChars       []string `json:"string_chars"`
	RawStringChars    []string `json:"raw_string_chars"`
	// RegexLiterals: a slash that starts a regular expression (after an operator or
	// return) is not a division, and its quotes are not string delimiters (JavaScript).
	RegexLiterals bool `json:"regex_literals,omitempty"`
	// TemplateExpressions: a ${...} inside a template literal is code, so a string or a nested
	// template inside it does not end the template (JavaScript, TypeScript).
	TemplateExpressions bool `json:"template_expressions,omitempty"`
	// InlineHeaders: a function can start after a separator on a line that holds other code
	// (a statement end, a comma, a brace), as in `let n=0;const f=…` (JavaScript, TypeScript).
	InlineHeaders bool `json:"inline_headers,omitempty"`
	// PercentLiterals: a Ruby percent literal (%q{...}, %r{...}, %w[...]) runs to its
	// closing bracket, nested brackets included, so the quotes and slashes in it are text.
	PercentLiterals bool   `json:"percent_literals,omitempty"`
	EscapeChar      string `json:"escape_char"`
	// SingleLineStrings — a regular (non-raw) string cannot continue past the
	// end of its line (JS/TS "..." and '...'). An unterminated quote then ends
	// with the line instead of swallowing the following code — which is what a
	// stray apostrophe in JSX text (<p>Зв'язок</p>) would otherwise do.
	SingleLineStrings bool     `json:"single_line_strings,omitempty"`
	CharDelimiters    []string `json:"char_delimiters,omitempty"`
	DocStringMarkers  []string `json:"doc_string_markers,omitempty"`
	IndentBased       bool     `json:"indent_based"`
	BlockEndKeyword   string   `json:"block_end_keyword,omitempty"` // For Ruby-like languages (end keyword)

	// Nested function support
	SupportsNested bool `json:"supports_nested"`

	// Language key for stdlib detection (e.g., "py", "go", "rs")
	LangKey string `json:"lang_key"`

	// Extra patterns for specialized finders (structfinder, etc.)
	ExtraPatterns map[string]string `json:"extra_patterns,omitempty"`

	// Compiled regex cache
	funcRegex         *regexp.Regexp
	classMethodOpenRe *regexp.Regexp
	headerOpenRe      *regexp.Regexp
	constructorRe     *regexp.Regexp
	declarationRe     *regexp.Regexp
	conversionRe      *regexp.Regexp
	quotedMethodRe    *regexp.Regexp
	classRegex        *regexp.Regexp
	structPatterns    map[string]*regexp.Regexp
	fieldRegex        *regexp.Regexp
	callRegex         *regexp.Regexp
	importRegex       *regexp.Regexp
	decoratorRe       *regexp.Regexp
	blockCommentRe    *regexp.Regexp
	nestingRe         *regexp.Regexp
	flatRe            *regexp.Regexp
	loopRe            *regexp.Regexp
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
			re, err := regexp.Compile(expandIdentPlaceholder(pattern))
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
			re, err := regexp.Compile(expandIdentPlaceholder(conf.FuncPattern))
			if err != nil {
				return nil, fmt.Errorf("invalid func regex for %s: %w", lang, err)
			}
			conf.funcRegex = re
		}

		// Compile the class method opening if specified
		if conf.ClassMethodOpen != "" {
			cmRe, err := regexp.Compile(expandIdentPlaceholder(conf.ClassMethodOpen))
			if err != nil {
				return nil, fmt.Errorf("invalid class method regex for %s: %w", lang, err)
			}
			conf.classMethodOpenRe = cmRe
		}

		// Compile the C++ header patterns if specified
		for _, p := range []struct {
			src string
			dst **regexp.Regexp
		}{{conf.HeaderOpenPattern, &conf.headerOpenRe}, {conf.ConstructorPattern, &conf.constructorRe}, {conf.DeclarationPattern, &conf.declarationRe}, {conf.ConversionOperatorPattern, &conf.conversionRe}} {
			if p.src == "" {
				continue
			}
			re, err := regexp.Compile(expandIdentPlaceholder(p.src))
			if err != nil {
				return nil, fmt.Errorf("invalid header pattern for %s (%s): %w", lang, p.src, err)
			}
			*p.dst = re
		}

		// Compile the quoted method name if specified
		if conf.QuotedMethodPattern != "" {
			qmRe, err := regexp.Compile(conf.QuotedMethodPattern)
			if err != nil {
				return nil, fmt.Errorf("invalid quoted method regex for %s: %w", lang, err)
			}
			conf.quotedMethodRe = qmRe
		}

		// Compile class regex if specified
		if conf.ClassPattern != "" {
			classRe, err := regexp.Compile(expandIdentPlaceholder(conf.ClassPattern))
			if err != nil {
				return nil, fmt.Errorf("invalid class regex for %s: %w", lang, err)
			}
			conf.classRegex = classRe
		}

		// Compile field pattern if specified
		if conf.FieldPattern != "" {
			fieldRe, err := regexp.Compile(expandIdentPlaceholder(conf.FieldPattern))
			if err != nil {
				return nil, fmt.Errorf("invalid field pattern for %s: %w", lang, err)
			}
			conf.fieldRegex = fieldRe
		}

		// Compile call regex if specified
		if conf.CallPattern != "" {
			callRe, err := regexp.Compile(expandIdentPlaceholder(conf.CallPattern))
			if err != nil {
				return nil, fmt.Errorf("invalid call regex for %s: %w", lang, err)
			}
			conf.callRegex = callRe
		}

		// Compile nesting/flat regex if specified
		if conf.NestingPattern != "" {
			nestingRe, err := regexp.Compile(conf.NestingPattern)
			if err != nil {
				return nil, fmt.Errorf("invalid nesting regex for %s: %w", lang, err)
			}
			conf.nestingRe = nestingRe
		}
		if conf.FlatPattern != "" {
			flatRe, err := regexp.Compile(conf.FlatPattern)
			if err != nil {
				return nil, fmt.Errorf("invalid flat regex for %s: %w", lang, err)
			}
			conf.flatRe = flatRe
		}
		if conf.LoopPattern != "" {
			loopRe, err := regexp.Compile(conf.LoopPattern)
			if err != nil {
				return nil, fmt.Errorf("invalid loop regex for %s: %w", lang, err)
			}
			conf.loopRe = loopRe
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

	sort.Strings(languages)
	return languages
}

// Regex getters for funcfinder
func (lc *LanguageConfig) FuncRegex() *regexp.Regexp {
	return lc.funcRegex
}

// ClassMethodOpenRegex matches the first line of a class method whose signature
// spans lines (it ends at the opening parenthesis); nil if the language has none.
func (lc *LanguageConfig) ClassMethodOpenRegex() *regexp.Regexp {
	return lc.classMethodOpenRe
}

// HeaderOpenRegex matches the first line of a signature that continues on the next lines; nil if
// the language has none.
func (lc *LanguageConfig) HeaderOpenRegex() *regexp.Regexp {
	return lc.headerOpenRe
}

// ConstructorRegex matches a constructor or destructor header inside a class body; nil if the
// language has none.
func (lc *LanguageConfig) ConstructorRegex() *regexp.Regexp {
	return lc.constructorRe
}

// DeclarationRegex matches a bodyless declaration; nil if the language has none.
func (lc *LanguageConfig) DeclarationRegex() *regexp.Regexp {
	return lc.declarationRe
}

// ConversionOperatorRegex matches a conversion operator's header; nil if the language has none.
func (lc *LanguageConfig) ConversionOperatorRegex() *regexp.Regexp {
	return lc.conversionRe
}

// QuotedMethodRegex matches a method named by a string literal, on the raw line; nil if the
// language has none.
func (lc *LanguageConfig) QuotedMethodRegex() *regexp.Regexp {
	return lc.quotedMethodRe
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

// Regex getters for complexity.go
func (lc *LanguageConfig) NestingRegex() *regexp.Regexp {
	return lc.nestingRe
}

func (lc *LanguageConfig) FlatRegex() *regexp.Regexp {
	return lc.flatRe
}

func (lc *LanguageConfig) LoopRegex() *regexp.Regexp {
	return lc.loopRe
}

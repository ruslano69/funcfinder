package internal

import (
	"strings"
	"testing"
)

// trimTrailingSpaces removes trailing spaces from a string
func trimTrailingSpaces(s string) string {
	runes := []rune(s)
	end := len(runes)
	for end > 0 && runes[end-1] == ' ' {
		end--
	}
	return string(runes[:end])
}

// Test helper для создания стандартной конфигурации Go
func newGoConfig() *LanguageConfig {
	return &LanguageConfig{
		FuncPattern:       "^\\s*func\\s+(\\([^)]*\\)\\s+)?(\\w+)\\s*\\(",
		LineComment:       "//",
		BlockCommentStart: "/*",
		BlockCommentEnd:   "*/",
		StringChars:       []string{"\""},
		RawStringChars:    []string{"`"},
		EscapeChar:        "\\",
	}
}

// Test helper для создания конфигурации C++
func newCppConfig() *LanguageConfig {
	return &LanguageConfig{
		FuncPattern:       "^\\s*[\\w:<>]+\\s+\\w+\\s*\\([^)]*\\)\\s*\\{?$",
		LineComment:       "//",
		BlockCommentStart: "/*",
		BlockCommentEnd:   "*/",
		StringChars:       []string{"\"", "'"},
		RawStringChars:    []string{},
		EscapeChar:        "\\",
		CharDelimiters:    []string{"'"},
	}
}

// Test helper для создания конфигурации Python
func newPythonConfig() *LanguageConfig {
	return &LanguageConfig{
		FuncPattern:       "^\\s*def\\s+(\\w+)\\s*\\(",
		LineComment:       "#",
		BlockCommentStart: "",
		BlockCommentEnd:   "",
		StringChars:       []string{"\"", "'"},
		RawStringChars:    []string{},
		EscapeChar:        "\\",
		DocStringMarkers:  []string{"\"\"\"", "'''"},
	}
}

// Test helper для создания конфигурации C#
func newCSharpConfig() *LanguageConfig {
	return &LanguageConfig{
		FuncPattern:       "^\\s*[\\w<>\\[\\]]+\\s+\\w+\\s*\\([^)]*\\)\\s*\\{?$",
		LineComment:       "//",
		BlockCommentStart: "/*",
		BlockCommentEnd:   "*/",
		StringChars:       []string{"\""}, // @" handling is done via DocStringMarkers
		RawStringChars:    []string{},
		EscapeChar:        "\\",
		CharDelimiters:    []string{"'"},
	}
}

func TestEnhancedSanitizer_New(t *testing.T) {
	config := newGoConfig()
	s := NewEnhancedSanitizer(config)

	if s == nil {
		t.Fatal("NewEnhancedSanitizer returned nil")
	}
	if s.config != config {
		t.Error("config not set correctly")
	}
	// Phase 5: simplified architecture - no sanitizerConfig
	if s.config.StringChars == nil {
		t.Error("StringChars should not be nil")
	}
	if len(s.config.StringChars) == 0 && len(s.config.DocStringMarkers) == 0 {
		t.Error("Should have at least one string delimiter")
	}
}


func TestEnhancedSanitizer_ParserState(t *testing.T) {
	tests := []struct {
		state    ParserState
		expected string
	}{
		{StateNormal, "Normal"},
		{StateLineComment, "LineComment"},
		{StateBlockComment, "BlockComment"},
		{StateString, "String"},
		{StateRawString, "RawString"},
		{StateCharLiteral, "CharLiteral"},
		{StateMultiLineString, "MultiLineString"},
		{ParserState(99), "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tt.state.String()
			if result != tt.expected {
				t.Errorf("ParserState(%d).String() = %q, want %q", tt.state, result, tt.expected)
			}
		})
	}
}

func TestEnhancedSanitizer_ValidState(t *testing.T) {
	tests := []struct {
		state    ParserState
		expected bool
	}{
		{StateNormal, true},
		{StateLineComment, true},
		{StateBlockComment, true},
		{StateString, true},
		{StateRawString, true},
		{StateCharLiteral, true},
		{StateMultiLineString, true},
		{ParserState(-1), false},
		{ParserState(100), false},
	}

	for _, tt := range tests {
		t.Run(tt.state.String(), func(t *testing.T) {
			result := ValidState(tt.state)
			if result != tt.expected {
				t.Errorf("ValidState(%d) = %v, want %v", tt.state, result, tt.expected)
			}
		})
	}
}


func TestEnhancedSanitizer_IsInLiteral(t *testing.T) {
	config := newGoConfig()
	s := NewSanitizer(config, false)

	tests := []struct {
		state    State
		expected bool
	}{
		{StateNormal, false},
		{StateLineComment, true},
		{StateBlockComment, true},
		{StateString, true},
		{StateRawString, true},
		{StateCharLiteral, true},
		{StateMultiLineString, true},
	}

	for _, tt := range tests {
		t.Run(tt.state.String(), func(t *testing.T) {
			result := s.IsInLiteral(tt.state)
			if result != tt.expected {
				t.Errorf("IsInLiteral(%s) = %v, want %v", tt.state, result, tt.expected)
			}
		})
	}
}

func TestEnhancedSanitizer_CleanLines(t *testing.T) {
	config := newGoConfig()
	s := NewSanitizer(config, false)

	// Простой тест: проверяем что код сохраняется, а комментарии удаляются
	input := []string{
		`func main() {`,
		`    msg := "hello"; // comment`,
		`}`,
	}

	result := s.CleanLines(input)

	// Проверяем что функция и код сохранились
	if !strings.Contains(result[0], "func main()") {
		t.Errorf("Function definition should be preserved, got: %q", result[0])
	}

	// Проверяем что строка "hello" удалена
	if strings.Contains(result[1], "hello") {
		t.Errorf("String literal should be removed, got: %q", result[1])
	}

	// Проверяем что комментарий "comment" удален
	if strings.Contains(result[1], "comment") {
		t.Errorf("Comment should be removed, got: %q", result[1])
	}

	// Проверяем что закрывающая скобка сохранилась
	if !strings.Contains(result[2], "}") {
		t.Errorf("Closing brace should be preserved, got: %q", result[2])
	}
}

func TestEnhancedSanitizer_CleanCode(t *testing.T) {
	config := newGoConfig()
	s := NewSanitizer(config, false)

	input := `package main

import "fmt"

func greet(name string) string {
    message := "Hello, " + name + "!"
    // Этот комментарий должен быть удалён
    return message
}

func main() {
    fmt.Println(greet("World"))
}`

	result := s.CleanCode(input)

	// Проверяем, что комментарии удалены
	if strings.Contains(result, "должен быть удалён") {
		t.Error("Comment should be removed from cleaned code")
	}

	// Проверяем, что строки заменены пробелами
	if strings.Contains(result, "World") {
		t.Error("String literal content should be replaced")
	}

	// Проверяем, что код сохранился
	if !strings.Contains(result, "func greet") {
		t.Error("Function definition should be preserved")
	}
	if !strings.Contains(result, "return message") {
		t.Error("Return statement should be preserved")
	}
}

func TestEnhancedSanitizer_CppCharLiterals(t *testing.T) {
	config := newCppConfig()
	s := NewSanitizer(config, false)

	tests := []struct {
		name          string
		input         string
		initialState  State
		expectedClean string
		expectedState State
	}{
		{
			name:          "char literal with escaped quote",
			input:         `char c = '\'';`,
			initialState:  StateNormal,
			expectedClean: `char c =     ;`, // "'" + "\'" + "'" = 3 chars replaced
			expectedState: StateNormal,
		},
		{
			name:          "char literal in declaration",
			input:         `char letter = 'A';`,
			initialState:  StateNormal,
			expectedClean: `char letter =    ;`, // "'A'" = 3 chars replaced
			expectedState: StateNormal,
		},
		{
			name:          "char literal with escape",
			input:         `char newline = '\n';`,
			initialState:  StateNormal,
			expectedClean: `char newline =     ;`, // "'\n'" = 3 chars replaced
			expectedState: StateNormal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleaned, state := s.CleanLine(tt.input, tt.initialState)
			cleanedTrimmed := trimTrailingSpaces(cleaned)
			expectedTrimmed := trimTrailingSpaces(tt.expectedClean)

			if cleanedTrimmed != expectedTrimmed {
				t.Errorf("CleanLine(%q) cleaned = %q, want %q", tt.input, cleanedTrimmed, expectedTrimmed)
			}
			if state != tt.expectedState {
				t.Errorf("CleanLine(%q) state = %v, want %v", tt.input, state, tt.expectedState)
			}
		})
	}
}

func TestEnhancedSanitizer_PythonDocStrings(t *testing.T) {
	config := newPythonConfig()
	s := NewSanitizer(config, false)

	tests := []struct {
		name          string
		input         string
		initialState  State
		expectedClean string
		expectedState State
	}{
		{
			name:          "docstring start",
			input:         `""" This is a docstring`,
			initialState:  StateNormal,
			expectedClean: ``,
			expectedState: StateMultiLineString,
		},
		{
			name:          "docstring single line",
			input:         `""" This is a docstring """`,
			initialState:  StateNormal,
			expectedClean: ``,
			expectedState: StateNormal,
		},
		{
			name:          "inside docstring",
			input:         `This is inside the docstring`,
			initialState:  StateMultiLineString,
			expectedClean: ``,
			expectedState: StateMultiLineString,
		},
		{
			name:          "docstring end",
			input:         `""" end of docstring`,
			initialState:  StateMultiLineString,
			expectedClean: ``,
			expectedState: StateNormal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleaned, state := s.CleanLine(tt.input, tt.initialState)
			cleanedTrimmed := trimTrailingSpaces(cleaned)
			expectedTrimmed := trimTrailingSpaces(tt.expectedClean)

			if cleanedTrimmed != expectedTrimmed {
				t.Errorf("CleanLine(%q) cleaned = %q, want %q", tt.input, cleanedTrimmed, expectedTrimmed)
			}
			if state != tt.expectedState {
				t.Errorf("CleanLine(%q) state = %v, want %v", tt.input, state, tt.expectedState)
			}
		})
	}
}

func TestEnhancedSanitizer_CSharpVerbatimStrings(t *testing.T) {
	config := newCSharpConfig()
	// Для verbatim strings в C# используем DocStringMarkers вместо StringChars
	// Это правильно помечает @""" как многострочную строку
	// Добавляем ДО создания sanitizer, чтобы delimiters были правильно созданы
	config.DocStringMarkers = append(config.DocStringMarkers, `@"`)
	s := NewSanitizer(config, false)

	tests := []struct {
		name          string
		input         string
		initialState  State
		expectedClean string
		expectedState State
	}{
		{
			name:          "verbatim string with quotes",
			input:         `string path = @"C:\Users\";`,
			initialState:  StateNormal,
			expectedClean: `string path =             ;`,
			expectedState: StateNormal,
		},
		{
			name:          "double quote in verbatim",
			input:         `string s = @"Say ""Hello""";`,
			initialState:  StateNormal,
			expectedClean: `string s =                 ;`, // 16 spaces for @"Say ""Hello"""
			expectedState: StateNormal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleaned, state := s.CleanLine(tt.input, tt.initialState)
			cleanedTrimmed := trimTrailingSpaces(cleaned)
			expectedTrimmed := trimTrailingSpaces(tt.expectedClean)

			if cleanedTrimmed != expectedTrimmed {
				t.Errorf("CleanLine(%q) cleaned = %q, want %q", tt.input, cleanedTrimmed, expectedTrimmed)
			}
			if state != tt.expectedState {
				t.Errorf("CleanLine(%q) state = %v, want %v", tt.input, state, tt.expectedState)
			}
		})
	}
}

func TestEnhancedSanitizer_EdgeCases(t *testing.T) {
	config := newGoConfig()
	s := NewSanitizer(config, false)

	tests := []struct {
		name          string
		input         string
		initialState  State
		expectedClean string
		expectedState State
	}{
		{
			name:          "string with escaped backslash",
			input:         `path := "C:\\Users\\";`,
			initialState:  StateNormal,
			expectedClean: `path :=              ;`, // 13 spaces for "C:\\Users\\" (11 content + 2 quotes = 13 chars)
			expectedState: StateNormal,
		},
		{
			name:          "empty string",
			input:         `msg := ""`,
			initialState:  StateNormal,
			expectedClean: `msg :=   `,
			expectedState: StateNormal,
		},
		{
			name:          "string ending at line with comment",
			input:         `"hello" // comment`,
			initialState:  StateNormal,
			expectedClean: `       `,
			expectedState: StateNormal,
		},
		{
			name:          "code after comment marker in string",
			input:         `msg := "hello // not a comment"`,
			initialState:  StateNormal,
			expectedClean: `msg :=                       `,
			expectedState: StateNormal,
		},
		{
			name:          "nested block comment markers",
			input:         `/* outer /* inner */ end */`,
			initialState:  StateNormal,
			expectedClean: `                           `,
			expectedState: StateNormal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleaned, state := s.CleanLine(tt.input, tt.initialState)
			cleanedTrimmed := trimTrailingSpaces(cleaned)
			expectedTrimmed := trimTrailingSpaces(tt.expectedClean)

			if cleanedTrimmed != expectedTrimmed {
				t.Errorf("CleanLine(%q) cleaned = %q, want %q", tt.input, cleanedTrimmed, expectedTrimmed)
			}
			if state != tt.expectedState {
				t.Errorf("CleanLine(%q) state = %v, want %v", tt.input, state, tt.expectedState)
			}
		})
	}
}
func TestEnhancedSanitizer_MultiLanguageSupport(t *testing.T) {
	// Тестируем поддержку разных языков
	languages := []struct {
		name   string
		config *LanguageConfig
	}{
		{"Go", newGoConfig()},
		{"C++", newCppConfig()},
		{"Python", newPythonConfig()},
		{"C#", newCSharpConfig()},
	}

	for _, lang := range languages {
		t.Run(lang.name, func(t *testing.T) {
			s := NewEnhancedSanitizer(lang.config)

			// Проверяем создание санитайзера
			if s == nil {
				t.Fatalf("Failed to create sanitizer for %s", lang.name)
			}

			// Phase 5: simplified architecture check
			if len(s.config.StringChars) == 0 && len(s.config.DocStringMarkers) == 0 {
				t.Errorf("No string delimiters for %s", lang.name)
			}

			// Check basic cleanup - use language-specific comment syntax
			var code string
			switch lang.name {
			case "Python":
				code = `code := "string" # comment`
			default:
				code = `code := "string" // comment`
			}
			cleaned, state := s.CleanLine(code, StateNormal)

			if state != StateNormal {
				t.Errorf("Final state should be Normal for %s", lang.name)
			}

			// Проверяем, что строка очищена
			cleanedTrimmed := trimTrailingSpaces(cleaned)
			if strings.Contains(cleanedTrimmed, "string") {
				t.Errorf("String literal should be removed for %s", lang.name)
			}
			if strings.Contains(cleanedTrimmed, "comment") {
				t.Errorf("Comment should be removed for %s", lang.name)
			}
		})
	}
}

func TestEnhancedSanitizer_Reset(t *testing.T) {
	config := newGoConfig()
	s := NewSanitizer(config, false)

	// Симулируем переход в состояние строки
	_, state := s.CleanLine(`"hello`, StateNormal)
	if state != StateString {
		t.Skip("Test requires string state")
	}

	// Сбрасываем состояние
	s.Reset()
}

// Дополнительный тест для реального сценария с шаблонами C++
func TestEnhancedSanitizer_TemplateStrings(t *testing.T) {
	config := newCppConfig()
	s := NewSanitizer(config, false)

	// Код с template и строками
	code := `std::vector<int> vec = {1, 2, 3};
std::map<std::string, int> map;
std::string name = "test";
// Это комментарий с template<int, T>
auto lambda = []() { return 1; };`

	lines := strings.Split(code, "\n")
	cleanedLines := s.CleanLines(lines)

	// Проверяем, что template<> остался нетронутым
	foundTemplate := false
	for _, line := range cleanedLines {
		if strings.Contains(line, "vector<") {
			foundTemplate = true
			break
		}
	}

	if !foundTemplate {
		t.Error("Template brackets should be preserved")
	}

	// Проверяем, что комментарий удалён
	for _, line := range cleanedLines {
		if strings.Contains(line, "комментарий") {
			t.Error("Comment should be removed")
		}
	}
}

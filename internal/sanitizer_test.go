package internal

import (
	"testing"
)

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

func TestCountBraces(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{
			name:     "empty string",
			input:    "",
			expected: 0,
		},
		{
			name:     "no braces",
			input:    "func main()",
			expected: 0,
		},
		{
			name:     "single opening brace",
			input:    "func main() {",
			expected: 1,
		},
		{
			name:     "single closing brace",
			input:    "}",
			expected: -1,
		},
		{
			name:     "balanced braces",
			input:    "{ }",
			expected: 0,
		},
		{
			name:     "nested braces",
			input:    "{ { } }",
			expected: 0,
		},
		{
			name:     "multiple opening braces",
			input:    "if true { for { {",
			expected: 3,
		},
		{
			name:     "multiple closing braces",
			input:    "} } }",
			expected: -3,
		},
		{
			name:     "mixed braces",
			input:    "{ { } { }",
			expected: 1,
		},
		{
			name:     "braces in text",
			input:    "map[string]int{\"key\": 1}",
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CountBraces(tt.input)
			if result != tt.expected {
				t.Errorf("CountBraces(%q) = %d, want %d", tt.input, result, tt.expected)
			}
		})
	}
}

func TestNewSanitizer(t *testing.T) {
	config := newGoConfig()

	tests := []struct {
		name   string
		useRaw bool
	}{
		{
			name:   "with raw strings processing",
			useRaw: true,
		},
		{
			name:   "without raw strings processing",
			useRaw: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewSanitizer(config, tt.useRaw)
			if s == nil {
				t.Fatal("NewSanitizer returned nil")
			}
			if s.config != config {
				t.Error("config not set correctly")
			}
			if s.useRaw != tt.useRaw {
				t.Errorf("useRaw = %v, want %v", s.useRaw, tt.useRaw)
			}
		})
	}
}

func TestCleanLine_LineComments(t *testing.T) {
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
			name:          "line without comment",
			input:         "func main() {",
			initialState:  StateNormal,
			expectedClean: "func main() {",
			expectedState: StateNormal,
		},
		{
			name:          "line with comment at end",
			input:         "func main() { // comment",
			initialState:  StateNormal,
			expectedClean: "func main() { ",
			expectedState: StateNormal,
		},
		{
			name:          "line with only comment",
			input:         "// this is a comment",
			initialState:  StateNormal,
			expectedClean: "",
			expectedState: StateNormal,
		},
		{
			name:          "line with comment in middle",
			input:         "x := 5 // assign value",
			initialState:  StateNormal,
			expectedClean: "x := 5 ",
			expectedState: StateNormal,
		},
		{
			name:          "empty line",
			input:         "",
			initialState:  StateNormal,
			expectedClean: "",
			expectedState: StateNormal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleaned, state := s.CleanLine(tt.input, tt.initialState)
			// Trim spaces для сравнения
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

func TestCleanLine_BlockComments(t *testing.T) {
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
			name:          "single line block comment",
			input:         "func main() { /* comment */ }",
			initialState:  StateNormal,
			expectedClean: "func main() {               }",
			expectedState: StateNormal,
		},
		{
			name:          "block comment start",
			input:         "func main() { /* comment",
			initialState:  StateNormal,
			expectedClean: "func main() { ",
			expectedState: StateBlockComment,
		},
		{
			name:          "inside block comment",
			input:         "this is inside comment",
			initialState:  StateBlockComment,
			expectedClean: "",
			expectedState: StateBlockComment,
		},
		{
			name:          "block comment end",
			input:         "end of comment */ }",
			initialState:  StateBlockComment,
			expectedClean: "                  }",
			expectedState: StateNormal,
		},
		{
			name:          "nested block comment markers",
			input:         "/* /* nested */ */",
			initialState:  StateNormal,
			expectedClean: "                */",
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

func TestCleanLine_Strings(t *testing.T) {
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
			name:          "simple string",
			input:         `msg := "hello"`,
			initialState:  StateNormal,
			expectedClean: "msg :=        ",
			expectedState: StateNormal,
		},
		{
			name:          "string with braces",
			input:         `msg := "{ } braces"`,
			initialState:  StateNormal,
			expectedClean: "msg :=             ",
			expectedState: StateNormal,
		},
		{
			name:          "string with escaped quote",
			input:         `msg := "say \"hello\""`,
			initialState:  StateNormal,
			expectedClean: "msg :=                 ",
			expectedState: StateNormal,
		},
		{
			name:          "string spanning multiple lines - start",
			input:         `msg := "start of`,
			initialState:  StateNormal,
			expectedClean: "msg :=         ",
			expectedState: StateString,
		},
		{
			name:          "string spanning multiple lines - end",
			input:         `long string"`,
			initialState:  StateString,
			expectedClean: "            ",
			expectedState: StateNormal,
		},
		{
			name:          "empty string",
			input:         `msg := ""`,
			initialState:  StateNormal,
			expectedClean: "msg :=   ",
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

func TestCleanLine_RawStrings(t *testing.T) {
	config := newGoConfig()

	tests := []struct {
		name          string
		useRaw        bool
		input         string
		initialState  State
		expectedClean string
		expectedState State
	}{
		{
			name:          "raw string without useRaw flag",
			useRaw:        false,
			input:         "msg := `raw string`",
			initialState:  StateNormal,
			expectedClean: "msg :=             ",
			expectedState: StateNormal,
		},
		{
			name:          "raw string with useRaw flag",
			useRaw:        true,
			input:         "msg := `raw string`",
			initialState:  StateNormal,
			expectedClean: "msg := `raw string`",
			expectedState: StateNormal,
		},
		{
			name:          "raw string with braces without useRaw",
			useRaw:        false,
			input:         "msg := `{ }`",
			initialState:  StateNormal,
			expectedClean: "msg :=       ",
			expectedState: StateNormal,
		},
		{
			name:          "raw string with braces with useRaw",
			useRaw:        true,
			input:         "msg := `{ }`",
			initialState:  StateNormal,
			expectedClean: "msg := `{ }`",
			expectedState: StateNormal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewSanitizer(config, tt.useRaw)
			cleaned, state := s.CleanLine(tt.input, tt.initialState)
			cleanedTrimmed := trimTrailingSpaces(cleaned)
			expectedTrimmed := trimTrailingSpaces(tt.expectedClean)

			if cleanedTrimmed != expectedTrimmed {
				t.Errorf("CleanLine(%q, useRaw=%v) cleaned = %q, want %q",
					tt.input, tt.useRaw, cleanedTrimmed, expectedTrimmed)
			}
			if state != tt.expectedState {
				t.Errorf("CleanLine(%q, useRaw=%v) state = %v, want %v",
					tt.input, tt.useRaw, state, tt.expectedState)
			}
		})
	}
}

func TestCleanLine_ComplexCases(t *testing.T) {
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
			name:          "code with string and comment",
			input:         `fmt.Println("hello") // print message`,
			initialState:  StateNormal,
			expectedClean: "fmt.Println(       ) ",
			expectedState: StateNormal,
		},
		{
			name:          "string containing comment-like text",
			input:         `msg := "// not a comment"`,
			initialState:  StateNormal,
			expectedClean: "msg :=                    ",
			expectedState: StateNormal,
		},
		{
			name:          "comment containing string-like text",
			input:         `// this is "not a string"`,
			initialState:  StateNormal,
			expectedClean: "",
			expectedState: StateNormal,
		},
		{
			name:          "multiple strings on one line",
			input:         `a := "first" + "second"`,
			initialState:  StateNormal,
			expectedClean: "a :=         +         ",
			expectedState: StateNormal,
		},
		{
			name:          "function with inline brace",
			input:         "func Handler() {",
			initialState:  StateNormal,
			expectedClean: "func Handler() {",
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

func TestSanitizer_MatchesAt(t *testing.T) {
	config := newGoConfig()
	s := NewSanitizer(config, false)

	tests := []struct {
		name     string
		input    string
		pos      int
		pattern  string
		expected bool
	}{
		{
			name:     "matches at beginning",
			input:    "// comment",
			pos:      0,
			pattern:  "//",
			expected: true,
		},
		{
			name:     "matches in middle",
			input:    "code // comment",
			pos:      5,
			pattern:  "//",
			expected: true,
		},
		{
			name:     "no match",
			input:    "code",
			pos:      0,
			pattern:  "//",
			expected: false,
		},
		{
			name:     "pattern too long",
			input:    "//",
			pos:      1,
			pattern:  "//",
			expected: false,
		},
		{
			name:     "empty pattern",
			input:    "code",
			pos:      0,
			pattern:  "",
			expected: true,
		},
		{
			name:     "unicode pattern",
			input:    "код /* комментарий */",
			pos:      4,
			pattern:  "/*",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runes := []rune(tt.input)
			result := s.matchesAt(runes, tt.pos, tt.pattern)
			if result != tt.expected {
				t.Errorf("matchesAt(%q, %d, %q) = %v, want %v",
					tt.input, tt.pos, tt.pattern, result, tt.expected)
			}
		})
	}
}

func TestSanitizer_MatchesAnyAt(t *testing.T) {
	config := newGoConfig()
	s := NewSanitizer(config, false)

	tests := []struct {
		name     string
		input    string
		pos      int
		patterns []string
		expected bool
	}{
		{
			name:     "matches first pattern",
			input:    `"string"`,
			pos:      0,
			patterns: []string{"\"", "'"},
			expected: true,
		},
		{
			name:     "matches second pattern",
			input:    "`rawstring`",
			pos:      0,
			patterns: []string{"\"", "`"},
			expected: true,
		},
		{
			name:     "no match",
			input:    "code",
			pos:      0,
			patterns: []string{"\"", "'", "`"},
			expected: false,
		},
		{
			name:     "empty patterns",
			input:    "code",
			pos:      0,
			patterns: []string{},
			expected: false,
		},
		{
			name:     "multi-char pattern match",
			input:    `@"verbatim"`,
			pos:      0,
			patterns: []string{"\"", "@\""},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runes := []rune(tt.input)
			result := s.matchesAnyAt(runes, tt.pos, tt.patterns)
			if result != tt.expected {
				t.Errorf("matchesAnyAt(%q, %d, %v) = %v, want %v",
					tt.input, tt.pos, tt.patterns, result, tt.expected)
			}
		})
	}
}

// Helper function для trimming trailing spaces
func trimTrailingSpaces(s string) string {
	runes := []rune(s)
	end := len(runes)
	for end > 0 && runes[end-1] == ' ' {
		end--
	}
	return string(runes[:end])
}

package main

import (
	"testing"
)

func TestLoadConfig(t *testing.T) {
	config, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	if config == nil {
		t.Fatal("LoadConfig() returned nil config")
	}

	// Verify expected languages are present
	expectedLangs := []string{"go", "c", "cpp", "cs", "java", "d"}
	for _, lang := range expectedLangs {
		if _, ok := config[lang]; !ok {
			t.Errorf("LoadConfig() missing language: %s", lang)
		}
	}
}

func TestLoadConfig_AllLanguagesHaveRegex(t *testing.T) {
	config, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	for lang, langConfig := range config {
		if langConfig.funcRegex == nil {
			t.Errorf("Language %s has nil funcRegex", lang)
		}
	}
}

func TestLoadConfig_LanguageConfigurations(t *testing.T) {
	config, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	tests := []struct {
		lang                string
		expectedLineComment string
		expectedBlockStart  string
		expectedBlockEnd    string
		expectedEscapeChar  string
	}{
		{
			lang:                "go",
			expectedLineComment: "//",
			expectedBlockStart:  "/*",
			expectedBlockEnd:    "*/",
			expectedEscapeChar:  "\\",
		},
		{
			lang:                "c",
			expectedLineComment: "//",
			expectedBlockStart:  "/*",
			expectedBlockEnd:    "*/",
			expectedEscapeChar:  "\\",
		},
		{
			lang:                "cpp",
			expectedLineComment: "//",
			expectedBlockStart:  "/*",
			expectedBlockEnd:    "*/",
			expectedEscapeChar:  "\\",
		},
		{
			lang:                "cs",
			expectedLineComment: "//",
			expectedBlockStart:  "/*",
			expectedBlockEnd:    "*/",
			expectedEscapeChar:  "\\",
		},
		{
			lang:                "java",
			expectedLineComment: "//",
			expectedBlockStart:  "/*",
			expectedBlockEnd:    "*/",
			expectedEscapeChar:  "\\",
		},
		{
			lang:                "d",
			expectedLineComment: "//",
			expectedBlockStart:  "/*",
			expectedBlockEnd:    "*/",
			expectedEscapeChar:  "\\",
		},
	}

	for _, tt := range tests {
		t.Run(tt.lang, func(t *testing.T) {
			langConfig, ok := config[tt.lang]
			if !ok {
				t.Fatalf("Language %s not found", tt.lang)
			}

			if langConfig.LineComment != tt.expectedLineComment {
				t.Errorf("LineComment = %q, want %q", langConfig.LineComment, tt.expectedLineComment)
			}
			if langConfig.BlockCommentStart != tt.expectedBlockStart {
				t.Errorf("BlockCommentStart = %q, want %q", langConfig.BlockCommentStart, tt.expectedBlockStart)
			}
			if langConfig.BlockCommentEnd != tt.expectedBlockEnd {
				t.Errorf("BlockCommentEnd = %q, want %q", langConfig.BlockCommentEnd, tt.expectedBlockEnd)
			}
			if langConfig.EscapeChar != tt.expectedEscapeChar {
				t.Errorf("EscapeChar = %q, want %q", langConfig.EscapeChar, tt.expectedEscapeChar)
			}

			// Verify FuncPattern is not empty
			if langConfig.FuncPattern == "" {
				t.Error("FuncPattern is empty")
			}

			// Verify regex is compiled
			if langConfig.funcRegex == nil {
				t.Error("funcRegex is nil (not compiled)")
			}
		})
	}
}

func TestLoadConfig_StringChars(t *testing.T) {
	config, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	tests := []struct {
		lang                 string
		expectedStringChars  []string
		expectedRawStrings   []string
	}{
		{
			lang:                 "go",
			expectedStringChars:  []string{"\""},
			expectedRawStrings:   []string{"`"},
		},
		{
			lang:                 "c",
			expectedStringChars:  []string{"\""},
			expectedRawStrings:   []string{},
		},
		{
			lang:                 "cpp",
			expectedStringChars:  []string{"\""},
			expectedRawStrings:   []string{},
		},
		{
			lang:                 "cs",
			expectedStringChars:  []string{"\""},
			expectedRawStrings:   []string{"@\""},
		},
		{
			lang:                 "java",
			expectedStringChars:  []string{"\""},
			expectedRawStrings:   []string{},
		},
		{
			lang:                 "d",
			expectedStringChars:  []string{"\""},
			expectedRawStrings:   []string{"`", "r\""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.lang, func(t *testing.T) {
			langConfig := config[tt.lang]

			if len(langConfig.StringChars) != len(tt.expectedStringChars) {
				t.Errorf("StringChars length = %d, want %d", len(langConfig.StringChars), len(tt.expectedStringChars))
			}
			for i, sc := range tt.expectedStringChars {
				if i >= len(langConfig.StringChars) || langConfig.StringChars[i] != sc {
					t.Errorf("StringChars[%d] = %q, want %q", i, langConfig.StringChars[i], sc)
				}
			}

			if len(langConfig.RawStringChars) != len(tt.expectedRawStrings) {
				t.Errorf("RawStringChars length = %d, want %d", len(langConfig.RawStringChars), len(tt.expectedRawStrings))
			}
			for i, rs := range tt.expectedRawStrings {
				if i >= len(langConfig.RawStringChars) || langConfig.RawStringChars[i] != rs {
					t.Errorf("RawStringChars[%d] = %q, want %q", i, langConfig.RawStringChars[i], rs)
				}
			}
		})
	}
}

func TestGetLanguageConfig(t *testing.T) {
	config, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	tests := []struct {
		name      string
		lang      string
		wantError bool
	}{
		{
			name:      "valid language - go",
			lang:      "go",
			wantError: false,
		},
		{
			name:      "valid language - c",
			lang:      "c",
			wantError: false,
		},
		{
			name:      "valid language - cpp",
			lang:      "cpp",
			wantError: false,
		},
		{
			name:      "valid language - cs",
			lang:      "cs",
			wantError: false,
		},
		{
			name:      "valid language - java",
			lang:      "java",
			wantError: false,
		},
		{
			name:      "valid language - d",
			lang:      "d",
			wantError: false,
		},
		{
			name:      "invalid language - python",
			lang:      "python",
			wantError: true,
		},
		{
			name:      "invalid language - javascript",
			lang:      "js",
			wantError: true,
		},
		{
			name:      "invalid language - empty",
			lang:      "",
			wantError: true,
		},
		{
			name:      "invalid language - uppercase",
			lang:      "GO",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			langConfig, err := config.GetLanguageConfig(tt.lang)

			if tt.wantError {
				if err == nil {
					t.Error("GetLanguageConfig() expected error, got nil")
				}
				if langConfig != nil {
					t.Error("GetLanguageConfig() expected nil config on error")
				}
			} else {
				if err != nil {
					t.Errorf("GetLanguageConfig() unexpected error: %v", err)
				}
				if langConfig == nil {
					t.Error("GetLanguageConfig() returned nil config")
				}
			}
		})
	}
}

func TestFuncRegex(t *testing.T) {
	config, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	tests := []struct {
		lang          string
		code          string
		shouldMatch   bool
		expectedName  string
	}{
		{
			lang:          "go",
			code:          "func main() {",
			shouldMatch:   true,
			expectedName:  "main",
		},
		{
			lang:          "go",
			code:          "func (s *Server) Handler() {",
			shouldMatch:   true,
			expectedName:  "Handler",
		},
		{
			lang:          "go",
			code:          "  func helper() {",
			shouldMatch:   true,
			expectedName:  "helper",
		},
		{
			lang:          "go",
			code:          "x := func() {",
			shouldMatch:   false,
		},
		{
			lang:          "c",
			code:          "int main()",
			shouldMatch:   true,
			expectedName:  "main",
		},
		{
			lang:          "c",
			code:          "void helper(int x, char *y)",
			shouldMatch:   true,
			expectedName:  "helper",
		},
		{
			lang:          "java",
			code:          "public void testMethod()",
			shouldMatch:   true,
			expectedName:  "testMethod",
		},
		{
			lang:          "cs",
			code:          "public int GetValue()",
			shouldMatch:   true,
			expectedName:  "GetValue",
		},
	}

	for _, tt := range tests {
		t.Run(tt.lang+"_"+tt.code[:min(20, len(tt.code))], func(t *testing.T) {
			langConfig := config[tt.lang]
			regex := langConfig.FuncRegex()

			if regex == nil {
				t.Fatal("FuncRegex() returned nil")
			}

			matches := regex.FindStringSubmatch(tt.code)

			if tt.shouldMatch {
				if matches == nil {
					t.Errorf("FuncRegex() expected match for %q", tt.code)
					return
				}

				// Extract function name (last non-empty capture group)
				funcName := ""
				for i := len(matches) - 1; i >= 1; i-- {
					if matches[i] != "" {
						funcName = matches[i]
						break
					}
				}

				if funcName != tt.expectedName {
					t.Errorf("FuncRegex() extracted name %q, want %q", funcName, tt.expectedName)
				}
			} else {
				if matches != nil {
					t.Errorf("FuncRegex() expected no match for %q, got %v", tt.code, matches)
				}
			}
		})
	}
}

// Helper function for min
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

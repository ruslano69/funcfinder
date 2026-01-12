package internal

import (
	"testing"
)

func TestNewDecoratorWindow(t *testing.T) {
	tests := []struct {
		name         string
		windowSize   int
		pattern      string
		expectNil    bool
	}{
		{
			name:       "valid python decorator pattern",
			windowSize: 15,
			pattern:    `^\s*@\w+`,
			expectNil:  false,
		},
		{
			name:       "small window size",
			windowSize: 3,
			pattern:    `^\s*@\w+`,
			expectNil:  false,
		},
		{
			name:       "large window size",
			windowSize: 100,
			pattern:    `^\s*@\w+`,
			expectNil:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			window := NewDecoratorWindow(tt.windowSize, tt.pattern)

			if window == nil {
				if !tt.expectNil {
					t.Fatal("NewDecoratorWindow() returned nil")
				}
				return
			}

			if window.pattern == nil {
				t.Error("pattern is nil")
			}

			if len(window.lines) != 0 {
				t.Errorf("initial lines length = %d, want 0", len(window.lines))
			}

			if len(window.lineNumbers) != 0 {
				t.Errorf("initial lineNumbers length = %d, want 0", len(window.lineNumbers))
			}
		})
	}
}

func TestDecoratorWindow_Add(t *testing.T) {
	window := NewDecoratorWindow(5, `^\s*@\w+`)

	tests := []struct {
		line       string
		lineNumber int
	}{
		{"@decorator", 1},
		{"def func():", 2},
		{"    pass", 3},
	}

	for _, tt := range tests {
		window.Add(tt.line, tt.lineNumber)
	}

	if len(window.lines) != 3 {
		t.Errorf("lines length = %d, want 3", len(window.lines))
	}

	if len(window.lineNumbers) != 3 {
		t.Errorf("lineNumbers length = %d, want 3", len(window.lineNumbers))
	}
}

func TestDecoratorWindow_AddWithOverflow(t *testing.T) {
	window := NewDecoratorWindow(3, `^\s*@\w+`)

	// Add more lines than window size
	for i := 1; i <= 5; i++ {
		window.Add("line", i)
	}

	// Should only keep last 3 lines
	if len(window.lines) != 3 {
		t.Errorf("lines length = %d, want 3 (window size)", len(window.lines))
	}

	if len(window.lineNumbers) != 3 {
		t.Errorf("lineNumbers length = %d, want 3 (window size)", len(window.lineNumbers))
	}

	// Check that we kept the last lines (3, 4, 5)
	if window.lineNumbers[0] != 3 {
		t.Errorf("lineNumbers[0] = %d, want 3", window.lineNumbers[0])
	}
	if window.lineNumbers[2] != 5 {
		t.Errorf("lineNumbers[2] = %d, want 5", window.lineNumbers[2])
	}
}

func TestDecoratorWindow_ExtractDecorators(t *testing.T) {
	tests := []struct {
		name                  string
		lines                 []string
		lineNumbers           []int
		expectedDecorators    []string
		expectedFirstLine     int
	}{
		{
			name: "single decorator",
			lines: []string{
				"@decorator",
				"def func():",
			},
			lineNumbers: []int{1, 2},
			expectedDecorators: []string{"@decorator"},
			expectedFirstLine:  1,
		},
		{
			name: "multiple decorators",
			lines: []string{
				"@decorator1",
				"@decorator2",
				"def func():",
			},
			lineNumbers: []int{1, 2, 3},
			expectedDecorators: []string{"@decorator1", "@decorator2"},
			expectedFirstLine:  1,
		},
		{
			name: "decorator with params",
			lines: []string{
				"@decorator(param=True)",
				"def func():",
			},
			lineNumbers: []int{1, 2},
			expectedDecorators: []string{"@decorator(param=True)"},
			expectedFirstLine:  1,
		},
		{
			name: "no decorators",
			lines: []string{
				"def func():",
			},
			lineNumbers: []int{1},
			expectedDecorators: []string{},
			expectedFirstLine:  -1,
		},
		{
			name: "decorators with blank lines",
			lines: []string{
				"",
				"@decorator",
				"def func():",
			},
			lineNumbers: []int{1, 2, 3},
			expectedDecorators: []string{"@decorator"},
			expectedFirstLine:  2,
		},
		{
			name: "decorators with indentation",
			lines: []string{
				"    @decorator",
				"    def method():",
			},
			lineNumbers: []int{1, 2},
			expectedDecorators: []string{"@decorator"}, // TrimSpace removes indentation
			expectedFirstLine:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			window := NewDecoratorWindow(15, `^\s*@\w+`)
			window.lines = tt.lines
			window.lineNumbers = tt.lineNumbers

			decorators, firstLine := window.ExtractDecorators()

			if len(decorators) != len(tt.expectedDecorators) {
				t.Errorf("decorators length = %d, want %d", len(decorators), len(tt.expectedDecorators))
			}

			for i, dec := range decorators {
				if i < len(tt.expectedDecorators) && dec != tt.expectedDecorators[i] {
					t.Errorf("decorators[%d] = %v, want %v", i, dec, tt.expectedDecorators[i])
				}
			}

			if firstLine != tt.expectedFirstLine {
				t.Errorf("firstLine = %d, want %d", firstLine, tt.expectedFirstLine)
			}
		})
	}
}

func TestDecoratorWindow_Clear(t *testing.T) {
	window := NewDecoratorWindow(5, `^\s*@\w+`)

	// Add some lines
	window.Add("@decorator", 1)
	window.Add("def func():", 2)

	if len(window.lines) == 0 {
		t.Fatal("Setup failed: window is empty")
	}

	// Clear window
	window.Clear()

	if len(window.lines) != 0 {
		t.Errorf("lines length after Clear() = %d, want 0", len(window.lines))
	}

	if len(window.lineNumbers) != 0 {
		t.Errorf("lineNumbers length after Clear() = %d, want 0", len(window.lineNumbers))
	}
}

func TestGetIndentLevel(t *testing.T) {
	tests := []struct {
		name          string
		line          string
		expectedLevel int
	}{
		{
			name:          "no indentation",
			line:          "def func():",
			expectedLevel: 0,
		},
		{
			name:          "4 spaces",
			line:          "    def method():",
			expectedLevel: 4,
		},
		{
			name:          "8 spaces",
			line:          "        nested",
			expectedLevel: 8,
		},
		{
			name:          "1 tab (counts as 4 spaces)",
			line:          "\tdef method():",
			expectedLevel: 4,
		},
		{
			name:          "2 tabs",
			line:          "\t\tdef nested():",
			expectedLevel: 8,
		},
		{
			name:          "mixed tabs and spaces",
			line:          "\t  def mixed():",
			expectedLevel: 6,
		},
		{
			name:          "empty line",
			line:          "",
			expectedLevel: 0,
		},
		{
			name:          "only spaces",
			line:          "    ",
			expectedLevel: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			level := GetIndentLevel(tt.line)
			if level != tt.expectedLevel {
				t.Errorf("GetIndentLevel(%q) = %d, want %d", tt.line, level, tt.expectedLevel)
			}
		})
	}
}

func TestIsEmptyOrComment(t *testing.T) {
	tests := []struct {
		name           string
		line           string
		commentChar    string
		expectedResult bool
	}{
		{
			name:           "empty line",
			line:           "",
			commentChar:    "#",
			expectedResult: true,
		},
		{
			name:           "only spaces",
			line:           "    ",
			commentChar:    "#",
			expectedResult: true,
		},
		{
			name:           "comment line",
			line:           "# This is a comment",
			commentChar:    "#",
			expectedResult: true,
		},
		{
			name:           "indented comment",
			line:           "    # Comment",
			commentChar:    "#",
			expectedResult: true,
		},
		{
			name:           "code line",
			line:           "def func():",
			commentChar:    "#",
			expectedResult: false,
		},
		{
			name:           "code with inline comment",
			line:           "x = 1  # comment",
			commentChar:    "#",
			expectedResult: false,
		},
		{
			name:           "C++ comment",
			line:           "// Comment",
			commentChar:    "//",
			expectedResult: true,
		},
		{
			name:           "C++ code",
			line:           "int x = 1;",
			commentChar:    "//",
			expectedResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsEmptyOrComment(tt.line, tt.commentChar)
			if result != tt.expectedResult {
				t.Errorf("IsEmptyOrComment(%q, %q) = %v, want %v", 
					tt.line, tt.commentChar, result, tt.expectedResult)
			}
		})
	}
}

func TestDecoratorWindow_Integration(t *testing.T) {
	// Test real-world scenario: processing Python code with decorators
	window := NewDecoratorWindow(15, `^\s*@\w+`)

	pythonCode := []string{
		"class MyClass:",
		"    @property",
		"    @cache",
		"    def my_method(self):",
		"        return 42",
	}

	// Simulate real processing: add lines until we hit function definition
	// In real usage, ExtractDecorators is called when we encounter "def"
	for i := 0; i <= 3; i++ {  // Stop at function definition, don't add body
		window.Add(pythonCode[i], i+1)
	}

	// Extract decorators when we hit the function definition
	// Note: The pattern checks trimmed lines, and decorators are stored trimmed
	decorators, firstLine := window.ExtractDecorators()

	if len(decorators) != 2 {
		t.Errorf("Found %d decorators, want 2 (got %v)", len(decorators), decorators)
	}

	if len(decorators) >= 1 && decorators[0] != "@property" {
		t.Errorf("decorators[0] = %v, want '@property' (TrimSpace removes indent)", decorators[0])
	}

	if len(decorators) >= 2 && decorators[1] != "@cache" {
		t.Errorf("decorators[1] = %v, want '@cache' (TrimSpace removes indent)", decorators[1])
	}

	if firstLine != 2 {
		t.Errorf("firstLine = %d, want 2", firstLine)
	}
}

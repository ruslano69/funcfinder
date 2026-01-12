package internal

import (
	"os"
	"path/filepath"
	"testing"
)

// Helper function to get Go language config for tests
func getGoConfig(t *testing.T) *LanguageConfig {
	config, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	return config["go"]
}

func TestParseFuncNames(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: []string{},
		},
		{
			name:     "single function",
			input:    "Handler",
			expected: []string{"Handler"},
		},
		{
			name:     "multiple functions",
			input:    "Handler,Middleware,Helper",
			expected: []string{"Handler", "Middleware", "Helper"},
		},
		{
			name:     "functions with spaces",
			input:    "Handler, Middleware, Helper",
			expected: []string{"Handler", "Middleware", "Helper"},
		},
		{
			name:     "functions with extra spaces",
			input:    "  Handler  ,  Middleware  ,  Helper  ",
			expected: []string{"Handler", "Middleware", "Helper"},
		},
		{
			name:     "trailing comma",
			input:    "Handler,Middleware,",
			expected: []string{"Handler", "Middleware"},
		},
		{
			name:     "leading comma",
			input:    ",Handler,Middleware",
			expected: []string{"Handler", "Middleware"},
		},
		{
			name:     "multiple commas",
			input:    "Handler,,,,Middleware",
			expected: []string{"Handler", "Middleware"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseFuncNames(tt.input)

			if len(result) != len(tt.expected) {
				t.Errorf("ParseFuncNames(%q) length = %d, want %d", tt.input, len(result), len(tt.expected))
				return
			}

			for i, exp := range tt.expected {
				if result[i] != exp {
					t.Errorf("ParseFuncNames(%q)[%d] = %q, want %q", tt.input, i, result[i], exp)
				}
			}
		})
	}
}

func TestNewFinder(t *testing.T) {
	config := getGoConfig(t)
	funcNames := []string{"Handler", "Helper"}

	tests := []struct {
		name        string
		funcNames   []string
		mapMode     bool
		extractMode bool
		useRaw      bool
	}{
		{
			name:        "map mode",
			funcNames:   []string{},
			mapMode:     true,
			extractMode: false,
			useRaw:      false,
		},
		{
			name:        "specific functions",
			funcNames:   funcNames,
			mapMode:     false,
			extractMode: false,
			useRaw:      false,
		},
		{
			name:        "extract mode",
			funcNames:   funcNames,
			mapMode:     false,
			extractMode: true,
			useRaw:      false,
		},
		{
			name:        "with raw strings",
			funcNames:   funcNames,
			mapMode:     false,
			extractMode: false,
			useRaw:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			finder := NewFinder(config, tt.funcNames, tt.mapMode, tt.extractMode, tt.useRaw)

			if finder == nil {
				t.Fatal("NewFinder returned nil")
			}
			if finder.config != config {
				t.Error("config not set correctly")
			}
			if finder.mapMode != tt.mapMode {
				t.Errorf("mapMode = %v, want %v", finder.mapMode, tt.mapMode)
			}
			if finder.extractMode != tt.extractMode {
				t.Errorf("extractMode = %v, want %v", finder.extractMode, tt.extractMode)
			}
			if finder.sanitizer == nil {
				t.Error("sanitizer is nil")
			}
		})
	}
}

func TestFindFunctions_SimpleGo(t *testing.T) {
	// Create temporary test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.go")

	code := `package main

import "fmt"

func main() {
	fmt.Println("hello")
}

func helper() {
	return
}
`
	if err := os.WriteFile(testFile, []byte(code), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	config := getGoConfig(t)

	// Test map mode - find all functions
	t.Run("map mode", func(t *testing.T) {
		finder := NewFinder(config, []string{}, true, false, false)
		result, err := finder.FindFunctions(testFile)

		if err != nil {
			t.Fatalf("FindFunctions() error = %v", err)
		}

		if len(result.Functions) != 2 {
			t.Errorf("Found %d functions, want 2", len(result.Functions))
		}

		// Check main function
		found := false
		for _, fn := range result.Functions {
			if fn.Name == "main" {
				found = true
				if fn.Start != 5 {
					t.Errorf("main start = %d, want 5", fn.Start)
				}
				if fn.End != 7 {
					t.Errorf("main end = %d, want 7", fn.End)
				}
			}
		}
		if !found {
			t.Error("main function not found")
		}
	})

	// Test specific function
	t.Run("specific function", func(t *testing.T) {
		finder := NewFinder(config, []string{"helper"}, false, false, false)
		result, err := finder.FindFunctions(testFile)

		if err != nil {
			t.Fatalf("FindFunctions() error = %v", err)
		}

		if len(result.Functions) != 1 {
			t.Errorf("Found %d functions, want 1", len(result.Functions))
		}

		if result.Functions[0].Name != "helper" {
			t.Errorf("Function name = %q, want \"helper\"", result.Functions[0].Name)
		}
	})
}

func TestFindFunctions_WithExtract(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.go")

	code := `package main

func Handler() {
	x := 5
	return
}
`
	if err := os.WriteFile(testFile, []byte(code), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	config := getGoConfig(t)
	finder := NewFinder(config, []string{"Handler"}, false, true, false)
	result, err := finder.FindFunctions(testFile)

	if err != nil {
		t.Fatalf("FindFunctions() error = %v", err)
	}

	if len(result.Functions) != 1 {
		t.Fatalf("Found %d functions, want 1", len(result.Functions))
	}

	fn := result.Functions[0]
	if len(fn.Lines) == 0 {
		t.Error("Function lines are empty in extract mode")
	}

	if len(fn.Lines) != 4 {
		t.Errorf("Function has %d lines, want 4", len(fn.Lines))
	}
}

func TestFindFunctions_NestedFunctions(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.go")

	code := `package main

func outer() {
	inner := func() {
		x := 5
	}
	inner()
}
`
	if err := os.WriteFile(testFile, []byte(code), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	config := getGoConfig(t)
	finder := NewFinder(config, []string{}, true, false, false)
	result, err := finder.FindFunctions(testFile)

	if err != nil {
		t.Fatalf("FindFunctions() error = %v", err)
	}

	// Should only find outer function
	if len(result.Functions) != 1 {
		t.Errorf("Found %d functions, want 1", len(result.Functions))
	}

	if result.Functions[0].Name != "outer" {
		t.Errorf("Function name = %q, want \"outer\"", result.Functions[0].Name)
	}
}

func TestFindFunctions_WithComments(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.go")

	code := `package main

// Handler handles requests
func Handler() {
	// x := "}" // this is not a brace
	return
}
`
	if err := os.WriteFile(testFile, []byte(code), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	config := getGoConfig(t)
	finder := NewFinder(config, []string{"Handler"}, false, false, false)
	result, err := finder.FindFunctions(testFile)

	if err != nil {
		t.Fatalf("FindFunctions() error = %v", err)
	}

	if len(result.Functions) != 1 {
		t.Fatalf("Found %d functions, want 1", len(result.Functions))
	}

	fn := result.Functions[0]
	if fn.Start != 4 {
		t.Errorf("Function start = %d, want 4", fn.Start)
	}
	if fn.End != 7 {
		t.Errorf("Function end = %d, want 7", fn.End)
	}
}

func TestFindFunctions_WithStrings(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.go")

	code := `package main

func StringFunc() {
	msg := "{ } braces in string"
	other := "another \"string\" with escapes"
	return
}
`
	if err := os.WriteFile(testFile, []byte(code), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	config := getGoConfig(t)
	finder := NewFinder(config, []string{"StringFunc"}, false, false, false)
	result, err := finder.FindFunctions(testFile)

	if err != nil {
		t.Fatalf("FindFunctions() error = %v", err)
	}

	if len(result.Functions) != 1 {
		t.Fatalf("Found %d functions, want 1", len(result.Functions))
	}

	fn := result.Functions[0]
	if fn.End != 7 {
		t.Errorf("Function end = %d, want 7 (braces in strings should be ignored)", fn.End)
	}
}

func TestFindFunctions_ReceiverMethods(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.go")

	code := `package main

type Server struct {}

func (s *Server) Handler() {
	return
}

func (s Server) Helper() {
	return
}
`
	if err := os.WriteFile(testFile, []byte(code), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	config := getGoConfig(t)
	finder := NewFinder(config, []string{}, true, false, false)
	result, err := finder.FindFunctions(testFile)

	if err != nil {
		t.Fatalf("FindFunctions() error = %v", err)
	}

	if len(result.Functions) != 2 {
		t.Errorf("Found %d functions, want 2", len(result.Functions))
	}

	expectedNames := map[string]bool{"Handler": false, "Helper": false}
	for _, fn := range result.Functions {
		if _, ok := expectedNames[fn.Name]; ok {
			expectedNames[fn.Name] = true
		}
	}

	for name, found := range expectedNames {
		if !found {
			t.Errorf("Method %q not found", name)
		}
	}
}

func TestFindFunctions_MultilineSignature(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.go")

	code := `package main

func LongSignature(
	param1 string,
	param2 int,
) {
	return
}
`
	if err := os.WriteFile(testFile, []byte(code), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	config := getGoConfig(t)
	finder := NewFinder(config, []string{"LongSignature"}, false, false, false)
	result, err := finder.FindFunctions(testFile)

	if err != nil {
		t.Fatalf("FindFunctions() error = %v", err)
	}

	if len(result.Functions) != 1 {
		t.Fatalf("Found %d functions, want 1", len(result.Functions))
	}

	fn := result.Functions[0]
	if fn.Name != "LongSignature" {
		t.Errorf("Function name = %q, want \"LongSignature\"", fn.Name)
	}
	if fn.Start != 3 {
		t.Errorf("Function start = %d, want 3", fn.Start)
	}
}

func TestFindFunctions_NoFunctions(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.go")

	code := `package main

import "fmt"

var x = 5
`
	if err := os.WriteFile(testFile, []byte(code), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	config := getGoConfig(t)
	finder := NewFinder(config, []string{}, true, false, false)
	result, err := finder.FindFunctions(testFile)

	if err != nil {
		t.Fatalf("FindFunctions() error = %v", err)
	}

	if len(result.Functions) != 0 {
		t.Errorf("Found %d functions, want 0", len(result.Functions))
	}
}

func TestFindFunctions_InvalidFile(t *testing.T) {
	config := getGoConfig(t)
	finder := NewFinder(config, []string{}, true, false, false)

	_, err := finder.FindFunctions("/nonexistent/file.go")
	if err == nil {
		t.Error("FindFunctions() expected error for nonexistent file, got nil")
	}
}

func TestFindFunctions_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "empty.go")

	if err := os.WriteFile(testFile, []byte(""), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	config := getGoConfig(t)
	finder := NewFinder(config, []string{}, true, false, false)
	result, err := finder.FindFunctions(testFile)

	if err != nil {
		t.Fatalf("FindFunctions() error = %v", err)
	}

	if len(result.Functions) != 0 {
		t.Errorf("Found %d functions in empty file, want 0", len(result.Functions))
	}
}

func TestFindFunctions_BlockComments(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.go")

	code := `package main

/*
func NotAFunction() {
	return
}
*/

func RealFunction() {
	/* x := "}" */
	return
}
`
	if err := os.WriteFile(testFile, []byte(code), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	config := getGoConfig(t)
	finder := NewFinder(config, []string{}, true, false, false)
	result, err := finder.FindFunctions(testFile)

	if err != nil {
		t.Fatalf("FindFunctions() error = %v", err)
	}

	if len(result.Functions) != 1 {
		t.Errorf("Found %d functions, want 1 (commented function should be ignored)", len(result.Functions))
	}

	if result.Functions[0].Name != "RealFunction" {
		t.Errorf("Function name = %q, want \"RealFunction\"", result.Functions[0].Name)
	}
}

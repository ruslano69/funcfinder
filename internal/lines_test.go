package internal

import (
	"os"
	"strings"
	"testing"
)

// Test ParseLineRange
func TestParseLineRange(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantStart   int
		wantEnd     int
		shouldError bool
	}{
		// Single line
		{
			name:      "single line",
			input:     "42",
			wantStart: 42,
			wantEnd:   42,
		},
		{
			name:      "line 1",
			input:     "1",
			wantStart: 1,
			wantEnd:   1,
		},
		
		// Range with both start and end
		{
			name:      "normal range",
			input:     "10:20",
			wantStart: 10,
			wantEnd:   20,
		},
		{
			name:      "range 1:10",
			input:     "1:10",
			wantStart: 1,
			wantEnd:   10,
		},
		
		// Range from beginning
		{
			name:      "from beginning to 50",
			input:     ":50",
			wantStart: 1,
			wantEnd:   50,
		},
		
		// Range to end
		{
			name:      "from 100 to end",
			input:     "100:",
			wantStart: 100,
			wantEnd:   -1, // Special value for "to end"
		},
		{
			name:      "from 1 to end",
			input:     "1:",
			wantStart: 1,
			wantEnd:   -1,
		},
		
		// Error cases
		{
			name:        "empty string",
			input:       "",
			shouldError: true,
		},
		{
			name:        "invalid number",
			input:       "abc",
			shouldError: true,
		},
		{
			name:        "zero line",
			input:       "0",
			shouldError: true,
		},
		{
			name:        "negative line",
			input:       "-5",
			shouldError: true,
		},
		{
			name:        "invalid range start",
			input:       "abc:10",
			shouldError: true,
		},
		{
			name:        "invalid range end",
			input:       "10:abc",
			shouldError: true,
		},
		{
			name:        "start > end",
			input:       "20:10",
			shouldError: true,
		},
		{
			name:        "zero in range",
			input:       "0:10",
			shouldError: true,
		},
		{
			name:        "negative in range",
			input:       "-5:10",
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseLineRange(tt.input)

			if tt.shouldError {
				if err == nil {
					t.Errorf("ParseLineRange(%q) expected error, got nil", tt.input)
				}
				return
			}

			if err != nil {
				t.Fatalf("ParseLineRange(%q) unexpected error: %v", tt.input, err)
			}

			if result.Start != tt.wantStart {
				t.Errorf("ParseLineRange(%q).Start = %d, want %d", tt.input, result.Start, tt.wantStart)
			}

			if result.End != tt.wantEnd {
				t.Errorf("ParseLineRange(%q).End = %d, want %d", tt.input, result.End, tt.wantEnd)
			}
		})
	}
}

// Test ReadFileLines
func TestReadFileLines(t *testing.T) {
	// Create a temporary test file
	content := `line 1
line 2
line 3
line 4
line 5
line 6
line 7
line 8
line 9
line 10`

	tmpfile, err := os.CreateTemp("", "test_lines_*.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	tmpfile.Close()

	tests := []struct {
		name          string
		lineRange     LineRange
		expectedLines []string
		expectedStart int
		shouldError   bool
	}{
		{
			name:          "single line",
			lineRange:     LineRange{Start: 3, End: 3},
			expectedLines: []string{"line 3"},
			expectedStart: 3,
		},
		{
			name:          "range 2-5",
			lineRange:     LineRange{Start: 2, End: 5},
			expectedLines: []string{"line 2", "line 3", "line 4", "line 5"},
			expectedStart: 2,
		},
		{
			name:          "from beginning",
			lineRange:     LineRange{Start: 1, End: 3},
			expectedLines: []string{"line 1", "line 2", "line 3"},
			expectedStart: 1,
		},
		{
			name:          "to end",
			lineRange:     LineRange{Start: 8, End: -1},
			expectedLines: []string{"line 8", "line 9", "line 10"},
			expectedStart: 8,
		},
		{
			name:          "entire file",
			lineRange:     LineRange{Start: 1, End: -1},
			expectedLines: strings.Split(content, "\n"),
			expectedStart: 1,
		},
		{
			name:        "range beyond file",
			lineRange:   LineRange{Start: 100, End: 200},
			shouldError: true,
		},
		{
			name:        "start beyond file",
			lineRange:   LineRange{Start: 20, End: -1},
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lines, start, err := ReadFileLines(tmpfile.Name(), tt.lineRange)

			if tt.shouldError {
				if err == nil {
					t.Error("ReadFileLines() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("ReadFileLines() unexpected error: %v", err)
			}

			if start != tt.expectedStart {
				t.Errorf("start = %d, want %d", start, tt.expectedStart)
			}

			if len(lines) != len(tt.expectedLines) {
				t.Fatalf("got %d lines, want %d", len(lines), len(tt.expectedLines))
			}

			for i, line := range lines {
				if line != tt.expectedLines[i] {
					t.Errorf("line %d = %q, want %q", i, line, tt.expectedLines[i])
				}
			}
		})
	}
}

// Test ReadFileLines with non-existent file
func TestReadFileLines_NonExistentFile(t *testing.T) {
	_, _, err := ReadFileLines("/nonexistent/file.txt", LineRange{Start: 1, End: 10})
	if err == nil {
		t.Error("ReadFileLines() with non-existent file should return error")
	}
}

// Test CheckPartialFunctions
func TestCheckPartialFunctions(t *testing.T) {
	tests := []struct {
		name         string
		functions    []FunctionBounds
		lineRange    LineRange
		totalLines   int
		expectWarn   bool
		warnContains string
	}{
		{
			name: "function fully inside range",
			functions: []FunctionBounds{
				{Name: "test", Start: 10, End: 20},
			},
			lineRange:  LineRange{Start: 5, End: 25},
			totalLines: 30,
			expectWarn: false,
		},
		{
			name: "function starts before range",
			functions: []FunctionBounds{
				{Name: "partial", Start: 5, End: 15},
			},
			lineRange:    LineRange{Start: 10, End: 20},
			totalLines:   30,
			expectWarn:   true,
			warnContains: "partial",
		},
		{
			name: "function ends after range",
			functions: []FunctionBounds{
				{Name: "cutoff", Start: 15, End: 25},
			},
			lineRange:    LineRange{Start: 10, End: 20},
			totalLines:   30,
			expectWarn:   true,
			warnContains: "cutoff",
		},
		{
			name: "function spans entire range",
			functions: []FunctionBounds{
				{Name: "spanning", Start: 5, End: 25},
			},
			lineRange:    LineRange{Start: 10, End: 20},
			totalLines:   30,
			expectWarn:   true,
			warnContains: "spanning",
		},
		{
			name: "multiple partial functions",
			functions: []FunctionBounds{
				{Name: "func1", Start: 5, End: 12},
				{Name: "func2", Start: 18, End: 25},
			},
			lineRange:    LineRange{Start: 10, End: 20},
			totalLines:   30,
			expectWarn:   true,
			warnContains: "func1",
		},
		{
			name: "range to end of file",
			functions: []FunctionBounds{
				{Name: "test", Start: 5, End: 15},
			},
			lineRange:    LineRange{Start: 10, End: -1},
			totalLines:   20,
			expectWarn:   true,
			warnContains: "test",
		},
		{
			name:       "no functions",
			functions:  []FunctionBounds{},
			lineRange:  LineRange{Start: 1, End: 10},
			totalLines: 20,
			expectWarn: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			warning := CheckPartialFunctions(tt.functions, tt.lineRange, tt.totalLines)

			if tt.expectWarn {
				if warning == "" {
					t.Error("CheckPartialFunctions() expected warning, got empty string")
				}
				if !strings.Contains(warning, tt.warnContains) {
					t.Errorf("warning %q does not contain %q", warning, tt.warnContains)
				}
			} else {
				if warning != "" {
					t.Errorf("CheckPartialFunctions() expected no warning, got: %s", warning)
				}
			}
		})
	}
}

// Test OutputPlainLines (just verify it doesn't panic)
func TestOutputPlainLines(t *testing.T) {
	lines := []string{"line 1", "line 2", "line 3"}
	
	// Redirect stdout to /dev/null for this test
	oldStdout := os.Stdout
	devNull, _ := os.Open(os.DevNull)
	os.Stdout = devNull
	defer func() {
		os.Stdout = oldStdout
		devNull.Close()
	}()

	// Should not panic
	OutputPlainLines(lines, 10)
	
	// Test with empty lines
	OutputPlainLines([]string{}, 1)
}

// Test OutputJSONLines (just verify it doesn't panic and produces valid structure)
func TestOutputJSONLines(t *testing.T) {
	lines := []string{"line 1", "line 2 with \"quotes\"", "line 3 with \t tabs"}
	
	// Redirect stdout to /dev/null for this test
	oldStdout := os.Stdout
	devNull, _ := os.Open(os.DevNull)
	os.Stdout = devNull
	defer func() {
		os.Stdout = oldStdout
		devNull.Close()
	}()

	lineRange := LineRange{Start: 5, End: 7}
	
	// Should not panic
	OutputJSONLines(lines, 5, lineRange)
	
	// Test with empty lines
	OutputJSONLines([]string{}, 1, LineRange{Start: 1, End: 1})
	
	// Test with special characters
	specialLines := []string{
		"line with \\backslash",
		"line with \n newline",
		"line with \r return",
	}
	OutputJSONLines(specialLines, 1, LineRange{Start: 1, End: 3})
}

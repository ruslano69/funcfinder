package main

import (
	"encoding/json"
	"testing"
)

func TestFormatGrepStyle(t *testing.T) {
	tests := []struct {
		name     string
		result   *FindResult
		expected string
	}{
		{
			name: "single function",
			result: &FindResult{
				Filename: "test.go",
				Functions: []FunctionBounds{
					{Name: "Handler", Start: 45, End: 78},
				},
			},
			expected: "Handler: 45-78;",
		},
		{
			name: "multiple functions",
			result: &FindResult{
				Filename: "test.go",
				Functions: []FunctionBounds{
					{Name: "Handler", Start: 45, End: 78},
					{Name: "Middleware", Start: 80, End: 95},
					{Name: "Helper", Start: 100, End: 120},
				},
			},
			expected: "Handler: 45-78; Middleware: 80-95; Helper: 100-120;",
		},
		{
			name: "no functions",
			result: &FindResult{
				Filename:  "test.go",
				Functions: []FunctionBounds{},
			},
			expected: "",
		},
		{
			name: "function with single line",
			result: &FindResult{
				Filename: "test.go",
				Functions: []FunctionBounds{
					{Name: "OneLiner", Start: 10, End: 10},
				},
			},
			expected: "OneLiner: 10-10;",
		},
		{
			name: "functions with large line numbers",
			result: &FindResult{
				Filename: "large.go",
				Functions: []FunctionBounds{
					{Name: "BigFunc", Start: 1000, End: 2500},
				},
			},
			expected: "BigFunc: 1000-2500;",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := FormatGrepStyle(tt.result)
			if output != tt.expected {
				t.Errorf("FormatGrepStyle() = %q, want %q", output, tt.expected)
			}
		})
	}
}

func TestFormatJSON(t *testing.T) {
	tests := []struct {
		name        string
		result      *FindResult
		expectedMap JSONOutput
	}{
		{
			name: "single function",
			result: &FindResult{
				Filename: "test.go",
				Functions: []FunctionBounds{
					{Name: "Handler", Start: 45, End: 78},
				},
			},
			expectedMap: JSONOutput{
				"Handler": {"start": 45, "end": 78},
			},
		},
		{
			name: "multiple functions",
			result: &FindResult{
				Filename: "test.go",
				Functions: []FunctionBounds{
					{Name: "Handler", Start: 45, End: 78},
					{Name: "Middleware", Start: 80, End: 95},
				},
			},
			expectedMap: JSONOutput{
				"Handler":    {"start": 45, "end": 78},
				"Middleware": {"start": 80, "end": 95},
			},
		},
		{
			name: "no functions",
			result: &FindResult{
				Filename:  "test.go",
				Functions: []FunctionBounds{},
			},
			expectedMap: JSONOutput{},
		},
		{
			name: "function at start of file",
			result: &FindResult{
				Filename: "test.go",
				Functions: []FunctionBounds{
					{Name: "init", Start: 1, End: 5},
				},
			},
			expectedMap: JSONOutput{
				"init": {"start": 1, "end": 5},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, err := FormatJSON(tt.result)
			if err != nil {
				t.Fatalf("FormatJSON() error = %v", err)
			}

			// Parse JSON output
			var outputMap JSONOutput
			if err := json.Unmarshal([]byte(output), &outputMap); err != nil {
				t.Fatalf("Failed to parse JSON output: %v", err)
			}

			// Compare maps
			if len(outputMap) != len(tt.expectedMap) {
				t.Errorf("FormatJSON() returned %d functions, want %d", len(outputMap), len(tt.expectedMap))
			}

			for name, expected := range tt.expectedMap {
				actual, ok := outputMap[name]
				if !ok {
					t.Errorf("FormatJSON() missing function %q", name)
					continue
				}
				if actual["start"] != expected["start"] || actual["end"] != expected["end"] {
					t.Errorf("FormatJSON() function %q = {start: %d, end: %d}, want {start: %d, end: %d}",
						name, actual["start"], actual["end"], expected["start"], expected["end"])
				}
			}
		})
	}
}

func TestFormatJSON_ValidJSON(t *testing.T) {
	// Test that output is valid JSON
	result := &FindResult{
		Filename: "test.go",
		Functions: []FunctionBounds{
			{Name: "Func1", Start: 10, End: 20},
			{Name: "Func2", Start: 30, End: 40},
		},
	}

	output, err := FormatJSON(result)
	if err != nil {
		t.Fatalf("FormatJSON() error = %v", err)
	}

	// Verify it's valid JSON
	var tmp interface{}
	if err := json.Unmarshal([]byte(output), &tmp); err != nil {
		t.Errorf("FormatJSON() produced invalid JSON: %v", err)
	}
}

func TestFormatExtract(t *testing.T) {
	tests := []struct {
		name     string
		result   *FindResult
		expected string
	}{
		{
			name: "single function",
			result: &FindResult{
				Filename: "test.go",
				Functions: []FunctionBounds{
					{
						Name:  "Handler",
						Start: 45,
						End:   48,
						Lines: []string{
							"func Handler(w http.ResponseWriter, r *http.Request) {",
							"    fmt.Println(\"hello\")",
							"    return",
							"}",
						},
					},
				},
			},
			expected: "// Handler: 45-48\nfunc Handler(w http.ResponseWriter, r *http.Request) {\n    fmt.Println(\"hello\")\n    return\n}",
		},
		{
			name: "multiple functions",
			result: &FindResult{
				Filename: "test.go",
				Functions: []FunctionBounds{
					{
						Name:  "First",
						Start: 10,
						End:   12,
						Lines: []string{
							"func First() {",
							"    // body",
							"}",
						},
					},
					{
						Name:  "Second",
						Start: 20,
						End:   22,
						Lines: []string{
							"func Second() {",
							"    // body",
							"}",
						},
					},
				},
			},
			expected: "// First: 10-12\nfunc First() {\n    // body\n}\n\n// Second: 20-22\nfunc Second() {\n    // body\n}",
		},
		{
			name: "no functions",
			result: &FindResult{
				Filename:  "test.go",
				Functions: []FunctionBounds{},
			},
			expected: "",
		},
		{
			name: "function with empty lines",
			result: &FindResult{
				Filename: "test.go",
				Functions: []FunctionBounds{
					{
						Name:  "Handler",
						Start: 5,
						End:   9,
						Lines: []string{
							"func Handler() {",
							"",
							"    fmt.Println(\"test\")",
							"",
							"}",
						},
					},
				},
			},
			expected: "// Handler: 5-9\nfunc Handler() {\n\n    fmt.Println(\"test\")\n\n}",
		},
		{
			name: "single line function",
			result: &FindResult{
				Filename: "test.go",
				Functions: []FunctionBounds{
					{
						Name:  "Getter",
						Start: 100,
						End:   100,
						Lines: []string{
							"func Getter() int { return 42 }",
						},
					},
				},
			},
			expected: "// Getter: 100-100\nfunc Getter() int { return 42 }",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := FormatExtract(tt.result)
			if output != tt.expected {
				t.Errorf("FormatExtract() = %q, want %q", output, tt.expected)
			}
		})
	}
}

func TestFormatExtract_PreservesIndentation(t *testing.T) {
	result := &FindResult{
		Filename: "test.go",
		Functions: []FunctionBounds{
			{
				Name:  "Method",
				Start: 10,
				End:   14,
				Lines: []string{
					"    func (s *Server) Method() {",
					"        if true {",
					"            doSomething()",
					"        }",
					"    }",
				},
			},
		},
	}

	output := FormatExtract(result)
	expected := "// Method: 10-14\n    func (s *Server) Method() {\n        if true {\n            doSomething()\n        }\n    }"

	if output != expected {
		t.Errorf("FormatExtract() didn't preserve indentation:\ngot:  %q\nwant: %q", output, expected)
	}
}

func TestFormatExtract_WithSpecialChars(t *testing.T) {
	result := &FindResult{
		Filename: "test.go",
		Functions: []FunctionBounds{
			{
				Name:  "StringFunc",
				Start: 5,
				End:   7,
				Lines: []string{
					`func StringFunc() string {`,
					`    return "Hello \"World\""`,
					"}",
				},
			},
		},
	}

	output := FormatExtract(result)
	expected := "// StringFunc: 5-7\nfunc StringFunc() string {\n    return \"Hello \\\"World\\\"\"\n}"

	if output != expected {
		t.Errorf("FormatExtract() didn't handle special chars correctly:\ngot:  %q\nwant: %q", output, expected)
	}
}

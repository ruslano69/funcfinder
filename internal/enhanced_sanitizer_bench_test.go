package internal

import (
	"testing"
)

// Benchmark configurations
var (
	goConfig = &LanguageConfig{
		FuncPattern:       "^\\s*func\\s+(\\w+)\\s*\\(",
		LineComment:       "//",
		BlockCommentStart: "/*",
		BlockCommentEnd:   "*/",
		StringChars:       []string{"\""},
		RawStringChars:    []string{"`"},
		EscapeChar:        "\\",
	}

	pythonConfig = &LanguageConfig{
		FuncPattern:      "^\\s*def\\s+(\\w+)\\s*\\(",
		LineComment:      "#",
		StringChars:      []string{"\"", "'"},
		EscapeChar:       "\\",
		DocStringMarkers: []string{"\"\"\"", "'''"},
	}

	cppConfig = &LanguageConfig{
		FuncPattern:       "^\\s*[\\w:<>]+\\s+\\w+\\s*\\([^)]*\\)\\s*\\{?$",
		LineComment:       "//",
		BlockCommentStart: "/*",
		BlockCommentEnd:   "*/",
		StringChars:       []string{"\"", "'"},
		CharDelimiters:    []string{"'"},
		EscapeChar:        "\\",
	}
)

// Benchmark simple lines (mostly StateNormal)
func BenchmarkCleanLine_Simple(b *testing.B) {
	s := NewSanitizer(goConfig, false)
	line := "func TestFunction(param1 string, param2 int) error {"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = s.CleanLine(line, StateNormal)
	}
}

// Benchmark lines with string literals
func BenchmarkCleanLine_WithStrings(b *testing.B) {
	s := NewSanitizer(goConfig, false)
	line := `fmt.Printf("Hello %s, value: %d", name, 42)`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = s.CleanLine(line, StateNormal)
	}
}

// Benchmark lines with line comments
func BenchmarkCleanLine_WithLineComment(b *testing.B) {
	s := NewSanitizer(goConfig, false)
	line := "func Process() { // This is a comment with some text"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = s.CleanLine(line, StateNormal)
	}
}

// Benchmark lines with block comments
func BenchmarkCleanLine_WithBlockComment(b *testing.B) {
	s := NewSanitizer(goConfig, false)
	line := "func Process() { /* inline block comment */ return nil }"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = s.CleanLine(line, StateNormal)
	}
}

// Benchmark complex lines with multiple state transitions
func BenchmarkCleanLine_Complex(b *testing.B) {
	s := NewSanitizer(goConfig, false)
	line := `log.Debug("Processing /* not a comment */", val) // real comment`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = s.CleanLine(line, StateNormal)
	}
}

// Benchmark raw strings (Go backticks)
func BenchmarkCleanLine_RawString(b *testing.B) {
	s := NewSanitizer(goConfig, false)
	line := "query := `SELECT * FROM users WHERE name = \"John\" // not comment`"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = s.CleanLine(line, StateNormal)
	}
}

// Benchmark Python docstrings
func BenchmarkCleanLine_PythonDocstring(b *testing.B) {
	s := NewSanitizer(pythonConfig, false)
	line := `    """This is a docstring with 'quotes' and "double quotes" """`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = s.CleanLine(line, StateNormal)
	}
}

// Benchmark C++ char literals
func BenchmarkCleanLine_CharLiterals(b *testing.B) {
	s := NewSanitizer(cppConfig, false)
	line := "char c = ';'; char d = '\\\\'; int x = 42;"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = s.CleanLine(line, StateNormal)
	}
}

// Benchmark escaped sequences in strings
func BenchmarkCleanLine_EscapedStrings(b *testing.B) {
	s := NewSanitizer(goConfig, false)
	line := `path := "C:\\Users\\Documents\\file.txt"`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = s.CleanLine(line, StateNormal)
	}
}

// Benchmark multiline state persistence
func BenchmarkCleanLine_MultilineComment(b *testing.B) {
	s := NewSanitizer(goConfig, false)
	lines := []string{
		"func Process() { /*",
		"  This is a long",
		"  multiline comment",
		"  */ return nil",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		state := StateNormal
		for _, line := range lines {
			_, state = s.CleanLine(line, state)
		}
	}
}

// Benchmark realistic code with mixed content
func BenchmarkCleanLine_Realistic(b *testing.B) {
	s := NewSanitizer(goConfig, false)
	lines := []string{
		"// Package documentation",
		"package main",
		"",
		`import "fmt"`,
		"",
		"/* Multi-line comment",
		"   explaining the function */",
		`func main() {`,
		`    name := "World" // User name`,
		`    fmt.Printf("Hello, %s!\n", name)`,
		`    /* Debug: */ log.Println("Started")`,
		`    query := ` + "`SELECT * FROM users`" + ``,
		"}",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		state := StateNormal
		for _, line := range lines {
			_, state = s.CleanLine(line, state)
		}
	}
}

// Benchmark long lines (stress test)
func BenchmarkCleanLine_LongLine(b *testing.B) {
	s := NewSanitizer(goConfig, false)
	// Create a long line with multiple strings
	line := `log.Debug("msg1", "msg2", "msg3", "msg4", "msg5", "msg6", "msg7", "msg8", "msg9", "msg10") // comment`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = s.CleanLine(line, StateNormal)
	}
}

// Benchmark empty and whitespace lines
func BenchmarkCleanLine_Empty(b *testing.B) {
	s := NewSanitizer(goConfig, false)
	line := ""

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = s.CleanLine(line, StateNormal)
	}
}

func BenchmarkCleanLine_Whitespace(b *testing.B) {
	s := NewSanitizer(goConfig, false)
	line := "                        "

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = s.CleanLine(line, StateNormal)
	}
}

// Benchmark state handlers individually

func BenchmarkHandleString(b *testing.B) {
	s := NewSanitizer(goConfig, false)
	line := `"This is a string with \\" escaped quote and more text"`
	runes := []rune(line)
	result := make([]rune, len(runes))
	copy(result, runes)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = s.handleString(runes, result, 1)
	}
}

func BenchmarkHandleBlockComment(b *testing.B) {
	s := NewSanitizer(goConfig, false)
	line := "/* This is a block comment with some content */"
	runes := []rune(line)
	result := make([]rune, len(runes))
	copy(result, runes)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = s.handleBlockComment(line, runes, result, 0)
	}
}

func BenchmarkHandleCharLiteral(b *testing.B) {
	s := NewSanitizer(cppConfig, false)
	line := "'x'"
	runes := []rune(line)
	result := make([]rune, len(runes))
	copy(result, runes)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = s.handleCharLiteral(runes, result, 1)
	}
}

func BenchmarkTryHandleMultiLineString_Python(b *testing.B) {
	s := NewSanitizer(pythonConfig, false)
	line := `"""This is a Python docstring with content"""`
	runes := []rune(line)
	result := make([]rune, len(runes))
	copy(result, runes)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = s.tryHandleMultiLineString(line, runes, result, 0)
	}
}

func BenchmarkTryHandleBlockComment_Nested(b *testing.B) {
	s := NewSanitizer(goConfig, false)
	line := "/* outer /* inner */ outer */"
	runes := []rune(line)
	result := make([]rune, len(runes))
	copy(result, runes)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = s.tryHandleBlockComment(line, runes, result, 0)
	}
}

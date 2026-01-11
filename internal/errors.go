// errors.go - Unified error handling for funcfinder utilities
package internal

import (
	"fmt"
	"os"
)

// ErrorType defines different categories of errors
type ErrorType int

const (
	// ErrInvalidArgs indicates invalid command-line arguments
	ErrInvalidArgs ErrorType = iota
	// ErrFileNotFound indicates the requested file doesn't exist
	ErrFileNotFound
	// ErrParsingFailed indicates failure to parse input
	ErrParsingFailed
	// ErrConfigLoad indicates failure to load configuration
	ErrConfigLoad
	// ErrUnsupportedLanguage indicates unsupported language
	ErrUnsupportedLanguage
	// ErrFileRead indicates failure to read file
	ErrFileRead
)

// FatalError prints an error message to stderr and exits with code 1
func FatalError(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "Error: "+format+"\n", args...)
	os.Exit(1)
}

// FatalErrorWithCode prints an error and exits with specific code
func FatalErrorWithCode(code int, format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "Error: "+format+"\n", args...)
	os.Exit(code)
}

// WarnError prints a warning message to stderr but continues execution
func WarnError(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "Warning: "+format+"\n", args...)
}

// InfoMessage prints an informational message to stderr
func InfoMessage(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "INFO: "+format+"\n", args...)
}

// PrintUsage prints usage information and exits
func PrintUsage(usageFunc func()) {
	usageFunc()
	os.Exit(1)
}

// PrintVersion prints version and exits successfully
func PrintVersion(toolName string) {
	const Version = "1.4.0"
	fmt.Printf("%s version %s\n", toolName, Version)
	os.Exit(0)
}

// FatalErrorMsg prints error message and exits
func FatalErrorMsg(msg string) {
	fmt.Fprintf(os.Stderr, "Error: %s\n", msg)
	os.Exit(1)
}

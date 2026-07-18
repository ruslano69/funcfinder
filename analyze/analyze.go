// Package analyze is funcfinder's public API for programmatic code mapping:
// walk a source tree and get per-file function/type boundaries, using the same
// engine the funcfinder CLI runs. It exists so other modules (e.g. distill-docs'
// code-ingest) can depend on funcfinder as a library without reaching into its
// internal packages, which Go forbids across module boundaries.
//
// The types are aliases of the internal implementation, so values flow through
// unchanged (no copying, no adapter layer).
package analyze

import "github.com/ruslano69/funcfinder/internal"

// Config is the language/analysis configuration (from LoadConfig).
type Config = internal.Config

// DirProcessor walks a directory and extracts function/type boundaries.
type DirProcessor = internal.DirProcessor

// DirResult is one file's analysis: its path, functions, types, and any error.
type DirResult = internal.DirResult

// FunctionBounds / ClassBounds are a single function's / type's name and line span.
type (
	FunctionBounds = internal.FunctionBounds
	ClassBounds    = internal.ClassBounds
)

// LoadConfig loads the built-in multi-language configuration.
func LoadConfig() (Config, error) { return internal.LoadConfig() }

// NewDirProcessor mirrors internal.NewDirProcessor: workers<=0 uses all CPUs;
// workMode is "functions", "structs", or "all".
func NewDirProcessor(config Config, workers int, recursive, useGitignore bool, workMode string) *DirProcessor {
	return internal.NewDirProcessor(config, workers, recursive, useGitignore, workMode)
}

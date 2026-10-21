package main

import (
	"fmt"
	"io"
	"os"

	"github.com/ruslano69/funcfinder/internal"
)

// editRequest is the input shared by --replace-func and --add-after.
type editRequest struct {
	inp, source, name string
	line              int
	withPath, oldPath string
	apply             bool
}

// handleReplaceFunc replaces one function in a file of the given language. The new function is read from withPath
// ("-" for stdin), so the replacement never passes through a shell. oldPath, when given, holds the function text the
// caller expects to replace; the edit is refused if the file differs. Without apply it only validates and reports.
func handleReplaceFunc(config internal.Config, req editRequest) {
	cfg, src, replacement, expected := loadEdit(config, req, true)
	res, err := internal.ReplaceFunction(cfg, src, internal.FuncTarget{Name: req.name, Line: req.line}, expected, replacement)
	if err != nil {
		internal.FatalError("%v", err)
	}
	report(req, res, fmt.Sprintf("%s: lines %d-%d", req.name, res.Start, res.End))
}

// handleAddAfter inserts the functions read from withPath after the named function.
func handleAddAfter(config internal.Config, req editRequest) {
	cfg, src, replacement, _ := loadEdit(config, req, false)
	res, err := internal.InsertAfterFunction(cfg, src, internal.FuncTarget{Name: req.name, Line: req.line}, replacement)
	if err != nil {
		internal.FatalError("%v", err)
	}
	report(req, res, fmt.Sprintf("new function(s) after %s (line %d)", req.name, res.End))
}

// loadEdit reads the language, the file, the new text and, for a replacement, the expected old text.
func loadEdit(config internal.Config, req editRequest, replacing bool) (*internal.LanguageConfig, []byte, []byte, []byte) {
	if req.inp == "" {
		internal.FatalError("--replace-func and --add-after require --inp")
	}
	if req.source == "" {
		internal.FatalError("--replace-func and --add-after require --source <language>")
	}
	if req.withPath == "" {
		internal.FatalError("--replace-func and --add-after require --with <file> (or - for stdin)")
	}
	if !replacing && req.oldPath != "" {
		internal.FatalError("--old applies to --replace-func only")
	}
	cfg, err := config.GetLanguageConfig(req.source)
	if err != nil {
		internal.FatalError("%v", err)
	}
	src, err := os.ReadFile(req.inp)
	if err != nil {
		internal.FatalError("reading %s: %v", req.inp, err)
	}
	var replacement []byte
	if req.withPath == "-" {
		replacement, err = io.ReadAll(os.Stdin)
	} else {
		replacement, err = os.ReadFile(req.withPath)
	}
	if err != nil {
		internal.FatalError("reading replacement: %v", err)
	}
	var expected []byte
	if req.oldPath != "" {
		expected, err = os.ReadFile(req.oldPath)
		if err != nil {
			internal.FatalError("reading --old: %v", err)
		}
	}
	return cfg, src, replacement, expected
}

// report prints the outcome, or writes the file when apply is set.
func report(req editRequest, res *internal.ReplaceResult, what string) {
	if !req.apply {
		fmt.Printf("%s would be changed; the new file is valid. Re-run with --apply to write it.\n", what)
		return
	}
	info, err := os.Stat(req.inp)
	if err != nil {
		internal.FatalError("%v", err)
	}
	if err := os.WriteFile(req.inp, res.Output, info.Mode().Perm()); err != nil {
		internal.FatalError("writing %s: %v", req.inp, err)
	}
	fmt.Printf("%s written to %s\n", what, req.inp)
}

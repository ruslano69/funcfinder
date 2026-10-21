package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/ruslano69/funcfinder/internal"
)

// editRequest is the input shared by the function edit commands.
type editRequest struct {
	inp, source, name string
	kind              string // "type" for a type declaration; empty for a function
	line              int
	withPath, oldPath string
	apply             bool
}

// handleReplaceFunc replaces one function in a file of the given language. The new function is read from withPath
// ("-" for stdin), so the replacement never passes through a shell. oldPath, when given, holds the function text the
// caller expects to replace; the edit is refused if the file differs. Without apply it only validates and reports.
func handleReplaceFunc(config internal.Config, req editRequest) {
	cfg, src, replacement, expected := loadEdit(config, req, "replace")
	res, err := internal.ReplaceFunction(cfg, src, internal.FuncTarget{Name: req.name, Line: req.line, Kind: req.kind}, expected, replacement)
	if err != nil {
		internal.FatalError("%v", err)
	}
	report(req, src, res, fmt.Sprintf("%s: lines %d-%d", req.name, res.Start, res.End))
}

// handleAddAfter inserts the functions read from withPath after the named function.
func handleAddAfter(config internal.Config, req editRequest) {
	cfg, src, replacement, _ := loadEdit(config, req, "add")
	res, err := internal.InsertAfterFunction(cfg, src, internal.FuncTarget{Name: req.name, Line: req.line, Kind: req.kind}, replacement)
	if err != nil {
		internal.FatalError("%v", err)
	}
	report(req, src, res, fmt.Sprintf("new function(s) after %s (line %d)", req.name, res.End))
}

// handleRemoveFunc deletes the named function from the file.
func handleRemoveFunc(config internal.Config, req editRequest) {
	cfg, src, _, _ := loadEdit(config, req, "remove")
	res, err := internal.RemoveFunction(cfg, src, internal.FuncTarget{Name: req.name, Line: req.line, Kind: req.kind})
	if err != nil {
		internal.FatalError("%v", err)
	}
	report(req, src, res, fmt.Sprintf("%s: lines %d-%d", req.name, res.Start, res.End))
}

// loadEdit reads the new text first and the file after it, so the file is as it is when the edit is prepared, not
// as it was before a long wait on standard input. Unless kind is "remove", it reads the new text; only "replace"
// takes --old.
func loadEdit(config internal.Config, req editRequest, kind string) (*internal.LanguageConfig, []byte, []byte, []byte) {
	if req.inp == "" {
		internal.FatalError("this edit requires --inp")
	}
	if req.source == "" {
		internal.FatalError("this edit requires --source <language>")
	}
	if kind != "remove" && req.withPath == "" {
		internal.FatalError("this edit requires --with <file> (or - for stdin)")
	}
	if kind != "replace" && req.oldPath != "" {
		internal.FatalError("--old applies to --replace-func only")
	}
	cfg, err := config.GetLanguageConfig(req.source)
	if err != nil {
		internal.FatalError("%v", err)
	}
	// A wrong --source for the file is the most common slip; the extension is the cheap check against it.
	if !hasExtension(cfg, req.inp) {
		internal.FatalError("%s does not have an extension of --source %s (%s); check --source", req.inp, req.source, strings.Join(cfg.Extensions, ", "))
	}
	var replacement []byte
	if kind != "remove" {
		if req.withPath == "-" {
			replacement, err = io.ReadAll(os.Stdin)
		} else {
			replacement, err = os.ReadFile(req.withPath)
		}
		if err != nil {
			internal.FatalError("reading replacement: %v", err)
		}
	}
	src, err := os.ReadFile(req.inp)
	if err != nil {
		internal.FatalError("reading %s: %v", req.inp, err)
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

// report prints the outcome, or writes the file when apply is set. Writing happens only if the file still holds
// the bytes the edit was prepared from; a change made in the meantime is not overwritten.
func report(req editRequest, src []byte, res *internal.ReplaceResult, what string) {
	// The preview shows what leaves the file and what enters it, so the change can be read before --apply.
	for _, line := range res.Removed {
		fmt.Printf("- %s\n", line)
	}
	for _, line := range res.Added {
		fmt.Printf("+ %s\n", line)
	}
	if !req.apply {
		fmt.Printf("%s: valid, not written. Re-run with --apply to write it.\n", what)
		return
	}
	now, err := os.ReadFile(req.inp)
	if err != nil {
		internal.FatalError("%v", err)
	}
	if !bytes.Equal(now, src) {
		internal.FatalError("%s changed while the edit was being prepared; nothing written. Run the command again.", req.inp)
	}
	info, err := os.Stat(req.inp)
	if err != nil {
		internal.FatalError("%v", err)
	}
	if err := os.WriteFile(req.inp, res.Output, info.Mode().Perm()); err != nil {
		internal.FatalError("writing %s: %v", req.inp, err)
	}
	fmt.Printf("%s: written to %s\n", what, req.inp)
}

// hasExtension reports whether the file name ends with one of the language's extensions.
func hasExtension(cfg *internal.LanguageConfig, path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	for _, e := range cfg.Extensions {
		if strings.ToLower(e) == ext {
			return true
		}
	}
	return false
}

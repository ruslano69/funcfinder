package main

import (
	"fmt"
	"io"
	"os"

	"github.com/ruslano69/funcfinder/internal"
)

// handleReplaceFunc replaces one top-level Go function in a file. The new function is read from withPath ("-" for
// stdin), so the replacement never passes through a shell. oldPath, when given, holds the function text the caller
// expects to replace; the edit is refused if the file differs. Without apply it only validates and reports.
func handleReplaceFunc(config internal.Config, inp, source, name string, line int, withPath, oldPath string, apply bool) {
	if inp == "" {
		internal.FatalError("--replace-func requires --inp")
	}
	if source != "go" {
		internal.FatalError("--replace-func supports only --source go for now")
	}
	if withPath == "" {
		internal.FatalError("--replace-func requires --with <file> (or - for stdin)")
	}

	cfg, err := config.GetLanguageConfig("go")
	if err != nil {
		internal.FatalError("%v", err)
	}
	src, err := os.ReadFile(inp)
	if err != nil {
		internal.FatalError("reading %s: %v", inp, err)
	}
	var replacement []byte
	if withPath == "-" {
		replacement, err = io.ReadAll(os.Stdin)
	} else {
		replacement, err = os.ReadFile(withPath)
	}
	if err != nil {
		internal.FatalError("reading replacement: %v", err)
	}
	var expected []byte
	if oldPath != "" {
		expected, err = os.ReadFile(oldPath)
		if err != nil {
			internal.FatalError("reading --old: %v", err)
		}
	}

	res, err := internal.ReplaceGoFunction(cfg, src, internal.GoFuncTarget{Name: name, Line: line}, expected, replacement)
	if err != nil {
		internal.FatalError("%v", err)
	}

	if !apply {
		fmt.Printf("%s: lines %d-%d would be replaced; the new file is valid. Re-run with --apply to write it.\n",
			name, res.Start, res.End)
		return
	}
	info, err := os.Stat(inp)
	if err != nil {
		internal.FatalError("%v", err)
	}
	if err := os.WriteFile(inp, res.Output, info.Mode().Perm()); err != nil {
		internal.FatalError("writing %s: %v", inp, err)
	}
	fmt.Printf("%s: replaced lines %d-%d in %s\n", name, res.Start, res.End, inp)
}

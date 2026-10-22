package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/ruslano69/funcfinder/internal"
)

func handleObjectCLI(read, edit, path, language, kind string, line, column int, apply bool) int {
	fail := func(err error) int {
		e, ok := err.(*internal.ObjectError)
		if !ok {
			e = &internal.ObjectError{Code: "io_error", Message: err.Error()}
		}
		_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"status": "error", "error": e})
		return 1
	}
	allowed := map[string]bool{"json": true}
	if read != "" && edit == "" {
		for _, k := range []string{"read-object", "inp", "source", "object-kind", "line", "column"} {
			allowed[k] = true
		}
	} else if edit != "" && read == "" {
		allowed["edit-object"], allowed["apply"] = true, true
	} else {
		return fail(&internal.ObjectError{Code: "invalid_options", Message: "choose read-object or edit-object"})
	}
	var bad string
	flag.Visit(func(f *flag.Flag) {
		if !allowed[f.Name] {
			bad = f.Name
		}
	})
	if bad != "" || flag.NArg() != 0 {
		return fail(&internal.ObjectError{Code: "invalid_options", Message: "unexpected option/argument in object mode: " + bad})
	}
	if read != "" {
		if path == "" || language == "" {
			return fail(&internal.ObjectError{Code: "invalid_options", Message: "read-object requires --inp and --source"})
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return fail(err)
		}
		obj, err := internal.ReadCodeObject(path, language, kind, read, line, column, src)
		if err != nil {
			return fail(err)
		}
		if err := json.NewEncoder(os.Stdout).Encode(obj); err != nil {
			return 1
		}
		return 0
	}
	var reader io.Reader = os.Stdin
	if edit != "-" {
		f, err := os.Open(edit)
		if err != nil {
			return fail(err)
		}
		defer f.Close()
		reader = f
	}
	edits, legacy, err := decodeCodeEdits(reader)
	if err != nil {
		return fail(err)
	}
	obj := edits[0].Object
	info, err := os.Lstat(obj.Path)
	if err != nil {
		return fail(err)
	}
	if !info.Mode().IsRegular() {
		return fail(&internal.ObjectError{Code: "invalid_path", Message: "object path must be a regular file, not a symlink"})
	}
	src, err := os.ReadFile(obj.Path)
	if err != nil {
		return fail(err)
	}
	out, results, err := internal.PlanCodeEdits(edits, src)
	if err != nil {
		return fail(err)
	}
	changed := !bytes.Equal(src, out)
	status := "preview"
	if apply {
		if changed {
			if err := writeCodeObject(obj.Path, src, out, info.Mode()); err != nil {
				return fail(err)
			}
		}
		status = "applied"
	}
	validation := "go_syntax_and_declaration_identity"
	if obj.Language == "rust" {
		validation = "rust_tokens_and_declaration_identity"
	}
	response := map[string]any{"status": status, "changed": changed, "results": results, "validation": validation}
	if legacy {
		response["object"] = results[0].Objects[0]
		delete(response, "results")
	}
	if err := json.NewEncoder(os.Stdout).Encode(response); err != nil {
		return 1
	}
	return 0
}

// Stage in the same directory and rename, so write failure cannot truncate the
// source. Recheck immediately before rename; this is not a cross-process lock.
func writeCodeObject(path string, before, after []byte, mode os.FileMode) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".funcfinder-edit-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err = f.Chmod(mode.Perm()); err == nil {
		_, err = f.Write(after)
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	current, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if !bytes.Equal(current, before) {
		return &internal.ObjectError{Code: "stale_revision", Message: "file changed while preparing the write"}
	}
	if err = os.Rename(f.Name(), path); err != nil {
		return fmt.Errorf("replacing file: %w", err)
	}
	return nil
}

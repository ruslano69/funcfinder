package internal

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
	"strings"
)

// CodeEdit wraps an existing object without changing its identity. Source is
// insertion text only; replacement continues to use Object.Source.
type CodeEdit struct {
	Operation string     `json:"operation"`
	Object    CodeObject `json:"object"`
	Source    string     `json:"source,omitempty"`
}
type CodeEditBatch struct {
	Protocol string     `json:"protocol"`
	Edits    []CodeEdit `json:"edits"`
}
type CodeEditResult struct {
	Operation string       `json:"operation"`
	ID        string       `json:"id"`
	Revision  string       `json:"revision"`
	Objects   []CodeObject `json:"objects"`
}
type codePatch struct {
	start, end, edit int
	text             string
}

// codeCatalog includes non-editable declarations as well: a trailing comment
// must never be able to swallow a neighboring var/import declaration.
func codeCatalog(src []byte) ([]string, error) {
	f, err := parser.ParseFile(token.NewFileSet(), "", src, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	var keys []string
	for _, d := range f.Decls {
		if gd, ok := d.(*ast.GenDecl); ok && gd.Tok == token.TYPE {
			for _, s := range gd.Specs {
				keys = append(keys, "type "+s.(*ast.TypeSpec).Name.Name)
			}
		} else {
			keys = append(keys, goSharedIdentity(d))
		}
	}
	return keys, nil
}

// PlanCodeEdits applies a transaction to one original file snapshot in memory.
// There are no intermediate writes or revisions; overlap/invalid final syntax
// refuses the entire batch. Results follow request order, not source order.
func PlanCodeEdits(edits []CodeEdit, src []byte) ([]byte, []CodeEditResult, error) {
	if len(edits) > 0 && edits[0].Object.Language == "rust" {
		return planRustCodeEdits(edits, src)
	}
	if len(edits) == 0 {
		return nil, nil, objectError("empty_batch", "at least one edit is required")
	}
	path := edits[0].Object.Path
	revision := codeRevision(src)
	for _, e := range edits {
		if e.Object.Protocol != "funcfinder.code/v1" {
			return nil, nil, objectError("unsupported_protocol", "expected funcfinder.code/v1 objects")
		}
		if e.Object.Language != "go" {
			return nil, nil, objectError("unsupported_language", "code edits currently support Go only")
		}
		if e.Object.Path != path {
			return nil, nil, objectError("multiple_files", "a batch must target one file")
		}
		if e.Object.Revision != revision {
			return nil, nil, objectError("stale_revision", "all edits must use the same current file snapshot")
		}
	}
	spans, err := goCodeObjects(path, src)
	if err != nil {
		return nil, nil, err
	}
	byID := map[string]int{}
	for i, s := range spans {
		byID[s.object.ID] = i
	}
	patches := []codePatch{}
	removes, replaces, inserts := map[string]int{}, map[string]int{}, map[string]int{}
	insertKeys := map[int][]string{}
	insertObjectCount := map[int]int{}
	for i, e := range edits {
		if e.Object.Path != path {
			return nil, nil, objectError("multiple_files", "a batch must target one file; read all objects from the same snapshot")
		}
		// Validate every original envelope before computing any patch.
		original := e.Object
		idx, exists := byID[original.ID]
		if exists {
			original.Source = spans[idx].object.Source
		}
		if _, _, err := ReplaceCodeObject(original, src); err != nil {
			return nil, nil, err
		}
		s := spans[idx]
		switch e.Operation {
		case "replace":
			if e.Source != "" {
				return nil, nil, objectError("invalid_operation", "replacement uses object.source, not source on the edit wrapper")
			}
			if _, ok := replaces[e.Object.ID]; ok {
				return nil, nil, objectError("overlapping_edits", "declaration replaced more than once")
			}
			replaces[e.Object.ID] = i
			out, _, err := ReplaceCodeObject(e.Object, src)
			if err != nil {
				return nil, nil, err
			}
			patches = append(patches, codePatch{s.start, s.end, i, string(out[s.start : len(out)-(len(src)-s.end)])})
		case "remove":
			if e.Source != "" || e.Object.Source != s.object.Source {
				return nil, nil, objectError("invalid_operation", "removal takes an unchanged object and no insertion source")
			}
			if _, ok := removes[e.Object.ID]; ok {
				return nil, nil, objectError("overlapping_edits", "declaration removed more than once")
			}
			removes[e.Object.ID] = i
			end := s.end
			text := ""
			if semi := goFollowingSemicolon(src, end); semi >= 0 {
				text = string(src[end:semi])
				end = semi + 1
			}
			patches = append(patches, codePatch{s.start, end, i, text})
		case "insert_after":
			if e.Object.Source != s.object.Source {
				return nil, nil, objectError("invalid_operation", "insertion anchor must be the unchanged read object")
			}
			if _, ok := inserts[e.Object.ID]; ok {
				return nil, nil, objectError("overlapping_edits", "combine insertions after one anchor into a single source")
			}
			inserts[e.Object.ID] = i
			payload := strings.ReplaceAll(e.Source, "\r\n", "\n")
			const prefix = "package edit\n"
			fs := token.NewFileSet()
			pf, err := parser.ParseFile(fs, "", prefix+payload+"\n", parser.ParseComments|parser.SkipObjectResolution)
			if err != nil || len(pf.Decls) == 0 {
				return nil, nil, objectError("invalid_replacement", "insertion must contain complete Go functions or types")
			}
			var members []string
			for _, d := range pf.Decls {
				switch d := d.(type) {
				case *ast.FuncDecl:
					if s.grouped {
						return nil, nil, objectError("invalid_replacement", "only type specs can be inserted into a type group")
					}
				case *ast.GenDecl:
					if d.Tok != token.TYPE {
						return nil, nil, objectError("invalid_replacement", "insertion accepts functions and types only")
					}
					for _, spec := range d.Specs {
						members = append(members, payload[fs.PositionFor(spec.Pos(), false).Offset-len(prefix):fs.PositionFor(spec.End(), false).Offset-len(prefix)])
					}
				default:
					return nil, nil, objectError("invalid_replacement", "unsupported insertion declaration")
				}
			}
			keys, _ := codeCatalog([]byte(prefix + payload + "\n"))
			insertKeys[i] = keys
			inserted, _ := goCodeObjects(path, []byte(prefix+payload+"\n"))
			insertObjectCount[i] = len(inserted)
			if s.grouped {
				// Prefix/trailing docs must not disappear during type-keyword removal.
				for _, c := range pf.Comments {
					contained := false
					for _, d := range pf.Decls {
						for _, spec := range d.(*ast.GenDecl).Specs {
							if spec.Pos() <= c.Pos() && c.End() <= spec.End() {
								contained = true
							}
						}
					}
					if !contained {
						return nil, nil, objectError("invalid_replacement", "cannot relocate declaration comments into a grouped spec")
					}
				}
				payload = strings.Join(members, "; ")
			}
			if bytes.Contains(src, []byte("\r\n")) {
				payload = strings.ReplaceAll(payload, "\n", "\r\n")
			}
			at := s.end
			separator := "; "
			if semi := goFollowingSemicolon(src, at); semi >= 0 {
				at = semi + 1
				separator = " "
			}
			lineStart := bytes.LastIndexByte(src[:s.start], '\n') + 1
			lineEnd := s.end
			if n := bytes.IndexByte(src[s.end:], '\n'); n >= 0 {
				lineEnd += n
			} else {
				lineEnd = len(src)
			}
			if strings.TrimSpace(string(src[lineStart:s.start])) == "" && strings.TrimSpace(string(src[s.end:lineEnd])) == "" {
				// Preserve readable declaration separation on ordinary lines.
				eol := "\n"
				if bytes.Contains(src, []byte("\r\n")) {
					eol = "\r\n"
				}
				separator = eol + eol
				if s.grouped {
					separator = eol + string(src[lineStart:s.start])
				}
				patches = append(patches, codePatch{s.end, s.end, i, separator + payload})
				continue
			}
			// Keep explicit boundaries even when the anchor ends at EOF or next
			// to a comment/another declaration. Whole-file parse validates context.
			patches = append(patches, codePatch{at, at, i, separator + payload + ";"})
		default:
			return nil, nil, objectError("invalid_operation", "operation must be replace, insert_after or remove")
		}
	}
	for id := range removes {
		if _, ok := replaces[id]; ok {
			return nil, nil, objectError("overlapping_edits", "cannot replace and remove the same declaration")
		}
	}
	sort.SliceStable(patches, func(i, j int) bool {
		if patches[i].start != patches[j].start {
			return patches[i].start < patches[j].start
		}
		return patches[i].end < patches[j].end
	})
	var out []byte
	cursor := 0
	lastInsert := -1
	for _, p := range patches {
		if p.start < cursor || (p.start == p.end && p.start == lastInsert) {
			return nil, nil, objectError("overlapping_edits", "edit ranges overlap")
		}
		out = append(out, src[cursor:p.start]...)
		out = append(out, p.text...)
		cursor = p.end
		if p.start == p.end {
			lastInsert = p.start
		}
	}
	out = append(out, src[cursor:]...)
	before, _ := codeCatalog(src)
	actual, err := codeCatalog(out)
	if err != nil {
		return nil, nil, objectError("invalid_result", err.Error())
	}
	// Catalog and object order coincide for editable entries. Track the result
	// indices while walking the original catalog, including vars/imports.
	results := make([]CodeEditResult, len(edits))
	nextRevision := codeRevision(out)
	for i, e := range edits {
		results[i] = CodeEditResult{Operation: e.Operation, ID: e.Object.ID, Revision: nextRevision, Objects: []CodeObject{}}
	}
	resultIndices := map[int][]int{}
	var expected []string
	spanIndex, objectIndex := 0, 0
	for _, key := range before {
		editable := strings.HasPrefix(key, "func ") || strings.HasPrefix(key, "type ")
		if !editable {
			expected = append(expected, key)
			continue
		}
		id := spans[spanIndex].object.ID
		spanIndex++
		if _, removed := removes[id]; !removed {
			expected = append(expected, key)
			if edit, ok := replaces[id]; ok {
				resultIndices[edit] = append(resultIndices[edit], objectIndex)
			}
			objectIndex++
		}
		if edit, ok := inserts[id]; ok {
			expected = append(expected, insertKeys[edit]...)
			for n := 0; n < insertObjectCount[edit]; n++ {
				resultIndices[edit] = append(resultIndices[edit], objectIndex)
				objectIndex++
			}
		}
	}
	if !sameNames(expected, actual) {
		return nil, nil, objectError("invalid_result", "edit changes neighboring declarations or insertion order")
	}
	after, err := goCodeObjects(path, out)
	if err != nil {
		return nil, nil, err
	}
	if len(after) != objectIndex {
		return nil, nil, objectError("invalid_result", "unexpected object count")
	}
	if err := rejectNewObjectDuplicates(src, out); err != nil {
		return nil, nil, err
	}
	for edit, indices := range resultIndices {
		for _, index := range indices {
			results[edit].Objects = append(results[edit].Objects, after[index].object)
		}
	}
	return out, results, nil
}

func rejectNewObjectDuplicates(before, after []byte) error {
	count := func(src []byte) map[string]int {
		m := map[string]int{}
		f, _ := parser.ParseFile(token.NewFileSet(), "", src, parser.SkipObjectResolution)
		add := func(name string) {
			if name != "_" {
				m[name]++
			}
		}
		for _, d := range f.Decls {
			switch d := d.(type) {
			case *ast.FuncDecl:
				if d.Recv == nil {
					if d.Name.Name != "init" {
						add(d.Name.Name)
					}
				} else {
					key := strings.TrimPrefix(goSharedIdentity(d), "func ")
					add("method " + strings.TrimPrefix(key, "*"))
				}
			case *ast.GenDecl:
				for _, s := range d.Specs {
					switch s := s.(type) {
					case *ast.TypeSpec:
						add(s.Name.Name)
					case *ast.ValueSpec:
						for _, n := range s.Names {
							add(n.Name)
						}
					case *ast.ImportSpec:
						if s.Name != nil && s.Name.Name != "." {
							add(s.Name.Name)
						}
					}
				}
			}
		}
		return m
	}
	old, current := count(before), count(after)
	for key, n := range current {
		if n > 1 && n > old[key] {
			return objectError("duplicate_declaration", "edit introduces a duplicate declaration: "+key)
		}
	}
	return nil
}

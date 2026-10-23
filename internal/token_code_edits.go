package internal

import (
	"bytes"
	"sort"
	"strings"
)

func planTokenCodeEdits(edits []CodeEdit, src []byte, language string, catalog func(string, []byte, string, string) ([]rustObjectItem, error), tokenize func([]byte) ([]rustObjectToken, error)) ([]byte, []CodeEditResult, error) {
	if len(edits) == 0 {
		return nil, nil, objectError("empty_batch", "at least one edit is required")
	}
	path := edits[0].Object.Path
	revision := codeRevision(src)
	for _, e := range edits {
		if e.Object.Protocol != "funcfinder.code/v1" {
			return nil, nil, objectError("unsupported_protocol", "expected funcfinder.code/v1")
		}
		if e.Object.Language != language {
			return nil, nil, objectError("unsupported_language", "all objects in a token transaction must use its backend language")
		}
		if e.Object.Path != path {
			return nil, nil, objectError("multiple_files", "a batch must target one file")
		}
		if e.Object.Revision != revision {
			return nil, nil, objectError("stale_revision", "file changed since these objects were read")
		}
	}
	if bytes.Contains(src, []byte("\r\n")) && bytes.Count(src, []byte("\r\n")) != bytes.Count(src, []byte("\n")) {
		return nil, nil, objectError("mixed_endings", "mixed line endings are not supported")
	}
	items, err := catalog(path, src, "", "module")
	if err != nil {
		return nil, nil, err
	}
	var patches []rustObjectPatch
	for i, e := range edits {
		var selected *rustObjectItem
		for j := range items {
			o := items[j].object
			if o.ID == e.Object.ID && o.Path == e.Object.Path && o.Language == e.Object.Language && o.Kind == e.Object.Kind && o.Name == e.Object.Name && o.Line == e.Object.Line && o.Column == e.Object.Column && o.EndLine == e.Object.EndLine {
				selected = &items[j]
				break
			}
		}
		if selected == nil {
			return nil, nil, objectError("invalid_identity", "token object identity/coordinates do not match this file")
		}
		s := *selected
		patch := rustObjectPatch{start: s.start, end: s.end, edit: i}
		switch e.Operation {
		case "remove":
			if e.Source != "" || e.Object.Source != s.object.Source {
				return nil, nil, objectError("invalid_operation", "removal takes an unchanged object and no source")
			}
			if language == "js" || language == "ts" {
				var err error
				patch.start, patch.end, patch.text, err = scriptObjectPatch(s, "", nil, e.Operation, language, src)
				if err != nil {
					return nil, nil, err
				}
			}
		case "replace", "insert_after":
			payload := e.Object.Source
			if e.Operation == "insert_after" {
				if e.Object.Source != s.object.Source {
					return nil, nil, objectError("invalid_operation", "insertion requires an unchanged anchor")
				}
				payload = e.Source
				patch.start, patch.end = s.end, s.end
			} else if e.Source != "" {
				return nil, nil, objectError("invalid_operation", "replacement uses object.source")
			}
			payload = strings.ReplaceAll(payload, "\r\n", "\n")
			parsed, err := catalog(path, []byte(payload), s.scope, s.context)
			if err != nil {
				return nil, nil, objectError("invalid_replacement", err.Error())
			}
			var roots []rustObjectItem
			for _, n := range parsed {
				if n.depth == 0 {
					roots = append(roots, n)
				}
			}
			if len(roots) == 0 {
				return nil, nil, objectError("invalid_replacement", "source defines no named item")
			}
			tokens, _ := tokenize([]byte(payload))
			if len(tokens) == 0 || roots[0].start != tokens[0].start && !((language == "js" || language == "ts") && strings.HasPrefix(roots[0].context, "binding:") && tokens[0].text == strings.TrimPrefix(roots[0].context, "binding:")) {
				return nil, nil, objectError("invalid_replacement", "inner attributes outside the edited item are refused")
			}
			for _, n := range roots {
				if n.object.Kind == "" {
					return nil, nil, objectError("invalid_replacement", "only named editable declarations can be inserted/replaced")
				}
			}
			if e.Operation == "replace" && (len(roots) != 1 || roots[0].object.Kind != s.object.Kind || roots[0].object.Name != s.object.Name || roots[0].attributes != s.attributes) {
				return nil, nil, objectError("contract_changed", "replacement must preserve item name, kind and outer attributes")
			}
			patch.items = parsed
			if language == "js" || language == "ts" {
				patch.start, patch.end, patch.text, err = scriptObjectPatch(s, payload, parsed, e.Operation, language, src)
				if err != nil {
					return nil, nil, err
				}
				if bytes.Contains(src, []byte("\r\n")) {
					patch.text = strings.ReplaceAll(patch.text, "\n", "\r\n")
				}
				patches = append(patches, patch)
				continue
			}

			if bytes.Contains(src, []byte("\r\n")) {
				payload = strings.ReplaceAll(payload, "\n", "\r\n")
			}
			if e.Operation == "insert_after" {
				eol := "\n"
				if bytes.Contains(src, []byte("\r\n")) {
					eol = "\r\n"
				}
				// Braced items need no semicolon; newline boundaries also
				// prevent a trailing line comment from consuming the suffix.
				payload = eol + payload + eol
			}
			patch.text = payload
		default:
			return nil, nil, objectError("invalid_operation", "operation must be replace, insert_after or remove")
		}
		patches = append(patches, patch)
	}
	sort.SliceStable(patches, func(i, j int) bool {
		if patches[i].start != patches[j].start {
			return patches[i].start < patches[j].start
		}
		return patches[i].end < patches[j].end
	})
	cursor := 0
	lastInsert := -1
	var out []byte
	for _, p := range patches {
		if p.start < cursor || (p.start == p.end && p.start == lastInsert) {
			return nil, nil, objectError("overlapping_edits", "token edit ranges overlap")
		}
		out = append(out, src[cursor:p.start]...)
		out = append(out, p.text...)
		cursor = p.end
		if p.start == p.end {
			lastInsert = p.start
		}
	}
	out = append(out, src[cursor:]...)
	after, err := catalog(path, out, "", "module")
	if err != nil {
		return nil, nil, objectError("invalid_result", err.Error())
	}
	// Construct the expected full item catalogue from original item spans.
	// A replaced/removed container owns all of its nested items.
	var expected []string
	indices := map[int][]int{}
	itemIndex := 0
	for _, patch := range patches {
		for itemIndex < len(items) && items[itemIndex].start < patch.start {
			expected = append(expected, items[itemIndex].key)
			itemIndex++
		}
		for itemIndex < len(items) && items[itemIndex].start < patch.end {
			itemIndex++
		}
		for _, n := range patch.items {
			if n.depth == 0 && n.object.Kind != "" {
				indices[patch.edit] = append(indices[patch.edit], len(expected))
			}
			expected = append(expected, n.key)
		}
	}
	for itemIndex < len(items) {
		expected = append(expected, items[itemIndex].key)
		itemIndex++
	}
	actual := make([]string, len(after))
	for i, n := range after {
		actual[i] = n.key
	}
	if !sameNames(expected, actual) {
		return nil, nil, objectError("invalid_result", "edit changes neighboring items or scope")
	}
	// Names are checked within lexical namespaces; cfg and trait resolution
	// remain compiler responsibilities rather than inferred here.
	count := func(items []rustObjectItem) map[string]int {
		m := map[string]int{}
		for _, n := range items {
			if n.object.Kind != "" {
				key := n.scope + "/" + n.object.Kind + ":" + n.object.Name
				if language == "goose" || language == "kotlin" || language == "cs" || language == "d" || language == "js" || language == "ts" {
					key = n.key
				}
				m[key]++
			}
		}
		return m
	}
	oldCounts, newCounts := count(items), count(after)
	for k, n := range newCounts {
		if n > 1 && n > oldCounts[k] {
			return nil, nil, objectError("duplicate_declaration", "edit introduces a duplicate named item")
		}
	}
	results := make([]CodeEditResult, len(edits))
	nextRevision := codeRevision(out)
	for i, e := range edits {
		results[i] = CodeEditResult{Operation: e.Operation, ID: e.Object.ID, Revision: nextRevision, Objects: []CodeObject{}}
		for _, index := range indices[i] {
			if index >= len(after) || after[index].object.Kind == "" {
				return nil, nil, objectError("invalid_result", "edited item disappeared")
			}
			results[i].Objects = append(results[i].Objects, after[index].object)
		}
	}
	return out, results, nil
}

package internal

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestScriptCodeObjectsCorpus(t *testing.T) {
	path := os.Getenv("FUNC_SCRIPT_ORACLE")
	if path == "" {
		t.Skip("set FUNC_SCRIPT_ORACLE to the independent TypeScript AST report")
	}
	var oracle struct {
		TypeScript     string          `json:"typescript"`
		Root           string          `json:"root"`
		Revision       string          `json:"revision"`
		Implementation json.RawMessage `json:"implementation"`
		Files          []struct {
			Path, Language, SHA256 string
			Errors                 []string
			Objects                []CodeObject
		} `json:"files"`
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &oracle); err != nil {
		t.Fatal(err)
	}
	type finding struct {
		Path, Reason, Name, Kind, Source, Want string
		Line, Column                           int
	}
	var findings, excluded, previewRefusals []finding
	files, objects, previews := 0, 0, 0
	for _, file := range oracle.Files {
		if len(file.Errors) > 0 {
			excluded = append(excluded, finding{Path: file.Path, Reason: strings.Join(file.Errors, "; ")})
			continue
		}
		source, err := os.ReadFile(file.Path)
		if err != nil {
			t.Fatal(err)
		}
		hash := sha256.Sum256(source)
		if hex.EncodeToString(hash[:]) != file.SHA256 {
			t.Fatalf("corpus changed: %s", file.Path)
		}
		items, err := scriptCodeItems(file.Path, source, "", "module", file.Language)
		if err != nil {
			excluded = append(excluded, finding{Path: file.Path, Reason: err.Error()})
			continue
		}
		files++
		key := func(obj CodeObject) string {
			return obj.Kind + ":" + obj.Name + ":" + codeObjectID("", obj.Kind, obj.Name, obj.Line, obj.Column)
		}
		actual := map[string]CodeObject{}
		for _, item := range items {
			if item.object.Kind != "" {
				actual[key(item.object)] = item.object
			}
		}
		for _, want := range file.Objects {
			obj, ok := actual[key(want)]
			if !ok {
				findings = append(findings, finding{Path: file.Path, Reason: "missing", Name: want.Name, Kind: want.Kind, Line: want.Line, Column: want.Column, Want: want.Source})
				continue
			}
			delete(actual, key(want))
			if obj.Source != want.Source {
				findings = append(findings, finding{Path: file.Path, Reason: "source span", Name: obj.Name, Kind: obj.Kind, Line: obj.Line, Column: obj.Column, Source: obj.Source, Want: want.Source})
				continue
			}
			objects++
			out, _, err := ReplaceCodeObject(obj, source)
			if object, ok := err.(*ObjectError); ok && object.Code == "mixed_endings" {
				previewRefusals = append(previewRefusals, finding{Path: file.Path, Reason: object.Code, Name: obj.Name})
				continue
			}
			if err != nil || string(out) != string(source) {
				reason := "identity changed source"
				if err != nil {
					reason = err.Error()
				}
				findings = append(findings, finding{Path: file.Path, Reason: reason, Name: obj.Name})
			} else {
				previews++
			}
		}
		for _, obj := range actual {
			findings = append(findings, finding{Path: file.Path, Reason: "false positive", Name: obj.Name, Kind: obj.Kind, Line: obj.Line, Column: obj.Column, Source: obj.Source})
		}
	}
	report := map[string]any{"typescript": oracle.TypeScript, "root": oracle.Root, "corpus_revision": oracle.Revision, "implementation": oracle.Implementation, "accepted_files": files, "exact_objects": objects, "identity_previews": previews, "excluded": excluded, "findings": findings, "preview_refusals": previewRefusals}
	if output := os.Getenv("FUNC_SCRIPT_REPORT"); output != "" {
		encoded, _ := json.MarshalIndent(report, "", "  ")
		if err := os.WriteFile(output, encoded, 0600); err != nil {
			t.Fatal(err)
		}
	}
	t.Logf("files=%d exact=%d previews=%d excluded=%d findings=%d", files, objects, previews, len(excluded), len(findings))
	if len(findings) > 0 {
		for i, finding := range findings {
			if i == 12 {
				break
			}
			t.Logf("%s: %s %s %s:%d:%d", finding.Path, finding.Reason, finding.Kind, finding.Name, finding.Line, finding.Column)
		}
		t.Fatalf("%d source-truth differences; see FUNC_SCRIPT_REPORT", len(findings))
	}
}

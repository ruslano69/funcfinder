package main

import (
	"bytes"
	"encoding/json"
	"io"

	"github.com/ruslano69/funcfinder/internal"
)

func decodeCodeEdits(reader io.Reader) ([]internal.CodeEdit, bool, error) {
	bad := func(message string) ([]internal.CodeEdit, bool, error) {
		return nil, false, &internal.ObjectError{Code: "invalid_json", Message: message}
	}
	decoder := json.NewDecoder(reader)
	var raw json.RawMessage
	if err := decoder.Decode(&raw); err != nil {
		return bad(err.Error())
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return bad("expected exactly one JSON document")
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil || fields == nil {
		return bad("expected a JSON object")
	}
	decode := func(value any) error {
		d := json.NewDecoder(bytes.NewReader(raw))
		d.DisallowUnknownFields()
		return d.Decode(value)
	}
	if _, batch := fields["edits"]; batch {
		var req internal.CodeEditBatch
		if err := decode(&req); err != nil {
			return bad(err.Error())
		}
		if req.Protocol != "funcfinder.edits/v1" {
			return nil, false, &internal.ObjectError{Code: "unsupported_protocol", Message: "expected funcfinder.edits/v1"}
		}
		if len(req.Edits) == 0 {
			return nil, false, &internal.ObjectError{Code: "empty_batch", Message: "at least one edit is required"}
		}
		return req.Edits, false, nil
	}
	if _, wrapped := fields["operation"]; wrapped {
		var req internal.CodeEdit
		if err := decode(&req); err != nil {
			return bad(err.Error())
		}
		return []internal.CodeEdit{req}, false, nil
	}
	var obj internal.CodeObject
	if err := decode(&obj); err != nil {
		return bad(err.Error())
	}
	return []internal.CodeEdit{{Operation: "replace", Object: obj}}, true, nil
}

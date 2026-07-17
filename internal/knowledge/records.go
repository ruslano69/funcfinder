package knowledge

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
)

// Record is one structured document for JSON ingest (TZ FR-1/FR-2):
// a changelog entry, a closed task, a decision — type and provenance
// declared per record, not inferred from file-wide chunking the way
// .md/.txt/.pdf ingest works.
type Record struct {
	Title         string `json:"title"`
	Content       string `json:"content"`
	Type          string `json:"type"`           // changelog|task|decision|... (default "changelog")
	Author        string `json:"author,omitempty"`
	RoleTags      string `json:"role_tags,omitempty"`
	SourceVersion string `json:"source_version,omitempty"`
	SourceRef     string `json:"source_ref,omitempty"`
}

// ParseRecordsFile reads a JSON file holding either a single record object
// or an array of records.
func ParseRecordsFile(path string) ([]Record, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return ParseRecords(data)
}

// ParseRecords parses raw JSON bytes as either a single record object or an
// array of records, and validates that every record has the fields ingest
// actually requires.
func ParseRecords(data []byte) ([]Record, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("empty JSON input")
	}

	var records []Record
	if trimmed[0] == '[' {
		if err := json.Unmarshal(trimmed, &records); err != nil {
			return nil, fmt.Errorf("parse JSON array: %w", err)
		}
	} else {
		var single Record
		if err := json.Unmarshal(trimmed, &single); err != nil {
			return nil, fmt.Errorf("parse JSON object: %w", err)
		}
		records = []Record{single}
	}

	for i, r := range records {
		if r.Title == "" {
			return nil, fmt.Errorf("record %d: title is required", i)
		}
		if r.Content == "" {
			return nil, fmt.Errorf("record %d (%q): content is required", i, r.Title)
		}
	}
	return records, nil
}

// AddRecords ingests each record via Add, building its metadata blob from
// the record's own author/role_tags/source_version/source_ref (so FR-3's
// generated columns stay populated the same way CLI ingest/record do) and
// defaulting Type to "changelog" when unset. embedFn is called per record
// body when non-nil; a nil return (or a nil embedFn) stores the record
// without a vector, same degrade-to-FTS behavior as file ingest.
func AddRecords(db *sql.DB, records []Record, embedFn func(text string) []float32) ([]int64, error) {
	ids := make([]int64, 0, len(records))
	for _, r := range records {
		docType := r.Type
		if docType == "" {
			docType = "changelog"
		}

		meta := struct {
			Author        string `json:"author,omitempty"`
			RoleTags      string `json:"role_tags,omitempty"`
			SourceVersion string `json:"source_version,omitempty"`
			SourceRef     string `json:"source_ref,omitempty"`
		}{r.Author, r.RoleTags, r.SourceVersion, r.SourceRef}
		metaJSON, err := json.Marshal(meta)
		if err != nil {
			return ids, fmt.Errorf("marshal metadata for %q: %w", r.Title, err)
		}

		var vec []float32
		if embedFn != nil {
			vec = embedFn(r.Content)
		}

		id, err := Add(db, r.Title, r.Content, docType, string(metaJSON), vec)
		if err != nil {
			return ids, fmt.Errorf("add record %q: %w", r.Title, err)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

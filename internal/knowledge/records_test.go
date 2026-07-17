package knowledge

import "testing"

func TestParseRecords_SingleObject(t *testing.T) {
	data := []byte(`{"title":"Fixed bug X","content":"root cause was Y","type":"changelog","author":"ruslan"}`)
	records, err := ParseRecords(data)
	if err != nil {
		t.Fatalf("ParseRecords: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("want 1 record, got %d", len(records))
	}
	r := records[0]
	if r.Title != "Fixed bug X" || r.Content != "root cause was Y" || r.Type != "changelog" || r.Author != "ruslan" {
		t.Fatalf("unexpected record: %+v", r)
	}
}

func TestParseRecords_Array(t *testing.T) {
	data := []byte(`[
		{"title":"Task A closed","content":"done","type":"task"},
		{"title":"Decision: use SQLite","content":"single-file, zero-infra","type":"decision","source_ref":"adr-1"}
	]`)
	records, err := ParseRecords(data)
	if err != nil {
		t.Fatalf("ParseRecords: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("want 2 records, got %d", len(records))
	}
	if records[0].Type != "task" || records[1].Type != "decision" || records[1].SourceRef != "adr-1" {
		t.Fatalf("unexpected records: %+v", records)
	}
}

func TestParseRecords_MissingRequiredFields(t *testing.T) {
	cases := []string{
		`{"content":"no title"}`,
		`{"title":"no content"}`,
		`[{"title":"ok","content":"ok"},{"title":"","content":"missing title"}]`,
		``,
		`   `,
	}
	for _, data := range cases {
		if _, err := ParseRecords([]byte(data)); err == nil {
			t.Errorf("ParseRecords(%q): want error, got nil", data)
		}
	}
}

func TestParseRecords_InvalidJSON(t *testing.T) {
	if _, err := ParseRecords([]byte(`{not json`)); err == nil {
		t.Fatal("want error for invalid JSON, got nil")
	}
	if _, err := ParseRecords([]byte(`[{not json}]`)); err == nil {
		t.Fatal("want error for invalid JSON array, got nil")
	}
}

func TestAddRecords(t *testing.T) {
	db := openDB(t)
	records := []Record{
		{Title: "Fixed bug X", Content: "root cause was Y", Type: "changelog", Author: "ruslan", SourceRef: "task-42"},
		{Title: "No type set", Content: "defaults to changelog"},
		{Title: "A decision", Content: "we chose SQLite", Type: "decision", RoleTags: "backend"},
	}

	ids, err := AddRecords(db, records, nil)
	if err != nil {
		t.Fatalf("AddRecords: %v", err)
	}
	if len(ids) != 3 {
		t.Fatalf("want 3 ids, got %d", len(ids))
	}

	n, err := Count(db)
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if n != 3 {
		t.Fatalf("want 3 docs stored, got %d", n)
	}

	docs, err := ReadRange(db, ids[0], 2)
	if err != nil {
		t.Fatalf("ReadRange: %v", err)
	}
	byTitle := map[string]Doc{}
	for _, d := range docs {
		byTitle[d.Title] = d
	}

	fixed, ok := byTitle["Fixed bug X"]
	if !ok {
		t.Fatal("Fixed bug X not found")
	}
	if fixed.Type != "changelog" || fixed.Author != "ruslan" {
		t.Errorf("Fixed bug X: type=%q author=%q, want changelog/ruslan", fixed.Type, fixed.Author)
	}

	defaulted, ok := byTitle["No type set"]
	if !ok {
		t.Fatal("No type set not found")
	}
	if defaulted.Type != "changelog" {
		t.Errorf("want default type=changelog, got %q", defaulted.Type)
	}

	decision, ok := byTitle["A decision"]
	if !ok {
		t.Fatal("A decision not found")
	}
	if decision.Type != "decision" || decision.RoleTags != "backend" {
		t.Errorf("A decision: type=%q role_tags=%q, want decision/backend", decision.Type, decision.RoleTags)
	}
}

func TestAddRecords_EmbedFnCalledPerRecord(t *testing.T) {
	db := openDB(t)
	var calls []string
	embedFn := func(text string) []float32 {
		calls = append(calls, text)
		return []float32{1, 2, 3}
	}

	records := []Record{
		{Title: "A", Content: "content a"},
		{Title: "B", Content: "content b"},
	}
	if _, err := AddRecords(db, records, embedFn); err != nil {
		t.Fatalf("AddRecords: %v", err)
	}
	if len(calls) != 2 || calls[0] != "content a" || calls[1] != "content b" {
		t.Errorf("want embedFn called with each record's content in order, got %v", calls)
	}
}

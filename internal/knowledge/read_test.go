package knowledge

import "testing"

func TestReadRange(t *testing.T) {
	db, err := Open(t.TempDir() + "/r.sqlite")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	var ids []int64
	for i := 0; i < 10; i++ {
		id, err := Add(db, "doc", "chunk body", "spec", "{}", nil)
		if err != nil {
			t.Fatalf("add %d: %v", i, err)
		}
		ids = append(ids, id)
	}
	mid := ids[5] // a chunk in the middle of the run

	// context=2 around the middle id returns the 5 contiguous neighbors.
	docs, err := ReadRange(db, mid, 2)
	if err != nil {
		t.Fatalf("ReadRange: %v", err)
	}
	if len(docs) != 5 {
		t.Fatalf("len(docs) = %d, want 5", len(docs))
	}
	for i, d := range docs {
		want := mid - 2 + int64(i)
		if d.ID != want {
			t.Errorf("docs[%d].ID = %d, want %d (order must be ascending id)", i, d.ID, want)
		}
	}

	// context=0 returns exactly the one requested chunk.
	docs, err = ReadRange(db, mid, 0)
	if err != nil {
		t.Fatalf("ReadRange context=0: %v", err)
	}
	if len(docs) != 1 || docs[0].ID != mid {
		t.Fatalf("ReadRange context=0 = %+v, want exactly [%d]", docs, mid)
	}

	// Negative context is clamped to 0, not an error.
	docs, err = ReadRange(db, mid, -3)
	if err != nil {
		t.Fatalf("ReadRange negative context: %v", err)
	}
	if len(docs) != 1 {
		t.Fatalf("ReadRange negative context len = %d, want 1", len(docs))
	}

	// A range that runs off the start of the table is truncated, not padded
	// or erroring: id=ids[0]-... doesn't exist, so fewer than 2*context+1 come back.
	docs, err = ReadRange(db, ids[0], 3)
	if err != nil {
		t.Fatalf("ReadRange near start: %v", err)
	}
	if len(docs) != 4 { // ids[0]..ids[3], nothing before ids[0]
		t.Fatalf("ReadRange near start len = %d, want 4 (%+v)", len(docs), docs)
	}

	// An id with no rows nearby at all returns an empty (not nil-error) slice.
	docs, err = ReadRange(db, 99999, 1)
	if err != nil {
		t.Fatalf("ReadRange out of range: %v", err)
	}
	if len(docs) != 0 {
		t.Fatalf("ReadRange out of range len = %d, want 0", len(docs))
	}
}

func TestReadBySource(t *testing.T) {
	db, err := Open(t.TempDir() + "/r2.sqlite")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	meta := `{"source_version":"manual.pdf"}`
	otherMeta := `{"source_version":"other.pdf"}`

	var want []int64
	for i := 0; i < 4; i++ {
		id, err := Add(db, "doc", "chunk", "lib_doc", meta, nil)
		if err != nil {
			t.Fatalf("add: %v", err)
		}
		want = append(want, id)
	}
	// Interleave a chunk from a different source — must not leak into the result.
	if _, err := Add(db, "other", "chunk", "lib_doc", otherMeta, nil); err != nil {
		t.Fatalf("add other: %v", err)
	}

	docs, err := ReadBySource(db, "manual.pdf")
	if err != nil {
		t.Fatalf("ReadBySource: %v", err)
	}
	if len(docs) != len(want) {
		t.Fatalf("len(docs) = %d, want %d", len(docs), len(want))
	}
	for i, d := range docs {
		if d.ID != want[i] {
			t.Errorf("docs[%d].ID = %d, want %d (must preserve ingest order)", i, d.ID, want[i])
		}
	}

	// Unknown source_version returns empty, not an error.
	docs, err = ReadBySource(db, "nope.pdf")
	if err != nil {
		t.Fatalf("ReadBySource unknown: %v", err)
	}
	if len(docs) != 0 {
		t.Fatalf("ReadBySource unknown len = %d, want 0", len(docs))
	}
}

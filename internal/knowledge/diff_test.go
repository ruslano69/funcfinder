package knowledge

import "testing"

func TestDiffDocs(t *testing.T) {
	from := openDB(t)
	to := openDB(t)

	// from: docs 1 (stays same), 2 (gets removed), 3 (gets changed)
	id1, err := Add(from, "Stays", "unchanged content", "general", "{}", nil)
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	id2, err := Add(from, "Removed later", "will be gone", "general", "{}", nil)
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	id3, err := Add(from, "Changes later", "old content", "general", "{}", nil)
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	// to: doc 1 identical (same id via explicit insert below), doc 2 absent
	// (removed), doc 3 present but changed, doc 4 new (added).
	if _, err = to.Exec(`INSERT INTO docs (id, title, content, type, metadata) VALUES (?, ?, ?, ?, ?)`,
		id1, "Stays", "unchanged content", "general", "{}"); err != nil {
		t.Fatalf("seed to id1: %v", err)
	}
	if _, err = to.Exec(`INSERT INTO docs (id, title, content, type, metadata) VALUES (?, ?, ?, ?, ?)`,
		id3, "Changes later", "NEW content", "general", "{}"); err != nil {
		t.Fatalf("seed to id3: %v", err)
	}
	id4, err := Add(to, "Brand new", "added content", "general", "{}", nil)
	if err != nil {
		t.Fatalf("Add to id4: %v", err)
	}

	diff, err := DiffDocs(from, to)
	if err != nil {
		t.Fatalf("DiffDocs: %v", err)
	}

	if len(diff.Added) != 1 || diff.Added[0].ID != id4 {
		t.Fatalf("want Added=[%d], got %+v", id4, diff.Added)
	}
	if len(diff.Removed) != 1 || diff.Removed[0].ID != id2 {
		t.Fatalf("want Removed=[%d], got %+v", id2, diff.Removed)
	}
	if len(diff.Changed) != 1 || diff.Changed[0].ID != id3 {
		t.Fatalf("want Changed=[%d], got %+v", id3, diff.Changed)
	}
}

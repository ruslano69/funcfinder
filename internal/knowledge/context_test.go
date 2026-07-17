package knowledge

import "testing"

// TestByRole verifies the FR-9 role filter matches whole tags within the
// comma-separated role_tags list, not substrings — "backend" must not match
// "backend2" or "frontend-backend-liaison".
func TestByRole(t *testing.T) {
	db := openDB(t)

	add := func(title, roleTags string) {
		t.Helper()
		meta := `{"role_tags":"` + roleTags + `"}`
		if _, err := Add(db, title, "content", "general", meta, nil); err != nil {
			t.Fatalf("Add %q: %v", title, err)
		}
	}
	add("Backend doc", "backend,security")
	add("Frontend doc", "frontend")
	add("Backend2 doc", "backend2")
	add("Liaison doc", "frontend-backend-liaison")
	add("No tags doc", "")

	docs, err := ByRole(db, "backend", 10)
	if err != nil {
		t.Fatalf("ByRole: %v", err)
	}
	if len(docs) != 1 || docs[0].Title != "Backend doc" {
		t.Fatalf("want exactly [Backend doc], got %+v", docs)
	}
}

// TestByRole_Limit verifies the limit is honored and results come back
// newest first.
func TestByRole_Limit(t *testing.T) {
	db := openDB(t)
	for _, title := range []string{"First", "Second", "Third"} {
		if _, err := Add(db, title, "content", "general", `{"role_tags":"ops"}`, nil); err != nil {
			t.Fatalf("Add %q: %v", title, err)
		}
	}
	docs, err := ByRole(db, "ops", 2)
	if err != nil {
		t.Fatalf("ByRole: %v", err)
	}
	if len(docs) != 2 {
		t.Fatalf("want 2 docs (limit), got %d", len(docs))
	}
	if docs[0].Title != "Third" || docs[1].Title != "Second" {
		t.Fatalf("want newest first [Third, Second], got [%s, %s]", docs[0].Title, docs[1].Title)
	}
}

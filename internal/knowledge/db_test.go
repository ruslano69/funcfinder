package knowledge

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

// TestMetadataGeneratedColumns verifies TZ FR-3: author/role_tags/source_version
// ride in the metadata JSON blob but must also be readable as their own
// queryable columns (Doc.Author/RoleTags/SourceVersion), not just parsed out
// of the JSON by the caller.
func TestMetadataGeneratedColumns(t *testing.T) {
	db := openDB(t)
	meta := `{"author":"ruslan","role_tags":"backend,security","source_version":"auth-v2.md"}`
	if _, err := Add(db, "Auth spec", "OAuth2 device flow", "spec", meta, nil); err != nil {
		t.Fatalf("Add: %v", err)
	}

	docs, err := ReadRange(db, 1, 0)
	if err != nil {
		t.Fatalf("ReadRange: %v", err)
	}
	if len(docs) != 1 {
		t.Fatalf("want 1 doc, got %d", len(docs))
	}
	d := docs[0]
	if d.Author != "ruslan" || d.RoleTags != "backend,security" || d.SourceVersion != "auth-v2.md" {
		t.Fatalf("generated columns not populated: %+v", d)
	}

	// The column, not just the JSON blob, must be directly queryable.
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM docs WHERE author = ?`, "ruslan").Scan(&count); err != nil {
		t.Fatalf("query by author column: %v", err)
	}
	if count != 1 {
		t.Fatalf("want 1 doc matching author column, got %d", count)
	}
}

// TestMetadataGeneratedColumns_Missing verifies a doc with no metadata at all
// (default '{}') gets empty, not NULL-panicking, generated columns.
func TestMetadataGeneratedColumns_Missing(t *testing.T) {
	db := openDB(t)
	if _, err := Add(db, "No meta", "content", "general", "{}", nil); err != nil {
		t.Fatalf("Add: %v", err)
	}
	docs, err := ReadRange(db, 1, 0)
	if err != nil {
		t.Fatalf("ReadRange: %v", err)
	}
	if docs[0].Author != "" || docs[0].RoleTags != "" || docs[0].SourceVersion != "" {
		t.Fatalf("want empty generated columns for doc without metadata, got %+v", docs[0])
	}
}

// TestMigrateMetadataColumns verifies Open() adds the FR-3 generated columns
// to a docs table created before they existed (pre-FR-3 schema), preserves
// existing rows, backfills the columns from their metadata, and is safe to
// call again (no "duplicate column" on a second Open of the same file).
func TestMigrateMetadataColumns(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.sqlite")

	// Simulate a store created before FR-3: no generated columns.
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open raw: %v", err)
	}
	if _, err = raw.Exec(`
		CREATE TABLE docs (
		    id         INTEGER PRIMARY KEY AUTOINCREMENT,
		    title      TEXT    NOT NULL,
		    content    TEXT    NOT NULL,
		    type       TEXT    NOT NULL DEFAULT 'general',
		    created_at INTEGER NOT NULL DEFAULT (strftime('%s','now')),
		    metadata   TEXT    NOT NULL DEFAULT '{}'
		);
		INSERT INTO docs (title, content, metadata)
		VALUES ('Old Doc', 'legacy content', '{"author":"legacy-author"}');
	`); err != nil {
		t.Fatalf("seed legacy schema: %v", err)
	}
	if err = raw.Close(); err != nil {
		t.Fatalf("close raw: %v", err)
	}

	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open (migrate): %v", err)
	}
	defer db.Close()

	docs, err := ReadRange(db, 1, 0)
	if err != nil {
		t.Fatalf("ReadRange after migrate: %v", err)
	}
	if len(docs) != 1 || docs[0].Title != "Old Doc" || docs[0].Author != "legacy-author" {
		t.Fatalf("migration lost or mis-backfilled data: %+v", docs)
	}

	// Re-opening the now-migrated file must not fail with "duplicate column".
	db2, err := Open(path)
	if err != nil {
		t.Fatalf("second Open (idempotency): %v", err)
	}
	db2.Close()
}

package knowledge

import (
	"database/sql"
	_ "modernc.org/sqlite"
)

const schema = `
PRAGMA journal_mode=WAL;
PRAGMA foreign_keys=ON;

CREATE TABLE IF NOT EXISTS docs (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    title      TEXT    NOT NULL,
    content    TEXT    NOT NULL,
    type       TEXT    NOT NULL DEFAULT 'general',
    created_at INTEGER NOT NULL DEFAULT (strftime('%s','now')),
    metadata   TEXT    NOT NULL DEFAULT '{}'
);

CREATE VIRTUAL TABLE IF NOT EXISTS docs_fts USING fts5(
    title,
    content,
    content='docs',
    content_rowid='id'
);

-- Read-only view over the FTS5 index exposing (term, doc-frequency, count).
-- Free from the existing index; carried into releases by VACUUM INTO. Powers
-- Suggest() — "which terms actually exist to search for".
CREATE VIRTUAL TABLE IF NOT EXISTS docs_vocab USING fts5vocab('docs_fts', 'row');

CREATE TABLE IF NOT EXISTS docs_vec (
    doc_id    INTEGER PRIMARY KEY REFERENCES docs(id) ON DELETE CASCADE,
    dim       INTEGER NOT NULL,
    embedding BLOB    NOT NULL
);

CREATE TRIGGER IF NOT EXISTS docs_fts_ai AFTER INSERT ON docs BEGIN
    INSERT INTO docs_fts(rowid, title, content)
    VALUES (new.id, new.title, new.content);
END;

CREATE TRIGGER IF NOT EXISTS docs_fts_ad AFTER DELETE ON docs BEGIN
    INSERT INTO docs_fts(docs_fts, rowid, title, content)
    VALUES ('delete', old.id, old.title, old.content);
END;

CREATE TRIGGER IF NOT EXISTS docs_fts_au AFTER UPDATE ON docs BEGIN
    INSERT INTO docs_fts(docs_fts, rowid, title, content)
    VALUES ('delete', old.id, old.title, old.content);
    INSERT INTO docs_fts(rowid, title, content)
    VALUES (new.id, new.title, new.content);
END;
`

// Open opens (or creates) a knowledge base at path and applies the schema.
func Open(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if _, err = db.Exec(schema); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

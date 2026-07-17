package knowledge

import (
	"database/sql"
	_ "modernc.org/sqlite"
)

const schema = `
PRAGMA journal_mode=WAL;
PRAGMA foreign_keys=ON;

-- author/role_tags/source_version are generated from metadata (TZ FR-3): the
-- JSON blob stays the single source of truth (one write path, no dual-write
-- drift), while these give queryable, indexable SQL columns over the fields
-- that actually need filtering (role_tags for FR-9 context(role), etc).
CREATE TABLE IF NOT EXISTS docs (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    title          TEXT    NOT NULL,
    content        TEXT    NOT NULL,
    type           TEXT    NOT NULL DEFAULT 'general',
    created_at     INTEGER NOT NULL DEFAULT (strftime('%s','now')),
    metadata       TEXT    NOT NULL DEFAULT '{}',
    author         TEXT GENERATED ALWAYS AS (json_extract(metadata, '$.author')) VIRTUAL,
    role_tags      TEXT GENERATED ALWAYS AS (json_extract(metadata, '$.role_tags')) VIRTUAL,
    source_version TEXT GENERATED ALWAYS AS (json_extract(metadata, '$.source_version')) VIRTUAL
);
-- Indexes on author/role_tags/source_version are created by
-- migrateMetadataColumns below, not here: on a docs table that predates these
-- columns, an unconditional CREATE INDEX in this same multi-statement exec
-- would run before the migration adds them and fail with "no such column".

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

// Open opens (or creates) a knowledge base at path, applies the schema, and
// migrates any pre-existing docs table (created before FR-3's generated
// columns existed) to add them.
func Open(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if _, err = db.Exec(schema); err != nil {
		db.Close()
		return nil, err
	}
	if err = migrateMetadataColumns(db); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

// metadataGeneratedColumns are the FR-3 columns generated from docs.metadata.
// CREATE TABLE IF NOT EXISTS is a no-op against an already-existing docs
// table, so a store created before these columns existed needs them added
// via ALTER TABLE instead.
var metadataGeneratedColumns = []struct{ name, expr string }{
	{"author", "json_extract(metadata, '$.author')"},
	{"role_tags", "json_extract(metadata, '$.role_tags')"},
	{"source_version", "json_extract(metadata, '$.source_version')"},
}

func migrateMetadataColumns(db *sql.DB) error {
	// table_xinfo, not table_info: plain table_info omits generated columns
	// entirely, so it would report author/role_tags/source_version as always
	// missing and re-add them on every open.
	rows, err := db.Query(`PRAGMA table_xinfo(docs)`)
	if err != nil {
		return err
	}
	existing := map[string]bool{}
	for rows.Next() {
		var cid int
		var name, ctype string
		var notNull int
		var dflt sql.NullString
		var pk, hidden int
		if err = rows.Scan(&cid, &name, &ctype, &notNull, &dflt, &pk, &hidden); err != nil {
			rows.Close()
			return err
		}
		existing[name] = true
	}
	if err = rows.Err(); err != nil {
		return err
	}
	rows.Close()

	for _, c := range metadataGeneratedColumns {
		if !existing[c.name] {
			if _, err = db.Exec(
				`ALTER TABLE docs ADD COLUMN ` + c.name + ` TEXT GENERATED ALWAYS AS (` + c.expr + `) VIRTUAL`,
			); err != nil {
				return err
			}
		}
		// Idempotent and cheap either way: covers both a freshly created
		// table (column existed, index did not yet) and a just-migrated one.
		if _, err = db.Exec(
			`CREATE INDEX IF NOT EXISTS docs_` + c.name + `_idx ON docs(` + c.name + `)`,
		); err != nil {
			return err
		}
	}
	return nil
}

package knowledge

import "database/sql"

// ReadRange returns the contiguous neighborhood of chunks around id — id-context
// through id+context, in id order — for reading a match in its full context
// instead of a single truncated chunk. Reference material is chunked at
// paragraph boundaries (see chunk.go), so one logical entry (a function's
// Syntax/Description/Parameters/Example block, say) routinely spans several
// chunks; a search snippet tells you *where* to look, this is how you actually
// read it. Missing ids (deleted docs, or id falls outside the table) are simply
// absent from the result — not an error.
func ReadRange(db *sql.DB, id int64, context int) ([]Doc, error) {
	if context < 0 {
		context = 0
	}
	rows, err := db.Query(
		`SELECT id, title, content, type, created_at, metadata
		   FROM docs WHERE id BETWEEN ? AND ? ORDER BY id`,
		id-int64(context), id+int64(context))
	if err != nil {
		return nil, err
	}
	return scanDocs(rows)
}

// ReadBySource returns every chunk ingested from the source file tagged
// sourceVersion (the --source-version value passed at ingest time), in
// ingest/id order — reconstructing that source document in full.
func ReadBySource(db *sql.DB, sourceVersion string) ([]Doc, error) {
	rows, err := db.Query(
		`SELECT id, title, content, type, created_at, metadata FROM docs
		   WHERE json_extract(metadata, '$.source_version') = ? ORDER BY id`,
		sourceVersion)
	if err != nil {
		return nil, err
	}
	return scanDocs(rows)
}

func scanDocs(rows *sql.Rows) ([]Doc, error) {
	defer rows.Close()
	var out []Doc
	for rows.Next() {
		var d Doc
		if err := rows.Scan(&d.ID, &d.Title, &d.Content, &d.Type, &d.CreatedAt, &d.Metadata); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

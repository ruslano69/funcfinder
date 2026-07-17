package knowledge

import (
	"database/sql"
	"fmt"
)

// Add inserts a document and optionally its embedding into the knowledge base.
// Returns the new document ID.
func Add(db *sql.DB, title, content, docType, metadata string, embedding []float32) (int64, error) {
	tx, err := db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	res, err := tx.Exec(
		`INSERT INTO docs (title, content, type, metadata) VALUES (?, ?, ?, ?)`,
		title, content, docType, metadata,
	)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}

	if len(embedding) > 0 {
		blob := float32SliceToBlob(embedding)
		if _, err = tx.Exec(
			`INSERT INTO docs_vec (doc_id, dim, embedding) VALUES (?, ?, ?)`,
			id, len(embedding), blob,
		); err != nil {
			return 0, fmt.Errorf("insert embedding: %w", err)
		}
	}

	return id, tx.Commit()
}

// Delete removes a document and its embedding by ID.
func Delete(db *sql.DB, id int64) error {
	_, err := db.Exec(`DELETE FROM docs WHERE id = ?`, id)
	return err
}

// DeleteByType removes every document of the given type (and any embeddings,
// via docs_vec's ON DELETE CASCADE). For regenerable document types — code
// maps compiled fresh from source on every publish (TZ FR-22), say — this is
// how a re-ingest replaces the previous snapshot instead of accumulating
// stale copies from earlier commits alongside it.
func DeleteByType(db *sql.DB, docType string) (int64, error) {
	res, err := db.Exec(`DELETE FROM docs WHERE type = ?`, docType)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// Count returns the total number of documents in the knowledge base.
func Count(db *sql.DB) (int64, error) {
	var n int64
	err := db.QueryRow(`SELECT COUNT(*) FROM docs`).Scan(&n)
	return n, err
}

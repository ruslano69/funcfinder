package knowledge

import (
	"database/sql"
	"fmt"
	"strings"
)

// Term is a corpus vocabulary entry drawn from the FTS5 index: the token, how
// many documents contain it, and its total occurrence count.
type Term struct {
	Term  string `json:"term"`
	Docs  int    `json:"docs"`
	Count int    `json:"count"`
}

// Suggest returns vocabulary terms that begin with prefix, most frequent first —
// the tokens that actually exist in the corpus's FTS index. It turns "guess a
// keyword and hope it's in the docs" into "look up what's really there", which
// is the cheap, precise front-door to FTS5 (and reveals inflected forms in
// morphologically rich languages: "сорт" -> сортировки, сортируемого, ...).
//
// The FTS5 tokenizer lowercase-folds terms, so the prefix is lowercased to match.
func Suggest(db *sql.DB, prefix string, limit int) ([]Term, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := db.Query(
		`SELECT term, doc, cnt FROM docs_vocab WHERE term LIKE ? ORDER BY cnt DESC, term LIMIT ?`,
		strings.ToLower(prefix)+"%", limit)
	if err != nil {
		if strings.Contains(err.Error(), "no such table") {
			return nil, fmt.Errorf("vocabulary not built for this release (re-publish to enable suggest): %w", err)
		}
		return nil, err
	}
	defer rows.Close()

	var out []Term
	for rows.Next() {
		var t Term
		if err := rows.Scan(&t.Term, &t.Docs, &t.Count); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

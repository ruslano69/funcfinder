package knowledge

import (
	"database/sql"
	"fmt"
	"math"
	"strings"
)

// WeakKeyIDF is the discriminating-power floor below which a term is a poor
// search key: it occurs in more than ~25% of documents (IDF = ln(N/df) < 1.4),
// so searching it floods results with weakly-relevant matches and "leads away
// from the solution". Using IDF (not a raw count) keeps this corpus-size
// independent — df=114 is noise in a 200-doc corpus but a precise anchor in a
// 3000-doc one.
const WeakKeyIDF = 1.4

// Term is a corpus vocabulary entry drawn from the FTS5 index: the token, its
// document frequency, total occurrence count, and IDF (discriminating power —
// higher is a sharper search key).
type Term struct {
	Term  string  `json:"term"`
	Docs  int     `json:"docs"`
	Count int     `json:"count"`
	IDF   float64 `json:"idf"`
}

// Weak reports whether the term is too common to be a good search key.
func (t Term) Weak() bool { return t.IDF < WeakKeyIDF }

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
	// Corpus size N, for IDF = ln(N/df).
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM docs`).Scan(&n); err != nil {
		return nil, err
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
		if n > 0 && t.Docs > 0 {
			t.IDF = math.Log(float64(n) / float64(t.Docs))
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

package knowledge

import (
	"database/sql"
	"sort"
	"strings"
)

// Metric selects the distance function used for vector search.
type Metric string

const (
	MetricCosine Metric = "cosine"
	MetricL2     Metric = "l2"
)

func (m Metric) sqlFunc() string {
	if m == MetricL2 {
		return "vec_distance_l2"
	}
	return "vec_distance_cosine"
}

// SearchFTS performs full-text keyword search via FTS5 / BM25.
func SearchFTS(db *sql.DB, query string, limit int) ([]Result, error) {
	rows, err := db.Query(`
		SELECT d.id, d.title, d.content, d.type, d.created_at, d.metadata,
		       bm25(docs_fts) AS rank
		FROM docs_fts
		JOIN docs d ON docs_fts.rowid = d.id
		WHERE docs_fts MATCH ?
		ORDER BY rank
		LIMIT ?
	`, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []Result
	for rows.Next() {
		var r Result
		if err = rows.Scan(
			&r.ID, &r.Title, &r.Content, &r.Type, &r.CreatedAt, &r.Metadata,
			&r.FTSRank,
		); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

// SearchVec performs vector similarity search. The distance function runs
// inside SQLite via a registered custom function — no bulk BLOB loading into Go.
// Pass docType="" to search all types; pass a non-empty string to pre-filter.
func SearchVec(db *sql.DB, embedding []float32, limit int, metric Metric, docType string) ([]Result, error) {
	blob := float32SliceToBlob(embedding)
	fn := metric.sqlFunc()

	var sb strings.Builder
	sb.WriteString(`
		SELECT d.id, d.title, d.content, d.type, d.created_at, d.metadata,
		       ` + fn + `(v.embedding, ?) AS dist
		FROM docs d
		JOIN docs_vec v ON v.doc_id = d.id
	`)

	args := []any{blob}
	if docType != "" {
		sb.WriteString(" WHERE d.type = ?")
		args = append(args, docType)
	}
	sb.WriteString(" ORDER BY dist LIMIT ?")
	args = append(args, limit)

	rows, err := db.Query(sb.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []Result
	for rows.Next() {
		var r Result
		if err = rows.Scan(
			&r.ID, &r.Title, &r.Content, &r.Type, &r.CreatedAt, &r.Metadata,
			&r.VecDist,
		); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

// SearchHybrid combines FTS5 and vector results via Reciprocal Rank Fusion.
// Pass query="" to skip FTS; pass nil/empty embedding to skip vector.
func SearchHybrid(db *sql.DB, query string, embedding []float32, limit int, metric Metric, docType string) ([]Result, error) {
	const rrfK = 60
	fetch := limit * 3
	if fetch < 30 {
		fetch = 30
	}

	scores := map[int64]float64{}
	byID := map[int64]Result{}

	if query != "" {
		fts, err := SearchFTS(db, query, fetch)
		if err != nil {
			return nil, err
		}
		for i, r := range fts {
			scores[r.ID] += 1.0 / float64(rrfK+i+1)
			byID[r.ID] = r
		}
	}

	if len(embedding) > 0 {
		vec, err := SearchVec(db, embedding, fetch, metric, docType)
		if err != nil {
			return nil, err
		}
		for i, r := range vec {
			scores[r.ID] += 1.0 / float64(rrfK+i+1)
			if _, exists := byID[r.ID]; !exists {
				byID[r.ID] = r
			}
		}
	}

	results := make([]Result, 0, len(scores))
	for id, score := range scores {
		r := byID[id]
		r.HybridScore = score
		results = append(results, r)
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].HybridScore > results[j].HybridScore
	})
	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}
	return results, nil
}

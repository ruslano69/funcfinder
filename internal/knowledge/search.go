package knowledge

import (
	"database/sql"
	"sort"
)

// SearchFTS performs full-text search using FTS5 / BM25.
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

// SearchVec performs vector similarity search using cosine distance.
// It loads all stored embeddings and computes distances in Go.
func SearchVec(db *sql.DB, embedding []float32, limit int) ([]Result, error) {
	rows, err := db.Query(`
		SELECT d.id, d.title, d.content, d.type, d.created_at, d.metadata,
		       v.embedding
		FROM docs d
		JOIN docs_vec v ON v.doc_id = d.id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type scored struct {
		Result
		dist float64
	}
	var all []scored
	for rows.Next() {
		var r Result
		var blob []byte
		if err = rows.Scan(
			&r.ID, &r.Title, &r.Content, &r.Type, &r.CreatedAt, &r.Metadata,
			&blob,
		); err != nil {
			return nil, err
		}
		dist := cosineDistance(embedding, blobToFloat32Slice(blob))
		all = append(all, scored{r, dist})
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}

	sort.Slice(all, func(i, j int) bool { return all[i].dist < all[j].dist })
	if limit > 0 && len(all) > limit {
		all = all[:limit]
	}

	results := make([]Result, len(all))
	for i, s := range all {
		results[i] = s.Result
		results[i].VecDist = s.dist
	}
	return results, nil
}

// SearchHybrid combines FTS5 and vector results via Reciprocal Rank Fusion.
// Pass an empty query to skip FTS; pass a nil/empty embedding to skip vector.
func SearchHybrid(db *sql.DB, query string, embedding []float32, limit int) ([]Result, error) {
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
		vec, err := SearchVec(db, embedding, fetch)
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

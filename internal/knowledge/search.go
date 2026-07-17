package knowledge

import (
	"database/sql"
	"regexp"
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

// Preview returns a keyword-in-context excerpt for displaying a search result:
// the FTS Snippet when the result came from a keyword match (already centered
// on the match, token-safe), or else a rune-safe truncation of Content — never
// a byte-index slice, which can split a multi-byte rune (Cyrillic/CJK/etc) and
// print U+FFFD. maxRunes bounds the fallback path only; the FTS snippet's own
// length is controlled by ftsSnippetTokens at query time.
func (r Result) Preview(maxRunes int) string {
	if r.Snippet != "" {
		return strings.ReplaceAll(r.Snippet, "\n", " ")
	}
	flat := strings.ReplaceAll(r.Content, "\n", " ")
	runes := []rune(flat)
	if len(runes) <= maxRunes {
		return flat
	}
	return string(runes[:maxRunes]) + "..."
}

// BuildFTSQuery transforms a plain query string into an FTS5 MATCH expression.
//
// Rules:
//   - Tokens that look like raw FTS5 operators (AND, OR, NOT, quoted phrases,
//     existing prefix wildcards) are passed through unchanged so callers can
//     write explicit boolean queries.
//   - All other plain tokens get a trailing "*" (prefix match) when prefix=true,
//     so "call graph" → "call* graph*" and matches callgraph, callback, etc.
//   - Special FTS5 characters inside plain tokens are escaped with double-quotes
//     so they don't corrupt the MATCH syntax.
func BuildFTSQuery(query string, prefix bool) string {
	// If the query looks like an explicit FTS5 expression, return it as-is:
	// starts with a phrase quote, contains grouping parens, or uses boolean operators.
	trimmed := strings.TrimSpace(query)
	if strings.HasPrefix(trimmed, `"`) ||
		strings.ContainsAny(trimmed, `()`) ||
		strings.Contains(trimmed, " AND ") ||
		strings.Contains(trimmed, " OR ") ||
		strings.Contains(trimmed, " NOT ") {
		return trimmed
	}

	if !prefix {
		return trimmed
	}

	tokens := strings.Fields(trimmed)
	out := make([]string, 0, len(tokens))
	for _, tok := range tokens {
		// Already has wildcard or is a bare operator word — keep as-is.
		if strings.HasSuffix(tok, "*") || tok == "AND" || tok == "OR" || tok == "NOT" {
			out = append(out, tok)
			continue
		}
		// Escape any FTS5-special characters by wrapping in double quotes,
		// then append the prefix wildcard outside the quotes.
		safe := `"` + strings.ReplaceAll(tok, `"`, `""`) + `"` + "*"
		out = append(out, safe)
	}
	return strings.Join(out, " ")
}

// ftsSnippetTokens is the number of tokens SQLite's snippet() includes around
// the match — enough context to see why a chunk matched without dumping the
// whole (possibly 800-rune) chunk.
const ftsSnippetTokens = 16

// SearchFTS performs full-text keyword search via FTS5 / BM25.
// Set prefix=true to automatically append wildcard "*" to each plain token.
//
// Each result carries a Snippet: a keyword-in-context excerpt built by
// SQLite's snippet() function, which operates on the tokenizer's token
// boundaries (never mid-word, never mid-UTF-8-rune — Cyrillic/CJK/etc are
// safe). Column -1 lets FTS5 pick whichever indexed column (title or content)
// actually contains the match, so the excerpt centers on it instead of always
// showing the start of the chunk.
func SearchFTS(db *sql.DB, query string, limit int, prefix bool) ([]Result, error) {
	ftsQuery := BuildFTSQuery(query, prefix)
	rows, err := db.Query(`
		SELECT d.id, d.title, d.content, d.type, d.created_at, d.metadata,
		       COALESCE(d.author,''), COALESCE(d.role_tags,''), COALESCE(d.source_version,''),
		       bm25(docs_fts) AS rank,
		       snippet(docs_fts, -1, '**', '**', '...', ?) AS snip
		FROM docs_fts
		JOIN docs d ON docs_fts.rowid = d.id
		WHERE docs_fts MATCH ?
		ORDER BY rank
		LIMIT ?
	`, ftsSnippetTokens, ftsQuery, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []Result
	for rows.Next() {
		var r Result
		if err = rows.Scan(
			&r.ID, &r.Title, &r.Content, &r.Type, &r.CreatedAt, &r.Metadata,
			&r.Author, &r.RoleTags, &r.SourceVersion,
			&r.FTSRank, &r.Snippet,
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
		       COALESCE(d.author,''), COALESCE(d.role_tags,''), COALESCE(d.source_version,''),
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
			&r.Author, &r.RoleTags, &r.SourceVersion,
			&r.VecDist,
		); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

// SearchRegex scans all documents and returns those whose title or content
// matches the Go regular expression pattern. Results are ordered by title.
// docType="" matches all types.
func SearchRegex(db *sql.DB, pattern string, limit int, docType string) ([]Result, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}

	var sb strings.Builder
	sb.WriteString(`SELECT id, title, content, type, created_at, metadata,
		COALESCE(author,''), COALESCE(role_tags,''), COALESCE(source_version,'') FROM docs`)
	args := []any{}
	if docType != "" {
		sb.WriteString(" WHERE type = ?")
		args = append(args, docType)
	}
	sb.WriteString(" ORDER BY id")

	rows, err := db.Query(sb.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []Result
	for rows.Next() {
		var r Result
		if err = rows.Scan(&r.ID, &r.Title, &r.Content, &r.Type, &r.CreatedAt, &r.Metadata,
			&r.Author, &r.RoleTags, &r.SourceVersion); err != nil {
			return nil, err
		}
		if re.MatchString(r.Title) || re.MatchString(r.Content) {
			results = append(results, r)
			if limit > 0 && len(results) >= limit {
				break
			}
		}
	}
	return results, rows.Err()
}

// Match is one distinct regex match found across the corpus by Enumerate: the
// matched text, how many documents contain it, and how many times it occurs
// in total.
type Match struct {
	Value string `json:"value"`
	Docs  int    `json:"docs"`
	Count int    `json:"count"`
}

// Enumerate is the completeness-audit primitive from
// docs/docsearch-server/HOW_TO_USE.md step 4: "did I miss a category" —
// distinct from SearchRegex, which returns whole matching *documents*.
// Enumerate extracts every distinct substring the pattern matches across the
// whole corpus (title+content) and tallies it, so a question like "which
// PB_Cipher_* constants actually exist in this corpus" is answered directly
// instead of by guessing a candidate list and checking each one. This is the
// first-class replacement for `search --json | grep -o <pattern> | sort -u`.
//
// Results are sorted by Count descending, then Value ascending, and capped at
// limit (0 or negative = no cap). A pattern that matches empty strings (e.g.
// unanchored `.*`) will produce one match per position and is the caller's
// own responsibility — Enumerate does not special-case it.
func Enumerate(db *sql.DB, pattern string, limit int) ([]Match, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}

	rows, err := db.Query(`SELECT title, content FROM docs`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := map[string]int{}
	docFreq := map[string]int{}
	for rows.Next() {
		var title, content string
		if err := rows.Scan(&title, &content); err != nil {
			return nil, err
		}
		matches := re.FindAllString(title+"\n"+content, -1)
		seenInDoc := map[string]bool{}
		for _, m := range matches {
			counts[m]++
			if !seenInDoc[m] {
				docFreq[m]++
				seenInDoc[m] = true
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	out := make([]Match, 0, len(counts))
	for v, c := range counts {
		out = append(out, Match{Value: v, Docs: docFreq[v], Count: c})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Value < out[j].Value
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// SearchHybrid combines FTS5 and vector results via Reciprocal Rank Fusion.
// Pass query="" to skip FTS; pass nil/empty embedding to skip vector.
// prefix controls whether plain FTS tokens get auto-wildcard expansion.
func SearchHybrid(db *sql.DB, query string, embedding []float32, limit int, metric Metric, docType string, prefix bool) ([]Result, error) {
	const rrfK = 60
	fetch := limit * 3
	if fetch < 30 {
		fetch = 30
	}

	scores := map[int64]float64{}
	byID := map[int64]Result{}

	if query != "" {
		fts, err := SearchFTS(db, query, fetch, prefix)
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

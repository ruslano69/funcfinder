package knowledge

// Doc is a single knowledge base entry.
type Doc struct {
	ID        int64
	Title     string
	Content   string
	Type      string
	CreatedAt int64
	Metadata  string
	// Author, RoleTags, and SourceVersion are queryable columns generated
	// from Metadata (TZ FR-3) — empty string if the metadata JSON omits them.
	Author        string
	RoleTags      string
	SourceVersion string
}

// Result wraps Doc with search scores populated during retrieval.
type Result struct {
	Doc
	FTSRank     float64
	VecDist     float64
	HybridScore float64
	// Snippet is a keyword-in-context excerpt around the matched term(s),
	// populated by FTS searches via SQLite's snippet() (empty for vec/regex
	// results, which have no FTS match to center on — callers fall back to
	// Content in that case).
	Snippet string
}

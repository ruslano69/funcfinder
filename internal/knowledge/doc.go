package knowledge

// Doc is a single knowledge base entry.
type Doc struct {
	ID        int64
	Title     string
	Content   string
	Type      string
	CreatedAt int64
	Metadata  string
}

// Result wraps Doc with search scores populated during retrieval.
type Result struct {
	Doc
	FTSRank     float64
	VecDist     float64
	HybridScore float64
}

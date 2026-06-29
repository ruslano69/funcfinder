package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ruslano69/funcfinder/internal"
	"github.com/ruslano69/funcfinder/internal/knowledge"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	// Global --db flag before the action.
	globalFS := flag.NewFlagSet("docsearch", flag.ContinueOnError)
	dbPath := globalFS.String("db", ".knowledge/docs.sqlite", "path to SQLite knowledge base")
	globalFS.Usage = printUsage

	// Handle --version before action parsing.
	for _, a := range os.Args[1:] {
		if a == "--version" || a == "-version" {
			internal.PrintVersion("docsearch")
			return
		}
	}

	// Collect args up to (not including) the action word.
	var preAction, postAction []string
	foundAction := false
	actions := map[string]bool{"init": true, "add": true, "search": true, "count": true}
	for i, a := range os.Args[1:] {
		if actions[a] {
			preAction = os.Args[1 : i+1]
			postAction = os.Args[i+2:]
			foundAction = true
			break
		}
	}
	if !foundAction {
		printUsage()
		os.Exit(1)
	}
	action := os.Args[len(preAction)+1]

	if err := globalFS.Parse(preAction); err != nil {
		os.Exit(1)
	}

	switch action {
	case "init":
		runInit(*dbPath)
	case "add":
		runAdd(*dbPath, postAction)
	case "search":
		runSearch(*dbPath, postAction)
	case "count":
		runCount(*dbPath, postAction)
	default:
		fatalf("unknown action %q", action)
	}
}

func runInit(dbPath string) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		fatalf("mkdir: %v", err)
	}
	db, err := knowledge.Open(dbPath)
	if err != nil {
		fatalf("open: %v", err)
	}
	db.Close()
	fmt.Fprintf(os.Stderr, "knowledge base ready: %s\n", dbPath)
}

func runAdd(dbPath string, args []string) {
	fs := flag.NewFlagSet("add", flag.ExitOnError)
	title := fs.String("title", "", "document title (required without --file)")
	content := fs.String("content", "", "document content (required without --file)")
	file := fs.String("file", "", "ingest a .txt/.md/.pdf file (splits into chunks)")
	chunkSize := fs.Int("chunk-size", 800, "max chunk size in runes (with --file)")
	chunkOverlap := fs.Int("chunk-overlap", 80, "overlap runes between chunks (with --file)")
	docType := fs.String("type", "general", "document type: general|tool_usage|error|scenario")
	meta := fs.String("meta", "{}", "document metadata as JSON")
	embeddingRaw := fs.String("embedding", "", "comma-separated float32 values (single doc only)")
	jsonOut := fs.Bool("json", false, "output JSON")
	fs.Parse(args)

	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		fatalf("mkdir: %v", err)
	}
	db, err := knowledge.Open(dbPath)
	if err != nil {
		fatalf("open db: %v", err)
	}
	defer db.Close()

	if *file != "" {
		opts := knowledge.ChunkOpts{MaxRunes: *chunkSize, OverlapRunes: *chunkOverlap}
		chunks, err := knowledge.IngestFile(*file, opts)
		if err != nil {
			var ocrErr *knowledge.OCRQualityError
			if errors.As(err, &ocrErr) {
				if *jsonOut {
					json.NewEncoder(os.Stdout).Encode(map[string]any{
						"error": "bad_ocr",
						"score": ocrErr.Score,
						"file":  ocrErr.Path,
					})
					os.Exit(2)
				}
				fmt.Fprintf(os.Stderr, "skipped: bad OCR quality in %s (score %.2f)\n", ocrErr.Path, ocrErr.Score)
				fmt.Fprintf(os.Stderr, "hint: run OCR correction (e.g. ocrmypdf) before indexing\n")
				os.Exit(2)
			}
			fatalf("ingest: %v", err)
		}
		var ids []int64
		for _, ch := range chunks {
			id, err := knowledge.Add(db, ch.Title, ch.Content, *docType, *meta, nil)
			if err != nil {
				fatalf("add chunk: %v", err)
			}
			ids = append(ids, id)
		}
		if *jsonOut {
			json.NewEncoder(os.Stdout).Encode(map[string]any{"ids": ids, "chunks": len(ids)})
		} else {
			fmt.Fprintf(os.Stderr, "ingested %d chunks from %s\n", len(ids), *file)
		}
		return
	}

	if *title == "" || *content == "" {
		fatalf("--title and --content are required (or use --file)")
	}
	emb := parseEmbedding(*embeddingRaw)
	id, err := knowledge.Add(db, *title, *content, *docType, *meta, emb)
	if err != nil {
		fatalf("add: %v", err)
	}
	if *jsonOut {
		json.NewEncoder(os.Stdout).Encode(map[string]any{"id": id})
	} else {
		fmt.Fprintf(os.Stderr, "added id=%d\n", id)
	}
}

func runSearch(dbPath string, args []string) {
	fs := flag.NewFlagSet("search", flag.ExitOnError)
	query := fs.String("query", "", "FTS/regex query string")
	embeddingRaw := fs.String("embedding", "", "comma-separated float32 values")
	mode := fs.String("mode", "hybrid", "search mode: fts|vec|hybrid|regex")
	metricRaw := fs.String("metric", "cosine", "distance metric: cosine|l2 (vec/hybrid modes)")
	filterType := fs.String("filter-type", "", "pre-filter by document type before vector or regex search")
	limit := fs.Int("limit", 10, "maximum results")
	prefix := fs.Bool("prefix", true, "auto-append wildcard to FTS tokens (e.g. call → call*)")
	jsonOut := fs.Bool("json", false, "output JSON")
	fs.Parse(args)

	if *query == "" && *embeddingRaw == "" {
		fatalf("--query or --embedding required")
	}

	metric := knowledge.MetricCosine
	if *metricRaw == "l2" {
		metric = knowledge.MetricL2
	}

	db, err := knowledge.Open(dbPath)
	if err != nil {
		fatalf("open db: %v", err)
	}
	defer db.Close()

	emb := parseEmbedding(*embeddingRaw)
	var results []knowledge.Result
	switch *mode {
	case "fts":
		results, err = knowledge.SearchFTS(db, *query, *limit, *prefix)
	case "vec":
		if len(emb) == 0 {
			fatalf("--embedding required for vec mode")
		}
		results, err = knowledge.SearchVec(db, emb, *limit, metric, *filterType)
	case "regex":
		if *query == "" {
			fatalf("--query required for regex mode")
		}
		results, err = knowledge.SearchRegex(db, *query, *limit, *filterType)
	default:
		results, err = knowledge.SearchHybrid(db, *query, emb, *limit, metric, *filterType, *prefix)
	}
	if err != nil {
		fatalf("search: %v", err)
	}
	printResults(results, *jsonOut)
}

func runCount(dbPath string, args []string) {
	fs := flag.NewFlagSet("count", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "output JSON")
	fs.Parse(args)

	db, err := knowledge.Open(dbPath)
	if err != nil {
		fatalf("open db: %v", err)
	}
	defer db.Close()

	n, err := knowledge.Count(db)
	if err != nil {
		fatalf("count: %v", err)
	}
	if *jsonOut {
		json.NewEncoder(os.Stdout).Encode(map[string]any{"count": n})
	} else {
		fmt.Println(n)
	}
}

func parseEmbedding(raw string) []float32 {
	raw = strings.TrimSpace(strings.Trim(strings.TrimSpace(raw), "[]"))
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]float32, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		f, err := strconv.ParseFloat(p, 32)
		if err != nil {
			fatalf("invalid embedding value %q: %v", p, err)
		}
		out = append(out, float32(f))
	}
	return out
}

func printResults(results []knowledge.Result, asJSON bool) {
	if asJSON {
		type jsonResult struct {
			ID          int64   `json:"id"`
			Title       string  `json:"title"`
			Content     string  `json:"content"`
			Type        string  `json:"type"`
			CreatedAt   int64   `json:"created_at"`
			Metadata    string  `json:"metadata"`
			FTSRank     float64 `json:"fts_rank,omitempty"`
			VecDist     float64 `json:"vec_dist,omitempty"`
			HybridScore float64 `json:"hybrid_score,omitempty"`
		}
		out := make([]jsonResult, len(results))
		for i, r := range results {
			out[i] = jsonResult{
				ID:          r.ID,
				Title:       r.Title,
				Content:     r.Content,
				Type:        r.Type,
				CreatedAt:   r.CreatedAt,
				Metadata:    r.Metadata,
				FTSRank:     r.FTSRank,
				VecDist:     r.VecDist,
				HybridScore: r.HybridScore,
			}
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(out)
		return
	}
	for _, r := range results {
		snippet := r.Content
		if len(snippet) > 120 {
			snippet = snippet[:117] + "..."
		}
		fmt.Printf("[%d] %s  (%s)\n    %s\n\n", r.ID, r.Title, r.Type, snippet)
	}
}

func printUsage() {
	fmt.Fprint(os.Stderr, `docsearch — knowledge base with FTS5 + vector hybrid search

Actions:
  init    Create or verify the knowledge base
  add     Add a document
  search  Search documents
  count   Print total document count

Usage:
  docsearch [--db <path>] init
  docsearch [--db <path>] add    --title <t> --content <c> [--type <t>] [--meta <json>] [--embedding <floats>] [--json]
  docsearch [--db <path>] add    --file <path.txt|md|pdf>  [--type <t>] [--chunk-size N] [--chunk-overlap N] [--json]
  docsearch [--db <path>] search --query <q>               [--embedding <floats>] [--mode fts|vec|hybrid] [--metric cosine|l2] [--filter-type <type>] [--limit N] [--json]
  docsearch [--db <path>] count  [--json]

Default --db: .knowledge/docs.sqlite
Embedding format: comma-separated float32 values, e.g. "0.1,0.2,0.3" or "[0.1,0.2,0.3]"
`)
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "error: "+format+"\n", args...)
	os.Exit(1)
}

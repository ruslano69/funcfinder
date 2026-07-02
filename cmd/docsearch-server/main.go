// docsearch-server — the release-aware truth server CLI (see
// docs/docsearch-server/TZ.md). This is the thin human/CI wrapper over the
// truth backbone (internal/truth) and the knowledge layer (internal/knowledge),
// split strictly along the CQRS line:
//
//	rewrite:  ingest, record, publish, set-channel
//	readonly: search, releases, channels
//
// A network read-server and MCP interface are separate increments; this CLI is
// the substrate they call into.
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
	"github.com/ruslano69/funcfinder/internal/embed"
	"github.com/ruslano69/funcfinder/internal/knowledge"
	"github.com/ruslano69/funcfinder/internal/truth"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}
	for _, a := range os.Args[1:] {
		if a == "--version" || a == "-version" {
			internal.PrintVersion("docsearch-server")
			return
		}
	}

	// Global --root before the action; everything after the action is parsed
	// by that action's own flag set (same convention as cmd/docsearch).
	globalFS := flag.NewFlagSet("docsearch-server", flag.ContinueOnError)
	root := globalFS.String("root", ".docsearch", "data root (control DB, write-log, releases)")
	embedModel := globalFS.String("embed-model", "", "Ollama-compatible embedding model; enables vector search (e.g. qwen3-embedding:0.6b). Empty = pure BYO/FTS")
	embedURL := globalFS.String("embed-url", embed.DefaultURL, "embedding endpoint URL")
	globalFS.Usage = printUsage

	actions := map[string]bool{
		"ingest": true, "record": true, "publish": true,
		"set-channel": true, "channels": true, "releases": true, "search": true,
		"suggest": true, "read": true, "enumerate": true, "serve": true, "mcp": true,
	}
	var preAction, postAction []string
	action := ""
	for i, a := range os.Args[1:] {
		if actions[a] {
			preAction = os.Args[1 : i+1]
			postAction = os.Args[i+2:]
			action = a
			break
		}
	}
	if action == "" {
		printUsage()
		os.Exit(1)
	}
	if err := globalFS.Parse(preAction); err != nil {
		os.Exit(1)
	}

	store, err := truth.Open(*root)
	if err != nil {
		fatalf("open store %s: %v", *root, err)
	}
	defer store.Close()

	emb := embed.New(*embedURL, *embedModel)

	switch action {
	case "ingest":
		runIngest(store, emb, postAction)
	case "record":
		runRecord(store, emb, postAction)
	case "publish":
		runPublish(store, postAction)
	case "set-channel":
		runSetChannel(store, postAction)
	case "channels":
		runChannels(store, postAction)
	case "releases":
		runReleases(store, postAction)
	case "search":
		runSearch(store, emb, postAction)
	case "suggest":
		runSuggest(store, postAction)
	case "read":
		runRead(store, postAction)
	case "enumerate":
		runEnumerate(store, postAction)
	case "serve":
		runServe(store, emb, postAction)
	case "mcp":
		runMCP(store, emb, postAction)
	}
}

// metaJSON builds the metadata blob carried on every document: provenance that
// the knowledge schema does not model as columns rides here as JSON.
func metaJSON(author, roleTags, sourceVersion, sourceRef string) string {
	m := map[string]string{}
	if author != "" {
		m["author"] = author
	}
	if roleTags != "" {
		m["role_tags"] = roleTags
	}
	if sourceVersion != "" {
		m["source_version"] = sourceVersion
	}
	if sourceRef != "" {
		m["source_ref"] = sourceRef
	}
	b, _ := json.Marshal(m)
	return string(b)
}

// embedOrNil returns the vector for text when the embedder is enabled, or nil
// (pure BYO/FTS) otherwise. Embedding failures degrade to nil with a warning
// rather than aborting an ingest.
func embedOrNil(emb *embed.Client, text string) []float32 {
	if !emb.Enabled() {
		return nil
	}
	v, err := emb.Embed(text)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warn: embed failed (%v); storing without vector\n", err)
		return nil
	}
	return v
}

func runIngest(s *truth.Store, emb *embed.Client, args []string) {
	fs := flag.NewFlagSet("ingest", flag.ExitOnError)
	title := fs.String("title", "", "document title (required without --file)")
	content := fs.String("content", "", "document content (required without --file)")
	file := fs.String("file", "", "ingest a .txt/.md/.pdf file (chunked)")
	docType := fs.String("type", "general", "spec|ТЗ|lib_doc|sprint|changelog|task|decision|general")
	roleTags := fs.String("role-tags", "", "comma-separated role tags for context() view filter")
	author := fs.String("author", "", "author (provenance)")
	sourceVersion := fs.String("source-version", "", "source version (provenance)")
	chunkSize := fs.Int("chunk-size", 800, "max chunk runes (with --file)")
	chunkOverlap := fs.Int("chunk-overlap", 80, "chunk overlap runes (with --file)")
	stripRunes := fs.String("strip-runes", "", "extra junk runes to strip at index time (e.g. an OCR separator glyph: --strip-runes Ω)")
	jsonOut := fs.Bool("json", false, "output JSON")
	fs.Parse(args)

	db, err := knowledge.Open(s.WriteLogPath())
	if err != nil {
		fatalf("open write-log: %v", err)
	}
	defer db.Close()
	meta := metaJSON(*author, *roleTags, *sourceVersion, "")

	if *file != "" {
		chunks, err := knowledge.IngestFile(*file, knowledge.ChunkOpts{
			MaxRunes: *chunkSize, OverlapRunes: *chunkOverlap})
		if err != nil {
			var ocr *knowledge.OCRQualityError
			if errors.As(err, &ocr) {
				fatalf("bad OCR in %s (score %.2f); fix the PDF first", ocr.Path, ocr.Score)
			}
			fatalf("ingest: %v", err)
		}
		// Normalize each chunk before indexing/embedding: strips unambiguous
		// garbage always, plus --strip-runes, keeping the FTS vocabulary clean.
		for i := range chunks {
			chunks[i].Content = knowledge.NormalizeForIndex(chunks[i].Content, *stripRunes)
		}
		// Batch-embed chunk bodies when embedding is enabled (one round-trip).
		var vecs [][]float32
		if emb.Enabled() {
			texts := make([]string, len(chunks))
			for i, ch := range chunks {
				texts[i] = ch.Content
			}
			if vecs, err = emb.EmbedBatch(texts); err != nil {
				fmt.Fprintf(os.Stderr, "warn: batch embed failed (%v); storing without vectors\n", err)
				vecs = nil
			}
		}
		var ids []int64
		for i, ch := range chunks {
			var v []float32
			if vecs != nil {
				v = vecs[i]
			}
			id, err := knowledge.Add(db, ch.Title, ch.Content, *docType, meta, v)
			if err != nil {
				fatalf("add chunk: %v", err)
			}
			ids = append(ids, id)
		}
		if *jsonOut {
			json.NewEncoder(os.Stdout).Encode(map[string]any{"chunks": len(ids), "ids": ids, "embedded": vecs != nil})
		} else {
			fmt.Fprintf(os.Stderr, "ingested %d chunks from %s (type=%s, embedded=%v)\n", len(ids), *file, *docType, vecs != nil)
		}
		return
	}

	if *title == "" || *content == "" {
		fatalf("--title and --content required (or use --file)")
	}
	normalized := knowledge.NormalizeForIndex(*content, *stripRunes)
	id, err := knowledge.Add(db, *title, normalized, *docType, meta, embedOrNil(emb, normalized))
	if err != nil {
		fatalf("add: %v", err)
	}
	if *jsonOut {
		json.NewEncoder(os.Stdout).Encode(map[string]any{"id": id})
	} else {
		fmt.Fprintf(os.Stderr, "ingested id=%d (type=%s)\n", id, *docType)
	}
}

func runRecord(s *truth.Store, emb *embed.Client, args []string) {
	fs := flag.NewFlagSet("record", flag.ExitOnError)
	title := fs.String("title", "", "short title of the result (required)")
	result := fs.String("result", "", "the result body: changelog/decision/closed task (required)")
	docType := fs.String("type", "changelog", "changelog|task|decision")
	sourceRef := fs.String("source-ref", "", "link to the source task/spec this answers")
	author := fs.String("author", "", "who produced this result")
	jsonOut := fs.Bool("json", false, "output JSON")
	fs.Parse(args)

	if *title == "" || *result == "" {
		fatalf("--title and --result required")
	}
	db, err := knowledge.Open(s.WriteLogPath())
	if err != nil {
		fatalf("open write-log: %v", err)
	}
	defer db.Close()

	meta := metaJSON(*author, "", "", *sourceRef)
	id, err := knowledge.Add(db, *title, *result, *docType, meta, embedOrNil(emb, *result))
	if err != nil {
		fatalf("record: %v", err)
	}
	// Feedback-loop provenance lives in the control DB, keyed by the write-log
	// doc id (VACUUM INTO preserves the id into the next release).
	if err := s.RecordProvenance(id, *author, *sourceRef); err != nil {
		fatalf("provenance: %v", err)
	}
	if *jsonOut {
		json.NewEncoder(os.Stdout).Encode(map[string]any{"id": id, "type": *docType})
	} else {
		fmt.Fprintf(os.Stderr, "recorded id=%d (%s) → rides the next publish\n", id, *docType)
	}
}

func runPublish(s *truth.Store, args []string) {
	fs := flag.NewFlagSet("publish", flag.ExitOnError)
	name := fs.String("name", "", "release version, e.g. 2026.07 or 2026.07.1 (required)")
	notes := fs.String("notes", "", "release notes")
	channel := fs.String("channel", "", "optionally point this channel at the new release")
	jsonOut := fs.Bool("json", false, "output JSON")
	fs.Parse(args)

	if *name == "" {
		fatalf("--name required")
	}
	rel, err := s.Publish(*name, *notes)
	if err != nil {
		fatalf("publish: %v", err)
	}
	if *channel != "" {
		if err := s.SetChannel(*channel, rel.Version); err != nil {
			fatalf("point channel: %v", err)
		}
	}
	if *jsonOut {
		json.NewEncoder(os.Stdout).Encode(map[string]any{
			"version": rel.Version, "path": s.ReleasePath(rel.Version), "channel": *channel})
	} else {
		fmt.Fprintf(os.Stderr, "published truth-%s → %s\n", rel.Version, s.ReleasePath(rel.Version))
		if *channel != "" {
			fmt.Fprintf(os.Stderr, "channel %s → %s\n", *channel, rel.Version)
		}
	}
}

func runSetChannel(s *truth.Store, args []string) {
	fs := flag.NewFlagSet("set-channel", flag.ExitOnError)
	name := fs.String("name", "", "channel: stable|testing (required)")
	release := fs.String("release", "", "release version to point at (required)")
	fs.Parse(args)
	if *name == "" || *release == "" {
		fatalf("--name and --release required")
	}
	if err := s.SetChannel(*name, *release); err != nil {
		fatalf("set-channel: %v", err)
	}
	fmt.Fprintf(os.Stderr, "channel %s → %s\n", *name, *release)
}

func runChannels(s *truth.Store, args []string) {
	fs := flag.NewFlagSet("channels", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "output JSON")
	fs.Parse(args)
	chans, err := s.Channels()
	if err != nil {
		fatalf("channels: %v", err)
	}
	if *jsonOut {
		json.NewEncoder(os.Stdout).Encode(chans)
		return
	}
	for _, c := range chans {
		rel := c.Release
		if rel == "" {
			rel = "(unassigned)"
		}
		fmt.Printf("%-9s → %s\n", c.Name, rel)
	}
}

func runReleases(s *truth.Store, args []string) {
	fs := flag.NewFlagSet("releases", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "output JSON")
	fs.Parse(args)
	rels, err := s.ListReleases()
	if err != nil {
		fatalf("releases: %v", err)
	}
	if *jsonOut {
		json.NewEncoder(os.Stdout).Encode(rels)
		return
	}
	if len(rels) == 0 {
		fmt.Fprintln(os.Stderr, "(no releases published yet)")
		return
	}
	for _, r := range rels {
		fmt.Printf("truth-%-10s  %s\n", r.Version, r.Notes)
	}
}

func runSearch(s *truth.Store, embc *embed.Client, args []string) {
	fs := flag.NewFlagSet("search", flag.ExitOnError)
	query := fs.String("query", "", "query string (required)")
	ref := fs.String("channel", truth.ChannelStable, "channel (stable|testing|unstable) or a release version")
	mode := fs.String("mode", "hybrid", "fts|vec|hybrid|regex")
	embeddingRaw := fs.String("embedding", "", "comma-separated float32 for vec/hybrid (BYO; overrides --embed-model)")
	limit := fs.Int("limit", 10, "max results")
	prefix := fs.Bool("prefix", true, "auto-append wildcard to FTS tokens")
	jsonOut := fs.Bool("json", false, "output JSON")
	fs.Parse(args)

	if *query == "" {
		fatalf("--query required")
	}
	path, err := s.Resolve(*ref)
	if err != nil {
		fatalf("resolve %q: %v", *ref, err)
	}
	db, err := truth.OpenRelease(path)
	if err != nil {
		fatalf("open %s: %v", path, err)
	}
	defer db.Close()

	// Query embedding: explicit --embedding wins; else the configured
	// --embed-model embeds the query live for vec/hybrid modes.
	emb := parseEmbedding(*embeddingRaw)
	if len(emb) == 0 && embc.Enabled() && (*mode == "vec" || *mode == "hybrid") {
		if emb, err = embc.Embed(*query); err != nil {
			fatalf("embed query: %v", err)
		}
	}
	var results []knowledge.Result
	switch *mode {
	case "fts":
		results, err = knowledge.SearchFTS(db, *query, *limit, *prefix)
	case "vec":
		if len(emb) == 0 {
			fatalf("--embedding required for vec mode")
		}
		results, err = knowledge.SearchVec(db, emb, *limit, knowledge.MetricCosine, "")
	case "regex":
		results, err = knowledge.SearchRegex(db, *query, *limit, "")
	default:
		results, err = knowledge.SearchHybrid(db, *query, emb, *limit, knowledge.MetricCosine, "", *prefix)
	}
	if err != nil {
		fatalf("search: %v", err)
	}

	if *jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(results)
		return
	}
	fmt.Fprintf(os.Stderr, "# %d results against %s\n", len(results), filepath.Base(path))
	for _, r := range results {
		fmt.Printf("[%d] %s  (%s)\n    %s\n\n", r.ID, r.Title, r.Type, r.Preview(120))
	}
}

func runSuggest(s *truth.Store, args []string) {
	fs := flag.NewFlagSet("suggest", flag.ExitOnError)
	prefix := fs.String("prefix", "", "term prefix — the vocabulary front-door for FTS (required)")
	ref := fs.String("channel", truth.ChannelStable, "channel or release version")
	relativeTo := fs.String("relative-to", "", "compute IDF relative to a partition (a doc type, e.g. reference_ru) instead of the whole corpus")
	numbers := fs.Bool("numbers", false, "include pure-digit tokens (page/line numbers), off by default as they are useless keys")
	limit := fs.Int("limit", 20, "max terms")
	jsonOut := fs.Bool("json", false, "output JSON")
	fs.Parse(args)

	if *prefix == "" {
		fatalf("--prefix required")
	}
	path, err := s.Resolve(*ref)
	if err != nil {
		fatalf("resolve %q: %v", *ref, err)
	}
	db, err := truth.OpenRelease(path)
	if err != nil {
		fatalf("open %s: %v", path, err)
	}
	defer db.Close()

	var terms []knowledge.Term
	if *relativeTo != "" {
		terms, err = knowledge.SuggestRelativeTo(db, *prefix, *relativeTo, *limit, *numbers)
	} else {
		terms, err = knowledge.Suggest(db, *prefix, *limit, *numbers)
	}
	if err != nil {
		fatalf("suggest: %v", err)
	}
	if *jsonOut {
		json.NewEncoder(os.Stdout).Encode(terms)
		return
	}
	scope := "corpus"
	if *relativeTo != "" {
		scope = "partition " + *relativeTo
	}
	fmt.Fprintf(os.Stderr, "# %d terms matching %q* in %s  (IDF relative to %s; higher = sharper key)\n",
		len(terms), *prefix, filepath.Base(path), scope)
	for _, t := range terms {
		mark := ""
		if t.Weak() {
			mark = "  ← weak key (too common)"
		}
		fmt.Printf("  %-24s docs=%-5d count=%-5d idf=%.2f%s\n", t.Term, t.Docs, t.Count, t.IDF, mark)
	}
}

// runRead is the "read the full page/chunk range" primitive from
// docs/docsearch-server/HOW_TO_USE.md step 5: a search snippet tells you
// *where* to look; this is how you actually read it, instead of answering
// from one truncated chunk. Two modes:
//
//	--id N [--context K]   the contiguous neighborhood id-K..id+K (default K=2)
//	--source <tag>         the whole ingested source file (its --source-version
//	                        provenance tag), in ingest order
//
// Before this command existed, reconstructing a document or a chunk's
// neighborhood required a throwaway SQL script against the release file
// directly — this is that script, promoted to a first-class primitive.
func runRead(s *truth.Store, args []string) {
	fs := flag.NewFlagSet("read", flag.ExitOnError)
	id := fs.Int64("id", 0, "chunk id to read (with --context)")
	context := fs.Int("context", 2, "chunks before/after --id to include")
	source := fs.String("source", "", "read the whole source file by its source_version provenance tag, instead of --id")
	ref := fs.String("channel", truth.ChannelStable, "channel or release version")
	jsonOut := fs.Bool("json", false, "output JSON")
	fs.Parse(args)

	if *source == "" && *id == 0 {
		fatalf("--id (with optional --context) or --source required")
	}
	path, err := s.Resolve(*ref)
	if err != nil {
		fatalf("resolve %q: %v", *ref, err)
	}
	db, err := truth.OpenRelease(path)
	if err != nil {
		fatalf("open %s: %v", path, err)
	}
	defer db.Close()

	var docs []knowledge.Doc
	if *source != "" {
		docs, err = knowledge.ReadBySource(db, *source)
	} else {
		docs, err = knowledge.ReadRange(db, *id, *context)
	}
	if err != nil {
		fatalf("read: %v", err)
	}

	if *jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(docs)
		return
	}
	if len(docs) == 0 {
		fmt.Fprintln(os.Stderr, "(no chunks found)")
		return
	}
	fmt.Fprintf(os.Stderr, "# %d chunks from %s\n", len(docs), filepath.Base(path))
	for _, d := range docs {
		fmt.Printf("--- id=%d  %s  (%s) ---\n%s\n\n", d.ID, d.Title, d.Type, d.Content)
	}
}

// runEnumerate is the completeness-audit primitive from
// docs/docsearch-server/HOW_TO_USE.md step 4: "did I find every X, not just
// the first one" — e.g. every PB_Cipher_* constant actually used in the
// corpus, not just the ones a guess list happened to include. Distinct from
// `search --mode regex`, which returns whole matching *documents*; this
// returns the distinct matched *substrings*, tallied. Replaces the
// `search --json | grep -o <pattern> | sort -u` this session did by hand.
func runEnumerate(s *truth.Store, args []string) {
	fs := flag.NewFlagSet("enumerate", flag.ExitOnError)
	pattern := fs.String("pattern", "", "regex pattern to enumerate distinct matches of, across the whole corpus (required)")
	ref := fs.String("channel", truth.ChannelStable, "channel or release version")
	limit := fs.Int("limit", 50, "max distinct matches (0 = unlimited)")
	jsonOut := fs.Bool("json", false, "output JSON")
	fs.Parse(args)

	if *pattern == "" {
		fatalf("--pattern required")
	}
	path, err := s.Resolve(*ref)
	if err != nil {
		fatalf("resolve %q: %v", *ref, err)
	}
	db, err := truth.OpenRelease(path)
	if err != nil {
		fatalf("open %s: %v", path, err)
	}
	defer db.Close()

	matches, err := knowledge.Enumerate(db, *pattern, *limit)
	if err != nil {
		fatalf("enumerate: %v", err)
	}

	if *jsonOut {
		json.NewEncoder(os.Stdout).Encode(matches)
		return
	}
	fmt.Fprintf(os.Stderr, "# %d distinct matches of /%s/ in %s\n", len(matches), *pattern, filepath.Base(path))
	for _, m := range matches {
		fmt.Printf("  %-30s docs=%-5d count=%d\n", m.Value, m.Docs, m.Count)
	}
}

func parseEmbedding(raw string) []float32 {
	raw = strings.Trim(strings.TrimSpace(raw), "[]")
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
			fatalf("bad embedding value %q: %v", p, err)
		}
		out = append(out, float32(f))
	}
	return out
}

func printUsage() {
	fmt.Fprint(os.Stderr, `docsearch-server — versioned truth server for teams (see docs/docsearch-server/TZ.md)

Rewrite (truth flows in):
  ingest      --title/--content or --file [--type --role-tags --author --source-version --strip-runes]
  record      --title --result [--type changelog|task|decision --source-ref --author]
  publish     --name <ver> [--notes --channel]
  set-channel --name stable|testing --release <ver>

Readonly (grounding):
  search      --query <q> [--channel stable|testing|unstable|<ver>] [--mode --embedding --limit]
  suggest     --prefix <p> [--channel <c> --relative-to <type> --numbers --limit N]   (FTS vocabulary + IDF; pure-digit tokens filtered unless --numbers)
  read        --id <n> [--context K] | --source <tag> [--channel <c>]   (read the full contiguous chunk range or whole source file — see docs/docsearch-server/HOW_TO_USE.md)
  enumerate   --pattern <regex> [--channel <c> --limit N]   (distinct matches across the corpus, tallied — the completeness-audit step)
  serve       --addr <a> --channel stable|testing [--pool N --lite]   (async read-server, hot-swaps on channel repoint)
  mcp         (MCP server over stdio — first-class interface for LLM agents)
  releases    [--json]
  channels    [--json]

Global:
  --root <dir>          data root (default .docsearch)
  --embed-model <m>     Ollama-compatible embedding model to enable vector search (e.g. qwen3-embedding:0.6b)
  --embed-url <url>     embedding endpoint (default http://localhost:11434/api/embed)
  --version
`)
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "error: "+format+"\n", args...)
	os.Exit(1)
}

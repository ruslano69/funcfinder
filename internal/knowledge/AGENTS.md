# internal/knowledge/

## Purpose

Persistent knowledge base backed by a single SQLite file. Combines FTS5 (BM25 keyword search) and cosine vector similarity search with Reciprocal Rank Fusion for hybrid retrieval. Ingests `.txt` (with charset detection), `.md`, and `.pdf` files with section-aware chunking, and crawls documentation websites (`web.go`). Designed for AI agents that accumulate documentation, error notes, and tool-usage records across sessions.

## Ownership

- All database logic, schema, and search algorithms live here.
- `cmd/docsearch/` is the only consumer; it must not duplicate business logic.
- Embeddings are stored as raw little-endian float32 BLOBs; cosine similarity is computed in Go (no native extension required).

## Local Contracts

- `Open(path string) (*sql.DB, error)` — open or create the knowledge base, apply schema, return a ready connection.
- `Add(db, title, content, type, metadata, embedding) (int64, error)` — insert a document; embedding may be nil.
- `IngestFile(path, ChunkOpts) ([]Chunk, error)` — read a file and split into indexable Chunks; dispatches by extension.
- `IngestURL(rootURL, CrawlOpts, CrawlProgress) ([]Chunk, error)` — crawl a docs site (same host+path prefix, content-hash dedup, `@version` normalization, `<main>`/`<article>` extraction) into Chunks. `IngestWeb` is an alias.
- `ChunkOpts{MaxRunes, OverlapRunes}` — chunking parameters (defaults: 800 / 80).
- `Delete(db, id)` — remove document and its embedding (cascade).
- `Count(db) (int64, error)` — total document count.
- `SearchFTS(db, query, limit)` — BM25 keyword search via FTS5.
- `SearchVec(db, embedding, limit)` — cosine-distance vector search (loads all embeddings into Go).
- `SearchHybrid(db, query, embedding, limit)` — Reciprocal Rank Fusion over FTS5 + vector results.
- Schema is applied idempotently on every `Open` call (all `CREATE ... IF NOT EXISTS`).
- FTS index is kept in sync via three SQL triggers (insert/delete/update on `docs`).

## Schema

```
docs       — canonical rows: id, title, content, type, created_at, metadata
docs_fts   — FTS5 virtual table (content='docs')
docs_vec   — doc_id, dim, embedding BLOB (float32 LE)
```

## Ingestion pipeline

```
IngestFile → ingestTXT / ingestMD / ingestPDF
               ↓
           docSection[]   (title + []paragraph)
               ↓
           sectionsToChunks  (chunk.go)
               ↓
           []Chunk  → caller calls Add() for each
```

- **TXT**: `toUTF8()` detects encoding via BOM → utf8.Valid → `charset.DetermineEncoding` fallback. Splits on blank lines.
- **MD**: regex splits on ATX headers (`# … ######`). `splitParagraphsMD` treats fenced code blocks as atomic — blank lines inside ` ``` ` do not create paragraph breaks. Section content never crosses a header boundary.
- **PDF**: `ledongthuc/pdf`, page by page. `normalizeWhitespace` collapses PDF spacing artefacts before paragraph split.
- **Web** (`web.go`): `IngestURL` BFS-crawls from a root URL, fetching HTML pages within the same host+path prefix (`isSameSite`), deduping by body hash and normalizing `@version` paths, extracting `<main>`/`<article>` text (chrome tags skipped) into the same `docSection` → `sectionsToChunks` path.
- **Chunking**: greedy fill up to `MaxRunes`, split only at paragraph boundaries, optional overlap (`OverlapRunes` runes from previous chunk's tail) for RAG context continuity.

## Work Guidance

- To add a new format, add `ingestXYZ.go` and register the extension in `IngestFile`.
- To add a new search mode, implement it in `search.go` and expose it through `SearchHybrid` or as a new exported function.
- Vector search is O(n) — acceptable for knowledge bases up to ~100k entries. If scale demands it, add an HNSW index layer without changing the public API.
- Do not add CGO dependencies; the pure-Go `modernc.org/sqlite` driver is intentional.

## Verification

```bash
go test ./internal/knowledge/...
```

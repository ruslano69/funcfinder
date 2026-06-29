# internal/knowledge/

## Purpose

Persistent knowledge base backed by a single SQLite file. Combines FTS5 (BM25 keyword search) and cosine vector similarity search with Reciprocal Rank Fusion for hybrid retrieval. Designed for AI agents that accumulate documentation, error notes, and tool-usage records across sessions.

## Ownership

- All database logic, schema, and search algorithms live here.
- `cmd/docsearch/` is the only consumer; it must not duplicate business logic.
- Embeddings are stored as raw little-endian float32 BLOBs; cosine similarity is computed in Go (no native extension required).

## Local Contracts

- `Open(path string) (*sql.DB, error)` — open or create the knowledge base, apply schema, return a ready connection.
- `Add(db, title, content, type, metadata, embedding) (int64, error)` — insert a document; embedding may be nil.
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

## Work Guidance

- To add a new search mode, implement it in `search.go` and expose it through `SearchHybrid` or as a new exported function.
- Vector search is O(n) — acceptable for knowledge bases up to ~100k entries. If scale demands it, add an HNSW index layer without changing the public API.
- Do not add CGO dependencies; the pure-Go `modernc.org/sqlite` driver is intentional.

## Verification

```bash
go test ./internal/knowledge/...
```

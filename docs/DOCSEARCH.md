# docsearch

A standalone knowledge-base CLI bundled with the funcfinder toolkit. It stores
documents in a single SQLite file and searches them with full-text search
(FTS5), vector similarity, regex, or a hybrid of FTS + vector — useful for
building a persistent memory/knowledge store an agent can query across
sessions (notes, runbooks, ingested docs, error logs, etc.).

Binary: `docsearch` (built from `cmd/docsearch`).

---

## Storage model

Everything lives in one SQLite file (default `.knowledge/docs.sqlite`,
override with `--db <path>`). `init` creates it if missing; `add` and `search`
also auto-create it via `Open`, so an explicit `init` is optional but cheap
and useful for scripting.

Schema (`internal/knowledge/db.go`):

| Table | Purpose |
|-------|---------|
| `docs` | `id, title, content, type, created_at, metadata` — the source of truth |
| `docs_fts` | FTS5 virtual table, kept in sync with `docs` via `AFTER INSERT/UPDATE/DELETE` triggers |
| `docs_vec` | `doc_id, dim, embedding (BLOB)` — optional vector per doc |

`type` is a free-form string (`general`, `tool_usage`, `error`, `scenario`,
...) used only for `--filter-type` pre-filtering; it isn't validated against
a fixed enum.

---

## Actions

```
docsearch [--db <path>] init
docsearch [--db <path>] add    --title <t> --content <c> [--type <t>] [--meta <json>] [--embedding <floats>] [--json]
docsearch [--db <path>] add    --file <path.txt|md|pdf>  [--type <t>] [--chunk-size N] [--chunk-overlap N] [--json]
docsearch [--db <path>] search --query <q>               [--embedding <floats>] [--mode fts|vec|hybrid|regex] [--metric cosine|l2] [--filter-type <type>] [--limit N] [--json]
docsearch [--db <path>] count  [--json]
docsearch --version
```

The `--db` flag must come **before** the action word (`init`/`add`/`search`/`count`);
everything after the action word is parsed by that action's own flag set.

### init

```bash
docsearch --db .knowledge/docs.sqlite init
```

Creates the parent directory and the SQLite file/schema. Idempotent — safe to
call every time a script starts.

### add — single document

```bash
docsearch add --title "Go error handling" \
  --content "Errors are values. Use errors.Is/errors.As to check types." \
  --type tool_usage --meta '{"source":"manual"}' --json
# {"id":1}
```

`--title` and `--content` are required unless `--file` is used. `--embedding`
accepts comma-separated floats, optionally wrapped in `[...]`.

### add — file ingestion (chunked)

```bash
docsearch add --file notes.md --type general --chunk-size 800 --chunk-overlap 80 --json
# {"chunks":2,"ids":[3,4]}
```

Supported extensions: `.txt`, `.md`, `.pdf` (dispatch by extension in
`internal/knowledge/ingest.go`). The file is split into `docSection`s, then
into `Chunk`s:

- **`.md`** — split on top-level headings (`splitMDSections`); each section
  becomes its own chunk boundary (chunks never cross section boundaries).
- **`.txt`** — converted to UTF-8 (`toUTF8`) then split into paragraphs.
- **`.pdf`** — see [PDF ingestion](#pdf-ingestion-and-ocr-quality-gate) below.

Within a section, paragraphs are merged greedily up to `--chunk-size` runes;
oversized sections are split at paragraph boundaries with `--chunk-overlap`
runes of overlap between consecutive chunks so context isn't lost at the
seam. Chunks that look like filler (repeated runs of punctuation — tables of
contents, `"--------"`, `". . . . 374"`) are dropped (`hasRepetitiveRuns`).

Each chunk is inserted via `--type`/`--meta` you supplied; embeddings are not
auto-generated for file ingestion — add them yourself in a follow-up pass if
needed (vector search requires an embedding per doc; see [Vector search](#vector-search--embeddings)).

#### PDF ingestion and OCR quality gate

PDF text is extracted by position (`extractPageText`): elements are grouped
into lines by Y-coordinate and spaced by X-gap, because the underlying
`ledongthuc/pdf` library's plain-text extraction glues words together when a
PDF has no explicit space glyphs. If the position-based result still looks
glued (`looksGlued` — few tokens or average word length > 15 chars), it falls
back to the library's `GetPlainText`.

Before committing to a full parse, `docsearch` samples up to 10 evenly-spaced
pages (skipping the first/last ~5%, which are often covers/indices) and
scores text quality (`pageTextQuality`: letter ratio, word-likeness ratio,
penalty for excessive single-character tokens — a signature of spaced-out
OCR like `"T e x t"`). If the average score is below `0.45`, ingestion is
**rejected** with an `OCRQualityError` rather than polluting the knowledge
base with garbage:

```bash
docsearch add --file scan.pdf --json
# {"error":"bad_ocr","score":0.31,"file":"scan.pdf"}   (exit code 2)
```

Without `--json` it prints a human-readable warning to stderr suggesting
`ocrmypdf` to fix the source file. This is a hard rejection, not a soft
warning — fix the PDF and re-run.

### search

```bash
docsearch search --query "error handling" --mode fts --limit 5 --json
```

| Flag | Default | Meaning |
|------|---------|---------|
| `--query` | — | FTS/regex query text (required unless `--embedding` given for `vec` mode) |
| `--embedding` | — | comma-separated floats for `vec`/`hybrid` modes |
| `--mode` | `hybrid` | `fts` \| `vec` \| `hybrid` \| `regex` |
| `--metric` | `cosine` | `cosine` \| `l2` — distance metric for `vec`/`hybrid` |
| `--filter-type` | — | restrict to one `type` before scoring (vec/regex modes) |
| `--limit` | `10` | max results |
| `--prefix` | `true` | auto-append `*` to FTS tokens (`call` → `call*`) so partial words match |
| `--json` | `false` | structured output instead of the text snippet view |

#### Search modes

- **`fts`** — SQLite FTS5 full-text search over `title`+`content`
  (`BuildFTSQuery`/`SearchFTS`). Fast, no embedding needed. Best for keyword
  lookups.
- **`vec`** — pure vector similarity against `docs_vec` using the chosen
  `--metric`. Requires `--embedding`; only docs that have a stored embedding
  are searchable. Best when you have a real embedding model and want
  semantic recall.
- **`regex`** — Go regex match over `content` performed in application code
  (`SearchRegex`), with optional `--filter-type`. Use for structural lookups
  FTS can't express (e.g. matching a specific error code pattern).
- **`hybrid`** (default) — runs both FTS and vector scoring and combines them
  into `HybridScore` (`SearchHybrid`). If you don't pass `--embedding`, it
  degenerates gracefully to FTS-only ranking — so `hybrid` is a safe default
  even before you have embeddings wired up.

JSON output fields are `id, title, content, type, created_at, metadata`, plus
whichever of `fts_rank` / `vec_dist` / `hybrid_score` the mode populated
(zero-value fields are omitted — e.g. an exact vector match has `vec_dist: 0`
and won't show the key at all).

### count

```bash
docsearch count --json   # {"count":4}
```

---

## Vector search & embeddings

`docsearch` does **not** compute embeddings itself — it stores and compares
whatever float vector you pass in via `--embedding`. Generate embeddings with
whatever model you have available (local or API) and pass them as
comma-separated floats. `internal/knowledge/vector.go` implements cosine and
L2 distance directly in SQL via registered SQLite functions
(`vec_distance_cosine`, `vec_distance_l2`) operating on the raw float32 BLOB,
so distance computation happens inside the query, not in Go.

If you never supply embeddings, stick to `--mode fts` or the default
`hybrid` (which falls back to FTS-only) — `vec` mode simply returns nothing
useful without them.

---

## Quick reference

```bash
# Build (part of build.ps1 / build.sh)
go build -o docsearch.exe ./cmd/docsearch

# One-time setup
docsearch --db .knowledge/docs.sqlite init

# Add a manual note
docsearch add --title "..." --content "..." --type general

# Ingest a doc, chunked
docsearch add --file README.md --type general

# Search (defaults to hybrid)
docsearch search --query "your question" --limit 5

# Check size
docsearch count
```

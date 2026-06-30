# Skill: docsearch — Project Knowledge Base

Use this skill when you need to persist notes, ingested documentation, or
findings across sessions, and later retrieve them by keyword, regex, or
vector similarity, instead of re-reading source files or re-deriving facts
each time.

---

## When to invoke

- "remember this for later" / "save this finding to the knowledge base"
- "what do we already know about X" / "search past notes for X"
- "ingest this doc/PDF/README into the knowledge base"
- "build a knowledge base for this project"
- any task requiring durable, queryable memory scoped to one project

---

## Mental model

One project = one SQLite file (default `.knowledge/docs.sqlite`). Three
backing tables: `docs` (source of truth), `docs_fts` (full-text index, kept
in sync via triggers), `docs_vec` (optional embedding per doc). `docsearch`
does **not** generate embeddings — only stores/compares ones you supply.

Full reference: [docs/DOCSEARCH.md](../docs/DOCSEARCH.md).

---

## Phase 1 — Init (once per project)

```bash
docsearch --db .knowledge/docs.sqlite init
```

Idempotent — safe to call at the start of every session.

---

## Phase 2 — Add knowledge

### A single note or finding
```bash
docsearch add --title "<short title>" --content "<the actual text>" \
  --type general --json
```

Use `--type` to tag what kind of entry this is — `general`, `tool_usage`,
`error`, `scenario`, or any project-specific tag. It's free-form and only
used later for `--filter-type`.

### A whole file (chunked automatically)
```bash
docsearch add --file README.md --type general --json
docsearch add --file spec.pdf  --type general --json
```

Supports `.txt`, `.md`, `.pdf`. One call = one file — to ingest a directory,
loop over it:
```bash
for f in docs/*.md; do
  docsearch add --file "$f" --type general --json
done
```

**PDF note**: bad-OCR scans are rejected outright (`{"error":"bad_ocr","score":...}`,
exit code 2) rather than polluting the knowledge base — if you hit this, the
source PDF needs re-OCR'ing (e.g. `ocrmypdf`), not a retry.

---

## Phase 3 — Search

```bash
# Keyword search (fast, no embedding needed)
docsearch search --query "<keywords>" --mode fts --limit 5 --json

# Default: hybrid (FTS + vector if you have embeddings; degrades to FTS alone if not)
docsearch search --query "<keywords>" --json

# Structural/pattern match
docsearch search --query "<regex>" --mode regex --json

# Semantic search (requires an embedding you generated yourself)
docsearch search --embedding "0.1,0.2,..." --mode vec --json
```

`--mode fts` (or the `hybrid` default without an embedding) covers the
overwhelming majority of "do we already know X" lookups. Reach for `regex`
when FTS tokenization won't match what you need (e.g. an exact error code).
Reach for `vec`/`hybrid` with embeddings only when you have a real embedding
model wired into the session — otherwise it's a no-op.

---

## Phase 4 — Sanity check

```bash
docsearch count --json
```

Confirms the knowledge base isn't empty before relying on search results.

---

## Golden Rule

```
init (once) → add (notes + files as you learn things) → search (before re-deriving anything)
```

Treat the knowledge base as cheaper than re-reading files or re-running
analysis: search it first, only fall back to fresh investigation (e.g. via
the `funcfinder` skill) when the knowledge base comes up empty — then add
what you find back in.

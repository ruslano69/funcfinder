# How to search — the path to success, not blind search

`docsearch-server` gives you three primitives (`suggest`, `search`, raw content
by id) and BM25 ranking. None of that guarantees a good answer by itself — a
single `search` call against a guessed query string is blind search: it might
hit, might hit noise, might miss the term entirely. This document is the
methodology that turns those primitives into a reliable path to the right
answer, distilled from live sessions against a 1800-page corpus (the PureBasic
manual, 3740 chunks) and a 33-file project-memory corpus.

Written for **agents driving docsearch-server** (via CLI or MCP) as much as for
humans — it is a workflow to follow, not just background reading.

## The loop

```
suggest (cheap, verify vocabulary)
  → search with a naming hypothesis (expect to miss; that's fine)
    → recognize noise (don't trust the first hit blindly)
      → completeness audit (did I miss a category?)
        → read the full page/chunk range by id (never answer from a snippet alone)
```

### 1. Probe the vocabulary first — `suggest`, not `search`

`suggest --prefix <p>` reads `docs_vocab` (a live `fts5vocab` view) — it costs
nothing and never touches ranking. Before spending a real query, check that
your term (or a plausible stem of it) actually exists in the corpus:

```bash
docsearch-server suggest --prefix "hash" --limit 15
docsearch-server suggest --prefix "fingerprint" --limit 15
```

Run it for **several candidate stems at once**, not just your first guess.
The corpus's own vocabulary — not your assumption of what it should be called —
is the source of truth. In the crypto-hash example, `hash` existed but was
weak (few docs); `fingerprint` turned out to be the term the manual actually
uses. `suggest` found that before a single real search was spent.

IDF (`Term.IDF`, exposed by `suggest`) tells you whether a term is a *sharp*
search key or a *weak* one (`Term.Weak()`, threshold `WeakKeyIDF = 1.4`) —
weak keys flood results with low-relevance matches. Prefer the sharper term
when you have a choice between synonyms.

### 2. Search with a naming hypothesis — and expect to be wrong

Domain corpora (API manuals, codebases, internal wikis) have naming
*conventions*. Guess the convention, search it, and treat a zero-result or a
wrong-looking result as information, not failure:

```bash
docsearch-server search --query "UseSHA2Fingerprint" --mode fts --limit 5
```

If the naming convention was right, this jumps straight past hundreds of
irrelevant pages to the exact reference entry. If it's wrong, you get zero
hits or noise — cheaply, in one query — and you revise the hypothesis. This
happened live: guessing `"Prototype EndPrototype"` (analogy with
`Import:EndImport`) returned **zero results**, because `EndPrototype` isn't a
real keyword. The correction (`"prototypes chapter"`) found it immediately.
**A zero-result query is not wasted — it prunes a wrong branch fast.**

### 3. Recognize noise before trusting a hit

BM25-ranked ≠ relevant to *your* question. A result can rank well while being
the wrong *kind* of page. In the crypto-hash search, `"Fingerprint library"`
returned real matches — but they were **changelog pages** ("Old:
`ExamineMD5Fingerprint()` / New: `UseMD5Fingerprint()`"), not the reference
section. Recognizing that from the snippet (keyword-in-context makes this
possible — see below) and re-querying with a more specific term
(`UseSHA2Fingerprint` instead of `"Fingerprint library"`) is what actually
found the reference chapter.

Rule of thumb: if the top hits share a suspicious shape (all "Old:/New:", all
one section type, all cross-references instead of definitions), the query is
underspecified — tighten it before reading further.

### 4. Audit for completeness — don't stop at the first hit

Once you have a real anchor (a section, a constant, a naming pattern), check
whether you found *all* instances of the category, not just the first one.
For the crypto-hash question, extracting every `PB_Cipher_*` constant
mentioned in the corpus (`grep -o 'PB_Cipher_[A-Za-z0-9]*' | sort -u` over the
JSON output) surfaced `PB_Cipher_HMAC` — a real algorithm-modifier that wasn't
in the original guess list (`MD5/SHA1/SHA2/SHA3/CRC32`).

This is a different move from steps 1–3: it's not "find the answer", it's
"prove I'm not missing part of the answer". Do it whenever the question has
the shape of an enumeration ("which X exist", "what are the Y").

### 5. Read the full page/chunk range — never answer from a snippet alone

A snippet (even a good keyword-in-context one, see below) is for **deciding
what to read**, not for **reading**. Reference material is chunked at ~800
runes with paragraph-boundary splitting (see `internal/knowledge/chunk.go`) —
a function's Syntax/Description/Parameters/Example/See-Also block routinely
spans 2–4 chunks. Answering from one chunk risks an incomplete or
out-of-context picture.

Once `search` gives you an anchor (a chapter number, a page number, a chunk
id), pull the **contiguous id range** around it and read it whole:

```bash
docsearch-server read --id <n> --context 3
```

or, to reconstruct one ingested source file completely, by its `source_version`
provenance tag:

```bash
docsearch-server read --source "manual.pdf"
```

(`read` is a first-class CLI/MCP primitive — see below; it used to require a
throwaway SQL script, which is how this gap was found in the first place.)

This is the step that turned "here are some snippets that mention inspect" into
a complete, accurate description of the HR/AX2009 integration doc, the full
Cipher chapter, and the full Library/Prototypes chapters. **The synthesis
happens on the full text, never on fragments.**

## What the tool does for you (so you don't have to route around it)

- **`snippet()` keyword-in-context** (added this session): FTS results carry a
  `Snippet` centered on the actual match, token-safe (never splits a
  multi-byte rune). This is what makes step 3 (recognize noise) possible from
  the CLI output alone — you see *why* a doc matched, not just its opening
  bytes. Use `Result.Preview(n)` when rendering results; don't re-truncate
  `Content` by byte index yourself (that reintroduces the U+FFFD bug this
  session found twice).
- **IDF-ranked `suggest`**: turns "guess a keyword and hope" into "look up
  what's really there" — the cheap, precise front door described in step 1.
- **`SearchHybrid`**: reciprocal-rank-fusion of FTS + vector when an embedder
  is configured — same loop applies, just with a second candidate-generation
  path.
- **`read --id <n> [--context k]` / `read --source <tag>`**: step 5's
  primitive. Returns the contiguous chunk neighborhood around a match, or an
  entire ingested source file by its `source_version` provenance tag, in id
  order — bypassing FTS entirely. Implemented in
  `internal/knowledge/read.go` (`ReadRange`, `ReadBySource`), wired into the
  CLI in `cmd/docsearch-server/main.go`. This used to require a throwaway SQL
  script every time; it doesn't anymore.

## What's still missing

Step 4 (the completeness audit — "did I miss a category") was still done **by
hand** this session: a raw `grep` over `search --json` output to enumerate
every `PB_Cipher_*` constant mentioned in the corpus. There is no first-class
primitive for "enumerate all matches of a pattern across the corpus" yet — a
candidate: `docsearch-server enumerate --pattern <regex>` (or expose
`SearchRegex` more directly for this use), returning distinct matches rather
than ranked documents. Until it exists, step 4's manual substitute is a
`search --json` + `grep`/`jq` pass over the output.

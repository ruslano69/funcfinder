# benchmarks — the spec-sheet harness

funcfinder's pitch is **comprehension at ~5% of the token cost and a fraction of
the time**, with a map that's *good enough to navigate* — not a perfect parser.
This harness produces the honest, per-project numbers that back that claim, and
it does so reproducibly so anyone can re-run them.

## The AST ruler (not a competitor — a measuring tape)

`cmd/astoracle` is a real Go parser (`go/ast`, zero heuristics). It emits the
exact same JSON shape as `funcfinder --dir <d> --all --json`, so the two can be
diffed directly. It is the **ground truth** against which funcfinder's regex
output is scored. Go only, on purpose: the ruler must be unimpeachable on the
flagship language. Other languages get their own native oracle (Python `ast`,
the TS compiler API, …) or tree-sitter as a second tier.

The ruler exists to *measure*, never to become the product. The target is not
"match the AST 1:1" — past "the agent reaches the right code", extra accuracy
costs tokens and time, which are the product. The ruler tells us **where** the
misses are so we can split them into cheap regex fixes (community engine) vs
fundamentally-hard cases (the parser-tier's reason to exist), quantified.

## The two columns this produces (phase 1)

- **Accuracy** — recall + precision of symbols vs the ruler, matched by
  `(name, line ±2)` per file, computed over the file intersection so
  file-selection differences don't corrupt the number.
- **Token savings** — tokens of the funcfinder map vs the raw source it
  replaces.

Time-to-comprehension and task-success are **phase 2**: they need an
agent-in-the-loop runner and are deliberately not faked here.

## Run it

```bash
# build the funcfinder + oracle binaries (from repo root)
go build -o funcfinder.exe ./cmd/funcfinder
go build -o astoracle.exe  ./cmd/astoracle

# point at a checked-out reference project (pin a commit for reproducibility)
funcfinder.exe --dir <proj> --all --json --no-gitignore > ff.json
astoracle.exe  <proj> > oracle.json

python benchmarks/specsheet.py \
  --funcfinder ff.json --oracle oracle.json \
  --root <proj> --label "tdtp (Go)" --sha <commit>
```

`tiktoken` is used for token counts if installed; otherwise it falls back to a
`chars/4` approximation (flagged in the output), which is conservative.

## Recorded rows

| Project | Commit | Lang | Recall | Precision | Token savings | Notes |
|---|---|---|---|---|---|---|
| ruslano69/tdtp-framework | `4ff012e` | Go | 98.1% → **99.1%** | 99.5% | 88.8%* | after the defined-type fix below |

\* verbose `--all --json`, chars/4 approx; higher with `--split`/compact map.

**The dovodka loop, demonstrated end-to-end on this row:**

1. *Ruler found the gap.* The original 1.9% recall miss was one clean category:
   Go *defined types* — `type X string`, `type X func(...)`, `type X []byte` —
   which funcfinder didn't detect (its `struct_type_patterns` only covered
   `struct` / `interface` / `= alias`). 66 instances.
2. *Cheap regex fix (community engine).* Added a `named` pattern plus
   single-line closing for brace-less type kinds (so they don't swallow the
   declarations after them). Pinned by `TestGoStructFinder_DefinedTypes`.
3. *Re-measured.* Recall **98.1% → 99.1%**, misses **66 → 30**, precision
   unchanged at 99.5%.

**The residual ~30 — where regex should stop.** It is no longer a tidy
category: it's mostly normal functions/structs *swallowed by an upstream
brace miscount* (e.g. a lexer with `'{'` / `'}'` char literals throwing off
`CountBraces`). That's the harder layer — a sanitizer refinement at best, and
otherwise exactly the "only AST gets it" territory that justifies the parser
tier. We deliberately do **not** chase it with more regex: past here the cost
of fixing exceeds what the missed symbols are worth to navigation.

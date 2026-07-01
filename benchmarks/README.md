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

The ruler walks the whole AST (`ast.Inspect`), not just top-level `f.Decls`, so
it captures **function-local type declarations** (`func f() { type T … }`) that
funcfinder — being line-based — also reports. An earlier top-level-only version
undercounted these and unfairly dented funcfinder's precision; a ruler that
undercounts is a broken ruler.

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
| ruslano69/tdtp-framework | `4ff012e` | Go | 98.1% → 99.1% | 99.5% | 88.8%* | after the defined-type fix (loop #1) |
| ruslano69/tdtp-framework | `2028d38` | Go | 99.1% → **100.0%** | **100.0%** | 88.8%* | after char-literal + inline-brace fixes; ruler corrected (loop #2) |

\* verbose `--all --json`, chars/4 approx; higher with `--split`/compact map.
Recall/precision are over the file intersection with the ruler.

**The dovodka loop, demonstrated end-to-end over two passes:**

*Loop #1 — defined types.* The first 1.9% recall miss was one clean category:
Go *defined types* — `type X string`, `type X func(...)`, `type X []byte` —
which funcfinder didn't detect (its `struct_type_patterns` only covered
`struct` / `interface` / `= alias`). 66 instances. Fixed with a `named` pattern
plus single-line closing for brace-less type kinds. Recall **98.1% → 99.1%**,
misses 66 → 30. Pinned by `TestGoStructFinder_DefinedTypes`.

*Loop #2 — the residual 30, which turned out to be two more clean categories,
not "only AST gets it":*

1. **Char/rune literals leaking braces (22 of 30).** `char_delimiters` was
   *unset for every language* in `languages.json`, so the sanitizer's
   char-literal machinery was dead code. A lexer line like `if c == '{' {` fed
   a phantom `{` into `CountBraces`, and the enclosing function stayed open and
   swallowed everything after it. Fix: set `"char_delimiters": ["'"]` for the
   languages where `'` is a rune/char literal (Go, C#, Java, D, Kotlin — C/C++
   already carried `'` in `string_chars`). *Not* Rust (lifetimes `'a`) or Scala
   (symbol literals `'sym`), where a lone `'` is legal and unpaired. Pinned by
   `TestCharLiteralBraces_NoSwallow` / `TestCharLiteralSanitized`.
2. **Inline balanced braces (the last 7).** `type Foo struct{}` /
   `struct{ io.Writer }` net to `braceCount == 0`, which the struct finder read
   as "multi-line signature, wait for a brace" — so it stayed open and swallowed
   the next type. Fix: when the line has no unmatched brace but *did* contain a
   `{`, close the type on its own line. Pinned by
   `TestGoStructFinder_InlineBraces_NoSwallow`.

Result: recall **99.1% → 100.0%** on the file intersection, misses 30 → 0.

*Loop #2b — the ruler audited itself.* Investigating the remaining <1% precision
gap showed the "false positives" weren't false at all: 14 were function-local
`type` declarations funcfinder correctly reports, which the top-level-only ruler
didn't emit. So the ruler was fixed (walk the whole AST). After that the two
agree symbol-for-symbol — **3366 / 3366**, recall and precision both **100.0%**,
zero misses in either direction. The lesson cuts both ways: the ruler measures
funcfinder, and funcfinder's output can expose blind spots in the ruler.

**What 100/100 does and doesn't mean.** It means: on tdtp, over the shared Go
files, funcfinder *locates* every function and type the parser sees (matched by
name and line ±2), and reports no phantom symbols. It does **not** mean "regex ==
AST": we compare symbol *presence*, not exact spans/signatures/fields; we model
two node kinds (functions, types), not the full grammar; and it's one project.

**The lesson for the tier boundary.** What looked like "harder, parser-only"
territory after loop #1 was, on inspection, two more *cheap* config/regex fixes
plus a bug in the measuring tape — no parser needed. The ruler earns its keep
precisely here: it stops us from hand-waving "the rest needs AST" and makes us
look. The genuine parser-tier cases are the ones that survive *after* the cheap
fixes are exhausted — and on tdtp there are none left to point at. The next
honest test is a project that still has residuals here.

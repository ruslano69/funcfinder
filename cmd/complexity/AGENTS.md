# cmd/complexity/

## Purpose

Cognitive complexity score per function, and the `--breakdown` figures and flags that show what the score is made of.
The output is a probe for where to read first, not a verdict on the code.

## Ownership

- `main.go`: flags, `reorderArgs`, the score (`calculateNestingDepth` and its brace, indent and block-keyword variants),
  and the output.
- `breakdown.go`: the breakdown counts, `breakdownThresholds` with `defaultThresholds`, `breakdownFlags` and
  `printBreakdown`.
- Language patterns (`nesting_pattern`, `flat_pattern`, `loop_pattern`) live in `internal/languages.json`, not here.
- Language detection without `-l` is `internal.DetectLanguage` (`internal/detect.go`). Do not reimplement it here.
- Files are analysed in parallel with `internal.ForEachParallel` and collected in input order; `analyzeFileComplexity`
  must not write shared state. The later sort by `MaxComplexity` is not stable, so the input order is what keeps the
  output the same from run to run.
- `-exclude` is `internal.FilterExcluded` (`internal/exclude.go`); the flag type is `stringList` in `exclude.go`.
- `-summary` counts are in `summary.go` (`summarize`, `printSummary`). They are for tracking a goal, so the limits line is
  printed with them and stays first in the output.

## Local Contracts

- The breakdown flags are a probe, not a verdict. Write output and docs so they point at where to read, not at a defect.
- Four limits, one per flag, are set on the command line: `-len` (lines; flags above N), `-if` (decisions; flags above N),
  `-deep` (branch depth; flags at N or more) and `-loop` (loop depth; flags at N or more). The defaults are
  `defaultThresholds`; `0` turns a flag off. The operators differ on purpose and are kept as they are.
- Every flag that takes a value must be listed in `reorderArgs` (`valueFlags`). A missing entry splits the value from its
  flag and makes the parse fail.
- Score levels: SIMPLE up to 3, MODERATE up to 6, HIGH up to 10, VERY_HIGH up to 15, CRITICAL above 15 (`ScoreSimple` and
  the others in `main.go`).
- The `loop-nesting=N` line of the plain output starts at 2 and is separate from the `-loop` flag.
- `triggered_flags` in `-j` and the `over:` line of the text come from one list, `breakdownChecks`. A new limit goes
  there and nowhere else.
- `-bool` reads `max_bool_operators`, the most operators on one line (`countMaxBoolOps`). `boolean_lines` stays a line
  count; the two are not interchangeable.
- The breakdown counts run on every function, with or without `--breakdown`, so they use the hand scanners `boolOps` and
  `hasDecision`, not `regexp`: `\b` patterns backtracked and took half of the analysis time. The scanners must give what
  the reference patterns in `breakdown_scan_test.go` give; change both together.

## Work Guidance

- A new limit: a field in `breakdownThresholds`, a default in `defaultThresholds`, a flag in `main.go`, the flag in
  `reorderArgs` if it takes a value, and tests in `breakdown_test.go` and `args_test.go`.
- User-facing behaviour is documented in `docs/COMPLEXITY.md` and `skills/funcfinder.md`. Every flag, including `-j`,
  `-v` and `-version`, appears in the command-line table of `docs/COMPLEXITY.md`; compare it with `complexity -h` after a
  flag change.
- A change to a language's `nesting_pattern` changes that language's scores; say so in `CHANGELOG.md`.

## Verification

```bash
go test ./cmd/complexity/
```

## Child DOX Index

None.

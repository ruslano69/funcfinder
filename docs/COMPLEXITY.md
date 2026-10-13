# complexity

`complexity` scores the cognitive complexity of each function. With `--breakdown` it also shows the figures behind the
score and the limits each function passes. The flags are a probe for where to read first, not a verdict on the code.

## Command line

```bash
complexity [flags] <directory or file>
```

| Flag | What it does |
|------|--------------|
| `<directory or file>` | What to analyze. A directory is walked recursively; a single file works too. Default `.` |
| `-l LANG` | Force the language (`go`, `py`, `rust`, ...). Without it, the language is the one with the most source files under the directory: recursive, and `.gitignore` is respected. A file is taken by its extension. The text output says what it detected and how many files it counted. A tie goes to the language key that sorts first. |
| `-n N` | Show only the N most complex functions. `0` shows all. |
| `-nosimple` | Hide functions at level SIMPLE (score 0 to 3). |
| `-t N` | Hide functions whose branch depth is below N. A filter: the function is removed from the output. |
| `-v` | Print each function's nesting history: the credited depth at each line. |
| `-breakdown` | Print the figures behind each score and the flags it passes (see below). |
| `-len N`, `-if N`, `-deep N`, `-loop N` | Limits of the `-breakdown` flags (see below). They add flags and never hide a function. |
| `-j` | Print JSON instead of text (see below). |
| `-exclude GLOB` | Leave out the files a glob matches: by base name (`*_test.go`) or by path under the analysed directory, with `/` (`gen/*.go`). Repeat the flag for more patterns. Language detection still counts every file. |
| `-summary` | Print counts instead of the function list: the limits in effect, functions per level, the functions each limit flags, and the total score. Cannot be combined with `-j`. |
| `-version` | Print the version and exit. |

Text output is ordered by score, highest first. The filters apply in this order: `-nosimple`, then `-t`, then `-n`.

`-j` prints every function with its figures: `complexity`, `level`, `lines_of_code`, `decision_count`,
`max_nesting_depth`, `max_loop_nesting_depth`, `max_bool_operators`, `boolean_lines` and `nesting_history`, and
`triggered_flags`: the limits it passes (`len`, `if`, `bool`, `deep`, `loop`), with the limits in effect (the defaults, or
your `-len`, `-if`, `-bool`, `-deep`, `-loop`). It ignores `-n`, `-t`, `-nosimple` and `-breakdown`, so filter the JSON
yourself, for example with `jq`.

Files listed in `.gitignore` are skipped. There is no flag to include them; `deps` and `callgraph` have `--no-gitignore`.

## Score

Each decision line (`if`, `match`, `for`, `while`, `loop` and the like) costs 1 plus its current nesting depth, so nested
decisions add up quickly while sibling branches do not compound. An `else`, `elif` or `case` that continues a branch
costs a flat 1.

Levels: SIMPLE 0 to 3, MODERATE 4 to 6, HIGH 7 to 10, VERY_HIGH 11 to 15, CRITICAL above 15.

A closure or nested function is not a separate entry. Its decisions count in the function that contains it, and its
nesting continues from the enclosing depth: an `if` inside a closure that sits inside an `if` is at depth 2. Checked on
Go closures.

## Breakdown

`--breakdown` prints, for each function: its size in lines, its decisions, its branch depth, its loop depth, and the
lines that combine conditions with `&&` / `||` or `and` / `or`. A line `over:` names the flags the function passes.
`--breakdown` does not change the score and does not hide any function.

## Breakdown flags and limits

| Flag | Limit | Flags when | Default |
|------|-------|------------|---------|
| `-len N` | lines | size is above N | 100 |
| `-if N` | decisions | decisions are above N | 10 |
| `-bool N` | operators on one line | more than N `&&`, `||`, `and` or `or` on one line | 2 |
| `-deep N` | branch depth | depth is N or more | 4 |
| `-loop N` | loop depth | loop depth is N or more | 3 |

`0` turns that flag off. The loop limit starts at three because a loop inside a loop is ordinary code; the plain output's
`loop-nesting=` line still starts at two.

`-bool` counts operators on the busiest line, not lines: `a && (b || c) && !d` has three, so the default flags a condition
with four operands. The text output shows the same figure as `boolean_lines` counts the lines that combine conditions.

`-t N` and `-deep N` both use branch depth, but they do different things: `-t` removes the functions below N from the
output, while `-deep` only marks the functions at N or more.

```bash
complexity internal/finder.go -l go --breakdown -len 200 -loop 0
```

## Tracking over time

`-summary` prints counts for a goal: the limits in effect, the functions at each level, the functions each limit flags,
and the total score. Keep the limits the same between runs, because a summary taken with other limits is another figure.
Read the flagged count together with the total score: splitting one function into several can lower the count without
lowering the complexity. The scope moves too: new functions change the totals, so compare summaries of the same code.

## Probing a codebase

The limits are yours to set. A probe over a codebase you do not know:

1. Run with the defaults on the directory and note the flagged functions.
2. Raise one limit at a time (for example `-len 200`) and see which functions drop off. Each one that stays is complex
   for a reason that limit does not explain.
3. Turn a flag off with `0` when it only adds noise for this code.

A flag names a property, not a defect: a long function can be a plain table, and a deep one can be a straightforward
state machine. Read the function before deciding what to change.

## Known limits

- Rust: the score follows `nesting_pattern` in `internal/languages.json`. A keyword anywhere on a line counts, and a
  `for` counts only as a loop. `--breakdown` still counts an `impl Trait for Type` header as a decision, so its decision
  count is higher than the score's on those lines.
- Language detection counts source files, not lines. When a directory holds several languages, only the detected one is
  analysed; `-l` picks another.
- Rust: `||` also opens a closure with no arguments (`|| expr`), and `-bool` counts it as an operator.
- The thresholds are conventions: McCabe's 10 for decisions, about 100 lines (Clippy's `too_many_lines` default), and 3
  to 4 levels of nesting. They are not measured links to defects.

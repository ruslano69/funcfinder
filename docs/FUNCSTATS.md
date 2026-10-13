# funcstats

`funcstats` counts the function calls in a source file or a directory, and gives the file statistics next to them: lines
(code, comments, blank), imports and the most called names. With `--uncalled` it lists the functions that no code line
names: candidates to check before deleting them.

## Command line

```bash
funcstats [flags] <source file>
funcstats [flags] --dir <directory>
```

| Flag | What it does |
|------|--------------|
| `<source file>` | One file: its statistics and call counts. |
| `--dir DIR` | Every source file under DIR, recursively, with the totals. |
| `-l LANG` | Force the language. Without it, the language is the one with the most source files under DIR (a file is taken by its extension). |
| `-n N` | Show the N most called names. `0` shows all. Names are listed by call count, most first, and by name among equal counts, so the list and the cut are the same on every run. |
| `-j`, `--json` | Print JSON instead of text. |
| `--uncalled` | Dir mode: list the functions that no code line names (see below). Needs `--dir`. |
| `--exclude GLOB` | Dir mode: leave out files whose base name, or whose path under DIR with `/`, matches the glob. Repeat the flag for more patterns. It applies to every dir-mode output. |
| `--version` | Print the version and exit. |
| `-h`, `--help` | Print the usage. |

## Uncalled functions

```bash
funcstats --dir . --uncalled --exclude "test_examples/*"
```

A name counts as used when it appears as an identifier in code anywhere under DIR, other than in its own declaration. A
comment or a string is not a use. Another name on a declaration's line is a use: `func a() { b() }` uses `b`.

Not candidates: functions in test files (`*_test.go`, `test_*.py`, `*_test.py`, `*.test.*`, `*.spec.*`), the entry points
`main` and `init`, and the Go interface methods that the runtime calls by themselves (`String`, `Error`, `Len`, `Less`,
`Swap`, `Close`, `Read`, `Write`, `MarshalJSON`, `UnmarshalJSON`, `ServeHTTP`).

An exported Go name outside an `internal` directory is marked `exported`: code outside this tree may call it. Go does not
let code outside a module import an `internal` package, so those names are not marked.

The list is for checking, not a verdict:

- A name used only in a string (a dispatch table keyed on names), through reflection, or from another repository looks
  uncalled.
- A function called only from another uncalled function is not a candidate, and a recursive function counts its own calls
  as uses. The list is conservative: it can miss dead code, but it does not flag a function the tree names.

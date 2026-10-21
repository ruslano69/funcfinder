# Changelog

## Unreleased

- `funcfinder --replace-func NAME --source LANG --with FILE|-` replaces one function in a file of any configured language. Its span comes from the finder; Go spans are also checked against `go/ast`. Options: `--line N` picks one of several functions with the same name, `--old FILE` refuses the edit if the current text differs from the expected text, and `--apply` writes the file (without it, the change is only validated). The replacement must keep the same top-level declarations, so renames, extra functions and imports are refused. Line endings are kept.

- Experimental D statistics and call graphs reuse per-thread token/call scratch and index declaration occurrences only for native function names. Retained result keys remain owned; ordinary stats skip identifier-use analysis, while `--uncalled` preserves it. On the measured internal Go tree with default GC, stats improve 58.7 → 22.3 ms and graph 41.4 → 25.5 ms; file-processing allocation falls 38%. Independent Go AST count checks and ownership/measurement limits are recorded in `benchmarks/ffd/STATS_BUFFERS.md`.

- D CLI tools use generated immutable language settings instead of repeatedly parsing embedded JSON. Registry/profile/documentation reads share allocation-free table lookup; existing native-parser configuration APIs receive equivalent adapters only for selected languages. Go freshness tests cover both generated artifacts, and D tests compare all settings with the JSON reference. JSON output/manifest readers and runtime settings are unchanged.

- Experimental D `callgraph` adds all advertised flag names, file/directory graphs for 15 native language profiles, reverse lookup, recursion-aware trees and shortest-path depth filtering. It shares token call recognition with D `funcstats`, including JS private names and optional receivers. Filtered JSON retains source paths/lines and file mode reports actual known-name counts; these deliberate Go differences and syntax-only AST scope are recorded in `benchmarks/ffd/CALLGRAPH_PORT.md`.

- Go `funcstats` excludes reserved Go keywords from call counts, including `if`, `for`, `switch`, `case` and `return` followed by parenthesized expressions. Real calls, recursion, builtins and existing type-conversion counts are preserved.

- Experimental D `funcstats` adds all advertised flags, 15-language file/directory metrics, sorted call counts/top-N, exclusions and uncalled candidates using shared native lexers/finders. Declaration-token exclusion preserves recursion; Go control syntax, interface signatures and composite-type conversions are excluded, generic/indexed calls retained. Active lexer context prevents import/annotation matches inside comments/literals. Syntax-only Go AST checks and profile limits are recorded separately from CLI agreement.

- Experimental D `ffd` accepts all Go boolean spellings and stops option parsing at `--` or the first positional argument. Extended CLI regressions compare actual output, repeated assignments, invalid values and parsing boundaries; D retains explicit `--struct=false` instead of Go's shorthand-preprocessor behavior.

- Experimental D `ffd` omits unused C# method results and conditional expression-end work for directory `--struct`, retaining body/scope recognition. Polly/Dapper parser allocations fall about 6.7% and parser medians improve 4–6%. Roslyn reference output now exposes nominal types and explicit post-attribute function headers; existing bodyless-type/start limitations remain separately recorded.

- Experimental D `ffd` omits unused Java method/constructor results for directory `--struct`, retaining local/anonymous-class scope traversal. Warmed parser allocations fall 40.6% on Gson; parser medians improve about 4%, with a small full-CLI effect. Independent grammar checks retain exact method/type spans.

- Experimental D `ffd` skips function recognition/results for Go-source directory `--struct`, retaining local/grouped types and alias handling. Warmed parser allocations fall 55–81% on gin/encoding-json; no consistent full-CLI speedup is established. Native Go AST checks retain exact function/type spans.

- Experimental D `ffd` omits unused function results/header-start calculations for C++ directory `--struct`, retaining function-boundary recognition and existing local-type scope. Warmed parser allocations fall about 8% on fmt/tinyxml2; full-CLI speed gains are inconclusive.

- Experimental D `ffd` avoids function discovery/materialization for D directory `--struct` after its existing shared preparation. arsd/D-YAML serial CLI improves 1.09–1.19x; independent type checks preserve and explicitly report existing union/struct omissions and manifest-constant name errors.

- Experimental D `ffd` skips unused function discovery for JavaScript/TypeScript directory `--struct`, reusing its existing type scanner. The measured Express/Zod CLI gain is 1.21–1.81x; type output and file ownership behavior are preserved.

- Accuracy fixes: Go C++ excludes expression/initializer headers while preserving real prototypes and constructors; Go Ruby reports singleton method names after their receivers. D C skips attribute argument lists during function discovery, and D Kotlin no longer includes a preceding property's `.value` in the next declaration's start. Minimal regressions are checked with independent syntax trees.
- D `ffd` accepts `--raw` through its existing Go/C#/JS/TS function cleaners while preserving normal type/directory parsing, and adds `--help`/`-h`. A CLI audit checks all advertised Go key names/types; native literal/profile differences remain.
- D `ffd` adds directory JSON `--split`, `--split-by dir|file`, `--out` and `--inc`, with Go-compatible manifests/FNV-1a checksums, changed-shard parsing, cache repair and deleted/empty shard reconciliation. Flattened-name collisions are rejected; a private profile stamp invalidates incompatible caches.
- D `ffd` adds file/directory `--tree` and `--tree-full`: class/method and nested-function trees, full signatures, type fields and sorted directory paths, preserving Go's output priorities.
- D `ffd` adds file/directory `--doc Name,Other`: adjacent declaration comments and first source lines for functions, types and Go aliases, using Go-compatible formatting and flag validation.
- D `ffd` file type maps/JSON now include initial token-derived field profiles for 15 languages: names, normalized type spellings and source lines. Direct-member ownership excludes methods, nested types and locals; complex field syntax remains incremental.
- D `ffd` Python parsed `--lines` now adjusts to whole scopes with Go-compatible reports, trailing trivia and complete-function filtering. Multiline decorators are retained in file JSON.
- Experimental D `ffd` adds file/directory `--names`, file `--struct`/`--all` output and `--type`/named `--struct` selection with native type metadata.
- Experimental D `ffd` adds single-file function maps and JSON, comma-separated `--func` selection, original-source `--extract`, and standalone/parsed `--lines` ranges using all 15 existing token finders.
- Go C++ discovery now retains nested-class constructors and recovers builtin conversion operators, ref-qualified methods and conditional `noexcept` methods with a token pass. Selection, source offsets and original extraction are preserved.

## v1.15.10 - 2026-10-19

A speed release for TypeScript, plus the D port's directory tool as benchmark material. The Go tools' output is unchanged
(byte-identical to 1.15.9 on the tracked trees); the only code change in the Go tools is the TypeScript scanners below.
`benchmarks/ffd/` adds `ffd`, a D executable over the 15 token finders with the Go tool's `--dir` JSON, a worker pool
(default 8, at most 16), and a by-key comparison with `funcfinder`; the D finders reuse their token, pair and text blocks
from file to file, which lifts their scaling at 8 threads (arsd 2.6x -> 4.9x). It is not part of the release archives.

- **TypeScript: the declaration and declarator patterns are read by scanners instead of regular expressions.** As for
  `func_pattern` in 1.15.8, `tsDeclarationSubmatch` (a bodyless signature: an overload, an interface member, an `abstract`
  method) and `tsDeclaratorSubmatch` (a later declarator of a `const` list after its `,`) return exactly what the patterns
  return, groups included, while `languages.json` holds them unchanged. The declaration pattern ran for every line that ends
  in `;` and the declarator pattern after every `,` of a line: together they were a tenth of a TypeScript run. They were
  compared with the patterns on 1.2 million fixture, suffix and generated lines (every group, nil against nil). Output is
  byte-identical to 1.15.9 for `funcfinder`, `callgraph`, `funcstats` and `complexity` on OmniRoute, zod, Express, lodash,
  arsd, tdtp-framework, kanzi, flask, symfony-routing, lua and the fixtures. Serial `funcfinder --dir --all` on OmniRoute
  2864 → 2574 ms (-10%), zod 224 → 192 ms (-14%); median with all threads: OmniRoute `--all` 490 → 453 ms,
  `callgraph` 827 → 788 ms, `complexity` 626 → 597 ms; zod `--all` 56 → 51 ms.

## v1.15.9 - 2026-10-18

An accuracy release for JavaScript and TypeScript. Functions that template literals, comments, JSX text or complex
headers used to hide are found: on the accepted independent Express/lodash/zod/OmniRoute profiles no named function
is missing any more (35,999 functions). Against 1.15.8, OmniRoute gains 57 functions and loses 4 false functions and
2 false types (code inside template strings); zod gains 29 functions and loses one false entry. Output for Express,
lodash and every other language is unchanged, byte for byte (`deps -j` has the same content; its order of equal counts
varies from run to run, as before).

The TypeScript recovery costs a little time. Median 1.15.8 → 1.15.9: OmniRoute `funcfinder --dir --all --json`
454 → 479 ms (3.4 million lines per second), `callgraph --json` 803 → 817 ms, `complexity -j` 610 → 628 ms; zod
`--all` 46 → 55 ms; arsd and tdtp-framework unchanged.

Known limits: the recovery runs only on TypeScript files with an unresolved header shape it can read (114 files of the
OmniRoute, zod and fixture trees), and only while `languages.json` holds the TS function pattern unchanged; false
entries, wrong ends and parse-error exclusions of the JS/TS profiles remain (see `TODO.md`); the D and JS/TS ports in
`benchmarks/` are experiments, not part of the release; timings come from one 16-core Windows machine.

- **JavaScript/TypeScript: functions missed after complex literals and headers are found.** Template interpolation
  keeps regexes, comments, nested templates and escaped backticks balanced across lines (`javascript_template.go`, one
  frame stack for line cleaning and token recovery). JS/TS block comments close at the first `*/`; bracketed JSX text
  is not read as a regex with invalid flags. TypeScript additionally recovers nested/multiline generics, split and
  curried arrows, dollar names, keyword/quoted methods and multiline object methods (`typescript_candidates.go`). The
  accepted independent Express/lodash/zod/OmniRoute profiles have no missing named function left (35,999 functions);
  false entries, wrong ends and parse-error exclusions remain listed in `TODO.md`. Against 1.15.8: OmniRoute gains 57
  functions and loses 4 false ones (code snippets inside template strings) and 2 false types; zod gains 29 and loses a
  false `async`; output for Express, lodash and all other languages is unchanged, byte for byte. A template now starts
  with an empty frame stack whatever its delimiter, so a template left open by an earlier text cannot change the next
  one (`TestCleanLinesFactsIgnoresEarlierFiles`).
- **TypeScript: the recovery runs only on files with a header shape it can read.** Its file gate passed 997 of
  OmniRoute's 8,852 TS files to a whole-file token pass, and 16 of them gained a function: a line starting with `(`
  in a file with any quote, a `const x =` followed by any parenthesized value, and generic calls whose arguments
  continue over lines all passed it. Now a `(` line must start with a quoted name in the raw text, and a split value
  or a generic argument list is followed over the next lines to see whether `=>`/`:` (or `:`/`{`) comes after its
  parenthesis. The gate passes 114 files of the OmniRoute, zod and fixture trees instead of about 1,040, with the
  same results: output is byte-identical to the unnarrowed gate on all twelve corpora. Median against 1.15.8,
  including the recovery's real work: OmniRoute `funcfinder --dir --all --json` 467 → 489 ms (it took 590 ms with the
  wide gate), `callgraph` 793 → 824 ms, `complexity` 617 → 633 ms; zod `--all` 46 → 56 ms, where the large schema files
  hold the recovered functions.

## v1.15.8 - 2026-10-18

A performance release for TypeScript and for every directory run. Output is unchanged, byte for byte (`deps -j` has the
same content; its order of equal counts varies from run to run, as before). `funcfinder --dir --all --json` on
OmniRoute (9,300 TS/JS files, 1.63 million lines) takes 457 ms instead of 775 ms in 1.15.7: 3.57 million lines per
second. Median 1.15.7 → 1.15.8 on OmniRoute: `funcstats --dir -j` 1216 → 832 ms, `callgraph --json` 1014 → 801 ms,
`complexity -j` 801 → 608 ms, `funcfinder --struct --json` 545 → 403 ms; kanzi (Go with Rust build artifacts)
`funcfinder --all` 68 → 44 ms; tdtp-framework 81 → 61 ms; arsd 136 → 128 ms.

Known limits: the TypeScript scanner stands in for the function pattern only while `languages.json` holds it unchanged
(the regular expression runs otherwise); the sanitizer's slow path is now the largest cost of a TypeScript run, and the
TS declaration and the D type, statement and attribute patterns are still regular expressions; timings come from one
16-core Windows machine.

- **Directory runs: the finders of a file share its cleaned lines, struct fields are no longer collected, and the walk
  and the file reads cost less.** In `--all` the function finder and the struct finder (and, for Go, the alias reader)
  each cleaned the whole file again; cleaning is a third of a TypeScript run. A per-job cache (`factsCache`) now keeps
  the first finder's facts for the others. The struct finder of a directory job no longer collects each type's fields,
  which a directory result never held. The walk reads directory entries (`filepath.WalkDir`) instead of one system
  call per entry, and reads a size only for source files: on a tree with build artifacts (kanzi's Rust `target/`,
  2800 files) it was a third of a run. A file is read whole and split into lines, instead of through a buffered reader
  line by line. Output is byte-identical for `funcfinder` (`--all --json`, `--json`, `--struct --json`, `--names
  --all`, `--tree`), `callgraph`, `funcstats` and `complexity` on OmniRoute, arsd, funcfinder, tdtp-framework, kanzi,
  flask, symfony-routing, lua and the fixtures; `deps` gives the same content (its order of equal counts already varied
  from run to run). Median `funcfinder --dir --all --json`: OmniRoute 770 → 579 ms, tdtp-framework 82 → 60 ms, kanzi
  68 → 42 ms, arsd 135 → 125 ms; `--struct --json` on OmniRoute 535 → 405 ms; `callgraph` on OmniRoute 1006 → 916 ms.

- **TypeScript: function headers are read by a scanner instead of the regular expression.** As for D in 1.15.7, the TS
  function pattern (a fifth of a TypeScript run) is replaced by `tsHeaderSubmatch`, which returns exactly the pattern's
  result, every group included, while `languages.json` holds the pattern unchanged. It was compared with the pattern on
  32.7 million lines and line suffixes of OmniRoute's 8,852 TS files and on 3 million generated header-shaped lines: no
  difference. The nested scan, the inline-header search, `callgraph` and `funcstats` use it.
- **TypeScript, JavaScript: the quoted-method pattern runs only on lines that start with a quote** (5% of a TypeScript
  run before).
- Output is byte-identical on OmniRoute, the fixtures, arsd and tdtp-framework for `funcfinder`, `callgraph`,
  `complexity` and `funcstats`. Median on OmniRoute: `funcfinder --dir --all --json` 757 → 636 ms, `funcstats`
  1201 → 898 ms, `callgraph` 1008 → 888 ms, `complexity` 790 → 682 ms; serial TS parsing 4.60 → 3.28 s.

## v1.15.7 - 2026-10-18

A performance release for D. Output is unchanged, byte for byte. `funcfinder --dir --all --json` on arsd (11 MB of D)
reads 2.16 million lines per second, up from 1.64 million in 1.15.6; D is no longer the slowest language.

Known limits: the scanner stands in for the D function pattern only while `languages.json` holds it unchanged (the
regular expression runs otherwise); the class, attribute, type and statement patterns are still regular expressions;
timings come from one 16-core Windows machine.

- **D: function headers are read by a scanner instead of the regular expression.** The D function pattern, a large
  alternation that backtracks, took about a quarter of a D run. `dHeaderSubmatch` walks the same structure and returns
  exactly the pattern's result: whether a line matches and which name it captures. It stands in for the pattern only
  while `languages.json` holds it unchanged. It was compared with the pattern on 1.8 million real lines of arsd and
  d-yaml (all lines, prepared lines and header joins) and 3 million generated header-shaped lines: no difference. The
  parser, `callgraph` and `funcstats` use it. Output is byte-identical on arsd, d-yaml, the fixtures, OmniRoute,
  tdtp-framework and lua for `funcfinder` (`--all --json`, `--map`, `--names`), `callgraph`, `complexity` and
  `funcstats` (including `--uncalled`). Median: arsd `funcfinder --dir --all --json` 175 → 133 ms, `funcstats`
  285 → 181 ms, `complexity` 240 → 186 ms; d-yaml `funcfinder` 70 → 26 ms; `markdown.d` alone 107 → 33 ms.

## v1.15.6 - 2026-10-18

A performance release for D. Output is unchanged, byte for byte. `funcfinder --dir --all --json` on arsd (11 MB of D)
reads 1.64 million lines per second, up from 1.50 million in 1.15.5.

Known limits: the D function pattern still runs in the preparation pass and the multiline-header join (about a
quarter of a D run); timings come from one 16-core Windows machine.

- **D: the lexer and the prefix pass skip plain lines.** A line of plain ASCII code is its own cleaned text, and a line
  whose first word is ordinary is not changed by the declaration-prefix pass; both now return such lines as they are
  instead of converting them to runes and back (64% and 93% of arsd's lines). Output is byte-identical on arsd, d-yaml,
  the fixtures and OmniRoute for `funcfinder`, `callgraph`, `complexity` and `funcstats`. Median on arsd:
  `funcfinder --dir --all --json` 192 → 175 ms, `funcstats` 312 → 284 ms; `webview.d` alone 126 → 115 ms.

## v1.15.5 - 2026-10-18

A performance release for D. Output is unchanged, byte for byte. `funcfinder --dir --all --json` on arsd (11 MB of D)
reads 1.50 million lines per second, up from 1.19 million in 1.15.4.

Known limits: the D function pattern still runs in the preparation pass and the multiline-header join, and the D
lexer still works on runes; timings come from one 16-core Windows machine.

- **D: the function pattern runs once per line, not twice.** The D preparation pass now hands each line's result of
  the function pattern to the finder, which ran the same pattern on the same text again (13% of a D run); a result is
  passed on only when the line did not change after the check. The pass also converts each line to runes once instead
  of rebuilding the string for every check, and the attribute-line pattern runs only on lines that start with `@` or
  the first letter of an attribute word. Output is byte-identical on arsd, d-yaml, the fixtures, OmniRoute and
  tdtp-framework for `funcfinder` (`--all --json`, `--map`), `callgraph`, `complexity` and `funcstats`. Median on arsd:
  `funcfinder --dir --all --json` 242 → 191 ms, `callgraph` 328 → 277 ms, `complexity` 303 → 252 ms, `funcstats`
  364 → 315 ms; `webview.d` alone 160 → 128 ms.

## v1.15.4 - 2026-10-18

A correctness release for directory walks: `.gitignore` patterns now mean what they mean to git, so the set of files
the tools read can change. Projects whose `.gitignore` uses `**`, middle-slash patterns, or (on Windows) any pattern
with a `/` are affected; parsing and output formats are unchanged.

Known limits: `!` negation, nested `.gitignore` files and `.git/info/exclude` are not read, and a tracked file that a
pattern matches is skipped, since the walk does not read git's index.

- **`.gitignore` follows git's glob rules.** This changes which files every tool reads. `*` and `?` match within one
  path segment, `**/`, `/**` and `a/**/b` work, and a pattern with a `/` at the start or in the middle is relative to
  the root. Before, `*` was left as a regex quantifier (`ab*c` matched `ac`), every line with `**` failed to compile
  and was dropped, and a middle-slash pattern matched at any depth. On Windows no pattern containing a `/` matched,
  since paths were compared with `\`. Only a line starting with `#` is a comment now. Checked against `git
  check-ignore` and `git ls-files --exclude-standard`: OmniRoute no longer reads 129 git-ignored files under nested
  `node_modules`; tdtp-framework skips `cmd/tdtpcli/commands/test.go`, which its `test.go` pattern matches although
  the file is tracked (the walk does not read git's index). Not supported yet: `!` negation, nested `.gitignore`
  files, `.git/info/exclude`.

## v1.15.3 - 2026-10-18

A performance release for D and for projects with a few very large files. Output is unchanged, byte for byte.
`funcfinder --dir --all --json` on arsd (11 MB of D) reads 1.09 million lines per second, up from 0.70 million.

Known limits: on arsd the run is still bounded by its largest file (`webview.d`, 182 ms alone); `complexity` and
`funcstats` do not use the largest-first order yet; timings come from one 16-core Windows machine.

- **Directory runs hand out the largest files first.** Workers took files in walk order, so a large file late in the
  tree started last and the run waited for it alone (arsd's `webview.d`, 12% of its lines). Results are unchanged.
- **D: fewer runs of the costly patterns.** The class pattern runs only on lines with a type keyword
  (`type_keywords`); it backtracked on every line and took a third of `webview.d`'s time. The header-join check runs
  its cheap tests before the function pattern, and statement lines skip the function pattern. The output is
  byte-identical on arsd, d-yaml, OmniRoute, tdtp-framework and the fixtures for `funcfinder` (`--all --json`,
  `--map`, `--tree`), `callgraph`, `complexity` and `funcstats`. Median on arsd: `funcfinder --dir --all --json`
  410 → 263 ms, `complexity` 450 → 327 ms, `funcstats` 521 → 394 ms, `callgraph` 489 → 366 ms; `webview.d` alone
  268 → 182 ms.

## v1.15.2 - 2026-10-18

- **JavaScript: discover expression functions and correct minified boundaries.**
  A JS token walk finds named expressions, direct variable function/arrow
  values, generators and object/class methods, including dollar/private names
  and continued declarators. Match each body to its closing brace, so adjacent
  functions on one line do not share the next body's end. Calls, literal text,
  anonymous callbacks and IIFE result bindings are not named functions. Keep
  original extraction, offsets and duplicate names; callgraph/funcstats retain
  real recursion and use declaration coordinates. Add `.cjs` discovery. On the
  recorded named-function profile, all 2458 lodash and 218 Express functions
  match header/end lines with zero missing/false functions; TypeScript output
  on pinned zod is unchanged. See `benchmarks/JAVASCRIPT_FIX.md` for scope,
  grammar versions and timing limits.

## v1.15.1 - 2026-10-17

- **Java: declaration context and function bounds.** Find methods of anonymous
  and local classes, constructors, annotated/generic multiline signatures,
  compact record constructors and bodyless members, including annotation
  defaults. Calls no longer become functions. Retain original extraction and
  declaration coordinates for `callgraph`/`funcstats`; nested method callers
  and genuine recursion remain visible. On Gson `e2292f9`, all 3492 reference
  functions match, with zero missing/false functions or wrong ends (previously
  367 missing and 55 false). All 767 named types match by name/header location;
  exact type ends are not scored. This is one syntax profile, not general Java
  coverage. Complete CLI parsing is slower than the old incomplete parser;
  timing ranges and reproduction are in `benchmarks/JAVA_FIX.md`.

- **C#: declaration context and function bounds.** Recognize multiline/generic
  methods, constructors/destructors, local functions, operators, explicit
  interface members and bodyless declarations. Calls, control statements,
  properties and primary type headers no longer become functions. Expression
  bodies retain nested lambdas and object initializers. `callgraph` and
  `funcstats` use declaration coordinates to exclude signature tokens while
  preserving recursion. Against SDK Roslyn, all 5116 Polly and 1592 Dapper
  functions match on 958 tracked files in the recorded default-symbol profile;
  disabled branches are reported separately. See `benchmarks/README.md`.

- **Go: `type ( ... )` groups and multi-line composite types.** Group members
  (`formBinding struct{}` inside `type ( ... )`) carry no `type` keyword of
  their own and were invisible; they are now read with a virtual `type `
  prefix (alias members `A = S` still match nothing and stay aliases). The
  `named` pattern also accepts a trailing `{`, so `type testRequests []struct {`
  spans its braces instead of being missed. On gin HEAD this closes the last 4
  misses (recall 99.7% → 100.0%, precision 100.0%, 1554/1554 symbols vs
  `go/ast`); self-host stays 100.0%/100.0%.

## v1.15.0 - 2026-10-13

A performance release. Each tool was profiled on two large real projects, OmniRoute (6,843 TypeScript/JavaScript
files, 1.57 million lines) and arsd (118 D files, 11 MB), and the profiles found work done twice, loops that used one
core, string building that copied the whole output per line, and costly patterns run on lines that cannot match.
Output is unchanged, byte for byte, except two ordering fixes: `callgraph` reports files in path order, and
`funcstats` lists names with equal counts by name; both outputs used to vary from run to run.

Median wall time on a 16-core/32-thread Windows machine, 1.14.0 → 1.15.0:

| Command | OmniRoute | arsd |
|---|---|---|
| `funcfinder --dir --all --json` | 1.87 → 0.85 s | 1.85 → 0.40 s |
| `funcfinder --dir --map` | 10.9 → 0.70 s | 4.5 → 0.39 s |
| `callgraph --dir --json` | 20.7 → 1.07 s | 10.6 → 0.49 s |
| `complexity -j` | 11.1 → 0.82 s | 7.8 → 0.44 s |
| `funcstats --dir -j` | 13.4 → 1.25 s | 8.1 → 0.52 s |

`funcfinder --dir --all --json` reads 1.85 million lines per second on OmniRoute end to end. On this repository the
directory job (walk, parse, JSON) runs at 2.16 million lines per second in process, against 1.11 million for 1.12.2 on
the same machine; a whole run there is half process start-up, since the tree is only 34k lines.

Known limits:

- D is still the slowest language: 0.71 million lines per second on arsd, where its function pattern dominates.
- `.gitignore` globs: `patternToRegex` leaves `*` as a regex quantifier and drops every line with `**` (they do not
  compile). The walk keeps that behaviour; fixing it changes which files the tools read and is tracked in `TODO.md`.
- Above the number of physical cores, more threads add CPU time without lowering wall time.
- The timings come from one machine. A machine with fewer cores gains less from the changes that add parallelism.

### Directory runs

- Each `.gitignore` pattern is checked against a path only when the path holds the pattern's literal part. Every
  pattern's regex ran on every path of the walk, and the walk runs before any file is processed: on OmniRoute (164
  patterns) matching took 0.89 s of a 1.9 s `funcfinder --dir` run, now 16 ms. The files read are unchanged.
  `funcfinder --dir --all --json`, median: OmniRoute 1.88 → 1.03 s, tdtp-framework 153 → 107 ms. `complexity`,
  `funcstats` and `callgraph` walk the same way and gain the same time.
- `--dir --map` (the default directory output) and `--dir --tree` write into one buffer. The map grew a string line by
  line, copying the whole output each time, so a large tree spent most of its run building the text. The output is
  byte-identical. `--dir --map`, median: OmniRoute 10.9 → 1.7 s, arsd 4.5 → 1.8 s, tdtp-framework 282 → 128 ms.

### Call graph (`callgraph`)

- A directory run no longer finds every file's functions twice. The directory job finds them in parallel, and the call
  scan used to find them again, one file after another: most of a run's time. The scan now takes the job's functions,
  runs over the files in parallel, and skips the signature check on lines that do not hold the caller's name.
  `--dir --json`, median: arsd 10.6 → 2.0 s, OmniRoute 20.3 → 4.6 s, tdtp-framework 707 → 295 ms.
- Files are reported in path order. Before, they came in the order the workers finished, so two runs of the same
  tree could differ in order. The edges are unchanged: on eight projects the JSON, the text lines and `--reverse` match
  the previous binary once its files are sorted.
- The call scan no longer copies the set of all the run's functions into a map for every file; it looks names up in
  the file's functions and then in that set. On OmniRoute (43k functions, 6.8k files) the copies were about 290
  million map inserts: CPU time 47 → 23 s, wall time 4.5 → 3.7 s. The output is unchanged.
- Import aliases are read in parallel, and only for the files the graph scans (files with functions); the walk that
  lists them runs while the directory job finds the functions. Reading them one file at a time was the largest
  sequential step of a run: 660 ms of 1.7 s on OmniRoute. The output is unchanged.

### Complexity (`complexity`)

- The breakdown counts (`decision_count`, `boolean_lines`, `max_bool_operators`) are read by a scanner instead of
  regular expressions. In 1.14.0 they took about half of the analysis time and made `complexity` 10-50% slower than
  1.12.2; it is now 7-37% faster than 1.12.2 (`-j`, median: tdtp-framework 470 → 302 ms, OmniRoute 11.7 → 8.5 s).
  The output is byte-identical, and a test compares the scanner with the old patterns on every fixture line.

### `complexity` and `funcstats --dir`

- Files are analysed in parallel, on all cores. Both tools used one core: a profile of `complexity` on OmniRoute put 95%
  of one core and 67% of the time in finding functions, which `funcfinder --dir` already does in parallel. Each file is
  also read once now; the finder used to read it again. The output is the same: `complexity` byte for byte, and
  `funcstats` the same content. Median: `complexity` on OmniRoute 10.9 → 2.2 s, on arsd 7.8 → 1.9 s; `funcstats --dir`
  on OmniRoute 12.9 → 2.3 s, on arsd 8.1 → 2.0 s.

### funcstats

- Names with the same call count are listed by name. They came in random order, so two runs of the same input could
  differ, and with `-n` so could which of the tied names made the cut. The counts are unchanged.

### TypeScript

- The bodyless-declaration pattern runs only on lines that can hold one: a line with a `func_requires` character that
  ends with `;`. It ran on every top-level line, and a profile of `funcfinder --dir` on OmniRoute put it at 6.2 of
  21 s of CPU, three times the function pattern itself. Over OmniRoute's 1.6 million lines the pattern now takes 0.43 s
  instead of 4.35 s, with the same 4216 matches. The output is unchanged. CPU time of `funcfinder --dir --all --json`
  on OmniRoute 18 → 13 s; single-threaded `complexity` 10.9 → 8.4 s.

### D

- The D function pattern runs only on lines that hold a `(` (`func_requires`), in the D preparation pass and in the
  finder; the attribute pattern skips blank lines. Most D lines have no `(`, and the lines that the lexer blanks
  (comments) are long runs of spaces, where the patterns backtracked for nothing. The output is unchanged. On arsd
  (11 MB of D): `funcfinder --dir --all --json` 1.8 → 0.4 s, `complexity` 7.9 → 3.1 s, `funcstats` 8.1 → 3.3 s,
  `callgraph` 10.7 → 4.3 s.

## v1.14.0 - 2026-10-13

### funcstats

- `--uncalled` lists the functions that no code line of a directory names: candidates to check before deleting. A
  name counts as used when it appears as an identifier in code, so a comment or a string is not a use. Test files,
  `main`/`init` and the Go interface methods are not candidates. On funcfinder, 8 of 385 functions qualify once the
  fixtures are excluded. The rules and the limits are in `docs/FUNCSTATS.md`.
- `--exclude GLOB` (repeatable) leaves out files in dir mode, the same way as `complexity --exclude`.
- Dir mode detects the language by walking the tree, as `complexity` does. Before, only the top level of the directory
  was looked at, so a project with its sources in subfolders failed.
- `docs/FUNCSTATS.md` lists every flag.

### Breaking: `stat` is now `funcstats`

- The call counter is `funcstats`. The name `stat` collided with the system command: where this binary came first on
  `PATH`, `stat file` ran funcfinder instead of the system tool. Rename your binary, and remove the old `stat` from your
  `PATH` (the Makefile used to install it in `/usr/local/bin`). The flags are unchanged.
- The release notes name what the tool reports: file statistics (LOC, comments, imports) and function call counts with
  hotspots.
- The `examples/swe-agent/` scripts used `stat --inp FILE --source LANG`, which the tool never accepted. They use
  `funcstats FILE -l LANG` and `--dir` now.

### Doc comments (`--doc`)

- In Rust, the `#[...]` attributes between a doc comment and its item are skipped, one line or several: the comment
  above the attributes is the doc. Before, the walk stopped at the attribute, so `#[derive]` or `#[inline]` hid the doc.
  Outside Rust, `#[` is an ordinary comment and is unchanged.

### Complexity (`complexity`)

- `--breakdown` prints the figures behind each score: size in lines, the number of decisions (if, elif, match, switch,
  for, while, loop, wherever they sit on the line), branch depth, loop depth, and the lines that combine conditions with
  && / || or and / or. It then names the thresholds a function passes: more than 100 lines, more than 10 decisions,
  branch depth 4 or more, loops nested three deep. The thresholds sit at the common warning levels (McCabe's 10 for decisions,
  about 100 lines, 3 to 4 levels of nesting), so a flag fires early. `--breakdown` does not change the score, and the same figures are in the JSON output
  (`decision_count`, `boolean_lines`).
- The `--breakdown` limits are flags: `-len N` (lines), `-if N` (decisions), `-deep N` (branch depth) and `-loop N`
  (loop depth). Each flag fires at the limit it sets, and 0 turns it off. The defaults are the limits named above.
  The nested-loops flag now names its depth in the text.
- The breakdown flags are a probe for where to read first, not a verdict, and the limits are tuned per codebase. The new
  `docs/COMPLEXITY.md` describes the score, the flags and how to probe with them. It fills the `COMPLEXITY.md` link in
  `docs/WINDOWS.md`, which pointed to a missing file.
- Without `-l`, `complexity` detects the language by walking the tree: the language with the most source files,
  recursive and `.gitignore`-aware. A project with its sources in subfolders works, and a single file is taken by its
  extension. Before, only the top level of a directory was looked at, a file failed with `No supported files found`,
  and the first language in map order was taken, which could differ from run to run.
- The JSON output has `triggered_flags`: the breakdown limits a function passes (`len`, `if`, `deep`, `loop`), with the
  limits in effect, so scripts need not repeat the thresholds.
- `-bool N` flags a line with more than N boolean operators (`&&`, `||`, `and`, `or`), default 2. It reads the busiest
  line, which `boolean_lines` (a count of lines) does not show, and the JSON gets `max_bool_operators` and the key
  `bool`. On funcfinder the default flags 27 functions, 10 of them only for this limit.
- `-exclude GLOB` (repeatable) leaves out files by base name, or by path under the analysed directory. On funcfinder,
  `-exclude "*_test.go"` takes out 474 of 877 functions: more than half of the code is in test files.
- `-summary` prints counts instead of the function list: the limits in effect, the functions at each level, the
  functions each limit flags, and the total score. It is for tracking a goal over time, and it cannot be combined
  with `-j`.
- The score counts a Rust decision wherever its keyword sits on the line, so an `if` or `match` inside an expression
  (`let v = match a { … }`) and a labelled loop (`'outer: loop`) now nest like their statement forms. Before, only a
  keyword at the start of the line counted: `canvas_view` in photocraft scored 290 and now scores 405. A `for` counts
  only as a loop (`for pat in expr`), so `impl Trait for Type` is not a decision. On photocraft 4,042 of 24,335 Rust
  functions score higher and none lower; functions at CRITICAL go from 695 to 955. `--breakdown` still counts `impl … for`
  headers as decisions (299 in photocraft), so its decision count is higher than the score's on those lines.

### Rust

- A raw string with more hashes (`r##"…"##`) is a raw string now, and so is a plain `r"…"`. Before, `r##"` opened an
  ordinary string, so a quote on a later line ended it and the rest of the file was misread: on photocraft, three
  `#[test]` functions were lost (`icon_theme.rs`, `svg.rs`). The fixed corpus output is unchanged.

## v1.13.1 - 2026-10-13

Fixes from the analysis of two real Rust projects (gridcraft, photocraft): Rust parsing no longer loses the rest of a
file after a char literal with a quote, the Rust shard graph has edges, an incremental split no longer reports unchanged
shards as changed, and the plain `deps` listing counts a Go project's own packages as internal. A directory run also
names the files it skipped.

### Rust

- A char literal that holds a quote (`'"'`) no longer opens a string that never closes: the rest of the file was lost,
  every function after it. Rust has char literals now, with a lookahead rule: a quote opens a literal only when it closes
  on the same line, so lifetimes (`'a`) stay code.
- Functions with `pub(super)`, `pub(self)` and `pub(in path)` are found, as `pub(crate)` already was. On the gridcraft
  workspace the listing went from 6320 to 6489 names.

### Output

- A directory run names the files it skipped because no language is configured for their extension: one INFO line on
  stderr, with the total and the most frequent kinds (at most six). Before, the skipped files were not mentioned, so a
  repository with Shell, PowerShell or HTML files looked analysed in full. stdout is unchanged.

### Dependencies (`deps`)

- In the plain listing, a Go import under the module of `go.mod` is internal. Before, every import with a `/` was
  external unless it had `internal/` or `vendor/` in it, so the project's own packages were counted as third-party. The
  module is read from the nearest `go.mod` at or above the analyzed directory, so a run on a subdirectory works too.

### Incremental split (`--split --inc`)

- A shard whose files hold no declaration (data tables, re-export modules) has no shard file, so it was never in the
  manifest, and every incremental run counted it as changed: "2 shards changed" on an unchanged tree. The manifest now
  keeps the checksum of each such shard in `empty_shards`, and the incremental pass reads it. A shard that loses all its
  declarations drops its shard file and its `shards` entry. Shard files of shards with declarations are unchanged.

### Shard graph (`deps --shards`)

- Rust: the inter-shard graph of a workspace had no edges. `use core_lib::f` matched no resolution rule, so the import
  was neither resolved nor counted, and the trust check stayed silent. Crate names now come from each `Cargo.toml`
  (`[lib] name`, else the `[package]` name with `-` read as `_`). A path into a crate walks its `src/` directories as far
  as the tree has files and takes the deepest shard reached. `std` and external crates are not counted. An unresolved
  crate import now produces the warning. On the gridcraft workspace the graph went from 0 to 69 edges. Go, TS/JS and
  Python resolution is unchanged.

## v1.13.0 - 2026-10-13

Two new listings for working with names: `--names` lists the kind, name and location of every declaration, and
`--doc` prints the comment above named declarations. Python dataclass and attrs classes are reported with their kind,
and one-line Go structs are structs. C++ and TypeScript/JavaScript parsing is more complete.

Known limits: `--doc` does not read Python docstrings yet (only the comment above `def`/`class`), and a Go receiver
method is listed as a `function`, because its receiver is not recorded.

### C++ parsing

- Functions are found in more shapes: signatures with nested parentheses or a
  body on the header line, signatures that continue on later lines, constructors
  and destructors (in a class and out of it), operator overloads, bodyless
  declarations, and class headers with a template prefix or an export macro.
- In a constructor's member-initializer list, a braced `member{...}` is an
  initialization, not the body.
- A conversion operator (`operator bool() const`) is no longer reported as a
  function named after its type.
- On fmt, recall of the real-project measurement rose from 49.9% to 97.5% and
  false functions fell from 21 to 7 (`benchmarks/cpp_tree_eval.py`).

- `override` and `final` after `const` or `noexcept` no longer hide a virtual
  declaration or definition (tinyxml2 recall 53.0% -> 99.8%).

### Output

- A directory run gives the same `--tree`, `--json` and grep output on every
  run: files are reported in path order and tree children are sorted by name.
  The tree used to change between runs, because its children came from a map.
- `--names` lists the names of a file or a directory, one per line: `kind<TAB>name<TAB>file:line`. The kinds are
  `function`, `method`, `class`, the kind of a type (`struct`, `enum`, `dataclass`...) and `alias` for a Go alias.
  `--struct` and `--all` add the types and the aliases. A file runs through the directory job, so the two give the same
  names. Signatures stay in `--tree-full`. A Go receiver method is listed as a `function` for now.
- `--doc NAME[,NAME...]` prints each named function, type or alias as `path:line: kind name`, the comment directly
  above it as written, and its first line. The body is left to `--extract`. Decorators above a declaration are skipped
  (the comment above them is the doc). Python docstrings are not read yet: for Python the comment above the `def` or
  `class` is shown.
- A one-line Go struct (`type X struct{}`) is a `struct`; it was `named`, because the catch-all `named` pattern matched
  it first. Two declarations in the corpus change kind, and nothing else.

### TypeScript / JavaScript

- `abstract class`, `export default class` and `export default abstract class` headers are
  classes: their methods get the class name, and a multi-line method in them is found (zod: 11 more
  functions, including the methods of `ZodType`).
- An abstract method without a body (`abstract run(): void;`) is listed as a function.

- Interface and overload signatures, and `declare`/`export function` overloads, are listed as bodyless
  functions: `write(line: string): void;`, `static create(...): T;`.
- Multi-line signatures in an interface are found, and they belong to the interface.
- A multi-line class header (`export class X<` ... `> {`) keeps its members: the class no longer ends
  on the header line.
- `const x = (expr).method(() => ...)` is a value, not a function: the arrow belongs to the call.

- A closing JSX tag (`</h4>`) no longer starts a regular expression, which blanked the code between it and
  the next slash. React components ending in a JSX body are found again.
- A typed component (`React.FC<{ children: T }> = (...) => (...)`) keeps its body: braces in a type argument
  and a `;` inside them are not the end of the signature.
- A base list on the next line (`class X : public A,` or `: public B {`) and a multi-line class header keep
  the class open, so its constructors and destructors keep their class.

- A parameter list that continues on the next lines inside an interface or a class is listed: `process(input: {`
  and optional methods `permits?(...)` are signatures, and a return type may hold `;` in its braces.
- `const x = (` whose parenthesis closes on a later line and is followed by no arrow is a value, and an
  immediately invoked function `(() => {...})()` is a value; a value assigned in a one-line block is a value too.
  hr_gemini: 24 false functions -> 0.

- A typed component or a method whose type-argument list runs over lines (`const X: React.FC<{` ... `}> = (...)`,
  `async list(): Promise<` ... `> {`) is found; a property (`items: Array<{`) is not a method.
- The braces of a type argument are not counted into a function body: a typed component with a block body
  ends with its own closing brace. hr_gemini: 11 missing -> 2, 24 false functions -> 0.
- A self-closing JSX tag after an expression attribute (`}/>`) no longer starts a regular expression.
- A `{` after `<` opens a type literal (`query<{ ... }>(`), also across lines: its braces are not counted into a
  function body.
- A function whose body opens and closes on its header line ends there, so what follows the closing brace
  (`});`) belongs to the caller. hr_gemini: 2 wrong ends -> 0, 1 missing -> 0 after the next item.
- A later declarator of a `const|let|var` list on one line is a function when its value is an arrow or a
  parenthesized header (`const id = f(), notification = (x) => ...`). The list's keyword applies to it.

### Long lines and read errors

- A line longer than 64 KB no longer stops a read. Minified bundles have such lines: a directory run skipped
  `jquery.js` without a word, and `--inp` on it failed with `token too long`. Every reader goes through one line
  reader with no length limit (`readFileLines`; the CLI tools call `internal.ReadLines`).
- `stat`, `complexity`, `deps` and `callgraph` used a scanner that stopped at such a line and ignored its error:
  `stat` counted 1 line of a 34-line file, and `complexity` panicked on the mismatch. They read through the same
  reader now, and a read error is reported instead of being dropped.
- A file that fails in a directory run is reported on stderr as `Warning: <path>: <error>`. The other files'
  results are kept.
- On the fixed corpus only `jquery.js` changes: it now contributes 139 functions.

### Determinism and speed

- The D and Scala lexers' text is no longer cleaned a second time by the sanitizer; the finders read its facts. The
  fixed corpus output is unchanged.
- Raw strings close at their own delimiter. `r"..."` (Python, D), `r#"..."#` (Rust) and `@"..."` (C# verbatim) used
  to close at their opener, so they never closed and the lines after them were blanked: `Example` in
  `test_csharp_verbatim.cs` was missed. A triple-quoted string with a raw prefix (`r"""`) is a docstring.
- A multi-line string closes at its own marker (a single quote inside a docstring is text), and the code after the
  closing quotes on the same line is code again (`""", {"key": 1})` keeps its brackets). Python's logical lines now
  read the sanitizer's facts; the separate Python string cleaner is gone. The fixed corpus output is unchanged by it.
- The Python struct finder reads the same facts. A docstring's text is not a class, a field or the end of one: before,
  a line like `class Fake(Base):` inside a docstring was a class, and an unindented line of a docstring ended the class
  above it. `pythonCodeLine` counts a line only when it starts outside a string and holds code.
- A `@dataclass` or `@attr.s` decorator now sets its class's kind (`dataclass`, `attrs`) in the struct tree
  (`--struct --tree`). The decorator was matched against the class line, which never holds it, so these classes were
  reported as `class`. Only the kind label changes: on the Python fixtures, 15 classes change kind and nothing else.
- A class header followed by a comment and then its body (`class Widget` / `// the body follows` / `{`) keeps its body:
  the header check reads the next code line, and a comment is not code. Before, the class ended on its header.
- Ruby: `end` followed by a comment (`end # the class`) closes its class, and a method (`end # the first method`) closes too. The structural finders read the end from
  the cleaned line, where the comment is blank; before, the raw line never matched and the class swallowed the next one.
  The structural finders read the cleaned text and the indentation through `LineFacts`, computed in one pass.

- A `.h` file is read with the C++ configuration on every run. `--dir` used to pick the C or the C++ configuration
  at random for `.h` (both claim it), so the functions of a header changed from run to run: on the repository the
  total ranged from 17.3k to 19.8k functions across runs of the same binary.
- Nested-path languages (TypeScript, JavaScript, Go, and others) clean each line once instead of twice. The output
  is unchanged; the repository benchmark takes about 5% less time and 12% fewer allocations.
- A file is read once per job: the finders and the Go alias pass share its lines (it used to be read up to three
  times). The four copies of the open-and-scan code are one helper.
- Language configurations compile their regular expressions on first use, not at startup: a run pays only for the
  languages of its files. A single-file run starts about 20 ms faster.
- A line in the normal state that holds no string, comment, or literal start is returned as it is, without the
  rune conversion. The output is unchanged; a test compares the fast and the slow path on every fixture line.
- A line that matches two struct patterns (Go `type X struct{ Y int }` matches `struct` and `named`) takes its kind
  from the patterns in sorted order, not map order. The names and spans did not change.
- Pre-filters skip lines that cannot hold a function header (`func_requires`: a `(` or `<`, for the languages
  whose every header pattern needs one) or a class or struct header (`type_keywords`). Both are checked on
  every fixture line by a test. Cleaning a line reuses the sanitizer's rune buffers.
- Corpus full run after these changes: about 1.09 s, against 1.59 s before them (median of 7).
- Corpus of 725 files, full run: about 1.75 s before, 1.53 s after (median of 5). The output is byte-identical.

### Go type aliases

- A Go type alias (`type A = S`, or a line `A = S` of a `type ( ... )` group) is a name for an existing type, not a
  new one. It is no longer in a Go file's `classes`; it is listed in the file's `aliases` list (`name`, `line`,
  `target`) in `--json` output, and only files that have aliases carry the key. A Go count of `classes` drops by
  the number of single-line aliases. `cmd/astoracle` lists aliases the same way (21/21 on this repository).

### Class bounds (all languages)

- A class whose body opens and closes on its header line (`struct A : B {};`)
  no longer keeps the next class open, so the next class's members are found
  under their own class. This changes class lookup for every language with
  classes; see `TODO.md` for what was re-measured.

## v1.12.2 - 2026-10-12

Function discovery and boundaries are more accurate across the supported
languages. The changes apply to all tools that use the shared finder.

### Parsing fixes

- **Shared boundaries**: bodyless declarations no longer take the next
  function's body. Multiline signatures and expression bodies end at their
  own last line, including expressions that continue after a brace block.
- **JavaScript / TypeScript**: recognize constructors, getters, static/async
  methods, multiline class methods, typed arrows and function-typed
  parameters. Handle regex literals, template expressions, generic headers,
  functions after line separators and bodies that close mid-line.
- **Python**: logical statements, multiline decorators, continued signatures,
  strings and inline suites determine function/class bounds and line-based
  extraction without treating literal text as declarations.
- **PHP**: recognize reference-returning and nested methods; mask heredoc and
  nowdoc contents while preserving code after closing labels and original
  extraction.
- **Scala**: recognize modified, parameterless, operator-named and inline
  methods; handle braceless expressions, multiline literals and declarations
  without bodies.
- **D**: recognize nested functions, conditional/linkage/attribute prefixes,
  pointer/type returns, inferred properties and multiline headers. Handle
  contracts, nested comments and token strings. Constructor-delegation calls
  are excluded while local constructors and postblits remain visible.
- **C**: recognize function headers and parenthesized names such as
  `int (name)(...)`; control statements and function-pointer typedefs/variables
  are not functions.
- **C++ / Java / C#**: improve qualified names, trailing return types,
  multiline Java signatures and throws clauses, and C# expression bodies.
- **Rust / Kotlin / Swift / Ruby**: improve nested functions, modified
  declarations, expression ends, Ruby operator methods and percent literals.

### Analysis tools

- Reverse call lookup finds callers of qualified methods. Rust trees group
  impl blocks by type and mark recursion on the traversal path.
- Dependency analysis handles multiline Rust imports and classifies them;
  `deps` and `stat` list the accepted language codes.

### Real-project verification

Independent evaluators now live in `benchmarks/`, with project pins, runtime
versions, executable hashes and explicit exclusions. Python (Flask, Requests,
hr_portal), PHP (Monolog, Symfony Routing), Scala (scopt, os-lib), D (D-YAML,
arsd) and C (Lua) match all measured function ends with zero missing or false
functions after these fixes.

These measurements retain their scope: six D files have reference-grammar
errors and remain excluded; Lua is measured in one recorded GCC configuration
with inactive branches reported separately. Outstanding gaps in other
languages remain tracked in `TODO.md`.

## v1.12.1 - 2026-09-24

Found while mapping a React/TypeScript codebase: funcfinder, `complexity` and
`callgraph` missed most of its components. On that project the fixes raised
the mapped functions from 625 to 725 in `src/` (96 → 107 files).

### Fixes (JS/TS)

- **Typed arrow functions** (`ts`): a type annotation between the name and `=`
  is now allowed — `const X: React.FC<P> = (...) =>`, `const run: Handler =
  (a) =>`. Every typed React component used to be invisible (only its inner
  functions showed up).
- **`export default function`** (`js`, `ts`): `default` was not allowed after
  `export`, so `export default function App()` was not found.
- **Values are not functions**: a `const|let|var x = (...)` line that is a
  complete statement without `=>`/`function` (`const total = (1 + 2) * 3;`)
  no longer opens a function body that swallowed the next function.
- **Quotes inside strings** (all languages): a string now closes only on the
  delimiter that opened it. An apostrophe inside `"..."` (`"it's"`,
  `"зв'язок"`) used to close the string and reopen one at the trailing
  quote, so the rest of the file was read as a string — braces uncounted,
  every following function lost.
- **`single_line_strings`** (new language option, set for `js`, `ts`): a
  regular quoted string ends with its line, so an apostrophe in JSX text
  (`<p>Зв'язок</p>`) no longer swallows the code after it.

### Build

- **One version source**: the new `VERSION` file is read by `build.sh`,
  `build.ps1` and the `Makefile`; the release workflow fails if the tag does
  not match it. Before, local builds reported `1.10.<commits>` (scripts) or
  `1.8.<commits>` (Makefile) whatever the release was.
- **`make build`** no longer tries to build `cmd/docsearch` (moved to its own
  project);
  `make release` tags `v$(cat VERSION)` instead of rewriting the Makefile.

## v1.12.0 - 2026-09-19

**Breaking: `complexity`'s `Complexity`/`Level` fields now mean something
different.** The exponential `2^(maxDepth-1)` formula is gone — expect
different numbers on the same code than v1.11.x produced. Full rationale
and worked examples below.

Two threads that turned up together: unifying how all five tools resolve
function boundaries and control-flow keywords, and a full rework of how
`complexity` scores a function, prompted by a design discussion about
whether loops deserve more weight than conditionals (they don't, in the
general score — but loop-in-loop now gets its own distinct signal; see
below).

### Parsing unification

- **`stat`**: was the only tool not using the shared `internal.CreateFinder`
  finder — a pure line-scan for calls had no concept of function
  boundaries, so a function's own `name(...)` signature line matched its
  own call-detection regex and counted as a call to itself. A function
  called twice showed 3; a function never called anywhere still showed 1,
  making it indistinguishable from dead code. Now resolves real function
  names via the shared finder and excludes exactly the signature-line
  occurrence — a genuine recursive call elsewhere in the same body still
  counts.
- **`callgraph`**: fixed the same underlying problem more surgically.
  `BuildFileCallGraph` skipped every `callee == caller` match across a
  function's *entire body* to avoid the signature-line problem above — as
  a side effect, it also hid every genuine direct-recursive call. Now only
  the actual signature line (re-verified via the language's own function
  regex, not by comparing to `FunctionBounds.Start` — a decorator pushes
  `Start` to the decorator line, not to `def name(`) is excluded; a
  recursive call anywhere else in the body now produces an edge.
- **`internal/languages.json` / `LanguageConfig`**: promoted `complexity`'s
  per-language "what's a nesting decision vs. a flat sibling" regexes from
  private Go maps (keyed by strings that didn't always match the real
  `LangKey` — `"rs"`/`"sw"` never matched Rust's/Swift's actual
  `"rust"`/`"swift"`, silently falling back to a generic pattern for both
  the whole time) into `nesting_pattern`/`flat_pattern`/`loop_pattern`
  JSON fields with `NestingRegex()`/`FlatRegex()`/`LoopRegex()` getters,
  mirroring the existing `CallRegex()`/`ImportRegex()` shape. Added
  patterns for the five languages that had none at all (cpp, kotlin, php,
  ruby, scala).

### `complexity`: control-flow-aware depth, not raw syntax

Depth used to mean "how many `{`/indent levels deep," full stop — a
function built entirely of nested struct literals with zero branching
scored `depth=5, VERY_HIGH`, while a function with one real `if` didn't
clear the `SIMPLE` threshold. Fixed by crediting a nesting level only when
the line opening it is an actual decision (`if`/`for`/`while`/`switch`/...,
not `else`/`case`/`default`/... and not a struct literal, closure, or
plain scope block) — reusing the patterns above uniformly across all three
depth strategies (brace, indent, block-keyword), where previously only the
unused Ruby path had this logic at all.

Fixing this surfaced several previously-dormant regex gaps, now fixed for
all 13 brace languages:
- `else`/`catch`/`finally`/`elseif` sitting on the *same* line as the
  previous block's closing brace (`"} else {"`, `"} catch (e) {"` —
  standard K&R/gofmt/prettier style) never matched a bare `^\s*` anchor.
- Every keyword group's trailing character class only checked what
  *followed* the keyword, never that the keyword itself *ended* there —
  so a bare call to a function whose name starts with a reserved word
  (`forwardRequest()`, `doWork()`, `ifPresent(x)`, all common in
  Go/Java/JS) matched as if it opened a `for`/`do`/`if`. Replaced with a
  plain word boundary (`\b`) everywhere: since these are reserved words in
  every language here, a boundary check is both sufficient and simpler
  than enumerating legal trailing shapes.
- `for _, v := range xs` / `for _ in range(n)` — idiomatic Go/Python loop
  variables — never matched Go's/Python's own nesting pattern at all,
  because the trailing character class excluded `_`.

### `complexity`: additive score replaces the exponential

The `2^(maxDepth-1)` formula turned small depth differences into wildly
different numbers (on this repo's own code: depth 5→6→7→8 was
`16→32→64→128`), and measured only the single deepest path — missing
functions that are complex from sheer *number* of shallow, independent
decisions rather than depth (this repo's own `LoadConfig()`, depth 3,
scores `60`/`CRITICAL` under the new formula for exactly this reason: a
long run of independent `if conf.XPattern != "" {...}` checks).

Replaced with an additive score (SonarSource Cognitive Complexity's
formula): each decision costs `1 + depth-at-that-point`; each flat sibling
(`elif`/`else`/`case`/`default`) costs a flat `1` with no depth bonus,
since it's an alternate branch of the same decision, not a fresh one. A
single `if` costs 1; seven nested `if`s cost 28 (not `2^6=64`); ten
independent sibling `if`s cost 10, not an exponential. `MaxNestingDepth`
is unchanged and still reported alongside the score as a separate,
purely structural signal. Level thresholds (`SIMPLE`…`CRITICAL`) now
bucket on the score; the cutoffs are a first calibration (the old
depth-based cutoffs run through the new formula's own triangular-number
case), not a final one.

### `complexity`: loop-nesting as its own signal

A loop nested inside another loop carries a distinct risk — O(n²)+ cost,
iteration-interaction bugs — that nesting a conditional simply doesn't.
Weighting loops higher in the general score was considered and rejected:
it would equally penalize the common, benign case of an `if` inside a
`for`. Instead, `MaxLoopNestingDepth` tracks loop-in-loop specifically
(reusing the same depth machinery with a loop-only pattern), surfaced as
an independent `loop-nesting=N` line in `complexity`'s output only when
`N>=2`, without changing `Complexity`/`Level`/`MaxNestingDepth`.

## v1.11.1 - 2026-08-13

Bugfix release, found while stress-testing funcfinder against the real
Django source tree (908 files).

- **`--struct`**: classes declared with keyword base-class args (e.g.
  `class Foo(metaclass=Bar):`) were silently skipped — the class-header
  regex didn't allow `=` in the base-class list. Fixed in Python's struct
  finder.
- **`--tree`**: for Python, methods were never grouped under their class —
  `FindFunctions()` never populated `result.Classes`/`ClassName`, so the
  tree always fell back to a flat function list. Now correctly nests
  methods (including under nested classes like `Meta`) under their class.
- **`deps`**: every single Python import — stdlib, first-party, and
  third-party alike — was misclassified as `stdlib`. A stray `""` entry in
  the stdlib prefix list matched everything via `strings.HasPrefix`.
  Replaced with a real Python 3 stdlib module set, plus local-package
  detection so a project's own dotted imports (e.g. `django.db.models`
  when scanning `django/`) are classified as internal rather than
  external, and bare third-party imports (`import requests`) are
  classified as external instead of the old internal-by-default fallback.
- `gofmt -w` cleanup across 18 files that had drifted from `gofmt`
  formatting (whitespace/alignment only, no behavior change).

## v1.11.0 - 2026-07-18

**docsearch and docsearch-server moved to their own product,
[distill-docs](https://github.com/ruslano69/distill-docs)** (full history
preserved). funcfinder returns to its focused code-analysis identity — 5 tools:
`funcfinder`, `stat`, `deps`, `complexity`, `callgraph`.

- **New public API**: package `analyze` exposes the code-mapping engine
  (`LoadConfig`, `NewDirProcessor`, `DirResult`) so other modules can use
  funcfinder as a library — this is how distill-docs consumes it for code-ingest.
- Removed `cmd/docsearch`, `cmd/docsearch-server`, and their `internal/*`
  packages; build/CI/release and docs slimmed to the 5 tools.
- No changes to the analysis tools themselves; the knowledge/truth-server
  features live on (and continue) in distill-docs.

## v1.10.0 - 2026-07-18

Завершение ТЗ docsearch-server: реализованы три последних функциональных
требования (FR-2, FR-15, FR-20) — **все FR-1…FR-22 закрыты**. Плюс веб-краулер
для `docsearch`, укрепление code-ingest после code-review и починка
версионирования в релизном пайплайне.

### 🕸️ docsearch — веб-краулер и авто-эмбеддинг

- **`docsearch add --url <site>`** — обходит сайт документации и индексирует
  все страницы в базу знаний. Держится в пределах host + path-префикса старта
  (`…/net/http` не уходит в `…/os` или на другой домен), дедуплицирует
  идентичные и версионированные (`@go1.x`/`@v1.x`) страницы по хешу,
  извлекает текст из `<main>`/`<article>`, пропуская nav/header/footer.
  `--max-pages` (по умолч. 200) ограничивает обход. Полностью покрыт тестами
  (14 тестов, 80–100% на `web.go`).
- **Авто-эмбеддинг через Ollama** (`--embed-model`/`--embed-url` на `add` и
  `search`) — однофайловый `docsearch` теперь сам генерирует эмбеддинги при
  добавлении и поиске, превращая `hybrid`/`vec` из «фактически FTS» в реальный
  семантический поиск без ручного `--embedding`. Файловые/краул-ингесты
  эмбеддят все чанки одним батч-запросом; при недоступном эндпоинте — деградация
  до FTS с предупреждением, а не падение. Переиспользует `internal/embed`
  (тот же клиент, что и docsearch-server). Явный `--embedding` по-прежнему
  перекрывает. Проверено вживую на `qwen3-embedding:0.6b`.

### 🆕 docsearch-server — новые возможности

- **HTTP/JSON read-server** (`serve-http`, FR-20) — та же stateless read-нода,
  что и TCP `serve` (общая релизная модель, тот же hot-swap канала), но по
  HTTP: `GET /search`, `/read`, `/context`, `/releases`, `/channels`,
  `/healthz`. Строго readonly по CQRS — любой write-verb на любом маршруте
  отдаёт 405; ингест/паблиш живут на writer/CLI/MCP-стороне. Режим поиска
  (`fts|vec|hybrid|regex`) и live-эмбеддинг зеркалят CLI `search`, так что
  HTTP- и CLI-грундинг не расходятся.
- **Политика хранения релизов** (`prune --keep N`, FR-15) — оставляет N
  новейших релизов, удаляет остальное (и файл, и строку в control-БД).
  Релиз, на который смотрит канал, не удаляется никогда, независимо от
  возраста; удаление идёт от старых к новым, чтобы сбой на середине не
  тронул более свежие релизы. Доступно из CLI и MCP.
- **Структурированный JSON-ингест** (FR-2) — `ingest --file records.json`
  принимает объект или массив записей changelog/task/decision с per-record
  провенансом (`author`/`role_tags`/`source_version`/`source_ref`); флаги
  `--author`/`--role-tags`/`--source-version` работают дефолтами для записей,
  не задавших своё поле.

### 🛡️ Надёжность (по итогам code-review)

- **Атомарный code-map ингест** — `publish --code-dir` теперь заменяет карту
  кода в одной транзакции; сбой на середине откатывается целиком, а не
  оставляет write-log наполовину переписанным (есть regression-тест с
  инъекцией сбоя).
- **Диагностика пустой карты** — misrooted `--code-dir`, стёрший непустую
  карту в пустую, теперь выдаёт предупреждение вместо тихой потери.
- **Провенанс-сигнал в MCP/CLI** — предупреждения code-map (dirty/no-git/
  пустая карта) больше не теряются при нуле файлов.

### 🔧 Релизный пайплайн

- Released-бинарники теперь собираются с `-ldflags -X …Version=<tag>` и
  корректно отвечают на `--version` (раньше показывали `dev`).

## v1.9.0 - 2026-07-17

Седьмой инструмент: **docsearch-server** — версионированный сервер правды для
команд с распределённой разработкой (см. `docs/docsearch-server/TZ.md`).
Построен с нуля в этой серии сессий: релизная модель, гибридный поиск,
MCP-сервер, и — как флагманская фича — funcfinder компилируется прямо в
релиз как структурная карта кода. Плюс пять реальных багов, найденных и
исправленных в существующем ядре toolkit по ходу работы (не косметика —
затрагивают вывод `funcfinder`/`callgraph`/`deps` на 8+ языках).

### 🆕 docsearch-server — версионированный источник правды

- **Релизная модель**: `ingest`/`record` копятся в live write-log; `publish`
  снимает неизменяемый `truth-YYYY.MM` через `VACUUM INTO`; каналы
  `stable`/`testing`/`unstable` — атомарные указатели на релиз. Пин на
  релиз = идентичный результат навсегда, пока релиз хранится.
- **Гибридный поиск**: FTS5 + вектор (BYO-эмбеддинги, любой
  Ollama-совместимый эндпоинт) + regex, с корректной деградацией до FTS.
  `suggest` отдаёт реальный словарь корпуса с IDF (weak-key,
  партиционно-относительный) вместо угадывания термина.
- **`enumerate`/`read`**: аудит полноты («все ли X найдены, не только
  первые») и чтение полного контекста по id-диапазону или source-файлу —
  вместо ad-hoc SQL-скриптов и `grep -o`.
- **MCP-сервер** (13 инструментов) — первичный интерфейс для агентов, строго
  по CQRS (readonly-грундинг vs rewrite-петля); TCP read-server с
  zero-downtime hot-swap канала под нагрузкой.
- **Метаданные как SQL, не блоб** (FR-3): `author`/`role_tags`/`source_version`
  — индексированные generated-колонки, не JSON для парсинга приложением.
  На них построены:
  - **`context(role)`** (FR-9) — та же база, разные линзы по тегам роли.
  - **`provenance`** (FR-17) — кто/когда/по какой ссылке внёс запись.
  - **`freeze`** (FR-11) — окно стабилизации перед релизом.
  - **`diff_releases`** (FR-18) — added/removed/changed между двумя релизами.
- **`publish --code-dir <path>`** (FR-22, флагманская фича): funcfinder
  прогоняется по указанному дереву исходников (все 15 языков,
  `.gitignore`-aware) и впекает в релиз структурную карту функций/типов —
  «где определена X» отвечается точным индексом, тегированным git-коммитом,
  а не приближённым вектором по блобам исходника. Повторный `publish
  --code-dir` заменяет карту, а не копит устаревшие снимки.

```bash
docsearch-server publish --name 2026.07 --channel stable --code-dir ./internal
docsearch-server search --query "BuildShardGraph" --channel stable
# → importresolver.go: BuildShardGraph: 74-124
```

### 🐛 Пять багов в ядре toolkit, найденные по ходу работы

- **`findFunctionsSimple` глотал функции с однострочным телом** — влияет на
  Rust, C, C++, C#, Java, D, Kotlin, PHP. `fn helper() -> i32 { 42 }` (Rust)
  или `{ return 42; }` на отдельной строке (K&R C) давали net-баланс скобок
  0, что было неотличимо от «скобки ещё не было вовсе» — функция
  оставалась «открытой» и молча поглощала весь код до конца файла, включая
  следующую функцию целиком.
- **`deps --shards` тихо возвращал пустой/leaf-only граф при неверном root**
  — если директория анализа на уровень ниже `tsconfig.json`/`go.mod`, алиасы
  не резолвились без единого предупреждения. Теперь два диагностических
  сигнала: поиск `tsconfig.json` выше по дереву и ratio-based
  предупреждение по доле разрешённых intra-project импортов.
- **`shardForDir` нестабильно терял ребро к родительскому пакету** — импорт
  `internal` мог случайно провалиться в `internal/knowledge` (совпадение по
  строковому префиксу, порядок итерации Go-карты не определён спецификацией).
  Теперь матчится точная родительская директория.
- **`callgraph --dir -l <lang>` был подсказкой, а не фильтром** — до фикса
  анализировал файлы всех языков независимо от `-l`. Теперь реально
  ограничивает скан.
- **HybridStructFinder глотал типы после brace-less `type_alias`** (TS/JS) —
  `type Handler = (req: Request) => void;` без скобок держал состояние
  открытым и поглощал следующий `class`/`interface` целиком.

## v1.8.1 - 2026-07-01

Патч-релиз: точность Go доведена до полного паритета с `go/ast`. funcfinder
теперь мапит символ-в-символ и `tdtp-framework` (3366/3366), и собственный
исходник (606/606) — **recall и precision 100%**. Все фиксы — дешёвая доводка
регекс-движка, без парсера; каждый найден новой измерительной «линейкой».

### 🎯 Точность Go: детекция символов

- **Char/rune-литералы больше не текут в подсчёт скобок.** `char_delimiters` был
  не задан ни одному языку — санитайзер не вычищал `'{'` / `'}'`, и строка
  лексера вроде `if c == '{' {` проглатывала функцию целиком. Включён для
  **Go, C#, Java, D, Kotlin** (C/C++ уже держали `'` в `string_chars`). Rust и
  Scala осознанно пропущены — там одиночная `'` легальна (лайфтаймы, symbol-
  литералы).
- **Defined-типы** — `type X string`, `type X func(...)`, `type X []byte`,
  `type X map[...]` — раньше не детектились (движок знал только
  struct/interface/alias).
- **Композитные defined-типы** — `type X map[string]struct{}` /
  `...interface{}` с вложенными сбалансированными скобками.
- **Inline-структуры** — `type X struct{}` / `struct{ io.Writer }` закрываются
  на своей строке, а не «проглатывают» следующий тип.
- **Дженерики** — `func F[T any](...)`, включая методы и мульти-параметрические
  constraints.

### 📏 AST-линейка и spec-sheet (внутренний контроль качества)

- `cmd/astoracle` — ground-truth экстрактор символов Go на `go/ast`. Это не
  продукт, а **«линейка»** для честного замера точности регекс-движка; обходит
  всё дерево (включая функционально-локальные типы), поэтому не недосчитывает.
- `benchmarks/specsheet.py` — считает recall/precision против линейки и экономию
  токенов на эталонных проектах, воспроизводимо.

Результат «bootstrap-теста» («компилятор компилирует сам себя»): funcfinder
понимает собственную кодовую базу без единого расхождения с `go/ast`.

## v1.8.0 - 2026-06-30

Первый бинарный релиз с времён v1.6: два новых инструмента (**docsearch**,
**callgraph**), поддержка Unicode-идентификаторов и ревью горячих путей.

### 🆕 docsearch — база знаний для агентов

Новый инструмент: персистентная база знаний на проект в одном SQLite-файле
(`.knowledge/docs.sqlite`), которую агент может наполнять и опрашивать между
сессиями — дешевле, чем перечитывать файлы или заново выводить уже известное.

- **Гибридный поиск**: FTS5 (полнотекстовый) + векторный (cosine/L2) + regex,
  и режим `hybrid` (FTS+вектор, корректно деградирует до FTS без эмбеддингов).
- **Ингест файлов** `.txt`/`.md`/`.pdf` с автоматическим чанкингом по секциям
  и overlap; PDF проходит через OCR-quality gate (плохие сканы отклоняются, а
  не засоряют базу).
- Действия: `init` / `add` / `search` / `count`. Эмбеддинги не считаются
  внутри — передаются снаружи, инструмент их хранит и сравнивает.

```bash
docsearch --db .knowledge/docs.sqlite init
docsearch add --file README.md --type general
docsearch search --query "как работает X" --limit 5
```

Документация: [docs/DOCSEARCH.md](docs/DOCSEARCH.md), скилл: [skills/docsearch.md](skills/docsearch.md).

### 🆕 callgraph — кто кого вызывает

Новый инструмент: строит граф вызовов (forward и reverse) по файлу или
директории. `--reverse --func Name` — impact-анализ «что сломается, если
поменять Name».

```bash
callgraph --dir . -l go --func ProcessDirectory --depth 2
callgraph --dir . -l go --reverse --func computeShardChecksum
```

### Unicode-идентификаторы (все 15 языков)

funcfinder/findstruct/callgraph/stat теперь находят имена функций, типов и
вызовов с не-ASCII буквами (`func Привет()`, `type Café`) — раньше терялись,
т.к. паттерны использовали `\w` (в Go RE2 только ASCII).

- Единый источник истины `internal/identifiers.go`: `identClass`
  (`[\p{L}\p{Nd}_]`) и `identStart` (`[\p{L}_]`). Языковые паттерны ссылаются
  через плейсхолдер `{IDENT}` (раскрывается при загрузке конфига); `callgraph`
  строит свой regex из тех же констант — detection имени и detection вызова не
  могут разойтись.
- 112 capture-групп в `languages.json` переведены `(\w+)` → `({IDENT}+)`.
- Без флага, из коробки: для ASCII-кода поведение и вывод идентичны прежним.

### Производительность и корректность

- **Санитайзер (`CleanLine`) ~2.5x быстрее** — `matchesAt` больше не аллоцирует
  на каждый символ (16148 → 6592 ns/op на реалистичном миксе).
- **Исправлен баг byte/rune** в санитайзере: на не-ASCII строках буфер
  раздувался и обработчики блочных комментариев/докстрингов могли смещаться.
- **JSON-вывод через `encoding/json`** (`formatDirResultsJSON`/`formatManifestJSON`)
  вместо ручной сборки строк: починено экранирование управляющих символов,
  формат манифеста теперь идентичен тому, что пишет `deps --update-manifest`.

### Чистка и тесты

- Убраны «велосипеды»: ручной hex → `encoding/hex`, пузырьковая сортировка →
  `sort.Strings`, обёртка `itoa`, `containsSubstr`.
- Удалена мёртвая ветка в `findFunctionsSimple`.
- Покрытие тестами `internal`: 42% → ~67% (суммарно по пакетам ~71%).

### Исправление

- `Makefile` `VERSION_BASE` рассинхронизировался (был 1.6 против 1.7 в
  build.ps1/build.sh) — выровнен.

## v1.7.0 - 2026-04-30

### Inter-Shard Dependency Graph (`deps --shards`)

Расширение `deps` для работы с шардами: строит граф зависимостей между шардами
и записывает `depends_on` в `manifest.json`. Агент видит архитектуру проекта без
чтения исходников.

#### Новые флаги `deps`

```bash
# Граф зависимостей между шардами
deps . -l go --shards --json

# Записать depends_on в manifest.json
deps . -l go --shards --update-manifest .codemap/manifest.json

# Без gitignore (нужно для cmd/ пакетов)
deps . -l go --shards --no-gitignore --update-manifest .codemap/manifest.json
```

#### Автоматическое определение алиасов (TypeScript / Next.js)

Читает `tsconfig.json` и резолвит `@/` алиасы в реальные пути:

```bash
# На Next.js проекте: @/ → src/, граф строится корректно
deps frontend -l ts --shards --update-manifest .codemap/manifest.json
```

Результат в `manifest.json`:
```json
{
  "path": "cmd_funcfinder.json",
  "files": 1,
  "total_functions": 8,
  "checksum": "5ddb4d84...",
  "depends_on": ["internal.json"]
}
```

#### Пример на реальном проекте (meetily, 269 файлов, TypeScript + Rust)

```
src_app.json          → src_components, src_hooks, src_lib, src_services, src_types
src_components.json   → src_hooks, src_lib, src_services, src_types
src_contexts.json     → src_hooks, src_lib, src_services
src_lib.json          → (нет зависимостей — дно стека)
src_types.json        → (нет зависимостей — дно стека)
```

Агент загружает `src_components_AISummary` и сразу знает: нужен ещё `src_types`.

#### Полный workflow

```bash
# 1. Карта кода с шардами
funcfinder --dir . --all --json --split

# 2. Граф зависимостей между шардами
deps . -l go --shards --no-gitignore --update-manifest .codemap/manifest.json

# 3. Агент читает манифест — видит и функции, и зависимости
cat .codemap/manifest.json
```

#### Новые внутренние модули

- `internal/shardutil.go` — `PathToShardName`, `ShardKeyForPath` (вынесено из dirprocessor)
- `internal/importresolver.go` — `BuildShardGraph`, `DetectModulePrefix`, `DetectTSAliases`
- `ShardInfo.DependsOn []string` — новое поле в манифесте

#### `CollectSourceFiles` — новый параметр `useGitignore`

```go
// Опциональный параметр, по умолчанию true
CollectSourceFiles(dir, langConfig, recursive, useGitignore...)
```

---

## v1.6.0 - 2026-04-29

### Large Codebase Support: Split, Incremental & Fast Checksums

#### Проблема которую решает v1.6.0

На проекте из 500 файлов `--dir . --json` генерирует ~375KB (~93K токенов) — это
половина контекстного окна только на карту кода. AI агент вынужден загружать всё,
чтобы найти один модуль. v1.6.0 решает это через sharding и инкрементальные
обновления.

#### --split: разбивка вывода на шарды

```bash
# Разбить по директориям (default)
funcfinder --dir . --all --json --split

# Разбить по файлам (гранулярнее)
funcfinder --dir . --all --json --split --split-by file --out .codemap
```

Результат — плоская структура файлов (`ls` даёт полный обзор):
```
.codemap/
  manifest.json          ← индекс, 500 байт
  internal.json          ← 27 файлов, 15KB
  cmd_funcfinder.json    ← 6 файлов, 3KB
  test_examples.json     ← 26 файлов, 22KB
```

Manifest содержит метаданные без загрузки шардов:
```json
{"path": "internal.json", "files": 27, "total_functions": 248, "total_classes": 40, "checksum": "5583a4ba..."}
```

#### --inc: инкрементальные обновления

```bash
# Первый запуск — создаёт шарды с чексуммами
funcfinder --dir . --json --split

# Повторные запуски — пересчитывает только изменённые
funcfinder --dir . --json --split --inc
# INFO: Incremental: 1 shards changed, 32 unchanged
```

Чексумма вычисляется по содержимому файлов (не по mtime), поэтому `git checkout`
не вызывает ложных пересчётов.

#### Быстрые хэши: FNV-1a 128-bit и XXH3

| Хэш | Скорость | Зависимости | Как включить |
|-----|----------|-------------|--------------|
| FNV-1a 128-bit | базовая | нет (stdlib) | дефолт |
| XXH3-128 | ~3-5x быстрее | `zeebo/xxh3` | `-tags xxh3` |

```bash
# Стандартная сборка (FNV-1a, zero deps)
./build.sh

# С XXH3
go build -tags xxh3 -ldflags "..." -o funcfinder ./cmd/funcfinder
```

#### Производительность на больших проектах

| Размер проекта | Полный JSON | Manifest | Один шард | Экономия токенов |
|---------------|-------------|----------|-----------|-----------------|
| 60 файлов     | 44KB / 11K tok | 0.5KB | 15KB / 4K tok | **3x** |
| 500 файлов    | ~375KB / 93K tok | 1KB | ~15KB / 4K tok | **~25x** |
| 5000 файлов   | ~3.7MB / 930K tok* | 5KB | ~15KB / 4K tok | **~230x** |

*превышает контекстное окно без --split

**Инкрементальный режим (500 файлов, 33 шарда):**
- Изменён 1 файл → пересчитывается 1 шард (3% работы), 32 пропускаются
- ~33x быстрее полного пересчёта на больших проектах

#### Остальные улучшения v1.6.0

- ✅ `--struct "TypeA,TypeB"` — shorthand вместо `--struct --type "TypeA,TypeB"`
- ✅ `stat` — поддержка директорий (режим `--dir`)
- ✅ Исправлен баг: `--dir .` пропускал все файлы из-за hidden-file check на root path

---

## v1.5.0 - 2026-01-12

### Extended Language Support: 4 New Languages

**Новые языки (без изменений Go кода!):**
- ✅ **Kotlin** - suspend functions, data classes, sealed classes, objects, coroutines
- ✅ **PHP** - classes, traits, interfaces, visibility modifiers (public/private/protected)
- ✅ **Ruby** - modules, class methods, methods with ? and ! characters
- ✅ **Scala** - case classes, traits, objects, pattern matching, functional programming

**Итого:** 15 языков (было 11)

**Тестирование:**
- Все 4 языка протестированы с `--map`, `--tree`, `--json`, `--extract`
- Поддерживаются все режимы работы (standalone, filter, tree visualization)
- Конфигурация добавлена в `internal/languages.json`

**Примеры:**
```bash
# Kotlin
funcfinder --inp App.kt --source kotlin --map
# Output: getUser: 5-7; fetchUserAsync: 9-12; findUserById: 14-16; main: 21-23;

# PHP
funcfinder --inp Controller.php --source php --tree
# Output: class UserController with methods

# Ruby
funcfinder --inp user.rb --source ruby --func valid? --extract
# Output: extracts method with special characters

# Scala
funcfinder --inp Service.scala --source scala --json --map
# Output: JSON with all functions
```

**Roadmap progress:**
- v2.0.0 target: 30+ languages → Current: 15/30 (50% progress!)

---

## v1.4.0 - 2026-01-06

### Line Range Filtering, Cross-Platform File Slicing & Code Analysis Utilities

**Новые возможности:**
- ✅ **--lines flag** - extract specific line ranges from files
- ✅ **Standalone mode** - use `--lines` without `--source` for plain text extraction
- ✅ **Filter mode** - combine `--lines` with `--map`, `--func`, or `--tree` to narrow search scope
- ✅ **Cross-platform sed alternative** - works on Windows without external tools
- ✅ **Flexible syntax** - supports `100:150`, `:50`, `100:`, single line `100`
- ✅ **JSON output** - `--lines` + `--json` for structured line data
- ✅ **Smart warnings** - INFO messages when using line filtering with function search

**Синтаксис:**
```bash
# Standalone mode: plain text extraction
funcfinder --inp file.txt --lines 10:50
funcfinder --inp config.yaml --lines :100 --json

# Filter mode: narrow function search to specific lines
funcfinder --inp server.go --source go --map --lines 100:300
funcfinder --inp api.ts --source ts --tree --lines 200:
funcfinder --inp main.java --source java --func handleRequest --lines 50:150
```

**Архитектурные изменения:**
- Добавлен `lines.go` - line range parsing and file reading logic
- Добавлена структура `LineRange` - represents line range with start/end
- Добавлен метод `FindFunctionsInLines()` в Finder - supports line offset for filtering
- Добавлен метод `findClassesWithOffset()` - class detection with line offset
- Обновлен `main.go` - standalone mode and filter mode support

**Функции в lines.go:**
- `ParseLineRange()` - parses range strings like "100:150", ":50", "100:", "100"
- `ReadFileLines()` - reads specific line ranges from files
- `CheckPartialFunctions()` - detects if functions are cut by line range
- `OutputPlainLines()` - plain text output with line numbers
- `OutputJSONLines()` - JSON output for line data

**Примеры использования:**
```bash
# Quick file slice (Windows-compatible, no sed needed)
funcfinder --inp app.log --lines 1000:1100

# Find functions only in specific area
funcfinder --inp large_file.go --source go --map --lines 500:1000

# Extract function in range with body
funcfinder --inp server.js --source js --func handleAPI --lines 100:300 --extract

# Tree view of limited scope
funcfinder --inp Calculator.java --source java --tree --lines 1:100
```

**Улучшения:**
- Cross-platform compatibility: no dependency on sed/tail/head
- Fast file slicing: ~10-50x faster than PowerShell alternatives on Windows
- Works with ANY file type in standalone mode
- Preserves line numbers from original file
- INFO messages to clarify filtering behavior

**Known Limitations:**
- Python files with `--lines` may have issues due to indent-based parsing (warning shown)
- Functions that start before or end after the range will be excluded
- No partial function bodies extracted (by design)

**Почему важно для Windows:**
- PowerShell alternatives to sed are 50x+ slower
- Native cross-platform solution
- No external tools required

### Additional Code Analysis Utilities

**Новые утилиты:**
- ✅ **stat.go** - function call counter for 9 languages
- ✅ **deps.go** - dependency analyzer for 9 languages
- ✅ Complete code analysis toolkit for AI agents

**stat - Function Call Counter:**
```bash
# Build and use
go build -o stat stat.go
stat finder.go -n 10

# Output:
# Language: Go
# Functions: 28
# append                    11
# len                       5
# CountBraces               4
```

**Возможности stat:**
- Подсчет вызовов функций в исходных файлах
- Поддержка 9 языков: Python, Go, Rust, JS/TS, Swift, C/C++, Java, D, C#
- Top-N фильтрация самых вызываемых функций
- Обработка комментариев и строковых литералов
- Поддержка декораторов (Python, Java)

**deps - Dependency Analyzer:**
```bash
# Build and use
go build -o deps deps.go
deps . -l go -n 10

# Output:
# Language: Go
# Total imports: 12
# Unique modules: 12
# stdlib: 6, external: 1, internal: 5
# fmt                             11 (std)
# strings                         10 (std)

# JSON output
deps . -l go -j > dependencies.json
```

**Возможности deps:**
- Анализ зависимостей модулей в проекте
- Классификация: stdlib, external, internal
- Подсчет использования каждого модуля
- JSON вывод для интеграции с CI/CD
- Поддержка 9 языков программирования

**Workflow для AI-агентов:**
```bash
# 1. Структура кода
funcfinder --inp api.go --source go --map

# 2. Самые вызываемые функции
stat api.go -l go -n 10

# 3. Граф зависимостей
deps . -l go -j
```

**Архитектура утилит:**
- Zero dependencies (только stdlib)
- Config-driven language support
- Regex-based parsing
- Совместимая архитектура с funcfinder
- 300-400 строк кода каждая

---

## v1.3.0 - 2026-01-02

### Tree Visualization & Class Hierarchy + Rust & Swift Support

**Новые возможности:**
- ✅ **Tree visualization mode** (`--tree`) - hierarchical display of functions and classes
- ✅ **Full tree mode** (`--tree-full`) - tree view with complete function signatures
- ✅ **Class detection** - automatic identification of classes/structs/interfaces
- ✅ **Method-class association** - methods are shown as children of their classes
- ✅ **Unicode tree rendering** - beautiful tree formatting with box-drawing characters (├──, └──, │)
- ✅ **Multi-language class support** - works with Go, C++, C#, Java, D, JS, TS, Python, Rust, Swift
- ✅ **Rust support** - structs, traits, enums, impl blocks, pub/async functions
- ✅ **Swift support** - classes, structs, protocols, enums, static/public functions

**Архитектурные изменения:**
- Добавлен `tree.go` - tree building and formatting logic
- Добавлена структура `ClassBounds` - для отслеживания границ классов
- Обновлена структура `FunctionBounds` - добавлены поля `ClassName` и `Scope`
- Обновлена структура `FindResult` - добавлено поле `Classes`
- Добавлен `TreeNode` - структура для построения иерархического дерева
- Обновлен `config.go` - добавлен `ClassRegex()` и `HasClasses()`
- Обновлен `finder.go` - добавлены методы `findClasses()` и `findClassForLine()`

**Обновления languages.json:**
- Добавлен `class_pattern` для всех языков с поддержкой классов:
  - Go: `type Name struct`
  - C++: `class/struct Name`
  - C#: `class/interface Name`
  - Java: `class/interface Name`
  - D: `class/struct/interface Name`
  - JavaScript: `class Name` (with export support)
  - TypeScript: `class Name` (with export support)
  - Python: `class Name`
  - **Rust**: `struct/trait/enum/impl Name` (NEW)
  - **Swift**: `class/struct/enum/protocol Name` (NEW)

**Расширяемая архитектура:**
- ✅ **Rust и Swift добавлены БЕЗ изменения Go кода** - только через конфигурацию!
- Демонстрация мощи data-driven архитектуры funcfinder
- Теперь поддерживаются **11 языков** (было 9)

**Форматы вывода:**
- Tree compact: Shows function/method names with line ranges
- Tree full: Shows complete function signatures in tree format
- JSON compatible: Existing JSON output includes class information

**Примеры использования:**
```bash
# Compact tree view
funcfinder --inp Calculator.java --source java --tree

# Tree with full signatures
funcfinder --inp api.ts --source ts --tree-full

# Python classes with decorators
funcfinder --inp models.py --source py --tree
```

**Улучшения:**
- Standalone functions shown at root level when no classes present
- Mixed classes and functions properly visualized
- Nested classes supported (shown in hierarchy)
- Decorators preserved in Python tree output

**Known Limitations:**
- Nested classes shown but not as separate tree nodes
- Anonymous classes not detected

---

## v1.2.0 - 2026-01-02

### Python Support & Decorator Detection

**Новые возможности:**
- ✅ **Python support** (py) - indent-based parsing
- ✅ **Decorator detection** (@decorator, @decorator(args))
- ✅ Support for async/await functions
- ✅ Support for generator functions (def name, async def)
- ✅ Class methods and static methods
- ✅ Multiline function signatures
- ✅ Decorators in JSON output

**Архитектурные изменения:**
- Добавлен `python_finder.go` - специализированный парсер для Python
- Добавлен `decorator.go` - структура DecoratorWindow для сбора декораторов
- Добавлен `finder_factory.go` - фабрика для выбора парсера
- Обновлена структура `FunctionBounds` - добавлено поле `Decorators`
- Обновлен `formatter.go` - декораторы включены в JSON output

**Поддерживаемые паттерны Python:**
- `def name():` - обычные функции
- `async def name():` - async функции
- `@decorator` + `def name():` - функции с декораторами
- Multiline signatures - `def name(\n  arg1,\n  arg2\n):`
- Class methods с декораторами @property, @staticmethod, @classmethod

**Тестовые примеры:**
- Создана директория `test_examples/` с примерами для всех 9 языков
- Python: test_example.py - комплексные примеры с декораторами
- Скрипт test_all_languages.sh для автоматического тестирования

**Улучшения:**
- Исправлен regex паттерн для Java (добавлена поддержка `{` в конце строки)
- Тестовые файлы перемещены в отдельную директорию

**Known Limitations:**
- Lambda functions не поддерживаются
- Nested functions внутри классов могут детектироваться некорректно

---

## v1.1.0 - 2025-12-31

### JavaScript/TypeScript Support & Version Flag

**Новые возможности:**
- ✅ Поддержка JavaScript (js)
- ✅ Поддержка TypeScript (ts)
- ✅ Обработка async/await функций
- ✅ Поддержка export функций
- ✅ **Generator functions** (function*, async function*)
- ✅ **Arrow functions** (const name = () => {}, async arrow functions)
- ✅ Arrow functions с let/var
- ✅ Поддержка generic типов (TypeScript)
- ✅ Single quotes (') и double quotes (")
- ✅ Template literals (`)
- ✅ **--version flag** для проверки версии

**Обновления:**
- Добавлены unit тесты для JS/TS
- Обновлена документация
- Примеры использования для JS/TS

**Поддерживаемые паттерны:**
- `function name() {}` - обычные функции
- `async function name() {}` - async функции
- `function* name() {}` - generator функции
- `async function* name() {}` - async generator функции
- `const name = () => {}` - arrow функции
- `const name = async () => {}` - async arrow функции
- `let/var name = () => {}` - arrow функции (let/var)
- `export function name() {}` - экспортируемые функции
- Generic функции TypeScript

**Известные ограничения:**
- Методы классов без ключевого слова "function": `methodName() {}`
- Function expressions: `const f = function() {}`

---

## v1.0.0 - 2025-12-31

### Первый релиз

**Основной функционал:**
- ✅ Поиск границ функций в исходниках
- ✅ Поддержка 6 языков программирования
- ✅ Три режима вывода (grep/json/extract)
- ✅ Обработка комментариев и строковых литералов
- ✅ Вложенные функции (closures, lambdas)

**Поддерживаемые языки:**
- Go
- C
- C++
- C#
- Java
- D

**CLI опции:**
- `--inp` - входной файл
- `--source` - язык программирования
- `--func` - поиск конкретных функций
- `--map` - карта всех функций
- `--json` - JSON вывод
- `--extract` - извлечение тел функций
- `--raw` - обработка raw strings

**Производительность:**
- Линейная сложность O(n)
- ~50ms на файл 5000 строк
- Экономия токенов: 95%+

**Тестирование:**
- ✅ Go stdlib (fmt/print.go)
- ✅ Собственные исходники
- ✅ Production код (TELAConverter.cs)

**Известные ограничения:**
- Regex паттерны могут не покрывать все edge cases
- Не обрабатывает препроцессор C/C++ (#ifdef)
- Raw strings в C# (@"...") требуют доработки

**Пакет:**
- Статический бинарник (3MB)
- Скрипты установки/удаления
- Полная документация
- Контрольные суммы (SHA256, MD5)

**Размер дистрибутива:** 1.8MB

# ffd: funcfinder in D

`ffd` reads a source tree with the token finders of all 15 languages that live next to it (`../*_port.d`), in parallel,
and prints what `funcfinder --dir` prints. It is the first executable of the D port that is more than a probe; the
contract and the rules for changing it are in [AGENTS.md](AGENTS.md).

```bash
python benchmarks/ffd/build.py                       # build/ffd/ffd[.exe]; needs LDC (set $LDC2 to the local compiler)
build/ffd/ffd --dir . --all --json                   # functions, types and Go aliases, as JSON
build/ffd/ffd --dir src --struct                     # types only, path:line: name
build/ffd/ffd --dir . --all --json --workers 4 --timing
```

Implemented: `--dir`, `--all`, `--struct`, `--json`, `--workers` (default 8, at most 16), `--recursive=false`,
`--no-gitignore`, `--timing`, `--version`. The root `.gitignore`, hidden names and the extension table follow the Go
tool exactly (the same files, the same "skipped N files" notice on 31 trees). File mode also supports `--inp`, `--source`,
`--map`, `--func`, `--extract`, `--lines`, `--type`, file `--struct`/`--all` and file/directory `--names`/`--doc` (see below).
File/directory `--tree` and `--tree-full` are also supported.
Directory JSON also supports `--split`, `--split-by dir|file`, `--out` and `--inc`.
`--raw` controls existing function cleaners in file mode; `--help`/`-h` lists the keys.
Not yet: `funcstats`, `callgraph`, `complexity`, `deps`.

## How close to Go

Measured by key (`ffd_compare.py`), Go 1.15.9 against `ffd` on 30 trees:

- **Functions:** all Go functions are found except 185 of about 98,000 (Rust macro bodies, some C++ templates, three in
  Kotlin and Ruby). `ffd` also finds functions Go does not (Python `requests` +29, C headers, Swift, Kotlin).
- **Types:** JavaScript and TypeScript are identical to Go's (`express`, `lodash`, `zod` are byte-identical in `--all`;
  OmniRoute's 6,971 types match). Other languages list their own named types: more than Go for Kotlin, Swift, Scala,
  Rust, Ruby, Python and PHP; fewer for C#, C/C++ and D. Counts per corpus: `ffd_deltas.json`.
- **Bytes:** identical on 6 of 30 trees (Java gson, Express, lodash, zod, D-YAML, Flask); elsewhere the keys match but
  the order of `classes` follows the source, not Go's struct-finder order, or the sets differ as above.

## Speed

Median of 5 to 7 interleaved runs of `--dir --all --json` on a 16-core/32-thread Windows machine, against the **released
Go 1.15.9** (`funcfinder-windows-amd64.exe` of the GitHub release; `ffd_compare.py` prints and records the reference's
version). Go defaults to all 32 threads, `ffd` to 8 (at most 16). Milliseconds; `@1` and `@8` give both tools the same
number of threads:

| Tree | Go @1 | ffd @1 | Go @8 | ffd @8 | Go default | ffd default |
|---|---:|---:|---:|---:|---:|---:|
| gson (Java) | 99 | 51 | 53 | 39 | 46 | 39 |
| fmt (C++) | 224 | 65 | 72 | 59 | 75 | 58 |
| polly (C#) | 205 | 107 | 95 | 72 | 91 | 73 |
| zod (TS) | 226 | 149 | 65 | 49 | 54 | 49 |
| Seal (Kotlin) | 108 | 37 | 38 | 28 | 31 | 29 |
| arsd (D) | 562 | 230 | 112 | 99 | 128 | 101 |
| flask (Python) | 44 | 25 | 21 | 23 | 21 | 23 |
| huginn (Ruby) | 140 | 93 | 45 | 56 | 53 | 57 |
| maybe (Ruby) | 164 | 92 | 68 | 70 | 57 | 66 |
| serde (Rust) | 62 | 40 | 26 | 32 | 25 | 31 |
| Alamofire (Swift) | 209 | 55 | 55 | 47 | 45 | 48 |
| Kingfisher (Swift) | 190 | 40 | 55 | 31 | 39 | 31 |
| OmniRoute (TS/JS) | 2917 | 2061 | 655 | 452 | 483 | 447 |
| tdtp-framework (Go) | 241 | 134 | 72 | 88 | 59 | 88 |

**Per thread `ffd` does much less work** (serially 29% to 79% less time, 45-79% on most trees) and **scales worse than Go**:
Go goes about 6 times faster from 1 to 32 threads, `ffd` about 4.5 times from 1 to 8 and not at all beyond (OmniRoute, before
the scanners below: 559 ms at 8 threads, 870 at 16, 1463 at 32). With 8 threads each `ffd` is ahead on 9 of 14 trees, level on
1, behind on 4; against Go's default of 32 threads it is ahead on 8 and behind on 6. The limit is the shared runtime
allocator and the finders that still allocate; the next steps are in `TODO.md`.

The TypeScript `declaration_pattern` and `declarator_pattern` are read by scanners (as `func_pattern` is): OmniRoute serial
2643 → 2061 ms, with 8 threads 611 → 452 ms; zod serial 206 → 149 ms. The Go tool has the same scanners in its own PR.

The tokenizers and cleaners of every finder keep their token, pair, stack and text blocks from file to file on each thread
(`reuse.d`), and the D finder keeps the text of its cleaned lines in one block. Parse time at 1 → 8 workers (ms, the same
outputs byte for byte on 30 trees × 4 modes): arsd (D) 213 → 189 serially and 82 → 39 with 8 threads (2.6× → 4.9×), Seal (Kotlin)
23 → 10 and 13 → 3, serde (Rust) 28 → 12 and 18 → 4, Alamofire (Swift) 35 → 21 and 28 → 10, huginn (Ruby) 74 → 51 and 33 → 14,
gson (Java) 36 → 23 and 24 → 7, OmniRoute 1906 → 1834 and 290 → 274 (6.6×). Allocation fell from 8-20 to 1-8 bytes per source
byte in most finders; what is left is the JS finder's phases on small files, C++ (35 on tdtp) and Ruby (7.7).

## File mode

Fresh measurements on the user's i3-12100 (4 cores/8 threads), 16 pinned projects and seven shuffled full-CLI samples
are in [PERFORMANCE_20261021.md](PERFORMANCE_20261021.md). Against current Go, geometric-mean D speedups are 2.02×
serial / 1.50× at eight workers for `--all`, and 1.86× / 1.42× for functions. Type-only mode is about 7% / 14% slower.
The four `--all` projects with equal output keys give 1.33× at eight workers. These are implementation comparisons,
with remaining accuracy differences; they do not isolate the programming language. The historical table above is a
different machine/baseline. The report also separates an observed C++ slowdown in current Go from D's advantage.

Map one file or select functions:

```powershell
build/ffd/ffd.exe --inp test_examples/test_example.go --source go --map
build/ffd/ffd.exe --inp test_examples/test_example.go --source go --map --json
build/ffd/ffd.exe --inp test_examples/test_example.go --source go --func SimpleFunction,FunctionWithParams --extract
build/ffd/ffd.exe --inp test_examples/test_edge_cases.cpp --source cpp --func "operator bool" --extract
```

File mode requires `--source` and either `--map` or `--func`. `--func` accepts comma-separated names and keeps
every overload; name-keyed JSON retains the last overload, as Go does. `--extract` takes precedence over `--json`.
Type and combined modes:

```powershell
build/ffd/ffd.exe --inp test_examples/test_example.ts --source ts --struct --map --json
build/ffd/ffd.exe --inp test_examples/test_example.ts --source ts --struct --type User --extract
build/ffd/ffd.exe --inp test_examples/test_example.ts --source ts --struct "User,UserService" --extract
build/ffd/ffd.exe --inp test_examples/test_example.go --source go --all --json
build/ffd/ffd.exe --inp test_examples/test_example.go --source go --names --all
build/ffd/ffd.exe --dir test_examples --names --all
```

`--type` filters types; `--struct A,B` is shorthand. `--all` requires `--map`, `--json` or a tree flag, rejects name selectors,
and gives JSON priority over extraction. Type/all modes do not support `--lines`. Type maps list field names;
JSON fields include `name`, normalized `type` and source `line`. Empty fields are omitted in struct JSON and kept
as an empty array in combined JSON. Native type/span differences from Go remain.

Initial field profiles cover direct named members in all 15 languages: Python class annotations/annotated `self`
attributes, Ruby literal attribute declarations, Go grouped named fields and common C-family/colon-style members.
Methods, nested-type contents, local variables and comment/literal contents are excluded. Complex/multiple C-family
declarators, embeddings, primary/tuple constructors, braceless declarations, attributes/macros and dynamic properties
remain unsupported or partial. Types are token spellings, without semantic resolution; results can differ from Go's
more limited regex field profiles. Field extraction runs only for file type maps/JSON/trees.

Display declaration and path trees:

```powershell
build/ffd/ffd.exe --inp test_examples/test_example.go --source go --tree-full
build/ffd/ffd.exe --inp test_examples/test_example.go --source go --all --tree
build/ffd/ffd.exe --dir test_examples --tree --workers 8
```

File trees group methods under classes and show nested functions; full trees include signatures. Type trees list
fields. Both directory flags produce Go's current path-tree output. JSON/extraction retain priority; tree flags
cannot be combined with each other or named function/type selection. Native declaration and field differences remain.

`--names` prints kind, name and `path:line`, separated by tabs. It accepts `--all`/`--struct`, but rejects output,
selection and range flags, including explicitly supplied false flags. Native type sets/kinds can differ from Go;
in particular the default C profile includes type rows Go omits. Aliases appear in type/all names modes.

Read declaration comments and the first source line:

```powershell
build/ffd/ffd.exe --inp test_examples/test_example.go --source go --doc SimpleFunction,Server
build/ffd/ffd.exe --dir test_examples --doc SimpleFunction,Server --workers 8
```

`--doc` searches functions, types and Go aliases, preserving overloads and ordering by path/source line. Each entry
contains `path:line: kind name`, the directly adjacent comment, and the first source line. Blank lines break comment
attachment; decorators and Rust attributes are skipped with Go's rules. Inline bodies can occur on the first line;
Python body docstrings are not read. `--doc` accepts worker/walk flags but rejects output/selection/range modes,
including explicitly false incompatible flags. A wholly missing selection exits 2. Native parser differences remain.

Read text lines without a language or parse a source range with global line coordinates:

```powershell
build/ffd/ffd.exe --inp README.md --lines 1:10
build/ffd/ffd.exe --inp README.md --lines :10 --json
build/ffd/ffd.exe --inp test_examples/test_example.go --source go --map --lines 15:17
```

Ranges accept `N`, `N:M`, `:M`, `N:` and `:`; numbering starts at 1 and an end past EOF is clipped.
Non-Python parsed ranges can cut declarations or lose their enclosing scope. Python ranges adjust to whole scopes
using Go's rules, then filter complete functions. If adjusted, the same text report precedes the result on stdout,
including with `--json`; the selected-range notice goes to stderr. Standalone text ranges never expand.
Ordinary file/directory Python maps retain the native D significant ends; underlying parser differences remain.
Extraction uses original source text, with Go-compatible newline handling (CRLF normalized except for Python).

Write split output and update changed shards:

```powershell
build/ffd/ffd.exe --dir src --all --json --split --out .codemap
build/ffd/ffd.exe --dir src --all --json --split --split-by file --out .codemap-files --inc
```

Split defaults to one shard per directory and `.codemap` output. The v1.0 manifest and raw-byte FNV-1a 128 checksums
match default Go builds; empty source files contribute to checksums. Alias-only files do not populate shards.
Ambiguous flattened shard names are rejected before writing. Native declaration differences remain.

`--inc` skips parsing/writing unchanged shards and repairs missing or invalid shard JSON. Deleted/emptied shards
are removed using the prior manifest. The private `.ffd-state` stamp invalidates cached results when the executable,
mode or scan profile changes; the first D run over an existing Go output therefore performs a full scan. Keep separate
output folders for Go and D. A normal split leaves unlisted files alone and invalidates the incremental stamp.

`--raw` is a parsing control, not an output format: Go/C#/JS/TS function cleaners can include raw-string contents
in brace counting. Types and directory scans keep normal parsing, as Go does. Other native lexers keep their literal
rules; D's preprocessing also ignores the flag in Go. Native raw-string/profile differences remain. Extraction always
returns original source. Combined output retains normal type metadata alongside raw-mode functions.

Validate the CLI against a specified Go binary:

```powershell
python benchmarks/ffd/file_checks.py --ffd build/ffd/ffd.exe --go funcfinder.exe
python benchmarks/ffd/type_checks.py --ffd build/ffd/ffd.exe --go funcfinder.exe
python benchmarks/ffd/python_range_checks.py --ffd build/ffd/ffd.exe --go funcfinder.exe
python benchmarks/ffd/field_checks.py --ffd build/ffd/ffd.exe --go funcfinder.exe
python benchmarks/ffd/doc_checks.py --ffd build/ffd/ffd.exe --go funcfinder.exe
python benchmarks/ffd/tree_checks.py --ffd build/ffd/ffd.exe --go funcfinder.exe
python benchmarks/ffd/split_checks.py --ffd build/ffd/ffd.exe --go funcfinder.exe
python benchmarks/ffd/incremental_checks.py --ffd build/ffd/ffd.exe
python benchmarks/ffd/raw_checks.py --ffd build/ffd/ffd.exe --go funcfinder.exe
python benchmarks/ffd/flag_checks.py --ffd build/ffd/ffd.exe --go funcfinder.exe
```

This is a CLI regression check; real-project accuracy and timing require the independent benchmark harnesses.
Directory JS/TS `--struct` skips unused function discovery; see the [measured pilot and checks](TYPES_ONLY_PILOT.md).
Directory D `--struct` also skips function work after shared preparation; see [measurements and accuracy gaps](D_TYPES_ONLY.md).
Directory modes and their known key differences are described in [AGENTS.md](AGENTS.md).

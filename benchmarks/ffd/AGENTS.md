# benchmarks/ffd/

## Purpose

`ffd` is the D port of funcfinder's file and directory discovery: one executable with the token finders of all 15
languages (`../*_port.d`). It is experimental: production stays Go, and CI runs only the `genconfig` Go test.
Directory mode covers `--dir [--all|--struct] [--json]`, `--workers`, `--recursive`, `--no-gitignore` and `--timing`.
File mode covers `--inp --source (--map|--func names) [--json|--extract] [--lines range]` and standalone
`--inp --lines range [--json]`. File type/all modes, `--tree`, `--names`, `--doc`, `--split` and the other four tools
are not implemented and say so.

## Ownership

- `ffd_main.d` flags, messages and exit codes; `ffd_walk.d` directory discovery; `ffd_ignore.d` the root `.gitignore`;
  `ffd_run.d` the worker pool and mode filter; `ffd_langs.d` language registry, configuration checks and the finder of
  each language; `ffd_json.d` output; `ffd_model.d` the neutral result; `ffd_goalias.d` Go aliases; `ffd_hybrid.d` the
  JavaScript/TypeScript types of Go's struct finder.
- `ffd_file.d` file-mode validation, name selection, line ranges, source extraction and file-specific JSON/text;
  `file_checks.py` compares file CLI contracts with Go using generated fixtures under ignored `build/`.
- `README.md` owns user-facing build/run examples and file/directory limitations.
- `genvectors/` (Go) writes `ts_declaration_vectors.jsonl`: 6,000 lines with what Go's TS declaration and declarator patterns
  return, which `ffd_vectors.d` (a unit test) holds the D scanners in `../typescript_header.d` to.
- `genconfig/` (Go) writes `ffd_config.json`, the resolved configuration of every language; `ffd_config.json` is
  compiled into `ffd` as a string import.
- `build.py` builds with LDC (`--test` builds and runs the unit tests); `ffd_compare.py` compares `ffd` with the Go tool
  by key, times both and checks `ffd_deltas.json`; `ffd_deltas.json` records the accepted differences.
- `../reuse.d` is the thread-local block reuse the finders and cleaners share.

## Local Contracts

- File mode uses the same `analyzeSource` token finders as `analyzeFile`; do not introduce a second parser. `--source`
  is explicit, except for standalone text `--lines`. File function JSON is Go's name-keyed `{name: {end, start}}`
  map (last duplicate wins), with Python decorator metadata; text/extraction retains all overloads. Empty function
  results exit 2, argument/IO failures exit 1, flag syntax failures exit 2. `--extract` takes precedence over `--json`.
- Read file source once and retain original lines independently of reusable parsing buffers. Extraction normalizes
  CRLF like Go's generic reader; Python retains CR as Go's dedicated reader does. Standalone `--lines` handles
  `N`, `N:M`, `:M`, `N:` and `:`, clips ends to EOF, rejects empty selections and needs no language/configuration.
  Parsed `--lines` parses the literal selected slice and restores global line coordinates. It does not implement
  Go's Python-specific automatic scope expansion/report; underlying D/Go span differences remain visible.
- **Keys, not bytes, are the contract.** The JSON has the Go tool's shape (`files[].path/functions[]/classes[]/aliases[]`,
  `total_*`, two-space indent, `\n\n` at the end, strings escaped as `encoding/json` does; `ffd_json.d` unit tests pin
  the escapes against Go 1.25) and `ffd_compare.py` compares `(name, line)` per file. Functions must match Go's; types
  match for JavaScript and TypeScript exactly (`ffd_hybrid.d`). Where the D finders are more accurate than Go they differ
  and the difference is recorded, never hidden (see Differences below). The order of `classes` follows the source, not
  Go's struct-finder order (structs and interfaces first), so outputs are not always byte-identical.
- **One finder per language: the token-based D finders.** Do not add a second (regex or Go-parity) finder or a switch
  between them. When Go and a D finder differ by convention, adapt the D finder in `ffd_langs.d` (Python functions start at
  their first decorator; a C++ function starts at its signature line, after a `template <...>` prefix, as Go reports it).
- **Languages are configured once, before any worker runs** (`prepareLanguages`): compiled `Regex` objects are only read
  afterwards, and the module-level patterns of `dlang_port.d` and `typescript_port.d` are `__gshared` for that reason.
  Do not configure per thread (building them in every worker cost more than parsing a small tree) and do not call a
  finder before `prepareLanguages`.
- **A regex a scanner stands for is proved against Go's regexp, not against another scanner**: the D scanners for the TS
  `func_pattern`, `declaration_pattern` and `declarator_pattern` are used only while the configuration holds the pattern text
  they were written for (`js_ts_model.Config`), the regex is then not compiled, and the vectors file (made by Go's regexp,
  kept current by a Go test) must pass; the `AuditTSHeader` build also compares every call with the D regex. Regenerate with
  `go run ./benchmarks/ffd/genvectors > benchmarks/ffd/ts_declaration_vectors.jsonl` when a generator or pattern changes.
- **`checkConfig` refuses a configuration the finders were not written for** (comment markers, delimiters, flags, the TS
  header pattern, the JS/TS struct patterns). A `languages.json` change needs `go run ./benchmarks/ffd/genconfig >
  benchmarks/ffd/ffd_config.json` (`go test ./benchmarks/ffd/genconfig` fails until it is done) and, if a pattern a
  hand-written matcher stands for changed, the matcher too.
- **Memory is reused from file to file on each thread.** The source bytes, the line array, the line facts, the cleaned
  text (an arena) and the token arrays live in thread-local blocks that the next file overwrites. Therefore nothing a
  finder returns may point into them: `toSyms` and the alias/type code copy every name (`idup`), and any new result string
  must be copied the same way. Appending with `~=` in a hot loop is slow in D (a runtime call each time); prefer sizing
  or a reused block. The D finder's cleaned-line text is one such block (`ownText` in `dlang_port.d`); the allocator
  lock, not the collector, was what kept it at 2.5× on 8 threads (the GC off changed nothing, fewer allocations did).
- **Threads: default 8, never more than 16.** The default is the measured best (OmniRoute 559 ms at 8 workers, 870 at 16,
  1463 at 32): the runtime allocator is shared and the finders still allocate. Reducing allocation, not adding workers,
  is how the run gets faster.
- **Walking follows the Go tool's `collectFiles` order and rules**: root `.gitignore` (git glob rules, `!` lines dropped,
  nested files not read), then hidden names (a leading `.`, but not `.gitignore`), then directories, then the extension
  (case-sensitive, `.h` goes to C++). `ffd_ignore.d` matches by walking tokens, not with a regex; its unit tests are the
  cases of the Go matcher. Stdout and stderr are binary on Windows: a line ends in LF.
- **Comparing**: `ffd_compare.py --ffd <exe> --go <funcfinder> --corpora-root <repo>/build` runs every corpus
  (`build/*/repos/*` plus local trees); `--expect benchmarks/ffd/ffd_deltas.json` checks the pinned ones against the
  recorded differences (counts per language and a hash of the sorted list) and fails on any change, better or worse;
  `--update` records the current ones. Corpora are not in worktrees: pass the main checkout's `build/`.
  **The Go reference must be the release binary, not a local build**: a stale `funcfinder.exe` (the checkout's root
  binaries are rebuilt by other work) once made the speed claims wrong. The script prints the reference's `--version` and
  `ffd_deltas.json` records it; a different version fails the check. Download the release archive and pass its
  `funcfinder` as `--go`; time with `--workers 1` and `--workers 8` as well as the defaults (Go's default is 32 threads).

## Differences from Go (pinned corpora, `--all`)

Functions: every Go function key is found except 185 of about 98,000 (all 30 corpora): 111 in Rust (`serde`: functions generated inside
`macro_rules` bodies, which the D finder does not expand), 71 in C++ (`fmt`: macro-like and template cases the token
finder does not take, e.g. `StreamableToString` after a `template` line), 2 in Kotlin (`Seal`) and 1 in Ruby (`huginn`).
`ffd` finds more than Go in Python (`requests` +29), C/C++ headers (`lua` +162, `fmt` +357), Swift, Kotlin, Ruby and a few
others. Types: JavaScript and TypeScript match exactly; the other languages list their own set of named types, which
is larger than Go's for Kotlin, Swift, Scala, Rust, Ruby, Python and PHP and smaller for C#, C/C++ and D (Go's struct
finder adds kinds the D finders do not list). The exact counts are in `ffd_deltas.json`.

## Work Guidance

- Change a finder in `../*_port.d`, not here, unless the difference is a Go convention (then in `ffd_langs.d`).
- After a change run the unit tests and the comparison with `--expect`; a difference that is a real improvement is
  recorded with `--update` and described in the commit.
- Measure with `ffd_compare.py --time N` (median of N interleaved runs) and `ffd --timing` (phases and the slowest
  files); `--timing-alloc` adds bytes allocated per language but calls `GC.stats()`, which locks the heap, so it
  distorts timings: never time with it.

## Verification

```bash
python benchmarks/ffd/build.py --test        # unit tests of all modules, compiled in
python benchmarks/ffd/build.py               # release build: build/ffd/ffd
python benchmarks/ffd/file_checks.py --ffd build/ffd/ffd.exe --go funcfinder.exe
go test ./benchmarks/ffd/genconfig ./benchmarks/ffd/genvectors   # ffd_config.json and the vectors are current
python benchmarks/ffd/ffd_compare.py --ffd build/ffd/ffd.exe --go funcfinder.exe --corpora-root <repo>/build --expect benchmarks/ffd/ffd_deltas.json
```

- File checks cover all 15 languages, multi-name selection, duplicate overloads, JSON, original extraction,
  CRLF/Unicode/long lines, standalone and parsed ranges, missing functions/files and invalid flags/ranges.
  Directory regression checks must retain the existing maps and compare one/eight-worker outputs after file-mode changes.

## Child DOX Index

None.

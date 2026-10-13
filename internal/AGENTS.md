# internal/

## Purpose

Core parsing engine. Implements function/struct discovery, body extraction, call-graph analysis, dependency resolution, shard splitting, output formatting, and incremental update logic for all 15+ supported languages.

## Ownership

- All language-specific parsing logic lives here.
- `cmd/` packages are consumers only; they must not bypass the public API of this package.
- Test files (`*_test.go`) are co-located with the code they test.

## Local Contracts

- Public API surface: `FindFunctions`, `FindStructs`, `ProcessDirectory`, `BuildCallGraph`, `WriteSplitOutput`, `WriteSplitOutputIncremental`.
- Language dispatch is handled by `finder_factory.go` (functions) and `struct_finder_factory.go` (structs); new languages must register here.
- `languages.json` is the canonical list of supported file extensions → language identifiers.
- Shard output goes to `.codemap/` by default; manifest is `.codemap/manifest.json`.
- Incremental mode (`--inc`) uses xxh3 checksums (`checksum_xxh3.go`) with stdlib fallback (`checksum_stdlib.go`).

## Work Guidance

- Adding a new language: implement the `Finder` interface, register in `finder_factory.go` and `struct_finder_factory.go`, add the extension in `languages.json`, add test fixtures in `test_examples/`.
- C function recognition in `languages.json` accepts parenthesized names in definitions/prototypes. A pointer declarator `(*name)` must not turn its return type into a function name; preserve original extraction and offsets. `c_parenthesized_test.go` covers these boundaries and selected-function mode.
- `enhanced_sanitizer.go` strips string literals and comments before parsing to prevent false positives; touch it carefully and run its bench test after changes.
- `python_finder.go` and `python_lines_processor.go` handle Python's indent-based scope; keep them in sync.
- Both Python paths share logical-statement boundaries: continuation lines and string contents do not dedent a scope, suite colons are read outside brackets, and inline suites end with their logical statement. Finder class ranges use the same rules; extraction retains original source and trailing trivia.
- PHP uses the nested finder so methods of classes inside functions are visible. `php_heredoc.go` masks heredoc/nowdoc text before function/class discovery, preserves trailing code after closing labels and restores original source for function extraction, including offset scans.
- D preparation in `d_lines.go` masks nested comments/literals, contracts and declaration prefixes, joins multiline return-type headers with original end coordinates, and rejects statement/type headers. D uses nested discovery with the nearest class scope; constructor-delegation calls must be filtered even in selected-function mode, while local constructors/postblits remain visible. Extraction retains original source. Reserved words and types are excluded as names. `prepareDLines` runs the function pattern only through `dFuncHeader`, which applies the `func_requires` gate (`(`) first, and runs `dAttributeLine` only on non-blank lines: blanked comments make long space-only lines, where the patterns backtracked for most of a D run.
- Scala preparation and endpoint checks in `scala_lines.go` handle braceless expressions, bodyless declarations, multiline signatures/literals, symbol-vs-char quotes and inline members. Function recognition stays configured in `languages.json`; shared nested-parser and sanitizer implementations are unchanged.
- C++ (`cpp` in `languages.json`) runs the simple scan with its header patterns; the other languages have none of them. `func_pattern` takes nested parentheses in parameters, a body on the header line, `operator` names, qualified constructors and a template prefix. `header_open_pattern` starts a signature that continues on later lines (any scope); `constructor_pattern` matches a constructor only inside a class and only when its name is that class's name (or `~` it); `declaration_pattern` matches a bodyless declaration ending in `;`, whose parameters are typed (a literal or a lone name is a direct initialization such as `Widget w(42);`); inside a class body `member_declaration_pattern` also takes a lone name. A line that starts with a `statement_words` entry is a statement, not a header; `conversion_operator_pattern` drops conversion operators, which the finder does not name. `initializer_lists` makes a constructor's `member{...}` a braced initialization rather than the body: `sigScanner.bodyAt` tells the brace counter where the body starts. Class bodies end by net brace depth, so a one-line class (`struct A : B {};`) does not hold the next class open. `class_pattern` skips a `template <...>` prefix and an export macro (`class FMT_API name`). Measured by `benchmarks/cpp_tree_eval.py`.
- TypeScript/JavaScript (`ts`, `js`, nested scan): `class_pattern` takes `export`/`default`/`abstract` modifiers and `interface`; a class header may span lines, so a class closes only after its body has opened and closed (`classOpened`), and a header with no body that is complete closes on its own line (`classHeaderContinues`). `declaration_pattern` (ts) lists bodyless signatures: overloads, interface and class signatures with a return type, and `abstract` methods; it is tried only at the top of a scope. A `const|let|var name = (` header is a value unless an arrow or a return type follows the parenthesis that closes on its line (`assignedParen`). `open_type_header_pattern` opens a typed header whose type argument runs over lines (`const X: React.FC<{`, `async list(): Promise<`), and `typedHeaderIsFunction` decides it. A `{` after `<` is a type literal (`sigScanner.typeBraces`; `braceDeltaSkippingTypeLiterals` in the nested scan): its braces and any `;` inside are not the body or the end of the signature. A body that opens and closes on its header line ends there (`bodyOnLine`); what follows its closing brace belongs to the caller. A later declarator of a `const|let|var` list on the same line (`const id = f(), notification = (x) => ...`) is read after its `,` with `declarator_pattern`, when its statement starts at the line start or after a `;` or `{` outside brackets (`listDeclarationBefore`); the list's keyword applies to it.
- Call graph logic (`callgraph.go`) operates on the output of the finder layer, not on raw source.
- `BuildDirCallGraph` reuses the directory job's `DirResult.Functions` (`dirFileCallGraph`); it finds a file's bounds
  again only when `DirResult.LangKey` differs from the language the extension gives (a `.h` under `-l c`). Files are
  scanned in parallel and reported in path order. `TestBuildDirCallGraph_ReusedBoundsMatchRefind` checks that reused
  and re-found bounds give the same graph on the fixtures. `buildFileCallGraph` looks a bare call up in the file's own
  functions and then in `knownFuncs`; it never copies `knownFuncs` (every function of the run) per file.
- Reverse lookup uses `ReverseCallers`: bare names include qualified callees with the same final name; qualified queries match exactly. Results are sorted, deduplicated name-based candidates, not type-resolved method identities. The full reverse graph preserves qualified keys.
- Language regular expressions compile on first use: `LoadConfig` only parses `languages.json`, and every accessor on `LanguageConfig` calls `ensure()` first. Read compiled fields through the accessors, never directly; `TestEveryLanguageCompiles` makes a broken pattern fail the build. `NewPythonFinder` and `NewPythonStructFinder` take the config by pointer: a copy would copy the `sync.Once`.
- `CleanLine` returns a line unchanged when `noLiteralStarts` holds: no configured string, char, doc-string, comment, or percent/regex marker occurs in it. A new marker field in `LanguageConfig` that the sanitizer reads must be added to `noLiteralStarts`; `TestCleanLineFastPathMatchesSlowPath` checks the fast path against `cleanLineSlow` on every fixture line. `char_literal_lookahead` (Rust) makes a quote in `char_delimiters` open a literal only when it closes on the same line (`charLiteralEnd`); a lifetime stays code.
- Read source files with `readFileLines` (or `ReadLines` from `cmd/`), never with `bufio.Scanner` on its default limit: a 64 KB line made the scanner stop, and a file with such a line (minified bundles) was silently truncated. `TestReadFileLines_LineLongerThan64KB` guards it.
- Pre-filters on cleaned lines: `func_requires` (a function header holds one of the words) and `type_keywords` (a class or struct header holds one of the words). A new function, class or struct pattern for a language must keep that word in every alternative, and the word goes into `languages.json`; `TestFuncRequiresNeverHidesAHeader` and `TestTypeKeywordsNeverHideAType` check every fixture line. Decisions about code read the `LineFacts` of a line (`CleanLinesFacts`: the cleaned text, the start state, the indentation of the raw line, and `CodeText`), computed in one pass. The raw line is read only for the output and for quoted method names and heredoc terminators.
- Rust `nesting_pattern` (`languages.json`) matches `if`, `match`, `while` or `loop` anywhere on the cleaned line, a `for` at the start of a line, and `for … in`; `impl X for Y` is not a decision. The complexity score of Rust functions depends on it: `cmd/complexity/rust_nesting_test.go` pins the cases.
- The D and Scala lexers clean their own language (`prepareDLines`, `cleanScalaLines`) and hand their text to the finders as `lexerFacts`; the sanitizer does not run on that text again. The Scala lexer keeps each literal as a pair of parentheses, which `scalaFunctionEnd` reads, so its text is not blanked like the sanitizer's.
- Python logical lines (`pythonLogicalLines`) read the sanitizer's `LineFacts`; there is no separate Python string cleaner. Multi-line strings close at their own marker, and the rest of the line after the closing quotes is code.
- `PythonStructFinder` (`python_struct_finder.go`) reads `LineFacts` too: a class header, a decorator, a field and the end of a block count only on a code line (`pythonCodeLine`: starts outside a string and has `CodeText`). Its header and field matches read `Cleaned`, and the block end uses `Indent`. A `@dataclass` or `@attr.s` decorator sets the kind of the next class header (`dataclass`, `attrs`); a code line that is neither a decorator nor a class header clears it.
- The finders on the simple path (`findFunctionsSimple`), the class finder (`findClassesWithOffset`, whose `classHeaderContinues` reads the cleaned next line), the structural finders (`StructFinder`, `HybridStructFinder`) and the block finder (`finder_block.go`) read `LineFacts`; a line that starts outside a literal with the language's line-comment marker has no `CodeText`; the block finder steps the state with `lineFacts` and skips heredoc bodies, which are not cleaned. The raw line is read only for the heredoc terminator. A Ruby `end` is `rubyBlockEndAt` on the cleaned line.
- Each file is read once per job (`lazyLines` in `dirprocessor.go`): finders that take lines get the shared lines; Python's finder still reads the file itself.
- `GetLanguageByExtension` visits languages in sorted order and gives `.h` to `cpp` (both `c` and `cpp` claim it). Keep the choice stable: `--dir` output must not depend on map order.
- Go type aliases (`type A = S`, and the `A = S` lines of a `type ( ... )` group) are read by `findGoAliases` (`go_aliases.go`) from sanitized lines and kept in `DirResult.Aliases`, apart from the types. Go has no `type_alias` struct pattern (`languages.json`): an alias must not come back as a type.
- `--names` (`names.go`) prints the directory job's results: `ProcessSingleFile` runs one file through `processFile`, and `FormatNames` lists the results. Kinds come from `FunctionBounds.ClassName` (method), `ClassBounds.Kind` (empty means a class from the class finder) and `AliasBounds`. The class finder and the struct finder share one entry per declaration: the struct finder's kind is copied onto it. `functionKind` and `classKind` are the one place the kinds are named.
- `--doc` (`docs.go`) reads the raw lines above a declaration (`DocLines`): line comments, or a block comment that ends on the line above, with decorators skipped. Comments are output only, so raw lines are fine there. `DirResult.LangKey` gives the comment markers (`LineComment`, `BlockCommentStart/End`). Python docstrings are not read yet.
- `structPatternOrder` tries the Go `named` pattern last: it also matches a one-line `type X struct{}`.
- Incremental split (`ProcessDirectoryIncremental`, `WriteSplitOutputIncremental`): a shard is a file's directory, and its checksum covers all its files, sorted. A shard with no declaration has no shard file; its checksum is in `Manifest.EmptyShards` (`emptyShardChecksums`), so the next run does not take it for changed. Both split writers record it.
- Shard graph (`importresolver.go`): `BuildShardGraphFor` takes a `ResolveContext` with the language's data (Go module prefix, TS/JS aliases, Rust crates). Rust crates come from `DetectRustCrates` (`importresolver_rust.go`): each `Cargo.toml` that names a `[package]` or `[lib]`. A Rust import resolves by crate name or `crate::`, and walks the crate's `src/` shard directories as far as the tree has files. Rust imports count toward `ShardGraphStats` only when they name a known crate.
- `shardutil.go` owns all `.codemap/` path conventions; do not hardcode shard paths elsewhere.
- `DetectLanguage` (`detect.go`) gives a directory's language by source-file count, ties by language key, and a file
  by its extension. `complexity` uses it when `-l` is absent.
- `FilterExcluded` (`exclude.go`) applies `complexity`'s `-exclude` globs to a file list: a pattern matches a base name,
  or a path under the analysed root with `/`. A malformed pattern is an error.
- `FindUncalled` (`uncalled.go`) lists the functions that no code line of a tree names (`funcstats --uncalled`). A use
  is an identifier in cleaned code, so comments and strings do not count. Test files, `main`/`init` and the Go interface
  methods are not candidates. The rules are in `docs/FUNCSTATS.md`.

## Verification

```bash
go test ./internal/...          # run all unit tests
go test -bench=. ./internal/... # run benchmarks (includes enhanced_sanitizer)
```

## Child DOX Index

- `internal/knowledge/` — SQLite knowledge base: FTS5 + vector hybrid search for agent memory — see [internal/knowledge/AGENTS.md](knowledge/AGENTS.md)

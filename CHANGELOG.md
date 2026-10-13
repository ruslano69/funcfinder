# Changelog

## Unreleased

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

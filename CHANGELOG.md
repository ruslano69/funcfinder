# Changelog

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

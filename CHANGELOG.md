# Changelog

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

---

## Планы на будущее

### v1.1.0 (планируется)
- [ ] Добавить Python support
- [ ] Добавить JavaScript/TypeScript support
- [ ] Улучшить regex паттерны для C#
- [ ] Добавить `--version` флаг
- [ ] Поддержка препроцессора C/C++

### v1.2.0 (планируется)
- [ ] Поддержка конфигурационных файлов
- [ ] Кастомные regex паттерны через CLI
- [ ] Фильтры по типу функций (public/private)
- [ ] Статистика (средний размер функций, топ-10)

### v2.0.0 (идеи)
- [ ] Интерактивный режим (TUI)
- [ ] Tree-sitter интеграция для точного парсинга
- [ ] Поддержка всех основных языков (30+)
- [ ] API server mode для интеграции в IDE

---

**Обратная связь:**
Создавайте issues или присылайте pull requests!

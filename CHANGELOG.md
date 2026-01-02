# Changelog

## v1.3.0 - 2026-01-02

### Tree Visualization & Class Hierarchy

**Новые возможности:**
- ✅ **Tree visualization mode** (`--tree`) - hierarchical display of functions and classes
- ✅ **Full tree mode** (`--tree-full`) - tree view with complete function signatures
- ✅ **Class detection** - automatic identification of classes/structs/interfaces
- ✅ **Method-class association** - methods are shown as children of their classes
- ✅ **Unicode tree rendering** - beautiful tree formatting with box-drawing characters (├──, └──, │)
- ✅ **Multi-language class support** - works with Go, C++, C#, Java, D, JS, TS, Python

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

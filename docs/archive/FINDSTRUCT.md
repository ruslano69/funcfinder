# findstruct Прототип v1.2.0

## Обзор

**findstruct** — это инструмент в экосистеме funcfinder для обнаружения сложных типов данных и их полей в исходном коде. Вместе с funcfinder (поиск функций) он покрывает две фундаментальные концепции программирования: поведение (функции) и состояние (данные).

## Архитектура v1.2.0

### Использует существующую инфраструктуру

- `internal/config.go` — загрузка конфигурации языков с поддержкой struct_patterns
- `internal/languages.json` — паттерны для всех типов данных
- `internal/sanitizer.go` — очистка от комментариев и строк
- Общий код для CLI и форматтеров

### Новые компоненты v1.2.0

- `internal/structfinder.go` — основной алгоритм поиска типов с поддержкой новых конфигураций
- `internal/struct_finder_factory.go` — фабрика с тремя типами finder'ов
- `internal/python_struct_finder.go` — Python-специфичный finder (без изменений)
- `internal/struct_formatter.go` — форматирование вывода
- `cmd/findstruct/main.go` — CLI интерфейс

### Фабричная модель

```
StructFinderFactory
├── FinderTypeBrace  → StructFinder (C++, C#, Java, D, Go, Rust, etc.)
├── FinderTypeIndent → PythonStructFinder (Python, Ruby)
└── FinderTypeHybrid → HybridStructFinder (JavaScript, TypeScript)
```

## Поддерживаемые языки (v1.2.0)

| Язык | Классы | Структуры | Интерфейсы | Enum | Type Alias | Поля |
|------|--------|-----------|------------|------|------------|------|
| C++  | ✅     | ✅        | ✅         | ✅   | —          | ✅   |
| C#   | ✅     | ✅        | ✅         | ✅   | —          | ✅   |
| Java | ✅     | —         | ✅         | ✅   | —          | ✅   |
| D    | ✅     | ✅        | ✅         | ✅   | —          | ✅   |
| Go   | —      | ✅        | ✅         | —    | ✅         | ✅   |
| C    | —      | ✅        | —          | ✅   | —          | ✅   |
| Python | ✅   | —         | ✅         | ✅   | —          | ✅   |
| JavaScript | ✅ | —         | ✅         | ✅   | ✅         | ✅   |
| TypeScript | ✅ | —         | ✅         | ✅   | ✅         | ✅   |
| Rust | —      | ✅        | ✅         | ✅   | —          | ✅   |
| Swift | ✅     | ✅        | ✅         | ✅   | —          | ✅   |
| Kotlin | ✅    | —         | ✅         | ✅   | —          | ✅   |
| PHP  | ✅     | —         | ✅         | —    | —          | ✅   |
| Ruby | ✅     | —         | —          | —    | —          | ✅   |
| Scala | ✅     | —         | ✅         | ✅   | —          | ✅   |

### Особенности языков

**Go** — поддержка `struct`, `interface` и `type` алиасов. Типы определяются ключевым словом `type`.

**C** — поддержка `struct`, `enum` и `typedef`. Включая вложенные структуры и анонимные структуры (C11).

**JavaScript/TypeScript** — гибридный finder для обработки классов с фигурными скобками и type-алиасов с их специфическим синтаксисом.

**Python** — использует **отступы** для определения границ типов. Поддерживает dataclass, NamedTuple, TypedDict, Enum, Protocol, attrs и абстрактные классы.

## Конфигурация

### Структура languages.json

```json
{
  "go": {
    "name": "Go",
    "struct_type_patterns": {
      "struct": "^\\s*type\\s+(\\w+)\\s+struct\\s*\\{",
      "interface": "^\\s*type\\s+(\\w+)\\s+interface\\s*\\{",
      "type_alias": "^\\s*type\\s+(\\w+)\\s*=\\s*"
    },
    "field_pattern": "^\\s*([a-zA-Z_][a-zA-Z0-9_]*)\\s+([a-zA-Z_][\\w\\[\\]*\\s]*)\\s*$"
  }
}
```

## Режимы работы

### --map (grep-style)
```bash
./findstruct --inp file.go --source go --map
# Point: 5-8; Rectangle: 11-15; User: 35-42;
```

### --tree (иерархия)
```bash
./findstruct --inp file.go --source go --tree
# Point (5-8) [struct]
# ├── X int: 6
# └── Y int: 7
```

### --json (для AI)
```bash
./findstruct --inp file.go --source go --map --json
```

### --type (конкретный тип)
```bash
./findstruct --inp file.go --source go --type User --json
```

## Тестовые файлы

- `test_examples/test_structs_go.go` — Go с struct, interface, type alias, вложенные структуры
- `test_examples/test_structs_c.c` — C с struct, enum, typedef, вложенные структуры, union
- `test_examples/test_structs.cpp` — C++ с классами, структурами, enum, template
- `test_examples/test_structs.java` — Java с интерфейсами, abstract class, record
- `test_examples/test_structs.cs` — C# с record, partial class, delegate
- `test_examples/test_structs.d` — D с template, union, alias this
- `test_examples/test_structs.py` — Python с dataclass, NamedTuple, TypedDict, Enum, Protocol, attrs, nested classes

## Результаты тестирования

### Go
```
Point: 5-8; Rectangle: 11-15; Shape: 18-20; Circle: 23-25;
User: 35-42; Address: 44-48; Order: 50-54; Database: 57-62;
Config: 70-75; Empty: 89-95; ComplexStruct: 109-116;
```

### C
```
Point: 7-10; Rectangle: 13-17; Flags: 20-24; Data: 27-34;
ArrayHolder: 50-54; Node: 57-61; Callback: 68-71;
Address: 82-86; Contact: 88-92; Company: 95-100;
```

## Известные ограничения

1. **Поля** — regex-подход даёт ложные срабатывания для некоторых паттернов
2. **Порядок name/type** — в некоторых случаях тип и имя поля меняются местами
3. **Generic типы** — не парсятся полностью (только имя типа)
4. **Вложенные типы** — базовая поддержка, требует доработки для некоторых языков

## Следующие шаги

### Фаза 1.1: Улучшение field extraction
- Улучшить regex паттерны для каждого языка
- Фильтровать ложные срабатывания
- Правильно определять порядок name/type

### Фаза 1.2: Связи между типами
- Определять наследование (extends, implements)
- Показывать поля, которые являются другими типами
- Строить граф зависимостей типов

### Фаза 2: Интеграция с funcfinder
- Добавить флаг `--struct` в основной бинарник
- Унифицированный вывод для функций и структур
- Общая документация

## Сборка

```bash
make build        # Сборка всех бинарников
make test         # Запуск тестов
go build ./cmd/findstruct/  # Сборка findstruct
```

## Заключение

findstruct v1.2.0 поддерживает **все 15 языков** основной утилиты funcfinder. Архитектура готова к дальнейшей интеграции с основным бинарником через флаг `--struct`.

**Ключевые достижения v1.2.0:**
- ✅ Поддержка 15 языков (C, C++, C#, Java, D, Go, Python, JS, TS, Rust, Swift, Kotlin, PHP, Ruby, Scala)
- ✅ Трёхуровневая фабрика (Brace, Indent, Hybrid)
- ✅ Гибридная поддержка для JavaScript/TypeScript
- ✅ Расширенная конфигурация в languages.json
- ✅ Тестовые файлы для Go и C

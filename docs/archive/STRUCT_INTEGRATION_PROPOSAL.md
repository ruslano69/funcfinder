# Интеграция findstruct в funcfinder через флаг --struct

## Обзор

Предложение: добавить флаг `--struct` в funcfinder для поиска структур/классов/интерфейсов, объединив возможности обеих утилит в одну.

## Текущее состояние

### Архитектурное сходство (95% общего кода)

```
┌─────────────────────────────────────────────────────────────┐
│                     ОБЩАЯ ИНФРАСТРУКТУРА                    │
├─────────────────────────────────────────────────────────────┤
│ • EnhancedSanitizer (670 строк) - обработка кода           │
│ • config.go + languages.json - конфигурация языков         │
│ • Форматтеры: --map, --tree, --json, --extract            │
│ • Фабричный паттерн для выбора парсера                     │
│ • Поддержка --lines для извлечения диапазонов              │
└─────────────────────────────────────────────────────────────┘
         ↓                                   ↓
┌──────────────────┐              ┌──────────────────┐
│   funcfinder     │              │   findstruct     │
├──────────────────┤              ├──────────────────┤
│ Поиск функций:   │              │ Поиск типов:     │
│ • func           │              │ • struct         │
│ • def            │              │ • class          │
│ • function       │              │ • interface      │
│ • method         │              │ • enum           │
│ + вложенность ✅ │              │ • union          │
└──────────────────┘              │ + поля типов ✅  │
                                  └──────────────────┘
```

### Что дают обе утилиты вместе

**Пример на Go файле:**
```bash
# funcfinder находит поведение (behavior)
$ funcfinder --source go --map --inp user.go
Area: 27-29; CreatePerson: 78-86;

# findstruct находит состояние (state)
$ findstruct --source go --map --inp user.go
User: 35-42; fields: ID, Name, Email, Address
Address: 44-48; fields: Street, City, Zip
```

**Полная картина = functions + structs** 🎯

## Предложение по интеграции

### Вариант 1: Унифицированная утилита (рекомендуется)

```bash
# Старое поведение (по умолчанию) - только функции
funcfinder --source go --map --inp file.go
Area: 27-29; CreatePerson: 78-86;

# Новый флаг --struct - только типы
funcfinder --source go --map --struct --inp file.go
User: 35-42; fields: ID, Name, Email
Address: 44-48; fields: Street, City

# Комбинированный режим --all (functions + structs)
funcfinder --source go --map --all --inp file.go
=== FUNCTIONS ===
Area: 27-29; CreatePerson: 78-86;

=== TYPES ===
User: 35-42; fields: ID, Name, Email
Address: 44-48; fields: Street, City
```

### Архитектура реализации

#### 1. Модификация main.go

```go
// cmd/funcfinder/main.go
var (
    // Существующие флаги
    source      = flag.String("source", "", "source language")
    funcStr     = flag.String("func", "", "function names (comma-separated)")
    mapMode     = flag.Bool("map", false, "map all functions")

    // НОВЫЕ ФЛАГИ
    structMode  = flag.Bool("struct", false, "find structs/classes/types instead of functions")
    typeStr     = flag.String("type", "", "type names to find (comma-separated)")
    allMode     = flag.Bool("all", false, "find both functions and structs")
)

func main() {
    flag.Parse()

    // Валидация: нельзя использовать --struct и --all одновременно
    if *structMode && *allMode {
        log.Fatal("Cannot use --struct and --all together")
    }

    // Определяем режим работы
    mode := "functions"  // по умолчанию
    if *structMode {
        mode = "structs"
    } else if *allMode {
        mode = "all"
    }

    switch mode {
    case "functions":
        processFunctions()  // существующий код

    case "structs":
        processStructs()    // новый код из findstruct

    case "all":
        processFunctions()
        fmt.Println("\n=== TYPES ===")
        processStructs()
    }
}
```

#### 2. Общий интерфейс Finder

```go
// internal/finder_interface.go (новый файл)
type CodeFinder interface {
    FindInFile(filename string) (interface{}, error)
    FindInLines(lines []string, startLine int, filename string) (interface{}, error)
}

// FunctionFinder реализует CodeFinder
type FunctionFinder struct {
    // существующий Finder
}

// StructFinder реализует CodeFinder
type StructFinder struct {
    // из findstruct
}
```

#### 3. Унифицированная фабрика

```go
// internal/unified_factory.go (новый файл)
type FinderFactory struct {
    config *LanguageConfig
}

func (f *FinderFactory) CreateFinder(mode string, args FinderArgs) CodeFinder {
    switch mode {
    case "functions":
        return NewFunctionFinder(f.config, args.FuncNames, args.MapMode, args.Extract)

    case "structs":
        return NewStructFinder(f.config, args.TypeNames, args.MapMode, args.Extract)

    default:
        log.Fatalf("Unknown mode: %s", mode)
        return nil
    }
}
```

#### 4. Унифицированный форматтер

```go
// internal/unified_formatter.go (новый файл)
func FormatResults(results interface{}, format string) string {
    switch r := results.(type) {
    case *FindResult:  // функции
        return formatFunctions(r, format)

    case *StructFindResult:  // типы
        return formatStructs(r, format)

    default:
        return ""
    }
}
```

### Конфигурация в languages.json

**Уже готово!** Все паттерны есть:
```json
{
  "go": {
    "func_pattern": "^\\s*func\\s+(\\([^)]*\\)\\s+)?(\\w+)\\s*\\(",
    "struct_type_patterns": {
      "struct": "^\\s*type\\s+(\\w+)\\s+struct\\s*\\{",
      "interface": "^\\s*type\\s+(\\w+)\\s+interface\\s*\\{"
    },
    "field_pattern": "^\\s*([a-zA-Z_][a-zA-Z0-9_]*)\\s+([a-zA-Z_][\\w\\[\\]*\\s]*)\\s*$"
  }
}
```

## Преимущества интеграции

### 1. Единая точка входа
```bash
# Вместо двух команд
funcfinder --source go --map --inp file.go
findstruct --source go --map --inp file.go

# Одна команда
funcfinder --source go --map --all --inp file.go
```

### 2. Меньше дублирования кода
- **Сейчас**: 2 CLI, 2 main.go, дублированная логика
- **После**: 1 CLI, унифицированная архитектура

### 3. Проще для пользователей
```bash
# Интуитивно понятно
funcfinder --struct         # ищем типы
funcfinder                  # ищем функции (по умолчанию)
funcfinder --all            # всё вместе
```

### 4. Комбинированный JSON вывод
```json
{
  "filename": "user.go",
  "functions": [
    {"name": "CreateUser", "start": 10, "end": 25}
  ],
  "types": [
    {
      "name": "User",
      "kind": "struct",
      "start": 5,
      "end": 9,
      "fields": [
        {"name": "ID", "type": "int", "line": 6}
      ]
    }
  ]
}
```

### 5. Для mini-SWE-agent
```bash
# Один вызов = полный контекст
funcfinder --source py --all --json --inp module.py > context.json

# AI получает:
# - Все функции (поведение)
# - Все классы (состояние)
# - Все поля (данные)
```

## План миграции

### Этап 1: Подготовка (1-2 часа)
- [x] Анализ архитектуры findstruct ✅
- [x] Документирование EnhancedSanitizer ✅
- [ ] Создание интерфейса `CodeFinder`
- [ ] Рефакторинг Finder → FunctionFinder

### Этап 2: Интеграция (2-3 часа)
- [ ] Добавление флагов `--struct`, `--type`, `--all` в main.go
- [ ] Создание `UnifiedFactory`
- [ ] Импорт кода из findstruct в funcfinder
- [ ] Унифицированный форматтер

### Этап 3: Тестирование (1-2 часа)
- [ ] Регрессионные тесты функций
- [ ] Тесты режима `--struct`
- [ ] Тесты комбинированного режима `--all`
- [ ] Интеграционные тесты для всех 15 языков

### Этап 4: Документация (1 час)
- [ ] Обновление README.md
- [ ] Примеры использования `--struct`
- [ ] Migration guide от findstruct к funcfinder

### Этап 5: Deprecation findstruct (опционально)
- [ ] Маркировка findstruct как deprecated
- [ ] Symlink: `findstruct` → `funcfinder --struct`
- [ ] Удаление дублирующегося кода

## Тестовые сценарии

### Сценарий 1: Python модуль
```bash
# До
$ funcfinder --source py --map --inp models.py
create_user: 10-25; validate_email: 30-35;

$ findstruct --source py --map --inp models.py
User: 5-8; fields: id, name, email

# После
$ funcfinder --source py --all --inp models.py
=== FUNCTIONS ===
create_user: 10-25; validate_email: 30-35;

=== TYPES ===
User: 5-8; fields: id, name, email
```

### Сценарий 2: Go package
```bash
# Только функции (по умолчанию)
$ funcfinder --source go --map --inp service.go
NewService: 10-15; Process: 20-45;

# Только типы
$ funcfinder --source go --struct --map --inp service.go
Service: 5-8; fields: db, cache
Config: 12-17; fields: Host, Port

# Всё вместе
$ funcfinder --source go --all --json --inp service.go
{
  "functions": [...],
  "types": [...]
}
```

### Сценарий 3: C++ класс
```bash
$ funcfinder --source cpp --all --tree --inp widget.cpp
FUNCTIONS:
├── Widget::Widget() (15-20)
└── Widget::paint() (25-40)

TYPES:
├── Widget (class) [10-50]
│   ├── x: int (line 12)
│   └── y: int (line 13)
└── Point (struct) [5-8]
    ├── x: double (line 6)
    └── y: double (line 7)
```

## Обратная совместимость

### Гарантии
1. **Поведение по умолчанию не меняется**: `funcfinder` без флагов работает как раньше
2. **Все существующие флаги остаются**: `--func`, `--map`, `--tree`, `--json`, `--extract`
3. **Старые скрипты работают**: никаких breaking changes

### Deprecation path
```bash
# findstruct продолжает работать (symlink)
$ findstruct --source go --map --inp file.go
Warning: findstruct is deprecated. Use 'funcfinder --struct' instead.
[результат как обычно]

# Альтернатива через funcfinder
$ funcfinder --struct --source go --map --inp file.go
[идентичный результат]
```

## Статус реализации

### Что уже готово ✅
- [x] EnhancedSanitizer - полностью рабочий (670 строк + 725 тестов)
- [x] StructFinder - протестирован на 15 языках
- [x] PythonStructFinder - indent-based detection
- [x] struct_formatter - все форматы (map/tree/json/extract)
- [x] languages.json - паттерны для всех типов
- [x] Тесты - stress tests для C++, Java, Python

### Что нужно сделать 🔨
- [ ] Добавить флаги `--struct`, `--all` в funcfinder/main.go
- [ ] Создать интерфейс `CodeFinder`
- [ ] Унифицированная фабрика
- [ ] Комбинированный JSON output
- [ ] Тесты интеграции
- [ ] Документация

### Трудоёмкость
**Оценка**: 6-8 часов чистого времени разработки
- 40% - рефакторинг и интеграция
- 30% - тестирование
- 20% - документация
- 10% - баг фиксы

## Альтернативные варианты

### Вариант 2: Раздельные утилиты (текущее состояние)
**Плюсы:**
- Разделение ответственности
- Независимые релизы

**Минусы:**
- Дублирование кода (main.go, CLI logic)
- Больше команд для пользователя
- Сложнее для mini-SWE-agent (два вызова)

### Вариант 3: Wrapper скрипт
```bash
#!/bin/bash
# funcall - вызывает funcfinder и findstruct
funcfinder "$@" && echo && findstruct "$@"
```

**Плюсы:**
- Простая реализация

**Минусы:**
- Костыль
- Нет унифицированного JSON
- Два процесса

## Вывод

**Рекомендация**: Интегрировать через флаг `--struct` (Вариант 1)

**Причины:**
1. 95% кода уже общий (EnhancedSanitizer, config, форматтеры)
2. Улучшает UX - одна команда вместо двух
3. Проще для AI-агентов (mini-SWE-agent)
4. Сохраняет обратную совместимость
5. Естественное развитие: funcfinder → code-finder

**Следующий шаг**: Прототип интеграции с тестами на Go/Python файлах

---

Документ создан: 2026-01-14
Статус: Proposal (требует обсуждения)
Автор: Claude (на основе анализа существующей архитектуры)

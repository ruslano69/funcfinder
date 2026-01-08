# Анализ вложенности: Производительность vs Читаемость

## 🎯 Критическое различие

**Не вся вложенность одинакова!** Важно различать два типа:

### 1. 🔴 ВЛОЖЕННЫЕ ЦИКЛЫ → Критично для производительности

```go
// O(n³) - КАТАСТРОФА для больших данных!
for i := 0; i < n; i++ {           // depth=1
    for j := 0; j < m; j++ {       // depth=2, loop_depth=2 ⚠️
        for k := 0; k < p; k++ {   // depth=3, loop_depth=3 🔴
            // Выполнится n*m*p раз!
            doSomething()
        }
    }
}
```

**Сложность:** O(n³) = **100³ = 1,000,000 операций**
**Приоритет:** 🔴 **КРИТИЧЕСКИЙ** - требует немедленной оптимизации

### 2. ⚠️ ВЛОЖЕННЫЕ IF → Просто читаемость

```go
// Сложно читать, но производительность O(1)
if user != nil {                   // depth=1
    if user.IsActive {             // depth=2
        if user.HasPerm("admin") { // depth=3
            if quota.Check() {     // depth=4
                doAction()         // Выполнится 1 раз
            }
        }
    }
}
```

**Сложность:** O(1) - константное время
**Приоритет:** ⚠️ **СРЕДНИЙ** - можно рефакторить при изменении

## 📊 Сравнение влияния

| Тип | Вложенность | Операции (n=1000) | Приоритет |
|-----|-------------|-------------------|-----------|
| **Циклы** | depth=2 (for+for) | 1,000,000 | 🔴 КРИТИЧЕСКИЙ |
| **Циклы** | depth=3 (for+for+for) | 1,000,000,000 | 🔴🔴 АВАРИЙНЫЙ |
| **If** | depth=4 (if+if+if+if) | 1 | ⚠️ Средний |
| **If** | depth=6 (6 уровней if) | 1 | ⚠️ Высокий |

## 🔍 Как определить вложенные циклы с complexity

### Метод 1: Визуальный анализ вывода

```bash
complexity slow_function.go -l go
```

Ищите в коде функций с высоким depth:

```go
// Если complexity показывает depth=4 для этой функции:
func ProcessMatrix(matrix [][]int) {
    for i := range matrix {              // depth=1
        for j := range matrix[i] {       // depth=2 ⚠️ LOOP NESTING
            for k := 0; k < 100; k++ {   // depth=3 🔴 O(n³)!
                compute(i, j, k)
            }
        }
    }
}
```

**Признаки вложенных циклов:**
- ✅ Функция имеет `depth >= 3`
- ✅ В коде есть `for`/`while` внутри `for`/`while`
- ✅ Код обрабатывает многомерные структуры (матрицы, графы)

### Метод 2: Поиск паттернов с grep

```bash
# Найти вложенные циклы в Go
funcfinder --inp file.go --source go --extract | \
  grep -A 10 "for.*{" | grep "for.*{"

# Найти вложенные циклы в Python
funcfinder --inp file.py --source py --extract | \
  grep -A 5 "for .* in" | grep "for .* in"

# Найти вложенные циклы в JavaScript
funcfinder --inp file.js --source js --extract | \
  grep -A 5 "for.*(" | grep "for.*("
```

### Метод 3: Используйте extract для анализа

```bash
# 1. Найти сложные функции
complexity . -l go -n 10

# 2. Извлечь код функции
funcfinder --inp slow.go --source go --func ProcessMatrix --extract

# 3. Визуально проверить на вложенные циклы
# Если есть for внутри for - это критично!
```

## 🚨 Примеры из реальной жизни

### Пример 1: КРИТИЧНО - Вложенные циклы

```python
# ❌ ПЛОХО: O(n²) - depth=3, но 2 вложенных цикла!
def find_duplicates(items):
    duplicates = []
    for i in range(len(items)):           # Outer loop
        for j in range(i + 1, len(items)): # Inner loop ⚠️
            if items[i] == items[j]:       # depth=3
                duplicates.append(items[i])
    return duplicates

# ✅ ХОРОШО: O(n) - используем set
def find_duplicates(items):
    seen = set()
    duplicates = set()
    for item in items:                     # Single loop
        if item in seen:                   # depth=2
            duplicates.add(item)
        seen.add(item)
    return list(duplicates)
```

**Результат:** 100x ускорение для 1000 элементов!

### Пример 2: НЕКРИТИЧНО - Вложенные if

```go
// ⚠️ ЧИТАЕМОСТЬ: depth=4, но O(1)
func ProcessRequest(req *Request) error {
    if req != nil {
        if req.Valid() {
            if req.User.HasPermission("write") {
                if req.Data.Validate() {
                    return saveData(req.Data)
                }
            }
        }
    }
    return ErrInvalid
}

// ✅ ЛУЧШЕ: depth=1, тот же O(1)
func ProcessRequest(req *Request) error {
    if req == nil || !req.Valid() {
        return ErrInvalid
    }
    if !req.User.HasPermission("write") {
        return ErrPermission
    }
    if !req.Data.Validate() {
        return ErrInvalidData
    }
    return saveData(req.Data)
}
```

**Результат:** Улучшена читаемость, производительность идентична.

## 💡 Стратегии оптимизации

### Для вложенных циклов (ПРИОРИТЕТ 1)

#### 1. Используйте структуры данных

```python
# ❌ O(n²) - вложенные циклы
def find_common(list1, list2):
    common = []
    for item1 in list1:
        for item2 in list2:
            if item1 == item2:
                common.append(item1)
    return common

# ✅ O(n) - set intersection
def find_common(list1, list2):
    return list(set(list1) & set(list2))
```

#### 2. Кэшируйте результаты

```go
// ❌ O(n²) - пересчитываем каждый раз
for i := range items {
    for j := range items {
        if expensive_check(items[i], items[j]) {
            process(i, j)
        }
    }
}

// ✅ O(n) - кэшируем результаты
cache := make(map[string]bool)
for i := range items {
    key := computeKey(items[i])
    if !cache[key] {
        cache[key] = expensive_check(items[i])
    }
    if cache[key] {
        process(i)
    }
}
```

#### 3. Используйте алгоритмы

```javascript
// ❌ O(n²) - сортировка пузырьком
for (let i = 0; i < arr.length; i++) {
    for (let j = 0; j < arr.length - i - 1; j++) {
        if (arr[j] > arr[j + 1]) {
            [arr[j], arr[j + 1]] = [arr[j + 1], arr[j]];
        }
    }
}

// ✅ O(n log n) - встроенная сортировка
arr.sort((a, b) => a - b);
```

### Для вложенных if (ПРИОРИТЕТ 2)

#### 1. Early returns (Guard clauses)

```go
// ❌ depth=4
func Process(data *Data) error {
    if data != nil {
        if data.Valid {
            if data.Size > 0 {
                if data.Check() {
                    return save(data)
                }
            }
        }
    }
    return ErrInvalid
}

// ✅ depth=1
func Process(data *Data) error {
    if data == nil || !data.Valid {
        return ErrInvalid
    }
    if data.Size <= 0 || !data.Check() {
        return ErrInvalid
    }
    return save(data)
}
```

#### 2. Extract methods

```python
# ❌ depth=5
def complex_validation(user, data):
    if user:
        if user.active:
            if user.has_permission('write'):
                if data:
                    if data.valid:
                        return True
    return False

# ✅ depth=2
def complex_validation(user, data):
    if not is_user_valid(user):
        return False
    return is_data_valid(data)

def is_user_valid(user):
    return user and user.active and user.has_permission('write')

def is_data_valid(data):
    return data and data.valid
```

## 📈 Workflow для анализа производительности

### Шаг 1: Найти сложные функции

```bash
complexity . -l go -n 20
```

### Шаг 2: Для каждой функции с depth >= 3

```bash
# Извлечь код
funcfinder --inp file.go --source go --func FuncName --extract > func.txt
```

### Шаг 3: Проверить на вложенные циклы

```bash
# Поиск паттернов циклов
grep -c "for" func.txt
# Если >= 2, проверьте вложенность визуально
```

### Шаг 4: Приоритизация

| Условие | Тип | Приоритет |
|---------|-----|-----------|
| depth >= 3 И есть вложенные циклы | 🔴 Performance | P0 - Сейчас |
| depth >= 4 И только if | ⚠️ Readability | P1 - При изменении |
| depth >= 6 И только if | 🔶 High | P1 - Скоро |

### Шаг 5: Измерьте улучшения

```bash
# Бенчмарк до оптимизации
go test -bench=. -benchmem

# Оптимизация

# Бенчмарк после
go test -bench=. -benchmem
```

## 🎓 Распознавание паттернов

### Вложенные циклы - Красные флаги

**Ключевые слова:**
- `for ... for`
- `while ... while`
- `forEach ... forEach`
- `for ... while` (mixed)

**Контексты:**
- Обработка матриц/2D массивов
- Поиск в неотсортированных данных
- Сравнение всех пар элементов
- Генерация комбинаций

**Типичные функции:**
- `findDuplicates`
- `processMatrix`
- `compareAll`
- `generatePairs`

### Вложенные if - Желтые флаги

**Ключевые слова:**
- `if ... if ... if`
- `switch` внутри `if`
- Длинные цепочки `else if`

**Контексты:**
- Валидация с множественными проверками
- Обработка конфигурации
- Парсинг сложных структур
- Бизнес-логика с множественными правилами

## 📚 Рекомендуемые ресурсы

### Книги
- "Introduction to Algorithms" (CLRS) - Big O notation
- "Clean Code" by Robert Martin - Reducing nesting
- "Refactoring" by Martin Fowler - Code smells

### Онлайн
- Big O Cheat Sheet: https://www.bigocheatsheet.com/
- Time Complexity: https://wiki.python.org/moin/TimeComplexity

## 🔧 Интеграция с CI/CD

```yaml
# .github/workflows/performance-check.yml
name: Performance Check

on: [push, pull_request]

jobs:
  complexity:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v2

      - name: Check for nested loops
        run: |
          # Найти функции с depth >= 3
          HIGH_COMPLEXITY=$(complexity . -l go --json | \
            jq '[.files[].functions[] | select(.max_depth >= 3)] | length')

          if [ "$HIGH_COMPLEXITY" -gt 0 ]; then
            echo "⚠️ Found $HIGH_COMPLEXITY functions with depth >= 3"
            echo "Please review for nested loops (performance issue)"

            # Extract and check for nested loops
            complexity . -l go -n 10
          fi
```

## 🎯 Checklist для Code Review

При ревью кода с высоким `complexity` depth:

### Вложенные циклы (depth >= 3 с циклами)
- [ ] Можно ли использовать map/set вместо вложенных циклов?
- [ ] Можно ли кэшировать результаты?
- [ ] Можно ли использовать встроенную функцию (sort, filter)?
- [ ] Измерена ли производительность на больших данных?
- [ ] Есть ли бенчмарки?

### Вложенные if (depth >= 4 с if)
- [ ] Можно ли использовать early returns?
- [ ] Можно ли извлечь методы?
- [ ] Можно ли упростить условия (De Morgan's laws)?
- [ ] Нужны ли все эти проверки?

---

**Вывод:** Используйте `complexity` для поиска глубокой вложенности, но **всегда проверяйте код вручную** на наличие вложенных циклов - они критичны для производительности! 🚀

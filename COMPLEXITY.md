# complexity - Анализатор когнитивной сложности

**Версия:** 1.4.0
**Языки:** 11 (Go, C, C++, C#, Java, D, JavaScript, TypeScript, Python, Rust, Swift)

## 🧠 Философия

> **Глубокая вложенность (nesting depth), а не количество веток — настоящая сложность кода.**

`complexity` измеряет когнитивную нагрузку через глубину вложенности управляющих конструкций, а не через цикломатическую сложность. Это более точно отражает сложность понимания кода человеком.

### Почему nesting depth?

**Плохой код (depth=4, но читабельный):**
```go
if err != nil {
    return err
}
if valid {
    return process()
}
return nil
```

**Плохой код (depth=4, сложный для понимания):**
```go
if user != nil {
    if user.IsActive {
        if user.HasPermission("admin") {
            if checkQuota(user) {
                // Здесь нужно удерживать в голове 4 условия
                doAction()
            }
        }
    }
}
```

`complexity` выявляет второй случай, который действительно требует рефакторинга.

## 🚀 Установка

```bash
# Собрать все утилиты (funcfinder, stat, deps, complexity)
./build.sh

# Или собрать только complexity
go build -o complexity complexity.go config.go errors.go \
  sanitizer.go finder.go python_finder.go finder_factory.go decorator.go
```

## 📖 Использование

### Базовое использование

```bash
# Анализ одного файла
complexity main.go -l go

# Анализ директории (рекурсивно)
complexity . -l go

# Автоопределение языка по расширению
complexity api.py

# Топ N самых сложных функций
complexity . -l go -n 10
```

### Флаги

```
--version           Показать версию и выйти
-l, --lang <lang>   Язык: go/c/cpp/cs/java/d/js/ts/py/rust/swift
-n <num>            Показать топ N функций по сложности
--json              Вывод в формате JSON
```

### Примеры вывода

#### Текстовый формат (по умолчанию)

```bash
complexity finder.go -l go
```

```
Average max complexity: 16.00
============================================================
Philosophy: Deep nesting (not branch count) is the real complexity
============================================================
#1 finder.go:238 findClassesWithOffset() depth=5 complexity=16 level=VERY_HIGH
  Lines: 44, File: finder.go

#2 finder.go:83 FindFunctionsInLines() depth=4 complexity=8 level=HIGH
  Lines: 104, File: finder.go

#3 finder.go:295 findClassForLine() depth=3 complexity=4 level=MODERATE
  Lines: 8, File: finder.go

#4 finder.go:45 NewFinder() depth=2 complexity=2 level=SIMPLE
  Lines: 13, File: finder.go

============================================================
Complexity distribution (by nesting depth):
SIMPLE: 3 ██████████████ (depth ≤ 2)
MODERATE: 1 ████ (depth = 3)
HIGH: 1 ██ (depth ≥ 4)
VERY_HIGH: 1 ██ (depth = 5)
============================================================
INFO: Language: Go
INFO: Files analyzed: 1
INFO: Total functions: 6
```

#### JSON формат

```bash
complexity api.py -l py --json
```

```json
{
  "files": [
    {
      "filename": "api.py",
      "functions": [
        {
          "name": "complex_handler",
          "start": 45,
          "end": 120,
          "lines": 75,
          "max_depth": 5,
          "complexity": 16,
          "level": "VERY_HIGH"
        },
        {
          "name": "simple_helper",
          "start": 125,
          "end": 135,
          "lines": 10,
          "max_depth": 1,
          "complexity": 1,
          "level": "SIMPLE"
        }
      ],
      "avg_complexity": 8.5
    }
  ],
  "summary": {
    "total_files": 1,
    "total_functions": 2,
    "simple": 1,
    "moderate": 0,
    "high": 0,
    "very_high": 1,
    "critical": 0
  }
}
```

## 📊 Уровни сложности

| Уровень | Глубина | NDC | Цвет | Рекомендация |
|---------|---------|-----|------|--------------|
| **SIMPLE** | ≤ 2 | 1-2 | 🟢 Зеленый | Отлично, продолжайте в том же духе |
| **MODERATE** | 3 | 4 | 🟡 Желтый | Приемлемо, но следите за ростом |
| **HIGH** | 4 | 8 | 🟠 Оранжевый | Рассмотрите упрощение при изменении |
| **VERY_HIGH** | 5 | 16 | 🔴 Красный | Высокий приоритет для рефакторинга |
| **CRITICAL** | ≥ 6 | ≥ 32 | 🔴 Красный жирный | Требуется немедленный рефакторинг |

### Формула расчета

```
NDC (Nesting Depth Complexity) = 2^(maxDepth - 1)
```

**Примеры:**
- depth=1 → NDC=1 (нет вложенности)
- depth=2 → NDC=2 (одно if)
- depth=3 → NDC=4 (if внутри if)
- depth=4 → NDC=8 (три уровня)
- depth=5 → NDC=16 (четыре уровня)
- depth=6 → NDC=32 (пять уровней - критично!)

## 🎯 Что анализируется

### Управляющие конструкции

Все языки поддерживают:
- `if/else/elif/elsif`
- `for/foreach/while/do-while`
- `switch/case/match`
- `try/catch/except/finally`

### Специфичные для языка

**Go:**
- `select`
- `defer` (в некоторых случаях)

**Python:**
- `with`
- `async with`
- Индентация-based блоки

**JavaScript/TypeScript:**
- Promise chains
- `async/await`

**Rust:**
- `match` arms
- `if let`
- `while let`

## 📈 Интерпретация результатов

### Хороший проект

```
✅ SIMPLE:    45 functions (depth ≤ 2)
⚠️  MODERATE:  8 functions (depth = 3)
🔶 HIGH:      2 functions (depth ≥ 4)
🔴 CRITICAL:  0 functions (depth ≥ 6)

🎯 Code Quality: ✅ Excellent - Low complexity, well-structured code
```

### Проект требует внимания

```
✅ SIMPLE:    12 functions (depth ≤ 2)
⚠️  MODERATE:  15 functions (depth = 3)
🔶 HIGH:      18 functions (depth ≥ 4)
🔴 CRITICAL:  5 functions (depth ≥ 6)

🎯 Code Quality: 🔴 Needs attention - Multiple high complexity functions
```

## 💡 Рекомендации по рефакторингу

### Техника 1: Early Returns

**До (depth=4):**
```go
func ProcessUser(user *User) error {
    if user != nil {
        if user.IsActive {
            if user.HasPermission("write") {
                if validateData(user.Data) {
                    return saveUser(user)
                }
                return ErrInvalidData
            }
            return ErrNoPermission
        }
        return ErrInactive
    }
    return ErrNilUser
}
```

**После (depth=1):**
```go
func ProcessUser(user *User) error {
    if user == nil {
        return ErrNilUser
    }
    if !user.IsActive {
        return ErrInactive
    }
    if !user.HasPermission("write") {
        return ErrNoPermission
    }
    if !validateData(user.Data) {
        return ErrInvalidData
    }
    return saveUser(user)
}
```

### Техника 2: Extraction Method

**До (depth=5):**
```python
def process_order(order):
    if order:
        if order.valid:
            if order.items:
                for item in order.items:
                    if item.in_stock:
                        if item.price > 0:
                            # Complex logic here
                            pass
```

**После (depth=3):**
```python
def process_order(order):
    if not order or not order.valid:
        return

    process_items(order.items)

def process_items(items):
    if not items:
        return

    for item in items:
        process_single_item(item)

def process_single_item(item):
    if not item.in_stock or item.price <= 0:
        return

    # Complex logic here (теперь на верхнем уровне!)
```

### Техника 3: Polymorphism

**До (depth=4):**
```typescript
function processPayment(payment: Payment) {
    if (payment.type === 'card') {
        if (payment.card.valid) {
            if (payment.amount > 0) {
                // Process card
            }
        }
    } else if (payment.type === 'paypal') {
        if (payment.paypal.token) {
            if (payment.amount > 0) {
                // Process PayPal
            }
        }
    }
}
```

**После (depth=2):**
```typescript
interface PaymentProcessor {
    process(payment: Payment): void;
}

class CardProcessor implements PaymentProcessor {
    process(payment: Payment) {
        if (!payment.card.valid || payment.amount <= 0) return;
        // Process card
    }
}

class PayPalProcessor implements PaymentProcessor {
    process(payment: Payment) {
        if (!payment.paypal.token || payment.amount <= 0) return;
        // Process PayPal
    }
}
```

## 🔧 Интеграция

### CI/CD Pipeline

```yaml
# .github/workflows/complexity.yml
name: Code Complexity Check

on: [push, pull_request]

jobs:
  complexity:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v2

      - name: Install complexity
        run: |
          go install github.com/yourusername/funcfinder/complexity@latest

      - name: Check complexity
        run: |
          complexity . -l go --json > complexity.json
          CRITICAL=$(jq '.summary.critical' complexity.json)
          if [ "$CRITICAL" -gt 0 ]; then
            echo "❌ Found $CRITICAL functions with CRITICAL complexity"
            exit 1
          fi
```

### Pre-commit Hook

```bash
#!/bin/bash
# .git/hooks/pre-commit

echo "Checking code complexity..."

CHANGED_FILES=$(git diff --cached --name-only --diff-filter=ACM | grep '\.go$')

for file in $CHANGED_FILES; do
    CRITICAL=$(complexity "$file" -l go --json 2>/dev/null | jq '.files[0].functions[] | select(.level == "CRITICAL") | .name' | wc -l)

    if [ "$CRITICAL" -gt 0 ]; then
        echo "❌ $file has $CRITICAL functions with CRITICAL complexity"
        echo "Please refactor before committing."
        exit 1
    fi
done

echo "✅ Complexity check passed"
```

### VS Code Task

```json
{
  "version": "2.0.0",
  "tasks": [
    {
      "label": "Check Complexity",
      "type": "shell",
      "command": "complexity ${file} -l go",
      "group": "test",
      "presentation": {
        "reveal": "always",
        "panel": "new"
      }
    }
  ]
}
```

## 📊 Сравнение с другими метриками

### Cyclomatic Complexity

**Цикломатическая сложность** считает количество путей выполнения:

```go
// Cyclomatic: 4 (хорошо)
// Nesting depth: 4 (плохо)
func process(a, b, c, d bool) {
    if a {
        if b {
            if c {
                if d {
                    doSomething()
                }
            }
        }
    }
}
```

**complexity** считает глубину вложенности и выявит эту проблему.

### Lines of Code (LOC)

```go
// LOC: 50 (средне)
// Nesting depth: 6 (CRITICAL)
func processOrder(order *Order) error {
    if order != nil {
        if order.Valid() {
            if len(order.Items) > 0 {
                for _, item := range order.Items {
                    if item.InStock() {
                        if item.Price > 0 {
                            // 40 lines of complex nested logic
                        }
                    }
                }
            }
        }
    }
    return nil
}
```

**complexity** покажет, что эта функция требует рефакторинга, несмотря на приемлемый LOC.

## 🎓 Best Practices

1. **Регулярно проверяйте:** Запускайте `complexity` после каждого крупного изменения
2. **Устанавливайте пороги:** В CI/CD блокируйте CRITICAL функции
3. **Фокус на HIGH+:** Начните с функций уровня HIGH и выше
4. **Рефакторьте постепенно:** Не нужно переписывать всё сразу
5. **Документируйте сложность:** Если не можете упростить, хотя бы объясните почему
6. **Используйте с другими метриками:** `complexity` + `stat` + `deps` = полная картина

## 🔬 Примеры анализа

### Пример 1: Хороший код

```bash
complexity clean_code.go -l go
```

```
Average max complexity: 2.00
============================================================
#1 ValidateInput() depth=2 complexity=2 level=SIMPLE
#2 ProcessData() depth=2 complexity=2 level=SIMPLE
#3 SaveResult() depth=1 complexity=1 level=SIMPLE
============================================================
✅ SIMPLE: 3 functions
```

### Пример 2: Код требует рефакторинга

```bash
complexity legacy_code.go -l go
```

```
Average max complexity: 24.00
============================================================
#1 LegacyHandler() depth=6 complexity=32 level=CRITICAL
  Lines: 250, File: legacy_code.go

#2 ProcessRequest() depth=5 complexity=16 level=VERY_HIGH
  Lines: 180, File: legacy_code.go

#3 ValidateData() depth=4 complexity=8 level=HIGH
  Lines: 95, File: legacy_code.go
============================================================
🔴 CRITICAL: 1 function
🔴 VERY_HIGH: 1 function
🔶 HIGH: 1 function

💡 Recommendations:
  • Priority: Review 1 critical complexity functions
  • Consider refactoring functions with depth ≥ 4
```

## 🤝 Contributing

См. [CONTRIBUTING.md](CONTRIBUTING.md) для деталей о том, как внести вклад в проект.

## 📄 License

MIT License - см. [LICENSE](LICENSE) файл для деталей.

---

**complexity** - Измеряйте когнитивную нагрузку, пишите понятный код 🧠

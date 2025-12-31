# Changelog

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

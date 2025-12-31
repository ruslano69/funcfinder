# Publishing funcfinder to GitHub

## Пошаговая инструкция

### Шаг 1: Создайте репозиторий на GitHub

1. Войдите на https://github.com
2. Нажмите "New repository"
3. Настройки:
   - **Repository name:** `funcfinder`
   - **Description:** `AI-optimized CLI tool for finding function boundaries in source code`
   - **Public** (рекомендуется для open source)
   - **НЕ** добавляйте README, .gitignore, LICENSE (уже есть)
4. Нажмите "Create repository"

### Шаг 2: Обновите module path

**ВАЖНО:** Замените `yourusername` на ваше имя пользователя GitHub!

В файле `go.mod`:
```go
module github.com/YOURUSERNAME/funcfinder
```

В файле `README.md` замените все `yourusername` на ваше имя.

### Шаг 3: Инициализируйте Git

```bash
cd funcfinder-github

# Инициализация
git init

# Добавление всех файлов
git add .

# Первый коммит
git commit -m "Initial commit: funcfinder v1.0.0

- Multi-language function boundary detection
- Support for Go, C, C++, C#, Java, D
- JSON/grep/extract output modes
- 95%+ token reduction for AI workflows
- Complete documentation and examples
- GitHub Actions CI/CD workflows"
```

### Шаг 4: Подключите remote и push

```bash
# Замените YOURUSERNAME на ваше имя!
git remote add origin https://github.com/YOURUSERNAME/funcfinder.git

# Переименуйте ветку в main (если нужно)
git branch -M main

# Первый push
git push -u origin main
```

### Шаг 5: Создайте первый релиз

#### Опция A: Через GitHub веб-интерфейс

1. На странице репозитория: Releases → "Create a new release"
2. Заполните:
   - **Tag:** `v1.0.0`
   - **Target:** `main`
   - **Title:** `v1.0.0 - Initial Release`
   - **Description:** (скопируйте из CHANGELOG.md)
3. Нажмите "Publish release"

GitHub Actions автоматически соберет бинарники для всех платформ!

#### Опция B: Через командную строку

```bash
# Создайте и push тег
git tag -a v1.0.0 -m "Release v1.0.0 - Initial Release"
git push origin v1.0.0

# GitHub Actions автоматически создаст релиз с бинарниками
```

### Шаг 6: Проверьте Actions

1. Перейдите в "Actions" на GitHub
2. Убедитесь, что CI успешно прошел (зеленая галочка)
3. Проверьте, что Release workflow создал релиз с бинарниками

### Шаг 7: Настройте GitHub Pages (опционально)

Для документации:

1. Settings → Pages
2. Source: "Deploy from a branch"
3. Branch: `main`, folder: `/docs` (или создайте)
4. Save

### Шаг 8: Добавьте Topics

На главной странице репозитория:

1. Нажмите ⚙️ Settings
2. About → Topics → Add:
   - `cli`
   - `golang`
   - `ai`
   - `code-analysis`
   - `developer-tools`
   - `token-optimization`

### Шаг 9: Настройте branch protection (опционально)

Settings → Branches → Add rule:
- Branch name pattern: `main`
- ✅ Require pull request reviews
- ✅ Require status checks to pass (CI)

### Шаг 10: Анонсируйте!

**После публикации:**

1. **Reddit:**
   - r/golang
   - r/programming
   - r/artificial

2. **Twitter/X:**
   ```
   🚀 Excited to release funcfinder v1.0.0!
   
   CLI tool that helps AI models navigate code efficiently:
   - 95%+ token reduction
   - 6 languages support
   - JSON output for AI integration
   
   Perfect for AI-driven development! 🤖
   
   https://github.com/YOURUSERNAME/funcfinder
   ```

3. **Hacker News:**
   - Show HN: [funcfinder] AI-optimized tool for code navigation

4. **Dev.to / Medium:**
   - Напишите статью о token optimization

## Структура репозитория после публикации

```
github.com/YOURUSERNAME/funcfinder/
├── .github/
│   └── workflows/
│       ├── ci.yml          ✅ Auto-testing
│       └── release.yml     ✅ Auto-releases
├── examples/
│   ├── example.go
│   └── example.c
├── .gitignore              ✅
├── LICENSE                 ✅ MIT
├── README.md               ✅ С badges
├── CONTRIBUTING.md         ✅
├── CHANGELOG.md            ✅
├── go.mod                  ✅
├── config.go
├── sanitizer.go
├── finder.go
├── formatter.go
├── main.go
└── languages.json
```

## После публикации пользователи смогут:

### Установить через go install
```bash
go install github.com/YOURUSERNAME/funcfinder@latest
```

### Скачать pre-built binary
```bash
# Linux
wget https://github.com/YOURUSERNAME/funcfinder/releases/download/v1.0.0/funcfinder-linux-amd64.tar.gz

# macOS
wget https://github.com/YOURUSERNAME/funcfinder/releases/download/v1.0.0/funcfinder-darwin-amd64.tar.gz

# Windows
# Download from Releases page
```

### Использовать в CI/CD
```yaml
- name: Install funcfinder
  run: go install github.com/YOURUSERNAME/funcfinder@latest

- name: Analyze code
  run: funcfinder --inp main.go --source go --map --json
```

## Проверка публикации

После публикации проверьте:

- [ ] Репозиторий доступен публично
- [ ] README отображается корректно
- [ ] CI проходит успешно (зеленая галочка)
- [ ] Release содержит бинарники для всех платформ
- [ ] `go install github.com/YOURUSERNAME/funcfinder@latest` работает
- [ ] Badges в README показывают правильный статус
- [ ] Topics добавлены
- [ ] LICENSE файл виден

## Обновление после публикации

### Для новых фич/фиксов:

```bash
# Внесите изменения
git add .
git commit -m "feat: add Python support"
git push origin main
```

### Для нового релиза:

```bash
# Обновите CHANGELOG.md
# Создайте тег
git tag -a v1.1.0 -m "Release v1.1.0"
git push origin v1.1.0

# GitHub Actions автоматически создаст релиз!
```

## Badges для README

После публикации обновите badges:

```markdown
[![Go Report Card](https://goreportcard.com/badge/github.com/YOURUSERNAME/funcfinder)](https://goreportcard.com/report/github.com/YOURUSERNAME/funcfinder)
[![CI](https://github.com/YOURUSERNAME/funcfinder/workflows/CI/badge.svg)](https://github.com/YOURUSERNAME/funcfinder/actions)
[![Release](https://img.shields.io/github/v/release/YOURUSERNAME/funcfinder)](https://github.com/YOURUSERNAME/funcfinder/releases)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
```

## Поддержка после публикации

Следите за:
- **Issues:** Отвечайте на вопросы и баг-репорты
- **Pull Requests:** Ревьюте и мержите контрибьюции
- **Discussions:** Отвечайте на вопросы сообщества
- **Security:** Следите за уязвимостями через Dependabot

## Продвижение

**Week 1:**
- [ ] Post на Reddit (r/golang, r/programming)
- [ ] Tweet о релизе
- [ ] Show HN на Hacker News

**Week 2:**
- [ ] Статья на Dev.to или Medium
- [ ] Добавить в awesome-go lists
- [ ] Добавить в Go Wiki

**Month 1:**
- [ ] Собрать feedback от пользователей
- [ ] Выпустить v1.1.0 с улучшениями
- [ ] Создать документацию на GitHub Wiki

---

**Готово к публикации! 🚀**

Удачи с запуском funcfinder на GitHub!

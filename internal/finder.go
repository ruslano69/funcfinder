package internal

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// FunctionBounds содержит информацию о границах функции
type FunctionBounds struct {
	Name       string
	Start      int      // Номер строки начала (1-based)
	End        int      // Номер строки конца (1-based)
	Lines      []string // Тело функции (если extractMode)
	Decorators []string // Декораторы функции (для Python, TypeScript, Java)
	ClassName  string   // Имя класса, к которому принадлежит функция
	Scope      string   // Scope функции (для совместимости)
}

// ClassBounds содержит информацию о границах класса
type ClassBounds struct {
	Name  string
	Start int
	End   int
}

// FindResult содержит результат поиска
type FindResult struct {
	Functions []FunctionBounds
	Classes   []ClassBounds
	Filename  string
}

// FunctionContext отслеживает функцию и её глубину вложенности
type FunctionContext struct {
	Func  *FunctionBounds
	Depth int
}

// Finder ищет функции в файле
type Finder struct {
	config      *LanguageConfig
	sanitizer   *Sanitizer
	funcNames   map[string]bool
	mapMode     bool
	extractMode bool
}

// NewFinder создает новый искатель функций
func NewFinder(config *LanguageConfig, funcNames []string, mapMode, extractMode, useRaw bool) *Finder {
	nameMap := make(map[string]bool)
	for _, name := range funcNames {
		nameMap[name] = true
	}
	
	return &Finder{
		config:      config,
		sanitizer:   NewSanitizer(config, useRaw),
		funcNames:   nameMap,
		mapMode:     mapMode,
		extractMode: extractMode,
	}
}

// FindFunctions ищет функции в файле
func (f *Finder) FindFunctions(filename string) (*FindResult, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()
	
	// Читаем файл построчно
	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}
	
	return f.FindFunctionsInLines(lines, 1, filename)
}

// FindFunctionsInLines ищет функции в предварительно прочитанных строках
// startLine - номер первой строки в lines (1-based) относительно оригинального файла
func (f *Finder) FindFunctionsInLines(lines []string, startLine int, filename string) (*FindResult, error) {
	lineOffset := startLine - 1 // Offset для корректировки номеров строк

	result := &FindResult{
		Filename:  filename,
		Functions: []FunctionBounds{},
		Classes:   []ClassBounds{},
	}

	// Если язык поддерживает классы, сначала находим все классы
	var classes []ClassBounds
	if f.config.HasClasses() {
		classes = f.findClassesWithOffset(lines, lineOffset)
		result.Classes = classes
	}

	// Проверяем, поддерживает ли язык вложенные функции
	if f.config.SupportsNested {
		return f.findFunctionsWithNesting(lines, lineOffset, classes, result)
	}

	// Старая логика для языков без вложенных функций
	return f.findFunctionsSimple(lines, lineOffset, classes, result)
}

// findFunctionsSimple - старая логика для языков без вложенных функций
func (f *Finder) findFunctionsSimple(lines []string, lineOffset int, classes []ClassBounds, result *FindResult) (*FindResult, error) {
	state := StateNormal
	var currentFunc *FunctionBounds
	depth := 0
	funcRegex := f.config.FuncRegex()

	for lineNum, line := range lines {
		// Очищаем строку от комментариев и литералов
		cleaned, newState := f.sanitizer.CleanLine(line, state)
		state = newState

		// Если мы внутри функции, отслеживаем баланс скобок
		if currentFunc != nil {
			if f.extractMode {
				currentFunc.Lines = append(currentFunc.Lines, line)
			}

			prevDepth := depth
			depth += CountBraces(cleaned)

			// Функция заканчивается только если мы ВЫХОДИМ из тела функции
			// (prevDepth > 0 && depth == 0), а не просто depth == 0
			// Это важно для multiline signatures с where clause в Rust:
			// fn foo<T>(...) -> Result<T>
			// where
			//     T: Deserialize,  // здесь depth == 0, но это не конец функции!
			// {
			//     ...
			// }
			if depth == 0 && prevDepth > 0 {
				// Конец функции
				currentFunc.End = lineNum + 1 + lineOffset // 1-based + offset
				result.Functions = append(result.Functions, *currentFunc)
				currentFunc = nil
			}
		} else {
			// Ищем начало новой функции
			matches := funcRegex.FindStringSubmatch(cleaned)
			if matches != nil {
				// Извлекаем имя функции
				funcName := ""
				// Для JS/TS с поддержкой arrow functions: проверяем группы 3 и 5
				if len(matches) > 5 {
					// Группа 3: function declarations (function name, function* name)
					// Группа 5: arrow functions (const name = ...)
					if matches[3] != "" {
						funcName = matches[3]
					} else if len(matches) > 5 && matches[5] != "" {
						funcName = matches[5]
					}
				}
				// Если имя еще не найдено, используем старую логику (последняя группа)
				if funcName == "" {
					for i := len(matches) - 1; i >= 1; i-- {
						if matches[i] != "" {
							funcName = matches[i]
							break
						}
					}
				}

				// Проверяем, нужно ли нам эту функцию
				if f.mapMode || f.funcNames[funcName] {
					// Определяем класс, к которому принадлежит функция
					className := ""
					if f.config.HasClasses() {
						className = f.findClassForLine(classes, lineNum+lineOffset)
					}

					// Ищем открывающую скобку
					braceCount := CountBraces(cleaned)
					if braceCount > 0 {
						// Скобка на той же строке
						currentFunc = &FunctionBounds{
							Name:      funcName,
							Start:     lineNum + 1 + lineOffset, // 1-based + offset
							Lines:     []string{},
							ClassName: className,
							Scope:     className,
						}
						if f.extractMode {
							currentFunc.Lines = append(currentFunc.Lines, line)
						}
						depth = braceCount

						if depth == 0 {
							// Функция на одной строке (маловероятно, но возможно)
							currentFunc.End = lineNum + 1 + lineOffset
							result.Functions = append(result.Functions, *currentFunc)
							currentFunc = nil
						}
					}
					// Если скобки нет, ждем её на следующих строках
					// (многострочная сигнатура)
					if braceCount == 0 {
						currentFunc = &FunctionBounds{
							Name:      funcName,
							Start:     lineNum + 1 + lineOffset,
							Lines:     []string{},
							ClassName: className,
							Scope:     className,
						}
						if f.extractMode {
							currentFunc.Lines = append(currentFunc.Lines, line)
						}
						depth = 0
					}
				}
			} else if currentFunc != nil && depth == 0 {
				// Продолжаем многострочную сигнатуру
				if f.extractMode {
					currentFunc.Lines = append(currentFunc.Lines, line)
				}
				braceCount := CountBraces(cleaned)
				if braceCount > 0 {
					depth = braceCount
					if depth == 0 {
						currentFunc.End = lineNum + 1 + lineOffset
						result.Functions = append(result.Functions, *currentFunc)
						currentFunc = nil
					}
				}
			}
		}
	}

	return result, nil
}

// findFunctionsWithNesting - новая логика для языков с вложенными функциями
func (f *Finder) findFunctionsWithNesting(lines []string, lineOffset int, classes []ClassBounds, result *FindResult) (*FindResult, error) {
	state := StateNormal
	funcStack := []*FunctionContext{} // Стек активных функций
	funcRegex := f.config.FuncRegex()

	for lineNum, line := range lines {
		// Очищаем строку от комментариев и литералов
		cleaned, newState := f.sanitizer.CleanLine(line, state)
		state = newState

		braceDelta := CountBraces(cleaned)

		// 1. Сохраняем предыдущие глубины ДО обновления
		prevDepths := make(map[int]int)
		for i, ctx := range funcStack {
			prevDepths[i] = ctx.Depth
		}

		// 2. Обновляем depth и Lines для ВСЕХ функций в стеке
		for _, ctx := range funcStack {
			if f.extractMode {
				ctx.Func.Lines = append(ctx.Func.Lines, line)
			}
			ctx.Depth += braceDelta
		}

		// 3. Ищем новые функции на ЛЮБОМ уровне вложенности
		matches := funcRegex.FindStringSubmatch(cleaned)
		if matches != nil {
			// Извлекаем имя функции
			funcName := ""
			// Для JS/TS с поддержкой arrow functions: проверяем группы 3 и 5
			if len(matches) > 5 {
				if matches[3] != "" {
					funcName = matches[3]
				} else if len(matches) > 5 && matches[5] != "" {
					funcName = matches[5]
				}
			}
			// Если имя еще не найдено, используем старую логику (последняя группа)
			if funcName == "" {
				for i := len(matches) - 1; i >= 1; i-- {
					if matches[i] != "" {
						funcName = matches[i]
						break
					}
				}
			}

			// Проверяем, нужно ли нам эту функцию
			if f.mapMode || f.funcNames[funcName] {
				// Определяем класс, к которому принадлежит функция
				className := ""
				if f.config.HasClasses() {
					className = f.findClassForLine(classes, lineNum+lineOffset)
				}

				newFunc := &FunctionBounds{
					Name:      funcName,
					Start:     lineNum + 1 + lineOffset,
					Lines:     []string{},
					ClassName: className,
					Scope:     className,
				}
				if f.extractMode {
					newFunc.Lines = append(newFunc.Lines, line)
				}

				// Добавляем новую функцию в стек
				ctx := &FunctionContext{
					Func:  newFunc,
					Depth: braceDelta,
				}
				funcStack = append(funcStack, ctx)
			}
		}

		// 4. Удаляем завершенные функции из стека (в обратном порядке)
		var newStack []*FunctionContext
		for i, ctx := range funcStack {
			prevDepth := prevDepths[i]
			// Функция завершается когда depth становится 0 после того как была > 0
			if ctx.Depth == 0 && prevDepth > 0 {
				// Конец функции
				ctx.Func.End = lineNum + 1 + lineOffset
				result.Functions = append(result.Functions, *ctx.Func)
				// Не добавляем в новый стек (удаляем)
			} else {
				// Оставляем в стеке
				newStack = append(newStack, ctx)
			}
		}
		funcStack = newStack
	}

	return result, nil
}

// ParseFuncNames разбирает строку с именами функций через запятую
func ParseFuncNames(funcStr string) []string {
	if funcStr == "" {
		return []string{}
	}
	parts := strings.Split(funcStr, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
// findClasses находит все классы в файле
func (f *Finder) findClasses(lines []string) []ClassBounds {
	return f.findClassesWithOffset(lines, 0)
}

// findClassesWithOffset находит все классы с учетом offset номеров строк
func (f *Finder) findClassesWithOffset(lines []string, lineOffset int) []ClassBounds {
	var classes []ClassBounds
	var currentClass *ClassBounds
	classRegex := f.config.ClassRegex()
	if classRegex == nil {
		return classes
	}

	state := StateNormal
	classDepth := 0

	for lineNum, line := range lines {
		cleaned, newState := f.sanitizer.CleanLine(line, state)
		state = newState

		if currentClass != nil {
			// Отслеживаем баланс скобок
			braceDelta := CountBraces(cleaned)
			classDepth += braceDelta

			// Если глубина стала 0, класс завершён
			if classDepth <= 0 {
				currentClass.End = lineNum + 1 + lineOffset
				classes = append(classes, *currentClass)
				currentClass = nil
				classDepth = 0
			}
		} else {
			// Ищем начало нового класса
			matches := classRegex.FindStringSubmatch(cleaned)
			if matches != nil {
				className := matches[1]
				// Проверяем, есть ли открывающая скобка на этой строке
				braceCount := strings.Count(cleaned, "{")
				if braceCount > 0 {
					classDepth = braceCount
				} else {
					classDepth = 0 // Ждём скобку на следующей строке
				}
				currentClass = &ClassBounds{
					Name:  className,
					Start: lineNum + 1 + lineOffset,
				}
			}
		}
	}

	// Если класс не был закрыт до конца файла
	if currentClass != nil {
		currentClass.End = len(lines) + lineOffset
		classes = append(classes, *currentClass)
	}

	return classes
}

// findClassForLine находит класс, которому принадлежит строка
func (f *Finder) findClassForLine(classes []ClassBounds, lineNum int) string {
	for _, class := range classes {
		if class.Start <= lineNum+1 && class.End >= lineNum+1 {
			return class.Name
		}
	}
	return ""
}

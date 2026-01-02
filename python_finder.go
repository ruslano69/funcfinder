package main

import (
	"fmt"
	"os"
	"strings"
)

// PythonFinder - парсер для Python с поддержкой отступов и декораторов
type PythonFinder struct {
	config          LanguageConfig
	funcNames       map[string]bool
	mode            string
	extract         bool
	decoratorWindow *DecoratorWindow
}

// NewPythonFinder создает новый парсер для Python
func NewPythonFinder(config LanguageConfig, funcNames string, mode string, extract bool) *PythonFinder {
	parsedNames := ParseFuncNames(funcNames)
	nameMap := make(map[string]bool)
	for _, name := range parsedNames {
		nameMap[name] = true
	}

	// Паттерн для Python декораторов: @decorator или @decorator(...)
	decoratorPattern := `^\s*@\w+`

	return &PythonFinder{
		config:          config,
		funcNames:       nameMap,
		mode:            mode,
		extract:         extract,
		decoratorWindow: NewDecoratorWindow(15, decoratorPattern),
	}
}

// FindFunctions находит функции в Python файле, используя анализ отступов
func (pf *PythonFinder) FindFunctions(filename string) (*FindResult, error) {
	content, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(content), "\n")
	functions := make([]FunctionBounds, 0)

	regex := pf.config.FuncRegex()
	if regex == nil {
		return nil, fmt.Errorf("failed to compile function regex")
	}

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		pf.decoratorWindow.Add(line, i+1)

		// Проверяем, начинается ли функция
		matches := regex.FindStringSubmatch(line)
		if matches == nil {
			continue
		}

		// Извлекаем имя функции (последняя непустая группа)
		funcName := ""
		for j := len(matches) - 1; j >= 1; j-- {
			if matches[j] != "" {
				funcName = matches[j]
				break
			}
		}

		if funcName == "" {
			continue
		}

		// Если в режиме поиска конкретных функций, проверяем имя
		if pf.mode != "map" && !pf.funcNames[funcName] {
			continue
		}

		// Извлекаем декораторы
		decorators, firstDecoratorLine := pf.decoratorWindow.ExtractDecorators()

		// Определяем начало функции (с учетом декораторов)
		startLine := i + 1
		if firstDecoratorLine > 0 {
			startLine = firstDecoratorLine
		}

		// Находим конец сигнатуры функции (может быть multiline)
		// Ищем строку с ':' в конце
		signatureEnd := i
		for j := i; j < len(lines); j++ {
			trimmed := strings.TrimSpace(lines[j])
			if strings.HasSuffix(trimmed, ":") {
				signatureEnd = j
				break
			}
		}

		// Находим конец функции на основе отступов
		funcIndent := GetIndentLevel(lines[signatureEnd])
		endLine := signatureEnd + 1

		// Ищем конец функции
		for j := signatureEnd + 1; j < len(lines); j++ {
			currentLine := lines[j]

			// Пропускаем пустые строки и комментарии
			if IsEmptyOrComment(currentLine, "#") {
				endLine = j + 1
				continue
			}

			currentIndent := GetIndentLevel(currentLine)

			// Если отступ вернулся к уровню функции или меньше, функция закончилась
			if currentIndent <= funcIndent {
				break
			}

			endLine = j + 1
		}

		// Собираем тело функции для extract режима
		var body []string
		if pf.extract {
			body = lines[startLine-1 : endLine]
		}

		function := FunctionBounds{
			Name:       funcName,
			Start:      startLine,
			End:        endLine,
			Lines:      body,
			Decorators: decorators,
		}

		functions = append(functions, function)

		// Пропускаем обработанные строки
		i = endLine - 1
	}

	return &FindResult{
		Functions: functions,
		Filename:  filename,
	}, nil
}

package internal

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

		// НЕ пропускаем строки если поддерживаем вложенные функции
		// Это позволяет находить вложенные функции внутри внешних
		if !pf.config.SupportsNested {
			// Пропускаем обработанные строки только для языков без вложенности
			i = endLine - 1
		}
	}

	classes := pf.findClasses(lines, functions)

	return &FindResult{
		Functions: functions,
		Classes:   classes,
		Filename:  filename,
	}, nil
}

// findClasses находит границы классов (переиспользуя PythonStructFinder)
// и проставляет ClassName у методов, лежащих внутри соответствующего класса.
// Для вложенных классов (например, `class Meta:` внутри модели Django)
// выбирается самый "узкий" охватывающий класс.
func (pf *PythonFinder) findClasses(lines []string, functions []FunctionBounds) []ClassBounds {
	structFinder := NewPythonStructFinder(pf.config, "", true, false)
	structResult, err := structFinder.FindStructuresInLines(lines, 1, "")
	if err != nil || structResult == nil {
		return nil
	}

	classes := make([]ClassBounds, 0, len(structResult.Types))
	for _, t := range structResult.Types {
		classes = append(classes, ClassBounds{Name: t.Name, Start: t.Start, End: t.End})
	}

	for i := range functions {
		fn := &functions[i]
		bestName := ""
		bestSpan := -1
		for _, c := range classes {
			if fn.Start < c.Start || fn.End > c.End {
				continue
			}
			span := c.End - c.Start
			if bestSpan == -1 || span < bestSpan {
				bestSpan = span
				bestName = c.Name
			}
		}
		fn.ClassName = bestName
	}

	return classes
}

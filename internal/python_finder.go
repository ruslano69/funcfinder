package internal

import (
	"fmt"
	"os"
	"strings"
)

// PythonFinder - парсер для Python с поддержкой отступов и декораторов
type PythonFinder struct {
	config    LanguageConfig
	funcNames map[string]bool
	mode      string
	extract   bool
}

// NewPythonFinder создает новый парсер для Python
func NewPythonFinder(config LanguageConfig, funcNames string, mode string, extract bool) *PythonFinder {
	parsedNames := ParseFuncNames(funcNames)
	nameMap := make(map[string]bool)
	for _, name := range parsedNames {
		nameMap[name] = true
	}

	return &PythonFinder{
		config:    config,
		funcNames: nameMap,
		mode:      mode,
		extract:   extract,
	}
}

// FindFunctions находит функции в Python файле, используя анализ отступов
func (pf *PythonFinder) FindFunctions(filename string) (*FindResult, error) {
	content, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(content), "\n")
	logical := pythonLogicalLines(lines)
	functions := make([]FunctionBounds, 0)

	regex := pf.config.FuncRegex()
	if regex == nil {
		return nil, fmt.Errorf("failed to compile function regex")
	}

	for i := 0; i < len(lines); i++ {
		if !logical[i].statementStart {
			continue
		}
		line := logical[i].code

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
		decorators, firstDecoratorLine := pythonDecorators(lines, logical, i)

		// Определяем начало функции (с учетом декораторов)
		startLine := i + 1
		if firstDecoratorLine > 0 {
			startLine = firstDecoratorLine
		}

		endLine := pythonDefinitionEnd(lines, logical, i)

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

	classes := pf.findClasses(lines, logical, functions)

	return &FindResult{
		Functions: functions,
		Classes:   classes,
		Filename:  filename,
	}, nil
}

// pythonLogicalLine marks physical lines on which a new statement can begin.
// Indentation on continuation lines and inside literals has no scope meaning.
type pythonLogicalLine struct {
	code           string
	statementStart bool
}

func pythonLogicalLines(lines []string) []pythonLogicalLine {
	result := make([]pythonLogicalLine, len(lines))
	state := pythonStringState{}
	depth := 0
	continued := false
	for i, line := range lines {
		inString := state.quote != 0
		code := state.cleanLine(line)
		result[i] = pythonLogicalLine{
			code:           code,
			statementStart: depth == 0 && !continued && !inString && !IsEmptyOrComment(line, "#"),
		}
		for _, ch := range code {
			switch ch {
			case '(', '[', '{':
				depth++
			case ')', ']', '}':
				if depth > 0 {
					depth--
				}
			}
		}
		continued = strings.HasSuffix(strings.TrimSpace(code), `\`)
	}
	return result
}

// Python triple strings close only with their own three-quote delimiter.
// Backslash escapes protect quotes even in raw strings. Keep scanning after a
// close, since a SQL string can be followed by arguments and closing brackets.
type pythonStringState struct {
	quote  rune
	triple bool
}

func (s *pythonStringState) cleanLine(line string) string {
	runes := []rune(line)
	code := make([]rune, len(runes))
	for i := range code {
		code[i] = ' '
	}
	for i := 0; i < len(runes); {
		ch := runes[i]
		if s.quote != 0 {
			if ch == '\\' {
				i += 2
				continue
			}
			if ch == s.quote && (!s.triple || (i+2 < len(runes) && runes[i+1] == ch && runes[i+2] == ch)) {
				if s.triple {
					i += 3
				} else {
					i++
				}
				s.quote = 0
				s.triple = false
				continue
			}
			i++
			continue
		}
		if ch == '#' {
			break
		}
		if ch == '\'' || ch == '"' {
			s.quote = ch
			s.triple = i+2 < len(runes) && runes[i+1] == ch && runes[i+2] == ch
			if s.triple {
				i += 3
			} else {
				i++
			}
			continue
		}
		code[i] = ch
		i++
	}
	return string(code)
}

// Find the suite colon outside parameter/type brackets. A trailing comment
// does not hide it, and an inline suite ends with its logical statement.
func pythonDefinitionEnd(lines []string, logical []pythonLogicalLine, start int) int {
	depth := 0
	headerEnd := start
	inline := false
header:
	for i := start; i < len(logical); i++ {
		for col, ch := range []rune(logical[i].code) {
			switch ch {
			case '(', '[', '{':
				depth++
			case ')', ']', '}':
				depth--
			case ':':
				if depth == 0 {
					headerEnd = i
					// Sanitization preserves rune positions, but hides string suites.
					tail := strings.TrimSpace(string([]rune(lines[i])[col+1:]))
					inline = tail != "" && !strings.HasPrefix(tail, "#")
					break header
				}
			}
		}
	}
	indent := GetIndentLevel(lines[start])
	end := headerEnd + 1
	for i := headerEnd + 1; i < len(lines); i++ {
		if logical[i].statementStart && (inline || GetIndentLevel(lines[i]) <= indent) {
			break
		}
		end = i + 1
	}
	return end
}

// Walk whole logical decorator statements, with no fixed look-back limit.
func pythonDecorators(lines []string, logical []pythonLogicalLine, start int) ([]string, int) {
	var decorators []string
	first := -1
	end := start - 1
	for i := start - 1; i >= 0; i-- {
		if !logical[i].statementStart {
			continue
		}
		if GetIndentLevel(lines[i]) != GetIndentLevel(lines[start]) || !strings.HasPrefix(strings.TrimSpace(logical[i].code), "@") {
			break
		}
		decorator := strings.TrimSpace(strings.Join(lines[i:end+1], "\n"))
		decorators = append([]string{decorator}, decorators...)
		first = i + 1
		end = i - 1
	}
	return decorators, first
}

// findClasses uses the same logical statements as function discovery, so text
// inside a docstring cannot become a class or prematurely close a real class.
// Для вложенных классов (например, `class Meta:` внутри модели Django)
// выбирается самый "узкий" охватывающий класс.
func (pf *PythonFinder) findClasses(lines []string, logical []pythonLogicalLine, functions []FunctionBounds) []ClassBounds {
	regex := pf.config.ClassRegex()
	if regex == nil {
		return nil
	}
	var classes []ClassBounds
	for i, line := range logical {
		if !line.statementStart {
			continue
		}
		if matches := regex.FindStringSubmatch(line.code); matches != nil {
			classes = append(classes, ClassBounds{Name: ExtractFuncName(matches), Start: i + 1, End: pythonDefinitionEnd(lines, logical, i)})
		}
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

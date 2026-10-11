package internal

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
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
	// sig is set while the signature has not opened its body yet.
	sig *sigScanner
	// sigLast is the line where that signature ends if it never gets a body.
	sigLast int
}

// sigEvent is what a line means to a signature that has not opened its body.
type sigEvent int

const (
	sigOpen         sigEvent = iota // still a signature: the body has not started
	sigBody                         // a '{' outside brackets opened the body
	sigEnd                          // a ';' outside brackets ended it: no body, so the function ends here
	sigUnterminated                 // the next function began first: no body and no ';' (Kotlin, Swift)
)

// declStart matches a line that begins a member declaration or an annotation.
// At the top level of a pending signature such a line is not part of the
// signature: it is what comes after it.
var declStart = regexp.MustCompile(`^(?:@|(?:(?:public|private|protected|internal|override|open|abstract|final|inline|suspend|const|lateinit|external|operator|infix|data|sealed|enum|companion|inner|static|mutating|lazy)\s+)*(?:val|var|fun|func|class|object|interface|typealias|init|constructor|struct|protocol|extension)\b)`)

// anonymousObject matches `object : T {`, an expression (object expression), not
// a declaration: it continues an expression body rather than ending it.
var anonymousObject = regexp.MustCompile(`^object\s*:`)

// startsDeclaration reports whether a trimmed line begins a declaration that is
// not part of the pending expression or signature above it.
func startsDeclaration(t string) bool {
	return declStart.MatchString(t) && !anonymousObject.MatchString(t)
}

// continuationStart matches a line that continues the expression before it: a
// call chain (.x), a safe call (?.x), an elvis (?:), or the else of an if.
var continuationStart = regexp.MustCompile(`^(?:\.|\?[.:]|else\b)`)

// continuesExpression reports whether the next non-blank line after line i
// continues the expression that line i closes. A block inside an expression body
// closes on its own brace, but the expression goes on: .onFailure { ... } follows.
func continuesExpression(lines []string, i int) bool {
	for j := i + 1; j < len(lines); j++ {
		if t := strings.TrimSpace(lines[j]); t != "" {
			return continuationStart.MatchString(t)
		}
	}
	return false
}

// sigScanner follows one function signature across lines.
//
// A regular expression cannot decide this on its own: a signature may span
// several lines, so the decision carries over from one line to the next. And
// ';' does not only end a declaration: it also separates the parts of a type,
// as in [u8; 4]. So brackets are counted, and '{' or ';' counts only outside
// them. A ';' that ends an expression body (`=> x;`) needs no case of its own:
// the function simply covers the lines up to it.
type sigScanner struct {
	brackets int
}

// feed reads one cleaned line of a pending signature.
func (s *sigScanner) feed(cleaned string) sigEvent {
	for i := 0; i < len(cleaned); i++ {
		switch cleaned[i] {
		case '(', '[':
			s.brackets++
		case ')', ']':
			if s.brackets > 0 {
				s.brackets--
			}
		case '{':
			if s.brackets == 0 {
				return sigBody
			}
		case ';':
			if s.brackets == 0 {
				return sigEnd
			}
		}
	}
	return sigOpen
}

// Finder ищет функции в файле
type Finder struct {
	config      *LanguageConfig
	sanitizer   *Sanitizer
	funcNames   map[string]bool
	mapMode     bool
	extractMode bool
}

// ExtractFuncName pulls the function/method name out of a FuncRegex match.
// The convention is "last non-empty group", with one special case: JS/TS's
// combined function-declaration/arrow-function pattern captures the name in
// group 3 for `function name(...)` and group 5 for `const name = (...) =>`,
// so those are checked first when both groups exist. Shared by the finder
// itself, callgraph.go (to recognize a function's own signature line and
// not record it as a call to itself) and stat.go (same reason).
func ExtractFuncName(matches []string) string {
	// Для JS/TS с поддержкой arrow functions: проверяем группы 3 и 5
	// Группа 3: function declarations (function name, function* name)
	// Группа 5: arrow functions (const name = ...)
	if len(matches) > 5 {
		if matches[3] != "" {
			return matches[3]
		}
		if matches[5] != "" {
			return matches[5]
		}
	}
	// Если имя еще не найдено, используем старую логику (последняя группа)
	for i := len(matches) - 1; i >= 1; i-- {
		if matches[i] != "" {
			return matches[i]
		}
	}
	return ""
}

// isCompleteValueAssignment — збіг гілки JS/TS `const|let|var name = (`
// (група 4), що насправді є звичайним значенням: рядок — завершена інструкція
// (`const total = (1 + 2) * 3;`) без `=>` і `function`. Така «функція» не має
// тіла й поглинула б наступну функцію до першої закритої дужки. Багаторядкова
// стрілкова функція (`const X = ({` … `}) => {`) не закінчується на `;`.
func isCompleteValueAssignment(matches []string, cleaned string) bool {
	if len(matches) <= 5 || (matches[4] != "const" && matches[4] != "let" && matches[4] != "var") {
		return false
	}
	if strings.Contains(cleaned, "=>") || strings.Contains(cleaned, "function") {
		return false
	}
	return strings.HasSuffix(strings.TrimSpace(cleaned), ";")
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
	var sig *sigScanner // the current function's signature, until its body opens
	var sigLast int     // where that signature ends if it never gets a body
	var afterBlock bool // the body closed, but the expression goes on (see continuesExpression)
	depth := 0
	funcRegex := f.config.FuncRegex()

	for lineNum, line := range lines {
		// Очищаем строку от комментариев и литералов
		cleaned, newState := f.sanitizer.CleanLine(line, state)
		state = newState

		// A header that starts while the signature still waits for its body or a
		// ';' means that signature had neither (Kotlin, Swift): it ends on its last
		// line, and the header is read as a new function below.
		if currentFunc != nil && sig != nil && sig.brackets == 0 && (funcRegex.MatchString(cleaned) || startsDeclaration(strings.TrimSpace(cleaned))) {
			currentFunc.End = sigLast
			result.Functions = append(result.Functions, *currentFunc)
			currentFunc = nil
			sig = nil
		}

		// Если мы внутри функции, отслеживаем баланс скобок
		if currentFunc != nil {
			if f.extractMode {
				currentFunc.Lines = append(currentFunc.Lines, line)
			}

			if sig != nil {
				atTop := sig.brackets == 0
				ev := sig.feed(cleaned)
				if t := strings.TrimSpace(cleaned); ev == sigOpen && t != "" && !(atTop && strings.HasPrefix(t, "}")) && !(atTop && declStart.MatchString(t)) {
					sigLast = lineNum + 1 + lineOffset
				}
				switch ev {
				case sigEnd:
					currentFunc.End = lineNum + 1 + lineOffset
					result.Functions = append(result.Functions, *currentFunc)
					currentFunc = nil
					sig = nil
					continue
				case sigBody:
					sig = nil
				}
			}

			if sig != nil {
				// Still pending: a brace here sits inside the parentheses of the signature or
				// the expression (a call argument, a default value), not in a body.
				continue
			}

			prevDepth := depth
			hasBrace := strings.Contains(cleaned, "{")
			depth += CountBraces(cleaned)

			// Функция заканчивается если мы ВЫХОДИМ из тела функции
			// (prevDepth > 0 && depth == 0) — а не просто depth == 0, что
			// важно для multiline signatures с where clause в Rust:
			// fn foo<T>(...) -> Result<T>
			// where
			//     T: Deserialize,  // здесь depth == 0, но это не конец функции!
			// {
			//     ...
			// }
			//
			// ИЛИ если открывающая и закрывающая скобка оказались на этой же
			// строке при ещё не открытой функции (prevDepth == 0, hasBrace):
			// K&R-стиль с однострочным телом сразу на строке скобки,
			//   int helper(void)
			//   { return 42; }
			// здесь depth остаётся 0 (1 открывающая + 1 закрывающая), но тело
			// уже закрылось — без hasBrace это неотличимо от "скобки ещё не
			// было вовсе", и функция осталась бы открытой до конца файла,
			// поглощая всё следующее.
			// A balanced brace while the signature is still open (a default value such
			// as remember { x } inside its parentheses) is part of the signature.
			// The function ends when its body closes, unless the expression goes on past
			// it (see continuesExpression). afterBlock waits for the first line that does
			// not continue the expression; a blank line is not that line.
			if depth == 0 && (prevDepth > 0 || hasBrace || (afterBlock && strings.TrimSpace(cleaned) != "")) {
				if continuesExpression(lines, lineNum) {
					afterBlock = true
				} else {
					// Конец функции
					currentFunc.End = lineNum + 1 + lineOffset // 1-based + offset
					result.Functions = append(result.Functions, *currentFunc)
					currentFunc = nil
					afterBlock = false
				}
			}
		} else {
			// Ищем начало новой функции
			matches := funcRegex.FindStringSubmatch(cleaned)
			if matches != nil {
				funcName := ExtractFuncName(matches)

				// Проверяем, нужно ли нам эту функцию
				if f.mapMode || f.funcNames[funcName] {
					// Определяем класс, к которому принадлежит функция
					className := ""
					if f.config.HasClasses() {
						className = f.findClassForLine(classes, lineNum+lineOffset)
					}

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

					// depth starts at 0 (nothing was open before this line);
					// apply this line's own brace delta the same way the
					// `currentFunc != nil` branch does for every later line,
					// so a single-line body on the signature line itself
					// (e.g. Rust `fn helper() -> i32 { 42 }`) closes right
					// here instead of being mistaken for "no brace yet, wait
					// for a multiline signature's opening brace".
					hasBrace := strings.Contains(cleaned, "{")
					depth = CountBraces(cleaned)

					if depth == 0 {
						// Сигнатура закрылась на этой строке: ';' — конец объявления (трейт,
						// прототип, '=> x;'), либо скобки закрыты и тело открыто и закрыто
						// тут же (однострочная функция). Скобка '{' внутри открытых круглых
						// скобок — значение по умолчанию (remember { x }), часть сигнатуры.
						sig = &sigScanner{}
						sigLast = lineNum + 1 + lineOffset
						ev := sig.feed(cleaned)
						// A header that ends in '=' starts an expression body on the next line: it is
						// not complete here, even when its braces balance.
						endsWithEq := strings.HasSuffix(strings.TrimSpace(cleaned), "=")
						if ev == sigEnd || (hasBrace && sig.brackets == 0 && !endsWithEq) {
							currentFunc.End = lineNum + 1 + lineOffset
							result.Functions = append(result.Functions, *currentFunc)
							currentFunc = nil
							sig = nil
						}
					}
					// Иначе: скобка открыта и ещё не закрыта (depth > 0), или
					// скобки нет вовсе (многострочная сигнатура) — в обоих
					// случаях функция остаётся открытой и продолжается веткой
					// `currentFunc != nil` на следующих строках.
				}
				// NB: there is no `else` here. A multiline signature opened
				// above (currentFunc set, depth == 0) is continued by the
				// `if currentFunc != nil` branch on the following lines, not
				// here — inside this `else` block currentFunc is always nil.
			}
		}
	}

	// The file ends with a function still open. A pending signature or expression
	// ends on its last line; a body whose expression was waiting for a continuation ends
	// on the last non-blank line.
	if currentFunc != nil && sig != nil {
		currentFunc.End = sigLast
		result.Functions = append(result.Functions, *currentFunc)
	} else if currentFunc != nil && afterBlock {
		for j := len(lines) - 1; j >= 0; j-- {
			if strings.TrimSpace(lines[j]) != "" {
				currentFunc.End = j + 1 + lineOffset
				break
			}
		}
		result.Functions = append(result.Functions, *currentFunc)
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

		// 0. Сигнатура без тела, начатая раньше, ждёт этой строки: она может
		// открыть тело или закончиться ';'. Если здесь начинается следующая функция,
		// сигнатура без тела и без ';' (Kotlin, Swift) кончилась на своей последней
		// строке. Решение принимается до обновления глубин (шаг 2).
		matches := funcRegex.FindStringSubmatch(cleaned)
		if matches != nil && isCompleteValueAssignment(matches, cleaned) {
			matches = nil
		}
		events := make([]sigEvent, len(funcStack))
		for i, ctx := range funcStack {
			if ctx.sig == nil {
				continue
			}
			if matches != nil && ctx.sig.brackets == 0 {
				events[i] = sigUnterminated
				ctx.sig = nil
				continue
			}
			atTop := ctx.sig.brackets == 0
			events[i] = ctx.sig.feed(cleaned)
			if events[i] != sigOpen {
				ctx.sig = nil
			} else if t := strings.TrimSpace(cleaned); t != "" && !(atTop && strings.HasPrefix(t, "}")) && !(atTop && declStart.MatchString(t)) {
				ctx.sigLast = lineNum + 1 + lineOffset
			}
		}

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

		// 3. Ищем новые функции на ЛЮБОМ уровне вложенности (matches — из шага 0)
		if matches != nil {
			funcName := ExtractFuncName(matches)

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

				// Если скобки сбалансированы на одной строке ({ ... }) — функция завершена
				if braceDelta == 0 && strings.Contains(cleaned, "{") {
					newFunc.End = lineNum + 1 + lineOffset
					result.Functions = append(result.Functions, *newFunc)
				} else if braceDelta == 0 {
					// Сигнатура без тела пока: ';' — конец объявления, функция занимает
					// эту строку; иначе ждём '{' в следующих строках.
					sig := &sigScanner{}
					if sig.feed(cleaned) == sigEnd {
						newFunc.End = lineNum + 1 + lineOffset
						result.Functions = append(result.Functions, *newFunc)
					} else {
						funcStack = append(funcStack, &FunctionContext{Func: newFunc, sig: sig, sigLast: lineNum + 1 + lineOffset})
					}
				} else {
					// Добавляем новую функцию в стек
					ctx := &FunctionContext{
						Func:  newFunc,
						Depth: braceDelta,
					}
					funcStack = append(funcStack, ctx)
				}
			}
		}

		// 4. Удаляем завершенные функции из стека (в обратном порядке)
		var newStack []*FunctionContext
		for i, ctx := range funcStack {
			ev := sigOpen
			if i < len(events) {
				ev = events[i]
			}
			if ev == sigUnterminated {
				// Следующая функция началась раньше тела или ';': сигнатура кончилась
				// на своей последней строке.
				ctx.Func.End = ctx.sigLast
				result.Functions = append(result.Functions, *ctx.Func)
				continue
			}
			if ev == sigEnd {
				// Объявление или выражение без тела: функция кончается на этой строке.
				ctx.Func.End = lineNum + 1 + lineOffset
				result.Functions = append(result.Functions, *ctx.Func)
				continue
			}
			prevDepth := prevDepths[i]
			// Функция завершается когда depth становится 0 после того как была > 0,
			// либо тело открылось и закрылось на этой же строке (`{ return 2; }`
			// под сигнатурой): глубина до строки была 0.
			if ctx.Depth == 0 && (prevDepth > 0 || ev == sigBody) {
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
			if matches != nil && len(matches) >= 2 {
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

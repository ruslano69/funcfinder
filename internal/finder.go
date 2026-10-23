package internal

import (
	"regexp"
	"strings"
)

// FunctionBounds содержит информацию о границах функции
type FunctionBounds struct {
	// SignatureLine identifies the name/parameter-list line when a language's
	// finder has that information. Zero retains regex-based signature detection.
	SignatureLine int    `json:"-"`
	SignatureName string `json:"-"` // declaration token, which differs for destructors/conversions
	Name          string
	Start         int      // Номер строки начала (1-based)
	End           int      // Номер строки конца (1-based)
	Lines         []string // Тело функции (если extractMode)
	Decorators    []string // Декораторы функции (для Python, TypeScript, Java)
	ClassName     string   // Имя класса, к которому принадлежит функция
	Scope         string   // Scope функции (для совместимости)
}

// ClassBounds содержит информацию о границах класса
type ClassBounds struct {
	Name  string
	Start int
	End   int
	Kind  string // kind of a type from the struct finder (struct, enum, dataclass...); empty for a class from the class finder
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
	// afterBlock: the body closed, but the expression goes on (see continuesExpression).
	afterBlock bool
	// bodyStarted: the signature opened its body on the current line; bodyDelta is the brace balance from
	// that brace on. The braces before it (a type argument's) are not the body's.
	bodyStarted bool
	bodyDelta   int
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
	// braceInits is set for a language whose constructors have a member-initializer list (C++).
	// After a top-level ':' the list's `member{...}` is a braced initialization, not the body: a
	// body brace follows ')' or a space. bodyAt is the position of the body's brace.
	braceInits bool
	initList   bool
	initBraces int
	typeBraces int
	bodyAt     int
}

// initPrefix reports whether a brace after c opens a braced initialization (`member{`, `member {`).
func initPrefix(c byte) bool {
	return c == '_' || c == '>' || (c >= '0' && c <= '9') || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

// prevNonSpace returns the last byte before position i that is not blank, or 0 if there is none.
func prevNonSpace(s string, i int) byte {
	for j := i - 1; j >= 0; j-- {
		if s[j] != ' ' && s[j] != '\t' {
			return s[j]
		}
	}
	return 0
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
		case ':':
			if s.braceInits && s.brackets == 0 && !s.initList && !strings.HasPrefix(cleaned[i:], "::") && (i == 0 || cleaned[i-1] != ':') {
				s.initList = true
			}
		case '{':
			if s.brackets == 0 {
				// A brace in a type argument (`React.FC<{ children: T }>`) is not the body.
				if s.typeBraces > 0 || prevNonSpace(cleaned, i) == '<' {
					s.typeBraces++
					continue
				}
				if s.initBraces > 0 || (s.initList && initPrefix(prevNonSpace(cleaned, i))) {
					s.initBraces++
					continue
				}
				s.bodyAt = i
				return sigBody
			}
		case '}':
			if s.brackets == 0 {
				if s.typeBraces > 0 {
					s.typeBraces--
				} else if s.initBraces > 0 {
					s.initBraces--
				}
			}
		case ';':
			// A ';' inside a type argument's braces separates its members, not the signature.
			if s.brackets == 0 && s.typeBraces == 0 {
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
	// dFuncHint holds, while a D file is scanned, prepareDLinesHinted's per-line result of the function pattern;
	// the nested scan skips the pattern on lines known not to match. Nil otherwise.
	dFuncHint []int8
}

// ExtractFuncName pulls the function/method name out of a FuncRegex match.
// The convention is "last non-empty group", with one special case: JS/TS's
// combined function-declaration/arrow-function pattern captures the name in
// group 3 for `function name(...)` and group 5 for `const name = (...) =>`,
// so those are checked first when both groups exist. Shared by the finder
// itself, callgraph.go (to recognize a function's own signature line and
// not record it as a call to itself) and funcstats (same reason).
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
	if strings.Contains(cleaned, "function") {
		return false
	}
	closed, arrow := assignedParen(cleaned)
	if closed {
		// The parenthesis after the '=' closes here: it is a parameter list only when an arrow or a return type
		// follows it. Otherwise the line assigns a value (const x = (a).b(), const x = (1 + 2) * 3).
		return !arrow
	}
	// The parenthesis continues on the next lines: an awaited value (const rows = (await db.query() is not an
	// arrow function, and neither is a statement that ends in ';' without an arrow.
	if strings.Contains(cleaned, "await") {
		return true
	}
	return !strings.Contains(cleaned, "=>") && strings.HasSuffix(strings.TrimSpace(cleaned), ";")
}

// declParenIndex returns the index of the parenthesis after the '=' of a declaration, after an optional
// `async` and an optional type-parameter list (`async <T>(`), or -1 when the text after the '=' does not
// open a parenthesis. The '=' of a default type argument (`<T = any>`) is not the declaration's.
func declParenIndex(cleaned string) int {
	eq := -1
	angles := 0
	for i := 0; i < len(cleaned); i++ {
		c := cleaned[i]
		if c == '<' {
			angles++
			continue
		}
		if c == '>' && angles > 0 && !(i > 0 && cleaned[i-1] == '=') {
			angles--
			continue
		}
		if c != '=' || angles > 0 {
			continue
		}
		var prev, next byte
		if i > 0 {
			prev = cleaned[i-1]
		}
		if i+1 < len(cleaned) {
			next = cleaned[i+1]
		}
		if next == '>' || next == '=' || prev == '=' || prev == '!' || prev == '<' || prev == '>' {
			continue
		}
		eq = i
		break
	}
	if eq < 0 {
		return -1
	}
	j := skipBlanks(cleaned, eq+1)
	if strings.HasPrefix(cleaned[j:], "async") {
		j = skipBlanks(cleaned, j+len("async"))
	}
	if j < len(cleaned) && cleaned[j] == '<' {
		depth := 0
		for ; j < len(cleaned); j++ {
			if cleaned[j] == '<' {
				depth++
			} else if cleaned[j] == '>' && !(j > 0 && cleaned[j-1] == '=') {
				depth--
				if depth == 0 {
					j++
					break
				}
			}
		}
		j = skipBlanks(cleaned, j)
	}
	if j < len(cleaned) && cleaned[j] == '(' {
		return j
	}
	return -1
}

// skipBlanks returns the index of the first byte at or after i that is neither a space nor a tab.
func skipBlanks(s string, i int) int {
	for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
		i++
	}
	return i
}

// openAngles counts the type arguments a line leaves open: '<' that no later '>' on the line closes.
func openAngles(cleaned string) int {
	n := 0
	for i := 0; i < len(cleaned); i++ {
		switch cleaned[i] {
		case '<':
			n++
		case '>':
			if !(i > 0 && cleaned[i-1] == '=') && n > 0 {
				n--
			}
		}
	}
	return n
}

// typedHeaderIsFunction reports whether a declaration or a member whose type-argument list runs over lines is a
// function: the list closes, and then a body '{', a signature ';', or an arrow (`= (`, `= async`, `= <`) follows.
// `const items: Item<{...}> = [...]` is a value, not a function.
func typedHeaderIsFunction(cleanedLines []string, i int) bool {
	depth := openAngles(cleanedLines[i])
	for j := i + 1; j < len(cleanedLines) && j < i+200; j++ {
		text := cleanedLines[j]
		for k := 0; k < len(text); k++ {
			switch text[k] {
			case '<':
				depth++
			case '>':
				if k > 0 && text[k-1] == '=' {
					continue
				}
				depth--
				if depth == 0 {
					return closedTypeIsFunction(text[k+1:], cleanedLines, j+1)
				}
			}
		}
	}
	return false
}

// closedTypeIsFunction looks at what follows a closed type-argument list: the rest of its line, or the next
// non-blank line when the rest is blank.
func closedTypeIsFunction(rest string, cleanedLines []string, next int) bool {
	t := strings.TrimSpace(rest)
	for j := next; t == "" && j < len(cleanedLines); j++ {
		t = strings.TrimSpace(cleanedLines[j])
	}
	if strings.HasPrefix(t, "{") || strings.HasPrefix(t, ";") {
		return true
	}
	if strings.HasPrefix(t, "=") && !strings.HasPrefix(t, "=>") {
		rhs := strings.TrimSpace(t[1:])
		return strings.HasPrefix(rhs, "(") || strings.HasPrefix(rhs, "async") || strings.HasPrefix(rhs, "<")
	}
	return false
}

// assignedParen looks at the parenthesis right after the '=' of a declaration. closed reports whether it
// closes on this line; arrow reports whether an arrow or a return type follows it. The arrow inside a value
// (`.map((r) => r.id)`) is not this parenthesis's. A parenthesis that opens another (`((`) wraps an
// expression, as in an immediately invoked function, and is a value.
func assignedParen(cleaned string) (closed, arrow bool) {
	open := declParenIndex(cleaned)
	if open < 0 {
		return false, false
	}
	rest := cleaned[open:]
	if strings.HasPrefix(strings.TrimLeft(rest[1:], " \t"), "(") {
		return true, false
	}
	depth := 0
	for i := 0; i < len(rest); i++ {
		switch rest[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				after := strings.TrimSpace(rest[i+1:])
				return true, strings.HasPrefix(after, "=>") || strings.HasPrefix(after, ":")
			}
		}
	}
	return false, false
}

// valueSpansLines reports whether the declaration on line i takes a parenthesis that closes on a later line
// and is followed by no arrow or return type: `const x = (` ... `).rows;` is a value, `const f = (` ... `) => {`
// is a function. cleanedLines are the sanitized lines of the file.
func valueSpansLines(cleanedLines []string, i int) bool {
	open := declParenIndex(cleanedLines[i])
	if open < 0 {
		return false
	}
	depth := 0
	for j := i; j < len(cleanedLines); j++ {
		text := cleanedLines[j]
		from := 0
		if j == i {
			from = open
		}
		for k := from; k < len(text); k++ {
			switch text[k] {
			case '(':
				depth++
			case ')':
				depth--
				if depth == 0 {
					after := strings.TrimSpace(text[k+1:])
					if after == "" {
						return !startsArrowOrType(cleanedLines, j+1)
					}
					return !(strings.HasPrefix(after, "=>") || strings.HasPrefix(after, ":"))
				}
			}
		}
	}
	return false
}

// startsArrowOrType reports whether the next non-blank line from index i starts with an arrow or a return type.
func startsArrowOrType(cleanedLines []string, i int) bool {
	for j := i; j < len(cleanedLines); j++ {
		if t := strings.TrimSpace(cleanedLines[j]); t != "" {
			return strings.HasPrefix(t, "=>") || strings.HasPrefix(t, ":")
		}
	}
	return false
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
	lines, err := readFileLines(filename)
	if err != nil {
		return nil, err
	}
	return f.FindFunctionsInLines(lines, 1, filename)
}

// FindFunctionsInLines ищет функции в предварительно прочитанных строках
// startLine - номер первой строки в lines (1-based) относительно оригинального файла
func (f *Finder) FindFunctionsInLines(lines []string, startLine int, filename string) (*FindResult, error) {
	if f.config.LangKey == "goose" {
		return f.findGooseFunctions(lines, startLine, filename)
	}
	if result, handled, err := f.sharedLineFunctions(lines, startLine, filename); handled {
		return result, err
	}

	var prepared []string
	var dHeaderEnds map[int]int
	var preparedFacts []LineFacts
	switch f.config.LangKey {
	case "cs":
		return f.findCSharpFunctions(lines, startLine, filename)
	case "java":
		return f.findJavaFunctions(lines, startLine, filename)
	case "js":
		return f.findJavaScriptFunctions(lines, startLine, filename)
	case "php":
		prepared = maskPHPHeredocs(lines, f.config)
		preparedFacts = f.sanitizer.CleanLinesFacts(prepared)
	case "d":
		prepared, dHeaderEnds, f.dFuncHint = prepareDLinesHinted(lines, f.config)
		defer func() { f.dFuncHint = nil }()
		preparedFacts = lexerFacts(prepared, lines)
	case "scala":
		cleaned := cleanScalaLines(lines)
		result, err := f.findFunctionsInPreparedLines(cleaned, lexerFacts(cleaned, lines), startLine, filename)
		if err != nil {
			return nil, err
		}
		for i := range result.Functions {
			fn := &result.Functions[i]
			fn.End = scalaFunctionEnd(cleaned, fn.Start-startLine, f.config) + startLine - 1
			if f.extractMode {
				fn.Lines = append([]string(nil), lines[fn.Start-startLine:fn.End-startLine+1]...)
			}
		}
		for line, code := range cleaned {
			for pos := 0; pos < len(code); pos++ {
				if !strings.ContainsRune("{;}", rune(code[pos])) {
					continue
				}
				matches := f.config.FuncRegex().FindStringSubmatch(code[pos+1:])
				if matches == nil {
					continue
				}
				name := ExtractFuncName(matches)
				if !f.mapMode && !f.funcNames[name] {
					continue
				}
				end := scalaFunctionEndAt(cleaned, line, pos+1, f.config) + startLine - 1
				className := f.findClassForLine(result.Classes, line+startLine)
				fn := FunctionBounds{Name: name, Start: line + startLine, End: end, ClassName: className, Scope: className}
				if f.extractMode {
					fn.Lines = append([]string(nil), lines[line:end-startLine+1]...)
				}
				result.Functions = append(result.Functions, fn)
			}
		}
		return result, nil
	default:
		return f.findFunctionsInPreparedLines(lines, f.sanitizer.CleanLinesFacts(lines), startLine, filename)
	}
	scanner := f
	if f.config.LangKey == "d" && !f.mapMode {
		all := *f
		all.mapMode = true // Constructor calls need the surrounding function context.
		scanner = &all
	}
	result, err := scanner.findFunctionsInPreparedLines(prepared, preparedFacts, startLine, filename)
	if err != nil {
		return nil, err
	}
	if f.config.LangKey == "d" {
		for i := range result.Functions {
			fn := &result.Functions[i]
			if end, ok := dHeaderEnds[fn.Start-startLine]; ok && fn.End < end+startLine {
				fn.End = end + startLine
			}
			// D permits local and nested types. Use the nearest class scope.
			fn.ClassName = ""
			classStart := -1
			for _, class := range result.Classes {
				if class.Start <= fn.Start && class.End >= fn.End && class.Start > classStart {
					fn.ClassName, classStart = class.Name, class.Start
				}
			}
			fn.Scope = fn.ClassName
		}
		result.Functions = filterDFunctions(result.Functions, prepared, startLine, f)
	}
	if f.extractMode {
		for i := range result.Functions {
			fn := &result.Functions[i]
			fn.Lines = append([]string(nil), lines[fn.Start-startLine:fn.End-startLine+1]...)
		}
	}
	return result, nil
}

func (f *Finder) findFunctionsInPreparedLines(lines []string, facts []LineFacts, startLine int, filename string) (*FindResult, error) {
	lineOffset := startLine - 1 // Offset для корректировки номеров строк

	result := &FindResult{
		Filename:  filename,
		Functions: []FunctionBounds{},
		Classes:   []ClassBounds{},
	}

	// Если язык поддерживает классы, сначала находим все классы
	var classes []ClassBounds
	if f.config.HasClasses() {
		if f.config.LangKey == "d" {
			classes = findDClasses(lines, lineOffset, f.config)
		} else {
			classes = f.findClassesWithOffset(facts, lineOffset)
		}
		if f.config.LangKey == "cpp" {
			classes = cppNestedClasses(facts, startLine, classes)
		}
		result.Classes = classes
	}

	// Проверяем, поддерживает ли язык вложенные функции
	// A language whose blocks end with a keyword (Ruby) has no braces to count.
	if f.config.BlockEndKeyword != "" {
		return f.findFunctionsByBlockEnd(lines, lineOffset, classes, result)
	}

	if f.config.SupportsNested {
		result, err := f.findFunctionsWithNesting(lines, facts, lineOffset, classes, result)
		if err == nil && tsUsesHeaderScanner(f.config) {
			result.Functions = f.recoverTSFunctions(lines, facts, lineOffset, classes, result.Functions)
		}
		return result, err
	}

	// Старая логика для языков без вложенных функций
	scanner := f
	if f.config.LangKey == "cpp" && !f.mapMode {
		all := *f
		all.mapMode = true // Recovery must also exclude bodies of unselected functions.
		scanner = &all
	}
	result, err := scanner.findFunctionsSimple(lines, facts, lineOffset, classes, result)
	if err == nil && f.config.LangKey == "cpp" {
		scanner.recoverCppMembers(lines, facts, startLine, result)
		if !f.mapMode {
			selected := result.Functions[:0]
			for _, fn := range result.Functions {
				if f.funcNames[fn.Name] {
					selected = append(selected, fn)
				}
			}
			result.Functions = selected
		}
	}
	return result, err
}

// findFunctionsSimple - старая логика для языков без вложенных функций
func (f *Finder) findFunctionsSimple(lines []string, facts []LineFacts, lineOffset int, classes []ClassBounds, result *FindResult) (*FindResult, error) {
	var cppExpressions map[int]map[string]bool
	if f.config.LangKey == "cpp" {
		cppExpressions = cppExpressionHeaders(facts)
	}
	var currentFunc *FunctionBounds
	var sig *sigScanner // the current function's signature, until its body opens
	var sigLast int     // where that signature ends if it never gets a body
	var afterBlock bool // the body closed, but the expression goes on (see continuesExpression)
	depth := 0
	funcRegex := f.config.FuncRegex()

	for lineNum, line := range lines {
		// comments and literals are blank in the cleaned text (LineFacts)
		cleaned := facts[lineNum].Cleaned

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

			// counted is the part of the line that the body's braces count in: a C++ signature's
			// initializer list may hold braces before the body's opening brace.
			counted := cleaned
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
					if sig.braceInits {
						counted = cleaned[sig.bodyAt:]
					}
					sig = nil
				}
			}

			if sig != nil {
				// Still pending: a brace here sits inside the parentheses of the signature or
				// the expression (a call argument, a default value), not in a body.
				continue
			}

			prevDepth := depth
			hasBrace := strings.Contains(counted, "{")
			depth += CountBraces(counted)

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
			// A class member whose signature continues on the next lines: its first line ends at
			// the opening parenthesis, and it is a member only inside a class body.
			if matches == nil && f.config.ClassMethodOpenRegex() != nil && f.config.HasClasses() && f.findClassForLine(classes, lineNum+lineOffset) != "" {
				matches = f.config.ClassMethodOpenRegex().FindStringSubmatch(cleaned)
			}
			// C++ headers the main pattern cannot take: a signature that continues on the next lines,
			// a constructor (no return type, named as its class), and a bodyless declaration. A line
			// that starts with a statement word (return, throw) is a statement, never one of them.
			hasHeaderPatterns := f.config.HeaderOpenRegex() != nil || f.config.ConstructorRegex() != nil || f.config.DeclarationRegex() != nil
			if matches == nil && hasHeaderPatterns && !startsWithReservedWord(cleaned, f.config.StatementWords) {
				cls := f.findClassForLine(classes, lineNum+lineOffset)
				if f.config.HeaderOpenRegex() != nil && openParens(cleaned) > 0 {
					matches = f.config.HeaderOpenRegex().FindStringSubmatch(cleaned)
				}
				if matches == nil && cls != "" && f.config.ConstructorRegex() != nil {
					if m := f.config.ConstructorRegex().FindStringSubmatch(cleaned); m != nil {
						if name := ExtractFuncName(m); name == cls || name == "~"+cls {
							matches = m
						}
					}
				}
				if matches == nil && f.config.DeclarationRegex() != nil {
					decl := f.config.DeclarationRegex()
					if cls != "" && f.config.MemberDeclarationRegex() != nil {
						decl = f.config.MemberDeclarationRegex()
					}
					matches = decl.FindStringSubmatch(cleaned)
				}
			}
			// A conversion operator's name is a type the compiler spells in its own canonical form, so its
			// header is not taken as a function (it would be named after the type's last word). The
			// word-named operators (operator new, operator co_await) are functions, not conversions.
			if matches != nil && f.config.ConversionOperatorRegex() != nil {
				if m := f.config.ConversionOperatorRegex().FindStringSubmatch(cleaned); m != nil && !wordOperators[strings.TrimSpace(m[1])] {
					matches = nil
				}
			}
			if matches != nil {
				funcName := ExtractFuncName(matches)
				if cppExpressions[lineNum][funcName] {
					continue
				}

				// Проверяем, нужно ли нам эту функцию
				if (f.mapMode || f.funcNames[funcName]) && !f.isKeywordName(funcName) {
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
						sig = &sigScanner{braceInits: f.config.InitializerLists}
						sigLast = lineNum + 1 + lineOffset
						ev := sig.feed(cleaned)
						// A header that ends in '=' starts an expression body on the next line: it is
						// not complete here, even when its braces balance.
						endsWithEq := strings.HasSuffix(strings.TrimSpace(cleaned), "=")
						closes := ev == sigEnd || (hasBrace && sig.brackets == 0 && !endsWithEq)
						if sig.braceInits {
							// A brace inside the parentheses (a default value `= {}`) or a braced initialization
							// (`: a{1}`) is not the body: the header closes here only when its own body does.
							closes = ev == sigEnd || (ev == sigBody && CountBraces(cleaned[sig.bodyAt:]) == 0 && !endsWithEq)
						}
						if closes {
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

// wordOperators are the C++ operators named by a word rather than by symbols.
var wordOperators = map[string]bool{"new": true, "delete": true, "co_await": true, "co_yield": true, "co_return": true}

// leadingWord matches the first identifier of a line.
var leadingWord = regexp.MustCompile(`^\s*([A-Za-z_]\w*)`)

// templatePrefix matches a `template <...>` prefix; the shortest match ends at the first '>'.
var templatePrefix = regexp.MustCompile(`^\s*template\s*<.*?>\s*`)

// startsWithReservedWord reports whether a line begins with one of the language's statement words
// (return, throw, ...): such a line is a statement, whatever its parentheses look like. A complete
// line may begin with a template prefix (`template <class T> void f(T);`), which is read past first.
func startsWithReservedWord(line string, words []string) bool {
	if openParens(line) == 0 {
		line = templatePrefix.ReplaceAllString(line, "")
	}
	m := leadingWord.FindStringSubmatch(line)
	if m == nil {
		return false
	}
	for _, w := range words {
		if w == m[1] {
			return true
		}
	}
	return false
}

// isKeywordName reports whether a match names a reserved word of the language, which
// is a statement such as if (x), not a function.
func (f *Finder) isKeywordName(name string) bool {
	if f.config.KeywordsNotNames && isExcludedWord(name, f.config.ExcludeWords) {
		return true
	}
	return isExcludedWord(name, f.config.StatementWords)
}

// findFunctionsWithNesting - новая логика для языков с вложенными функциями
func (f *Finder) findFunctionsWithNesting(lines []string, facts []LineFacts, lineOffset int, classes []ClassBounds, result *FindResult) (*FindResult, error) {
	funcStack := []*FunctionContext{} // Стек активных функций
	funcRegex := f.config.FuncRegex()
	dHeaders := dUsesHeaderScanner(f.config)             // D: the header scanner stands for funcRegex
	tsHeaders := tsUsesHeaderScanner(f.config)           // TS: likewise (ts_header.go)
	tsDeclarations := tsUsesDeclarationScanner(f.config) // TS: the declaration and declarator patterns (ts_declaration.go)
	tsDeclarators := tsUsesDeclaratorScanner(f.config)
	// The whole file, cleaned line by line: a declaration's value may span lines. The facts come from the caller.
	cleanedLines := cleanedTexts(facts)
	// typeDepth: the type-literal braces (`query<{`) still open from earlier lines; they are not code braces.
	typeDepth := 0

	for lineNum, line := range lines {
		// Очищаем строку от комментариев и литералов
		inComment := facts[lineNum].StartState == StateBlockComment // the line starts inside a block comment
		startsNormal := facts[lineNum].StartState == StateNormal    // the line starts outside any string or comment
		cleaned := cleanedLines[lineNum]

		var braceDelta int
		braceDelta, typeDepth = braceDeltaSkippingTypeLiterals(cleaned, typeDepth)

		// 0. Сигнатура без тела, начатая раньше, ждёт этой строки: она может
		// открыть тело или закончиться ';'. Если здесь начинается следующая функция,
		// сигнатура без тела и без ';' (Kotlin, Swift) кончилась на своей последней
		// строке. Решение принимается до обновления глубин (шаг 2).
		// A line that cannot hold a header is not run through the pattern (see func_requires).
		// D: the preparation pass already ran the pattern on this exact text when its hint is not unknown.
		var matches []string
		if f.config.MayStartFunction(cleaned) && !(f.dFuncHint != nil && f.dFuncHint[lineNum] == dHintNoMatch) {
			if dHeaders {
				matches = dHeaderSubmatch(cleaned) // the D pattern's result, without running it (d_header.go)
			} else if tsHeaders {
				matches = tsHeaderSubmatch(cleaned) // the TS pattern's result (ts_header.go)
			} else {
				matches = funcRegex.FindStringSubmatch(cleaned)
			}
		}
		// A method named by a string literal is read from the raw line: the sanitizer blanks
		// the literal, so the cleaned line has no name.
		if matches == nil && startsNormal && f.config.QuotedMethodRegex() != nil && f.config.MayQuoteMethod(line) {
			matches = f.config.QuotedMethodRegex().FindStringSubmatch(line)
		}
		if matches != nil && isCompleteValueAssignment(matches, cleaned) {
			matches = nil
		}
		if matches != nil && len(matches) > 5 && (matches[4] == "const" || matches[4] == "let" || matches[4] == "var") && valueSpansLines(cleanedLines, lineNum) {
			matches = nil
		}
		// A class method whose parameter list continues on the next lines: its first line is
		// the method only at the top of a class body, not inside another function.
		if matches == nil && len(funcStack) == 0 && f.config.ClassMethodOpenRegex() != nil && f.config.HasClasses() {
			if f.findClassForLine(classes, lineNum+lineOffset) != "" {
				matches = f.config.ClassMethodOpenRegex().FindStringSubmatch(cleaned)
			}
		}
		// A member signature whose parameter list continues on the next lines (`process(input: {`), inside a class
		// or an interface: its first line ends with the parenthesis still open, and the signature ends with ';'.
		if matches == nil && len(funcStack) == 0 && f.config.HeaderOpenRegex() != nil && f.config.HasClasses() && openParens(cleaned) > 0 {
			if f.findClassForLine(classes, lineNum+lineOffset) != "" {
				matches = f.config.HeaderOpenRegex().FindStringSubmatch(cleaned)
			}
		}
		// A declaration or a member whose type-argument list runs over lines (`const X: React.FC<{` ... `}> = (`,
		// `async list(): Promise<` ... `> {`): its header is a function when the list closes into a body, a
		// signature or an arrow. Its braces are type braces, so it is read by the signature scanner.
		typedOpen := false
		if matches == nil && len(funcStack) == 0 && f.config.OpenTypeHeaderRegex() != nil && openAngles(cleaned) > 0 {
			if m := f.config.OpenTypeHeaderRegex().FindStringSubmatch(cleaned); m != nil && typedHeaderIsFunction(cleanedLines, lineNum) {
				matches = m
				typedOpen = true
			}
		}
		// A bodyless declaration at the top of a scope (TypeScript's `abstract run(): void;`): a
		// method without a body ends with ';' on its own line, and is listed as a function.
		if matches == nil && len(funcStack) == 0 && f.config.DeclarationRegex() != nil && f.config.MayDeclare(cleaned) {
			if tsDeclarations {
				matches = tsDeclarationSubmatch(cleaned)
			} else {
				matches = f.config.DeclarationRegex().FindStringSubmatch(cleaned)
			}
		}
		events := make([]sigEvent, len(funcStack))
		for i, ctx := range funcStack {
			if ctx.sig == nil {
				continue
			}
			// A declaration that starts while this signature waits for its body ends it on its
			// last line, as a new function does (Kotlin, Swift: no body and no ';').
			if (matches != nil || startsDeclaration(strings.TrimSpace(cleaned))) && ctx.sig.brackets == 0 {
				events[i] = sigUnterminated
				ctx.sig = nil
				continue
			}
			atTop := ctx.sig.brackets == 0
			events[i] = ctx.sig.feed(cleaned)
			if events[i] == sigBody {
				ctx.bodyStarted = true
				ctx.bodyDelta = CountBraces(cleaned[ctx.sig.bodyAt:])
			}
			if events[i] != sigOpen {
				ctx.sig = nil
			} else if t := strings.TrimSpace(cleaned); (t != "" || (!inComment && facts[lineNum].CodeText)) && !(atTop && strings.HasPrefix(t, "}")) && !(atTop && declStart.MatchString(t)) {
				ctx.sigLast = lineNum + 1 + lineOffset
			}
		}

		// A header still open from an earlier line takes this line: a separator here is part of
		// its signature, not the start of another function.
		headerOpen := false
		for _, ctx := range funcStack {
			if ctx.sig != nil {
				headerOpen = true
			}
		}

		// The functions that start after a separator on this line (TypeScript and JavaScript).
		var inlineStarts []inlineStart
		if f.config.InlineHeaders && !headerOpen && f.config.MayStartFunction(cleaned) {
			inlineStarts = inlineHeaderStarts(cleaned, funcRegex, f.config.DeclaratorRegex(), tsHeaders, tsDeclarators)
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
			// A pending signature's braces sit inside its parentheses (a default value such
			// as context: {} ): they are not the body, so they do not change the depth.
			if ctx.sig == nil {
				if ctx.bodyStarted {
					ctx.Depth += ctx.bodyDelta
					ctx.bodyStarted = false
				} else {
					ctx.Depth += braceDelta
				}
			}
		}

		// 3. Ищем новые функции на ЛЮБОМ уровне вложенности (matches — из шага 0)
		if matches != nil {
			funcName := ExtractFuncName(matches)

			// Проверяем, нужно ли нам эту функцию
			if (f.mapMode || f.funcNames[funcName]) && !f.isKeywordName(funcName) {
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
				// Braces that balance on this line but sit inside an open parenthesis (a
				// generic argument, a call's block) do not close the header: the expression
				// goes on, so the function stays open.
				// A header that ends in '=' starts an expression body on the next line: it is not
				// complete here, even when its braces balance (as on the simple path).
				endsWithEq := strings.HasSuffix(strings.TrimSpace(cleaned), "=") || strings.HasSuffix(strings.TrimSpace(cleaned), "=>")
				probe := &sigScanner{}
				bodyOnLine := probe.feed(cleaned) == sigBody
				if typedOpen {
					sig := &sigScanner{}
					sig.feed(cleaned)
					funcStack = append(funcStack, &FunctionContext{Func: newFunc, sig: sig, sigLast: lineNum + 1 + lineOffset})
				} else if braceDelta < 0 && bodyOnLine && openParens(cleaned[:probe.bodyAt]) == 0 && !endsWithEq && closeIndex(cleaned[probe.bodyAt:], 0) >= 0 {
					// The body opens and closes on this line, and the line closes more braces after it (`}});`): what
					// follows the body belongs to the caller. A return type's braces are not this case: they balance.
					newFunc.End = lineNum + 1 + lineOffset
					result.Functions = append(result.Functions, *newFunc)
				} else if braceDelta == 0 && bodyOnLine && openParens(cleaned) == 0 && !endsWithEq {
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

		// 3b. TypeScript and JavaScript: a function can start after a separator on this line
		// (`let n=0;const f=…`, `cookie='';constructor(){`, `}, m(){`). Each is read from its own
		// start, and its body is counted from there.
		if len(inlineStarts) > 0 {
			for _, s := range inlineStarts {
				done, open := f.inlineFunction(cleaned, line, s, lineNum+lineOffset, classes)
				if done != nil {
					result.Functions = append(result.Functions, *done)
				} else if open != nil {
					funcStack = append(funcStack, open)
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
			// The function waits for the first non-blank line that does not continue an expression
			// body (see continuesExpression), as the simple path does.
			nonBlank := strings.TrimSpace(cleaned) != ""
			// The body closed somewhere on this line, and the depth left is not zero: another function
			// header opened after the close (`}; function g() {`), or an outer brace followed it (`}};`).
			// Then the function is over here. A close followed by the function's own body (`}) => {`)
			// is not, and neither is a close that returns to zero: the expression may still go on.
			closeAt := -1
			if prevDepth > 0 {
				closeAt = closeIndex(cleaned, prevDepth)
			}
			if closeAt >= 0 && ctx.Depth != 0 && (ctx.Depth < 0 || headerAfter(inlineStarts, closeAt)) {
				ctx.Func.End = lineNum + 1 + lineOffset
				result.Functions = append(result.Functions, *ctx.Func)
			} else if ctx.Depth == 0 && (prevDepth > 0 || ev == sigBody || (ctx.afterBlock && nonBlank)) {
				if body, ok := arrowBodyTail(cleaned); ok {
					ctx.sig = body
					ctx.sigLast = lineNum + 1 + lineOffset
					newStack = append(newStack, ctx)
				} else if continuesExpression(lines, lineNum) {
					ctx.afterBlock = true
					newStack = append(newStack, ctx)
				} else {
					// Конец функции
					ctx.Func.End = lineNum + 1 + lineOffset
					result.Functions = append(result.Functions, *ctx.Func)
					// Не добавляем в новый стек (удаляем)
				}
			} else {
				// Оставляем в стеке
				newStack = append(newStack, ctx)
			}
		}
		funcStack = newStack
	}

	// The file ends with functions still open. A pending signature ends on its last line;
	// an expression waiting for a continuation ends on the last non-blank line.
	for _, ctx := range funcStack {
		if ctx.sig != nil {
			ctx.Func.End = ctx.sigLast
			result.Functions = append(result.Functions, *ctx.Func)
		} else if ctx.afterBlock {
			for j := len(lines) - 1; j >= 0; j-- {
				if strings.TrimSpace(lines[j]) != "" {
					ctx.Func.End = j + 1 + lineOffset
					break
				}
			}
			result.Functions = append(result.Functions, *ctx.Func)
		}
	}

	return result, nil
}

// braceDeltaSkippingTypeLiterals returns the brace balance of a line that is not a type literal's, and the number
// of type-literal braces still open after it. A '{' that follows '<' (or sits inside such a literal) opens a type
// literal (`query<{ id: string }>(`): its braces are the type's, not the code's.
func braceDeltaSkippingTypeLiterals(cleaned string, typeDepth int) (int, int) {
	delta := 0
	for i := 0; i < len(cleaned); i++ {
		switch cleaned[i] {
		case '{':
			if typeDepth > 0 || prevNonSpace(cleaned, i) == '<' {
				typeDepth++
			} else {
				delta++
			}
		case '}':
			if typeDepth > 0 {
				typeDepth--
			} else {
				delta--
			}
		}
	}
	return delta, typeDepth
}

// closeIndex returns the byte offset of the brace that brings a cleaned line down to a depth
// of zero, for a body that was depth levels deep before the line, or -1 when the line does not.
func closeIndex(cleaned string, depth int) int {
	run := 0
	for i := 0; i < len(cleaned); i++ {
		switch cleaned[i] {
		case '{':
			run++
		case '}':
			run--
		}
		if run <= -depth {
			return i
		}
	}
	return -1
}

// headerAfter reports whether a function header starts after byte pos of the line.
func headerAfter(starts []inlineStart, pos int) bool {
	for _, s := range starts {
		if s.at > pos {
			return true
		}
	}
	return false
}

// openParens returns how many parentheses and brackets a cleaned line leaves open. Unlike a
// sigScanner it does not stop at a brace: a brace inside a type argument (React.FC<{...}>)
// is still text here, and the parentheses after it still count.
func openParens(cleaned string) int {
	n := 0
	for _, ch := range cleaned {
		switch ch {
		case '(', '[':
			n++
		case ')', ']':
			n--
		}
	}
	return n
}

// arrowBodyTailRe matches a line that ends in an arrow, optionally followed by an open
// parenthesis: the body of the arrow function starts on this line or the next.
var arrowBodyTailRe = regexp.MustCompile(`=>\s*\(?\s*$`)

// arrowBodyTail reports whether a cleaned line ends in such an arrow, and returns the
// signature scanner for the text after the last arrow, which counts its open parentheses.
func arrowBodyTail(cleaned string) (*sigScanner, bool) {
	t := strings.TrimSpace(cleaned)
	if !arrowBodyTailRe.MatchString(t) {
		return nil, false
	}
	sig := &sigScanner{}
	sig.feed(t[strings.LastIndex(t, "=>")+2:])
	return sig, true
}

// inlineHeaderSeparators are the characters after which a function header can start on a line
// that already holds code: the end of a statement, a separator, or a brace.
const inlineHeaderSeparators = ";,{}"

// inlineStart is a function header that begins after a separator on a cleaned line, at byte offset at. re reads
// the header from there: the function pattern, or for a later declarator of a const list (declarator) the
// declarator pattern.
type inlineStart struct {
	at         int
	re         *regexp.Regexp
	declarator bool
	tsScan     bool // the TS header scanner stands for re (ts_header.go)
	declScan   bool // for a declarator: tsDeclaratorSubmatch stands for re (ts_declaration.go)
}

// listDeclarationRe opens a declaration statement: the keyword that the declarators of its list follow.
var listDeclarationRe = regexp.MustCompile(`^(?:export\s+)?(?:const|let|var)\s`)

// inlineHeaderStarts returns the headers that begin after a separator on a cleaned line, in order. The start of
// the line is not among them: the caller reads it. A later declarator of a const|let|var list begins after its ','.
func inlineHeaderStarts(cleaned string, funcRegex, declRegex *regexp.Regexp, tsScan, declScan bool) []inlineStart {
	var starts []inlineStart
	for i := 0; i < len(cleaned); i++ {
		if !strings.ContainsRune(inlineHeaderSeparators, rune(cleaned[i])) {
			continue
		}
		p := i + 1
		for p < len(cleaned) && (cleaned[p] == ' ' || cleaned[p] == '\t') {
			p++
		}
		if p >= len(cleaned) {
			continue
		}
		if tsScan && tsHeaderSubmatch(cleaned[p:]) != nil || !tsScan && funcRegex.MatchString(cleaned[p:]) {
			starts = append(starts, inlineStart{at: p, re: funcRegex, tsScan: tsScan})
		} else if declRegex != nil && cleaned[i] == ',' && (declScan && tsDeclaratorSubmatch(cleaned[p:]) != nil || !declScan && declRegex.MatchString(cleaned[p:])) && listDeclarationBefore(cleaned, i) {
			starts = append(starts, inlineStart{at: p, re: declRegex, declarator: true, declScan: declScan})
		}
	}
	return starts
}

// listDeclarationBefore reports whether the ',' at comma separates the declarators of a const|let|var statement.
func listDeclarationBefore(cleaned string, comma int) bool {
	start := statementStart(cleaned, comma)
	return start >= 0 && listDeclarationRe.MatchString(strings.TrimSpace(cleaned[start:comma]))
}

// statementStart returns the offset where the statement that holds the ',' at comma starts: after the last ';' or
// '{' outside any bracket, or 0 for the line start. It returns -1 when the comma sits inside an open bracket.
func statementStart(cleaned string, comma int) int {
	depth := 0
	for j := comma - 1; j >= 0; j-- {
		switch cleaned[j] {
		case ')', ']', '}':
			depth++
		case '(', '[':
			if depth == 0 {
				return -1
			}
			depth--
		case '{':
			if depth == 0 {
				return j + 1
			}
			depth--
		case ';':
			if depth == 0 {
				return j + 1
			}
		}
	}
	return 0
}

// headerEnd reads a header outside its brackets: sigEnd for a ';' that ends it, sigBody for a
// '{' that opens its body, sigOpen when neither is on the text. It returns the offset of the
// character that decided it.
func headerEnd(text string) (sigEvent, int) {
	brackets := 0
	for i := 0; i < len(text); i++ {
		switch text[i] {
		case '(', '[':
			brackets++
		case ')', ']':
			brackets--
		case '{':
			if brackets == 0 {
				return sigBody, i
			}
		case ';':
			if brackets == 0 {
				return sigEnd, i
			}
		}
	}
	return sigOpen, -1
}

// inlineFunction reads the function whose header starts at s.at of a cleaned line, after a
// separator. It returns the function when it ends on this line, or the open frame when its
// body or its signature goes on to the next lines.
func (f *Finder) inlineFunction(cleaned, line string, s inlineStart, lineIdx int, classes []ClassBounds) (*FunctionBounds, *FunctionContext) {
	p := s.at
	var matches []string
	if s.tsScan {
		matches = tsHeaderSubmatch(cleaned[p:])
	} else if s.declScan {
		matches = tsDeclaratorSubmatch(cleaned[p:])
	} else {
		matches = s.re.FindStringSubmatch(cleaned[p:])
	}
	if matches == nil {
		return nil, nil
	}
	if s.declarator {
		// A later declarator has no keyword of its own: the one that opened its list is the keyword, so the
		// match reads as the const alternative of func_pattern (keyword in group 4, name in group 5).
		matches = []string{matches[0], "", "", "", "const", matches[1]}
	}
	// A value assigned in a block (`{const x=(await q()).rows;}`) is not a function, as on the line path.
	if isCompleteValueAssignment(matches, cleaned[p:]) {
		return nil, nil
	}
	name := ExtractFuncName(matches)
	if !(f.mapMode || f.funcNames[name]) || f.isKeywordName(name) {
		return nil, nil
	}
	className := ""
	if f.config.HasClasses() {
		className = f.findClassForLine(classes, lineIdx)
	}
	fn := &FunctionBounds{Name: name, Start: lineIdx + 1, Lines: []string{}, ClassName: className, Scope: className}
	if f.extractMode {
		fn.Lines = append(fn.Lines, line)
	}
	ev, at := headerEnd(cleaned[p:])
	switch ev {
	case sigEnd:
		fn.End = lineIdx + 1
		return fn, nil
	case sigBody:
		depth := 0
		for i := p + at; i < len(cleaned); i++ {
			switch cleaned[i] {
			case '{':
				depth++
			case '}':
				depth--
			}
			if depth == 0 {
				fn.End = lineIdx + 1
				return fn, nil
			}
		}
		return nil, &FunctionContext{Func: fn, Depth: depth}
	default:
		sig := &sigScanner{}
		sig.feed(cleaned[p:])
		return nil, &FunctionContext{Func: fn, sig: sig, sigLast: lineIdx + 1}
	}
}

// hasCodeText reports whether a raw line holds code the sanitizer blanked: a line that is
// only a string literal (a template in an expression body) is blank once cleaned, but it
// is not a comment and not empty.
func hasCodeText(line string) bool {
	t := strings.TrimSpace(line)
	return t != "" && !strings.HasPrefix(t, "//") && !strings.HasPrefix(t, "/*")
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
	return f.findClassesWithOffset(f.sanitizer.CleanLinesFacts(lines), 0)
}

// findClassesWithOffset находит все классы с учетом offset номеров строк
func (f *Finder) findClassesWithOffset(facts []LineFacts, lineOffset int) []ClassBounds {
	var classes []ClassBounds
	var currentClass *ClassBounds
	classRegex := f.config.ClassRegex()
	if classRegex == nil {
		return classes
	}

	classDepth := 0
	// classOpened: the body's '{' has been seen. A header may span lines (`export class X<` ... `> {`),
	// and the class must not end on a header line, where the depth is still zero.
	classOpened := false

	for lineNum := range facts {
		cleaned := facts[lineNum].Cleaned

		if currentClass != nil {
			// Отслеживаем баланс скобок
			if strings.Contains(cleaned, "{") {
				classOpened = true
			}
			classDepth += CountBraces(cleaned)

			// Если тело открылось и глубина стала 0, класс завершён
			if classOpened && classDepth <= 0 {
				currentClass.End = lineNum + 1 + lineOffset
				classes = append(classes, *currentClass)
				currentClass = nil
				classDepth = 0
				classOpened = false
			}
		} else {
			// Ищем начало нового класса
			var matches []string
			if f.config.MayStartType(cleaned) {
				matches = classRegex.FindStringSubmatch(cleaned)
			}
			if matches != nil && len(matches) >= 2 {
				// `class A;` is a forward declaration: it has no body, so it opens no class.
				if !strings.Contains(cleaned, "{") && strings.Contains(cleaned, ";") {
					continue
				}
				className := matches[1]
				currentClass = &ClassBounds{
					Name:  className,
					Start: lineNum + 1 + lineOffset,
				}
				classDepth = 0 // Ждём скобку на следующей строке
				// Проверяем, есть ли открывающая скобка на этой строке. The depth is net: a body that
				// opens and closes on this line (`struct A : B {};`) ends with the line, and must not
				// hold the class open over the code that follows.
				if strings.Contains(cleaned, "{") {
					classOpened = true
					classDepth = CountBraces(cleaned)
					if classDepth <= 0 {
						currentClass.End = lineNum + 1 + lineOffset
						classes = append(classes, *currentClass)
						currentClass = nil
						classDepth = 0
						classOpened = false
					}
				} else if !classHeaderContinues(facts, lineNum, cleaned) {
					// A complete header without a body (`case class Success(v: T) extends Result`) is a class of
					// one line: the next code must not be taken for its body.
					currentClass.End = lineNum + 1 + lineOffset
					classes = append(classes, *currentClass)
					currentClass = nil
					classDepth = 0
				}
			}
		}
	}

	// Если класс не был закрыт до конца файла
	if currentClass != nil {
		currentClass.End = len(facts) + lineOffset
		classes = append(classes, *currentClass)
	}

	return classes
}

// classHeaderContinues reports whether a class header with no '{' goes on: an unclosed generic list or
// parenthesis (`export class X<`, `case class X(`), or a body that starts on the next line (`class X` `{`).
func classHeaderContinues(facts []LineFacts, i int, cleaned string) bool {
	if strings.Count(cleaned, "<") > strings.Count(cleaned, ">") || openParens(cleaned) > 0 {
		return true
	}
	// A base list that goes on (`class X : public A,` or `class X extends`) continues on the next line.
	t := strings.TrimSpace(cleaned)
	if strings.HasSuffix(t, ",") || strings.HasSuffix(t, ":") || strings.HasSuffix(t, "extends") || strings.HasSuffix(t, "implements") {
		return true
	}
	for j := i + 1; j < len(facts); j++ {
		if t := strings.TrimSpace(facts[j].Cleaned); t != "" {
			// The body opens here, or a base list starts (`: public B {`, `extends B {`).
			return strings.HasPrefix(t, "{") || strings.HasPrefix(t, ":") || strings.HasPrefix(t, "extends") || strings.HasPrefix(t, "implements")
		}
	}
	return false
}

// findClassForLine находит класс, которому принадлежит строка
func (f *Finder) findClassForLine(classes []ClassBounds, lineNum int) string {
	if f.config.LangKey == "cpp" {
		name, start := "", -1
		for _, class := range classes {
			if class.Start <= lineNum+1 && class.End >= lineNum+1 && class.Start > start {
				name, start = class.Name, class.Start
			}
		}
		return name
	}
	for _, class := range classes {
		if class.Start <= lineNum+1 && class.End >= lineNum+1 {
			return class.Name
		}
	}
	return ""
}

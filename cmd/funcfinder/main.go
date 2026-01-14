package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/ruslano69/funcfinder/internal"
)

const Version = "1.5.0"

func main() {
	// Парсинг аргументов командной строки
	version := flag.Bool("version", false, "print version and exit")
	inp := flag.String("inp", "", "input file with source code")
	source := flag.String("source", "", "source language: go/c/cpp/cs/java/d/js/ts/py")

	// Function finding flags
	funcStr := flag.String("func", "", "function names to find (comma-separated)")

	// Struct finding flags (NEW)
	structMode := flag.Bool("struct", false, "find structs/classes/types instead of functions")
	typeStr := flag.String("type", "", "type names to find (comma-separated)")
	allMode := flag.Bool("all", false, "find both functions and structs")

	// Output mode flags
	mapMode := flag.Bool("map", false, "map all functions/types in file")
	treeMode := flag.Bool("tree", false, "output in tree format")
	treeFull := flag.Bool("tree-full", false, "output in tree format with signatures")
	jsonOut := flag.Bool("json", false, "output in JSON format")
	extract := flag.Bool("extract", false, "extract function/type bodies")

	// Advanced flags
	rawMode := flag.Bool("raw", false, "include raw strings in brace counting")
	linesRange := flag.String("lines", "", "extract specific line range (format: start:end, :end, start:, or single line)")

	flag.Parse()

	// Обработка флага --version
	if *version {
		internal.PrintVersion("funcfinder")
	}

	// Валидация параметров
	if *inp == "" {
		internal.FatalError("--inp parameter is required")
	}

	// --source не обязателен если используется только --lines (standalone mode)
	standaloneLines := *linesRange != "" && *source == ""

	if *source == "" && !standaloneLines {
		internal.FatalError("--source parameter is required (or use --lines alone for plain text extraction)")
	}

	// Standalone --lines mode: просто вывести строки без парсинга
	if standaloneLines {
		lineRange, err := internal.ParseLineRange(*linesRange)
		if err != nil {
			internal.FatalError("parsing line range: %v", err)
		}

		lines, startLine, err := internal.ReadFileLines(*inp, lineRange)
		if err != nil {
			internal.FatalError("reading lines: %v", err)
		}

		// JSON output или plain
		if *jsonOut {
			internal.OutputJSONLines(lines, startLine, lineRange)
		} else {
			internal.OutputPlainLines(lines, startLine)
		}
		os.Exit(0)
	}

	// Валидация режимов работы
	// Определяем главный режим: functions (по умолчанию), structs, или all
	workMode := "functions"
	if *structMode && *allMode {
		internal.FatalError("--struct and --all are mutually exclusive")
	}
	if *structMode {
		workMode = "structs"
	} else if *allMode {
		workMode = "all"
	}

	// Взаимоисключающие режимы для functions/structs
	if workMode == "functions" {
		if *funcStr == "" && !*mapMode && !*treeMode && !*treeFull {
			internal.FatalError("either --func, --map, or --tree must be specified")
		}
		if *funcStr != "" && (*mapMode || *treeMode || *treeFull) {
			internal.FatalError("--func is mutually exclusive with --map and --tree")
		}
		if *typeStr != "" {
			internal.FatalError("--type can only be used with --struct or --all")
		}
	} else if workMode == "structs" {
		if *typeStr == "" && !*mapMode && !*treeMode && !*treeFull {
			internal.FatalError("either --type, --map, or --tree must be specified with --struct")
		}
		if *typeStr != "" && (*mapMode || *treeMode || *treeFull) {
			internal.FatalError("--type is mutually exclusive with --map and --tree")
		}
		if *funcStr != "" {
			internal.FatalError("--func cannot be used with --struct")
		}
	} else if workMode == "all" {
		if !*mapMode && !*treeMode && !*treeFull && !*jsonOut {
			internal.FatalError("--all requires --map, --tree, or --json output mode")
		}
		if *funcStr != "" || *typeStr != "" {
			internal.FatalError("--func and --type cannot be used with --all (use --map instead)")
		}
	}

	if *treeMode && *treeFull {
		internal.FatalError("--tree and --tree-full are mutually exclusive")
	}

	// Загружаем конфигурацию языков
	config, err := internal.LoadConfig()
	if err != nil {
		internal.FatalError("loading config: %v", err)
	}

	// Получаем конфигурацию для выбранного языка
	langConfig, err := config.GetLanguageConfig(*source)
	if err != nil {
		internal.FatalError("%v\nSupported languages: %s", err, strings.Join(config.GetSupportedLanguages(), ", "))
	}

	// Определяем режим работы
	mode := "func"
	if *mapMode || *treeMode || *treeFull {
		mode = "map"
	}

	// Для --tree-full нужны тела функций/типов для извлечения сигнатур
	extractMode := *extract || *treeFull

	// Обработка в зависимости от workMode
	switch workMode {
	case "functions":
		processFunctions(langConfig, *funcStr, mode, extractMode, *rawMode, *inp, *linesRange, *mapMode, *treeMode, *treeFull, *jsonOut, *extract)

	case "structs":
		processStructs(langConfig, *typeStr, mode, extractMode, *inp, *linesRange, *mapMode, *treeMode, *treeFull, *jsonOut, *extract)

	case "all":
		processAll(langConfig, mode, extractMode, *rawMode, *inp, *linesRange, *mapMode, *treeMode, *treeFull, *jsonOut, *extract)
	}
}

// processFunctions обрабатывает режим поиска функций (по умолчанию)
func processFunctions(langConfig *internal.LanguageConfig, funcStr, mode string, extractMode, rawMode bool, inp, linesRange string, mapMode, treeMode, treeFull, jsonOut, extract bool) {
	// Создаем подходящий парсер в зависимости от языка
	finder := internal.CreateFinder(langConfig, funcStr, mode, extractMode, rawMode)

	var result *internal.FindResult
	var err error

	// Если указан --lines, применяем фильтр по строкам
	if linesRange != "" {
		lineRange, err := internal.ParseLineRange(linesRange)
		if err != nil {
			internal.FatalError("parsing line range: %v", err)
		}

		lines, startLine, err := internal.ReadFileLines(inp, lineRange)
		if err != nil {
			internal.FatalError("reading lines: %v", err)
		}

		// Warning: --lines may cut through function bodies
		if lineRange.End == -1 {
			internal.InfoMessage("Using --lines filter (%d:EOF). Functions outside this range will be excluded.", lineRange.Start)
		} else {
			internal.InfoMessage("Using --lines filter (%d:%d). Functions outside this range will be excluded.", lineRange.Start, lineRange.End)
		}

		// IMPORTANT: For Python need to handle it specially
		// For now we only support standard Finder with --lines
		if langConfig.IndentBased {
			internal.WarnError("--lines with Python may produce incorrect results for indent-based parsing")
		}

		// Cast to *internal.Finder to access FindFunctionsInLines
		if stdFinder, ok := finder.(*internal.Finder); ok {
			result, err = stdFinder.FindFunctionsInLines(lines, startLine, inp)
			if err != nil {
				internal.FatalError("%v", err)
			}
		} else {
			internal.FatalError("--lines is not yet supported for Python files")
		}
	} else {
		// Standard mode: read entire file
		result, err = finder.FindFunctions(inp)
		if err != nil {
			internal.FatalError("%v", err)
		}
	}

	// Если ничего не найдено
	if len(result.Functions) == 0 {
		if mapMode || treeMode || treeFull {
			internal.FatalErrorWithCode(2, "No functions found in file")
		} else {
			internal.FatalErrorWithCode(2, "Specified functions not found")
		}
	}

	// Форматируем и выводим результат
	var output string
	if extract {
		output = internal.FormatExtract(result)
	} else if jsonOut {
		output, err = internal.FormatJSON(result)
		if err != nil {
			internal.FatalError("formatting output: %v", err)
		}
	} else if treeMode {
		output = internal.FormatTreeCompact(result)
	} else if treeFull {
		output = internal.FormatTreeFull(result)
	} else {
		output = internal.FormatGrepStyle(result)
	}

	fmt.Println(output)
}

// processStructs обрабатывает режим поиска структур/классов (--struct)
func processStructs(langConfig *internal.LanguageConfig, typeStr, mode string, extractMode bool, inp, linesRange string, mapMode, treeMode, treeFull, jsonOut, extract bool) {
	// Проверяем поддержку struct patterns
	if !langConfig.HasStructSupport() {
		internal.FatalError("Language %s does not have struct/type pattern support", langConfig.Name)
	}

	// Создаем struct finder через фабрику
	factory := internal.NewStructFinderFactory()
	structFinder := factory.CreateStructFinder(langConfig, typeStr, mapMode, extractMode)

	var result *internal.StructFindResult
	var err error

	// Для struct mode пока не поддерживаем --lines
	if linesRange != "" {
		internal.FatalError("--lines is not yet supported with --struct mode")
	}

	// Находим типы в файле
	result, err = structFinder.FindStructures(inp)
	if err != nil {
		internal.FatalError("%v", err)
	}

	// Если ничего не найдено
	if len(result.Types) == 0 {
		if mapMode || treeMode || treeFull {
			internal.FatalErrorWithCode(2, "No types found in file")
		} else {
			internal.FatalErrorWithCode(2, "Specified types not found")
		}
	}

	// Форматируем и выводим результат
	var output string
	if extract {
		// Для extract режима нужны все строки файла
		allLines, _, err := internal.ReadFileLines(inp, internal.LineRange{Start: 1, End: -1})
		if err != nil {
			internal.FatalError("reading file: %v", err)
		}
		output = internal.FormatStructExtract(result, allLines)
	} else if jsonOut {
		output, err = internal.FormatStructJSON(result)
		if err != nil {
			internal.FatalError("formatting output: %v", err)
		}
	} else if treeMode || treeFull {
		output = internal.FormatStructTree(result)
	} else {
		output = internal.FormatStructMap(result)
	}

	fmt.Println(output)
}

// processAll обрабатывает комбинированный режим (--all): функции + структуры
func processAll(langConfig *internal.LanguageConfig, mode string, extractMode, rawMode bool, inp, linesRange string, mapMode, treeMode, treeFull, jsonOut, extract bool) {
	// Для --all режима пока не поддерживаем --lines
	if linesRange != "" {
		internal.FatalError("--lines is not yet supported with --all mode")
	}

	// Создаем function finder (всегда в режиме "map" для поиска всех функций)
	funcFinder := internal.CreateFinder(langConfig, "", "map", extractMode, rawMode)
	funcResult, err := funcFinder.FindFunctions(inp)
	if err != nil {
		internal.FatalError("finding functions: %v", err)
	}

	// Создаем struct finder (если язык поддерживает)
	var structResult *internal.StructFindResult
	if langConfig.HasStructSupport() {
		factory := internal.NewStructFinderFactory()
		structFinder := factory.CreateStructFinder(langConfig, "", true, extractMode) // mapMode=true для --all
		structResult, err = structFinder.FindStructures(inp)
		if err != nil {
			internal.FatalError("finding types: %v", err)
		}
	}

	// Проверяем что хоть что-то найдено
	funcCount := len(funcResult.Functions)
	typeCount := 0
	if structResult != nil {
		typeCount = len(structResult.Types)
	}

	if funcCount == 0 && typeCount == 0 {
		internal.FatalErrorWithCode(2, "No functions or types found in file")
	}

	// Форматируем и выводим результат
	if jsonOut {
		// Комбинированный JSON
		outputCombinedJSON(funcResult, structResult)
	} else if extract {
		// Комбинированный extract
		if funcCount > 0 {
			fmt.Println("=== FUNCTIONS ===")
			fmt.Println(internal.FormatExtract(funcResult))
		}
		if typeCount > 0 {
			if funcCount > 0 {
				fmt.Println()
			}
			fmt.Println("=== TYPES ===")
			// Читаем файл для extract режима
			allLines, _, err := internal.ReadFileLines(inp, internal.LineRange{Start: 1, End: -1})
			if err != nil {
				internal.FatalError("reading file: %v", err)
			}
			fmt.Println(internal.FormatStructExtract(structResult, allLines))
		}
	} else if treeMode || treeFull {
		// Комбинированный tree
		if funcCount > 0 {
			fmt.Println("=== FUNCTIONS ===")
			if treeFull {
				fmt.Println(internal.FormatTreeFull(funcResult))
			} else {
				fmt.Println(internal.FormatTreeCompact(funcResult))
			}
		}
		if typeCount > 0 {
			if funcCount > 0 {
				fmt.Println()
			}
			fmt.Println("=== TYPES ===")
			fmt.Println(internal.FormatStructTree(structResult))
		}
	} else {
		// Комбинированный map (grep-style)
		if funcCount > 0 {
			fmt.Println("=== FUNCTIONS ===")
			fmt.Println(internal.FormatGrepStyle(funcResult))
		}
		if typeCount > 0 {
			if funcCount > 0 {
				fmt.Println()
			}
			fmt.Println("=== TYPES ===")
			fmt.Println(internal.FormatStructMap(structResult))
		}
	}
}

// outputCombinedJSON выводит объединенный JSON для функций и типов
func outputCombinedJSON(funcResult *internal.FindResult, structResult *internal.StructFindResult) {
	fmt.Println("{")
	fmt.Printf("  \"filename\": %q,\n", funcResult.Filename)

	// Functions
	fmt.Println("  \"functions\": [")
	for i, fn := range funcResult.Functions {
		fmt.Printf("    {\"name\": %q, \"start\": %d, \"end\": %d}", fn.Name, fn.Start, fn.End)
		if i < len(funcResult.Functions)-1 {
			fmt.Println(",")
		} else {
			fmt.Println()
		}
	}
	fmt.Println("  ],")

	// Types
	fmt.Println("  \"types\": [")
	if structResult != nil {
		for i, typ := range structResult.Types {
			fmt.Printf("    {\"name\": %q, \"kind\": %q, \"start\": %d, \"end\": %d, \"fields\": [", typ.Name, typ.Kind, typ.Start, typ.End)
			for j, field := range typ.Fields {
				fmt.Printf("{\"name\": %q, \"type\": %q, \"line\": %d}", field.Name, field.Type, field.Line)
				if j < len(typ.Fields)-1 {
					fmt.Print(", ")
				}
			}
			fmt.Print("]}")
			if i < len(structResult.Types)-1 {
				fmt.Println(",")
			} else {
				fmt.Println()
			}
		}
	}
	fmt.Println("  ]")
	fmt.Println("}")
}

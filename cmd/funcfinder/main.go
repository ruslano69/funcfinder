package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/ruslano69/funcfinder/internal"
)

const Version = "1.6.0"

func main() {
	// Парсинг аргументов командной строки
	version := flag.Bool("version", false, "print version and exit")

	// Режим файла
	inp := flag.String("inp", "", "input file with source code")
	source := flag.String("source", "", "source language: go/c/cpp/cs/java/d/js/ts/py")

	// Режим каталога
	dir := flag.String("dir", "", "directory to scan for source files (auto-detects language by extension)")
	workers := flag.Int("workers", 0, "number of parallel workers (default: number of CPU cores)")
	recursive := flag.Bool("recursive", true, "scan directories recursively")
	noGitignore := flag.Bool("no-gitignore", false, "ignore .gitignore files")

	// Function/Type finding flags
	funcStr := flag.String("func", "", "function names to find (comma-separated)")
	structMode := flag.Bool("struct", false, "find structs/classes/types instead of functions")
	typeStr := flag.String("type", "", "type names to find (comma-separated)")
	allMode := flag.Bool("all", false, "find both functions and structs")

	// Output mode flags
	mapMode := flag.Bool("map", false, "map all functions/types in file(s)")
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

	// Валидация: либо -inp либо -dir должно быть указано
	if *inp == "" && *dir == "" {
		internal.FatalError("either --inp (single file) or --dir (directory) parameter is required")
	}

	if *inp != "" && *dir != "" {
		internal.FatalError("--inp and --dir are mutually exclusive")
	}

	// Загружаем конфигурацию языков
	config, err := internal.LoadConfig()
	if err != nil {
		internal.FatalError("loading config: %v", err)
	}

	// Режим обработки каталога
	if *dir != "" {
		handleDirectoryMode(config, *dir, *workers, *recursive, !*noGitignore, *funcStr, *mapMode, *treeMode, *treeFull, *jsonOut, *extract, *structMode, *allMode)
		return
	}

	// Режим обработки одного файла (существующая логика)
	handleFileMode(config, *inp, *source, *funcStr, *typeStr, *structMode, *allMode, *mapMode, *treeMode, *treeFull, *jsonOut, *extract, *rawMode, *linesRange)
}

func handleDirectoryMode(config internal.Config, dirPath string, workers int, recursive, useGitignore bool, funcStr string, mapMode, treeMode, treeFull, jsonOut, extract, structMode, allMode bool) {
	// Проверяем существование директории
	info, err := os.Stat(dirPath)
	if err != nil {
		if os.IsNotExist(err) {
			internal.FatalError("directory does not exist: %s", dirPath)
		}
		internal.FatalError("accessing directory: %v", err)
	}

	if !info.IsDir() {
		internal.FatalError("path is not a directory: %s", dirPath)
	}

	// Определяем режим работы
	workMode := "functions"
	if structMode && allMode {
		internal.FatalError("--struct and --all are mutually exclusive")
	}
	if structMode {
		workMode = "structs"
	} else if allMode {
		workMode = "all"
	}

	// Валидация параметров
	if funcStr == "" && !mapMode && !treeMode && !treeFull {
		internal.FatalError("either --func, --map, or --tree must be specified in directory mode")
	}

	if funcStr != "" && (mapMode || treeMode || treeFull) {
		internal.FatalError("--func is mutually exclusive with --map and --tree")
	}

	if treeMode && treeFull {
		internal.FatalError("--tree and --tree-full are mutually exclusive")
	}

	internal.InfoMessage("Scanning directory: %s (mode=%s, recursive=%v, workers=%d, gitignore=%v)", dirPath, workMode, recursive, workers, useGitignore)

	// Создаем процессор директорий
	processor := internal.NewDirProcessor(config, workers, recursive, useGitignore, workMode)

	// Обрабатываем директорию
	results, err := processor.ProcessDirectory(dirPath)
	if err != nil {
		internal.FatalError("processing directory: %v", err)
	}

	// Выводим результат
	output := internal.AggregateDirResults(results, jsonOut, treeMode, treeFull)
	fmt.Println(output)

	// Статистика
	totalFuncs := 0
	totalClasses := 0
	totalFiles := len(results)
	for _, r := range results {
		totalFuncs += len(r.Functions)
		totalClasses += len(r.Classes)
	}

	if workMode == "all" || workMode == "structs" {
		internal.InfoMessage("Processed %d files, found %d functions, %d classes/types", totalFiles, totalFuncs, totalClasses)
	} else {
		internal.InfoMessage("Processed %d files, found %d functions", totalFiles, totalFuncs)
	}
}

func handleFileMode(config internal.Config, inp, source, funcStr, typeStr string, structMode, allMode, mapMode, treeMode, treeFull, jsonOut, extract, rawMode bool, linesRange string) {
	// --source не обязателен если используется только --lines (standalone mode)
	standaloneLines := linesRange != "" && source == ""

	if source == "" && !standaloneLines {
		internal.FatalError("--source parameter is required (or use --lines alone for plain text extraction)")
	}

	// Standalone --lines mode: просто вывести строки без парсинга
	if standaloneLines {
		lineRange, err := internal.ParseLineRange(linesRange)
		if err != nil {
			internal.FatalError("parsing line range: %v", err)
		}

		lines, startLine, err := internal.ReadFileLines(inp, lineRange)
		if err != nil {
			internal.FatalError("reading lines: %v", err)
		}

		// JSON output или plain
		if jsonOut {
			internal.OutputJSONLines(lines, startLine, lineRange)
		} else {
			internal.OutputPlainLines(lines, startLine)
		}
		os.Exit(0)
	}

	// Валидация режимов работы
	workMode := "functions"
	if structMode && allMode {
		internal.FatalError("--struct and --all are mutually exclusive")
	}
	if structMode {
		workMode = "structs"
	} else if allMode {
		workMode = "all"
	}

	// Взаимоисключающие режимы
	if workMode == "functions" {
		if funcStr == "" && !mapMode && !treeMode && !treeFull {
			internal.FatalError("either --func, --map, or --tree must be specified")
		}
		if funcStr != "" && (mapMode || treeMode || treeFull) {
			internal.FatalError("--func is mutually exclusive with --map and --tree")
		}
		if typeStr != "" {
			internal.FatalError("--type can only be used with --struct or --all")
		}
	} else if workMode == "structs" {
		if typeStr == "" && !mapMode && !treeMode && !treeFull {
			internal.FatalError("either --type, --map, or --tree must be specified with --struct")
		}
		if typeStr != "" && (mapMode || treeMode || treeFull) {
			internal.FatalError("--type is mutually exclusive with --map and --tree")
		}
		if funcStr != "" {
			internal.FatalError("--func cannot be used with --struct")
		}
	} else if workMode == "all" {
		if !mapMode && !treeMode && !treeFull && !jsonOut {
			internal.FatalError("--all requires --map, --tree, or --json output mode")
		}
		if funcStr != "" || typeStr != "" {
			internal.FatalError("--func and --type cannot be used with --all (use --map instead)")
		}
	}

	if treeMode && treeFull {
		internal.FatalError("--tree and --tree-full are mutually exclusive")
	}

	// Получаем конфигурацию для выбранного языка
	langConfig, err := config.GetLanguageConfig(source)
	if err != nil {
		internal.FatalError("%v\nSupported languages: %s", err, strings.Join(config.GetSupportedLanguages(), ", "))
	}

	// Определяем режим работы
	mode := "func"
	if mapMode || treeMode || treeFull {
		mode = "map"
	}

	// Для --tree-full нужны тела функций/типов
	extractMode := extract || treeFull

	// Обработка в зависимости от workMode
	switch workMode {
	case "functions":
		processFunctions(langConfig, funcStr, mode, extractMode, rawMode, inp, linesRange, mapMode, treeMode, treeFull, jsonOut, extract)

	case "structs":
		processStructs(langConfig, typeStr, mode, extractMode, inp, linesRange, mapMode, treeMode, treeFull, jsonOut, extract)

	case "all":
		processAll(langConfig, mode, extractMode, rawMode, inp, linesRange, mapMode, treeMode, treeFull, jsonOut, extract)
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

		// Для Python: умный анализ scope областей видимости
		if langConfig.IndentBased {
			// Pass 1: Анализ scope областей видимости
			scopes, err := internal.AnalyzePythonScopes(inp)
			if err != nil {
				internal.FatalError("analyzing Python scopes: %v", err)
			}

			// Pass 2: Валидация и коррекция диапазона
			fixedStart, fixedEnd, adjustments := internal.ValidateAndFixLineRange(scopes, lineRange.Start, lineRange.End)

			// Выводим отчёт об корректировках
			if len(adjustments) > 0 {
				report := internal.FormatLineAdjustmentReport(adjustments, lineRange.Start, lineRange.End, fixedStart, fixedEnd)
				fmt.Println(report)
			}

			// Обновляем диапазон
			lineRange.Start = fixedStart
			lineRange.End = fixedEnd
			linesRange = fmt.Sprintf("%d:%d", fixedStart, fixedEnd)
		}

		lines, startLine, err := internal.ReadFileLines(inp, lineRange)
		if err != nil {
			internal.FatalError("reading lines: %v", err)
		}

		// Предупреждение: --lines может разрезать тела функций
		if lineRange.End == -1 {
			internal.InfoMessage("Using --lines filter (%d:EOF). Functions outside this range will be excluded.", lineRange.Start)
		} else {
			internal.InfoMessage("Using --lines filter (%d:%d). Functions outside this range will be excluded.", lineRange.Start, lineRange.End)
		}

		// Cast to *internal.Finder to access FindFunctionsInLines
		if stdFinder, ok := finder.(*internal.Finder); ok {
			result, err = stdFinder.FindFunctionsInLines(lines, startLine, inp)
			if err != nil {
				internal.FatalError("%v", err)
			}
		} else {
			// Python finder - используем тот же метод для консистентности
			result, err = finder.FindFunctions(inp)
			if err != nil {
				internal.FatalError("%v", err)
			}
			// Фильтруем результаты по запрошенному диапазону
			filtered := make([]internal.FunctionBounds, 0)
			for _, fn := range result.Functions {
				if fn.Start >= lineRange.Start && (lineRange.End == -1 || fn.End <= lineRange.End) {
					filtered = append(filtered, fn)
				}
			}
			result.Functions = filtered
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

	// Создаем function finder (всегда в режиме "map")
	funcFinder := internal.CreateFinder(langConfig, "", "map", extractMode, rawMode)
	funcResult, err := funcFinder.FindFunctions(inp)
	if err != nil {
		internal.FatalError("finding functions: %v", err)
	}

	// Создаем struct finder (если язык поддерживает)
	var structResult *internal.StructFindResult
	if langConfig.HasStructSupport() {
		factory := internal.NewStructFinderFactory()
		structFinder := factory.CreateStructFinder(langConfig, "", true, extractMode)
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
		outputCombinedJSON(funcResult, structResult)
	} else if extract {
		if funcCount > 0 {
			fmt.Println("=== FUNCTIONS ===")
			fmt.Println(internal.FormatExtract(funcResult))
		}
		if typeCount > 0 {
			if funcCount > 0 {
				fmt.Println()
			}
			fmt.Println("=== TYPES ===")
			allLines, _, err := internal.ReadFileLines(inp, internal.LineRange{Start: 1, End: -1})
			if err != nil {
				internal.FatalError("reading file: %v", err)
			}
			fmt.Println(internal.FormatStructExtract(structResult, allLines))
		}
	} else if treeMode || treeFull {
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

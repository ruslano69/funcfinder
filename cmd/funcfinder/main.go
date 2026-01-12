package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/yourusername/funcfinder/internal"
)

const Version = "1.5.0"

func main() {
	// Парсинг аргументов командной строки
	version := flag.Bool("version", false, "print version and exit")
	inp := flag.String("inp", "", "input file with source code")
	source := flag.String("source", "", "source language: go/c/cpp/cs/java/d/js/ts/py")
	funcStr := flag.String("func", "", "function names to find (comma-separated)")
	mapMode := flag.Bool("map", false, "map all functions in file")
	treeMode := flag.Bool("tree", false, "output functions in tree format")
	treeFull := flag.Bool("tree-full", false, "output functions in tree format with signatures")
	jsonOut := flag.Bool("json", false, "output in JSON format")
	extract := flag.Bool("extract", false, "extract function bodies")
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

	// Взаимоисключающие режимы
	if *funcStr == "" && !*mapMode && !*treeMode && !*treeFull {
		internal.FatalError("either --func, --map, or --tree must be specified")
	}

	if *funcStr != "" && (*mapMode || *treeMode || *treeFull) {
		internal.FatalError("--func is mutually exclusive with --map and --tree")
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

	// Для --tree-full нужны тела функций для извлечения сигнатур
	extractMode := *extract || *treeFull

	// Создаем подходящий парсер в зависимости от языка
	finder := internal.CreateFinder(langConfig, *funcStr, mode, extractMode, *rawMode)

	var result *internal.FindResult

	// Если указан --lines, применяем фильтр по строкам
	if *linesRange != "" {
		lineRange, err := internal.ParseLineRange(*linesRange)
		if err != nil {
			internal.FatalError("parsing line range: %v", err)
		}

		lines, startLine, err := internal.ReadFileLines(*inp, lineRange)
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
			result, err = stdFinder.FindFunctionsInLines(lines, startLine, *inp)
			if err != nil {
				internal.FatalError("%v", err)
			}
		} else {
			internal.FatalError("--lines is not yet supported for Python files")
		}
	} else {
		// Standard mode: read entire file
		result, err = finder.FindFunctions(*inp)
		if err != nil {
			internal.FatalError("%v", err)
		}
	}

	// Если ничего не найдено
	if len(result.Functions) == 0 {
		if *mapMode || *treeMode || *treeFull {
			internal.FatalErrorWithCode(2, "No functions found in file")
		} else {
			internal.FatalErrorWithCode(2, "Specified functions not found")
		}
	}

	// Форматируем и выводим результат
	var output string
	if *extract {
		output = internal.FormatExtract(result)
	} else if *jsonOut {
		output, err = internal.FormatJSON(result)
		if err != nil {
			internal.FatalError("formatting output: %v", err)
		}
	} else if *treeMode {
		output = internal.FormatTreeCompact(result)
	} else if *treeFull {
		output = internal.FormatTreeFull(result)
	} else {
		output = internal.FormatGrepStyle(result)
	}

	fmt.Println(output)
}

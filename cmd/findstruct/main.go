package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/ruslano69/funcfinder/internal"
)

const Version = "1.0.0"

func main() {
	// Парсинг аргументов командной строки
	version := flag.Bool("version", false, "print version and exit")
	inp := flag.String("inp", "", "input file with source code")
	source := flag.String("source", "", "source language: cpp/cs/java/d/py")
	typeStr := flag.String("type", "", "type names to find (comma-separated)")
	mapMode := flag.Bool("map", false, "map all types in file")
	treeMode := flag.Bool("tree", false, "output types in tree format")
	jsonOut := flag.Bool("json", false, "output in JSON format")
	extract := flag.Bool("extract", false, "extract type bodies")
	linesRange := flag.String("lines", "", "extract specific line range (format: start:end, :end, start:, or single line)")

	flag.Parse()

	// Обработка флага --version
	if *version {
		internal.PrintVersion("findstruct")
		os.Exit(0)
	}

	// Валидация параметров
	if *inp == "" {
		internal.FatalError("--inp parameter is required")
	}

	if *source == "" {
		internal.FatalError("--source parameter is required")
	}

	// Взаимоисключающие режимы
	if *typeStr == "" && !*mapMode && !*treeMode {
		internal.FatalError("either --type or --map or --tree must be specified")
	}

	if *typeStr != "" && (*mapMode || *treeMode) {
		internal.FatalError("--type is mutually exclusive with --map and --tree")
	}

	// Загружаем конфигурацию языков
	config, err := internal.LoadConfig()
	if err != nil {
		internal.FatalError("loading config: %v", err)
	}

	// Получаем конфигурацию для выбранного языка
	langConfig, err := config.GetLanguageConfig(*source)
	if err != nil {
		internal.FatalError("%v\nSupported languages: cpp, cs, java, d, py", err)
	}

	// Проверяем, что язык поддерживает классы/структуры или имеет indent_based (Python)
	if !langConfig.HasClasses() && !langConfig.IndentBased {
		internal.FatalError("language %s does not support type definitions", *source)
	}

	// Создаём структурный finder через фабрику
	factory := internal.NewStructFinderFactory()
	finder := factory.CreateStructFinder(langConfig, *typeStr, *mapMode || *treeMode, *extract)

	var result *internal.StructFindResult

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

		if lineRange.End == -1 {
			internal.InfoMessage("Using --lines filter (%d:EOF). Types outside this range will be excluded.", lineRange.Start)
		} else {
			internal.InfoMessage("Using --lines filter (%d:%d). Types outside this range will be excluded.", lineRange.Start, lineRange.End)
		}

		result, err = finder.FindStructuresInLines(lines, startLine, *inp)
		if err != nil {
			internal.FatalError("%v", err)
		}
	} else {
		// Standard mode: read entire file
		result, err = finder.FindStructures(*inp)
		if err != nil {
			internal.FatalError("%v", err)
		}
	}

	// Если ничего не найдено
	if len(result.Types) == 0 {
		if *mapMode || *treeMode {
			internal.FatalErrorWithCode(2, "No types found in file")
		} else {
			internal.FatalErrorWithCode(2, "Specified types not found")
		}
	}

	// Форматируем и выводим результат
	var output string
	if *extract {
		// Read full file for extract mode
		allLines, _, err := internal.ReadFileLines(*inp, internal.LineRange{Start: 1, End: -1})
		if err != nil {
			internal.FatalError("reading file: %v", err)
		}
		output = internal.FormatStructExtract(result, allLines)
	} else if *jsonOut {
		output, err = internal.FormatStructJSON(result)
		if err != nil {
			internal.FatalError("formatting output: %v", err)
		}
	} else if *treeMode {
		output = internal.FormatStructTree(result)
	} else {
		output = internal.FormatStructMap(result)
	}

	fmt.Println(output)
}

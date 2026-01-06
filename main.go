package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

const Version = "1.4.0"

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
		fmt.Printf("funcfinder version %s\n", Version)
		os.Exit(0)
	}
	
	// Валидация параметров
	if *inp == "" {
		fmt.Fprintln(os.Stderr, "Error: --inp parameter is required")
		flag.Usage()
		os.Exit(1)
	}

	// --source не обязателен если используется только --lines (standalone mode)
	standaloneLines := *linesRange != "" && *source == ""

	if *source == "" && !standaloneLines {
		fmt.Fprintln(os.Stderr, "Error: --source parameter is required (or use --lines alone for plain text extraction)")
		flag.Usage()
		os.Exit(1)
	}
	
	// Standalone --lines mode: просто вывести строки без парсинга
	if standaloneLines {
		lineRange, err := ParseLineRange(*linesRange)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing line range: %v\n", err)
			os.Exit(1)
		}

		lines, startLine, err := ReadFileLines(*inp, lineRange)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading lines: %v\n", err)
			os.Exit(1)
		}

		// JSON output или plain
		if *jsonOut {
			OutputJSONLines(lines, startLine, lineRange)
		} else {
			OutputPlainLines(lines, startLine)
		}
		os.Exit(0)
	}

	// Взаимоисключающие режимы
	if *funcStr == "" && !*mapMode && !*treeMode && !*treeFull {
		fmt.Fprintln(os.Stderr, "Error: either --func, --map, or --tree must be specified")
		flag.Usage()
		os.Exit(1)
	}

	if *funcStr != "" && (*mapMode || *treeMode || *treeFull) {
		fmt.Fprintln(os.Stderr, "Error: --func is mutually exclusive with --map and --tree")
		flag.Usage()
		os.Exit(1)
	}

	if *treeMode && *treeFull {
		fmt.Fprintln(os.Stderr, "Error: --tree and --tree-full are mutually exclusive")
		flag.Usage()
		os.Exit(1)
	}
	
	// Загружаем конфигурацию языков
	config, err := LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}
	
	// Получаем конфигурацию для выбранного языка
	langConfig, err := config.GetLanguageConfig(*source)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		fmt.Fprintf(os.Stderr, "Supported languages: %s\n", strings.Join(config.GetSupportedLanguages(), ", "))
		os.Exit(1)
	}

	// Определяем режим работы
	mode := "func"
	if *mapMode || *treeMode || *treeFull {
		mode = "map"
	}

	// Для --tree-full нужны тела функций для извлечения сигнатур
	extractMode := *extract || *treeFull

	// Создаем подходящий парсер в зависимости от языка
	finder := CreateFinder(langConfig, *funcStr, mode, extractMode, *rawMode)

	var result *FindResult

	// Если указан --lines, применяем фильтр по строкам
	if *linesRange != "" {
		lineRange, err := ParseLineRange(*linesRange)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing line range: %v\n", err)
			os.Exit(1)
		}

		lines, startLine, err := ReadFileLines(*inp, lineRange)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading lines: %v\n", err)
			os.Exit(1)
		}

		// Warning: --lines may cut through function bodies
		if lineRange.End == -1 {
			fmt.Fprintf(os.Stderr, "INFO: Using --lines filter (%d:EOF). Functions outside this range will be excluded.\n", lineRange.Start)
		} else {
			fmt.Fprintf(os.Stderr, "INFO: Using --lines filter (%d:%d). Functions outside this range will be excluded.\n", lineRange.Start, lineRange.End)
		}

		// IMPORTANT: For Python need to handle it specially
		// For now we only support standard Finder with --lines
		if langConfig.IndentBased {
			fmt.Fprintln(os.Stderr, "WARNING: --lines with Python may produce incorrect results for indent-based parsing")
		}

		// Cast to *Finder to access FindFunctionsInLines
		if stdFinder, ok := finder.(*Finder); ok {
			result, err = stdFinder.FindFunctionsInLines(lines, startLine, *inp)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		} else {
			fmt.Fprintln(os.Stderr, "Error: --lines is not yet supported for Python files")
			os.Exit(1)
		}
	} else {
		// Standard mode: read entire file
		result, err = finder.FindFunctions(*inp)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	}
	
	// Если ничего не найдено
	if len(result.Functions) == 0 {
		if *mapMode || *treeMode || *treeFull {
			fmt.Fprintln(os.Stderr, "No functions found in file")
		} else {
			fmt.Fprintln(os.Stderr, "Specified functions not found")
		}
		os.Exit(2)
	}

	// Форматируем и выводим результат
	var output string
	if *extract {
		output = FormatExtract(result)
	} else if *jsonOut {
		output, err = FormatJSON(result)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error formatting output: %v\n", err)
			os.Exit(1)
		}
	} else if *treeMode {
		output = FormatTreeCompact(result)
	} else if *treeFull {
		output = FormatTreeFull(result)
	} else {
		output = FormatGrepStyle(result)
	}

	fmt.Println(output)
}

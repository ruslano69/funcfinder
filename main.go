package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	// Парсинг аргументов командной строки
	inp := flag.String("inp", "", "input file with source code")
	source := flag.String("source", "", "source language: go/c/cpp/cs/java/d/js/ts")
	funcStr := flag.String("func", "", "function names to find (comma-separated)")
	mapMode := flag.Bool("map", false, "map all functions in file")
	jsonOut := flag.Bool("json", false, "output in JSON format")
	extract := flag.Bool("extract", false, "extract function bodies")
	rawMode := flag.Bool("raw", false, "include raw strings in brace counting")
	
	flag.Parse()
	
	// Валидация параметров
	if *inp == "" {
		fmt.Fprintln(os.Stderr, "Error: --inp parameter is required")
		flag.Usage()
		os.Exit(1)
	}
	
	if *source == "" {
		fmt.Fprintln(os.Stderr, "Error: --source parameter is required")
		flag.Usage()
		os.Exit(1)
	}
	
	// Взаимоисключающие режимы
	if *funcStr == "" && !*mapMode {
		fmt.Fprintln(os.Stderr, "Error: either --func or --map must be specified")
		flag.Usage()
		os.Exit(1)
	}
	
	if *funcStr != "" && *mapMode {
		fmt.Fprintln(os.Stderr, "Error: --func and --map are mutually exclusive")
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
		fmt.Fprintf(os.Stderr, "Supported languages: go, c, cpp, cs, java, d, js, ts\n")
		os.Exit(1)
	}
	
	// Парсим имена функций
	funcNames := ParseFuncNames(*funcStr)
	
	// Создаем искатель и ищем функции
	finder := NewFinder(langConfig, funcNames, *mapMode, *extract, *rawMode)
	result, err := finder.FindFunctions(*inp)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	
	// Если ничего не найдено
	if len(result.Functions) == 0 {
		if *mapMode {
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
	} else {
		output = FormatGrepStyle(result)
	}
	
	fmt.Println(output)
}

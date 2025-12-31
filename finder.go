package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// FunctionBounds содержит информацию о границах функции
type FunctionBounds struct {
	Name  string
	Start int      // Номер строки начала (1-based)
	End   int      // Номер строки конца (1-based)
	Lines []string // Тело функции (если extractMode)
}

// FindResult содержит результат поиска
type FindResult struct {
	Functions []FunctionBounds
	Filename  string
}

// Finder ищет функции в файле
type Finder struct {
	config      *LanguageConfig
	sanitizer   *Sanitizer
	funcNames   map[string]bool
	mapMode     bool
	extractMode bool
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
	
	result := &FindResult{
		Filename:  filename,
		Functions: []FunctionBounds{},
	}
	
	state := StateNormal
	var currentFunc *FunctionBounds
	depth := 0
	funcRegex := f.config.FuncRegex()
	
	for lineNum, line := range lines {
		// Очищаем строку от комментариев и литералов
		cleaned, newState := f.sanitizer.CleanLine(line, state)
		state = newState
		
		// Если мы внутри функции, отслеживаем баланс скобок
		if currentFunc != nil {
			if f.extractMode {
				currentFunc.Lines = append(currentFunc.Lines, line)
			}
			
			depth += CountBraces(cleaned)
			
			if depth == 0 {
				// Конец функции
				currentFunc.End = lineNum + 1 // 1-based
				result.Functions = append(result.Functions, *currentFunc)
				currentFunc = nil
			}
		} else {
			// Ищем начало новой функции
			matches := funcRegex.FindStringSubmatch(cleaned)
			if matches != nil {
				// Извлекаем имя функции (последняя группа захвата)
				funcName := ""
				for i := len(matches) - 1; i >= 1; i-- {
					if matches[i] != "" {
						funcName = matches[i]
						break
					}
				}
				
				// Проверяем, нужно ли нам эту функцию
				if f.mapMode || f.funcNames[funcName] {
					// Ищем открывающую скобку
					braceCount := CountBraces(cleaned)
					if braceCount > 0 {
						// Скобка на той же строке
						currentFunc = &FunctionBounds{
							Name:  funcName,
							Start: lineNum + 1, // 1-based
							Lines: []string{},
						}
						if f.extractMode {
							currentFunc.Lines = append(currentFunc.Lines, line)
						}
						depth = braceCount
						
						if depth == 0 {
							// Функция на одной строке (маловероятно, но возможно)
							currentFunc.End = lineNum + 1
							result.Functions = append(result.Functions, *currentFunc)
							currentFunc = nil
						}
					}
					// Если скобки нет, ждем её на следующих строках
					// (многострочная сигнатура)
					if braceCount == 0 {
						currentFunc = &FunctionBounds{
							Name:  funcName,
							Start: lineNum + 1,
							Lines: []string{},
						}
						if f.extractMode {
							currentFunc.Lines = append(currentFunc.Lines, line)
						}
						depth = 0
					}
				}
			} else if currentFunc != nil && depth == 0 {
				// Продолжаем многострочную сигнатуру
				if f.extractMode {
					currentFunc.Lines = append(currentFunc.Lines, line)
				}
				braceCount := CountBraces(cleaned)
				if braceCount > 0 {
					depth = braceCount
					if depth == 0 {
						currentFunc.End = lineNum + 1
						result.Functions = append(result.Functions, *currentFunc)
						currentFunc = nil
					}
				}
			}
		}
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

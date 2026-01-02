package main

// LanguageFinder - интерфейс для парсеров разных языков
type LanguageFinder interface {
	FindFunctions(filename string) (*FindResult, error)
}

// CreateFinder создает подходящий парсер в зависимости от языка
func CreateFinder(config *LanguageConfig, funcNamesStr string, mode string, extract bool, useRaw bool) LanguageFinder {
	// Для языков на основе отступов (Python) используем специальный парсер
	if config.IndentBased {
		return NewPythonFinder(*config, funcNamesStr, mode, extract)
	}

	// Для остальных языков (C-подобных со скобками) используем стандартный парсер
	// Парсим строку с именами функций в массив
	funcNames := ParseFuncNames(funcNamesStr)
	return NewFinder(config, funcNames, mode == "map", extract, useRaw)
}

package main

import (
	"regexp"
	"strings"
)

// DecoratorWindow хранит скользящее окно строк для поиска декораторов
type DecoratorWindow struct {
	lines       []string
	lineNumbers []int
	maxSize     int
	pattern     *regexp.Regexp
}

// NewDecoratorWindow создает новое окно с указанным размером и паттерном декораторов
func NewDecoratorWindow(maxSize int, decoratorPattern string) *DecoratorWindow {
	var pattern *regexp.Regexp
	if decoratorPattern != "" {
		pattern = regexp.MustCompile(decoratorPattern)
	}

	return &DecoratorWindow{
		lines:       make([]string, 0, maxSize),
		lineNumbers: make([]int, 0, maxSize),
		maxSize:     maxSize,
		pattern:     pattern,
	}
}

// Add добавляет строку в окно
func (dw *DecoratorWindow) Add(line string, lineNum int) {
	dw.lines = append(dw.lines, line)
	dw.lineNumbers = append(dw.lineNumbers, lineNum)

	// Если превысили максимальный размер, удаляем самую старую строку
	if len(dw.lines) > dw.maxSize {
		dw.lines = dw.lines[1:]
		dw.lineNumbers = dw.lineNumbers[1:]
	}
}

// ExtractDecorators извлекает декораторы, которые находятся непосредственно перед функцией
// Возвращает список декораторов (без @) и номер строки первого декоратора
func (dw *DecoratorWindow) ExtractDecorators() ([]string, int) {
	if dw.pattern == nil {
		return nil, -1
	}

	decorators := make([]string, 0)
	firstDecoratorLine := -1

	// Идем от конца к началу окна, собирая декораторы
	// Пропускаем последнюю строку (это сама функция)
	for i := len(dw.lines) - 2; i >= 0; i-- {
		line := strings.TrimSpace(dw.lines[i])

		// Пустые строки пропускаем
		if line == "" {
			continue
		}

		// Комментарии пропускаем
		if strings.HasPrefix(line, "#") || strings.HasPrefix(line, "//") {
			continue
		}

		// Проверяем, является ли строка декоратором
		if dw.pattern.MatchString(line) {
			decorators = append([]string{line}, decorators...) // добавляем в начало
			firstDecoratorLine = dw.lineNumbers[i]
		} else {
			// Если встретили не-декоратор и не-пустую строку, останавливаемся
			break
		}
	}

	return decorators, firstDecoratorLine
}

// Clear очищает окно
func (dw *DecoratorWindow) Clear() {
	dw.lines = dw.lines[:0]
	dw.lineNumbers = dw.lineNumbers[:0]
}

// GetIndentLevel возвращает уровень отступа строки (количество пробелов в начале)
func GetIndentLevel(line string) int {
	indent := 0
	for _, ch := range line {
		if ch == ' ' {
			indent++
		} else if ch == '\t' {
			indent += 4 // считаем табуляцию как 4 пробела
		} else {
			break
		}
	}
	return indent
}

// IsEmptyOrComment проверяет, является ли строка пустой или комментарием
func IsEmptyOrComment(line string, commentPrefix string) bool {
	trimmed := strings.TrimSpace(line)
	return trimmed == "" || strings.HasPrefix(trimmed, commentPrefix)
}

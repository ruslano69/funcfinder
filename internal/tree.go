package internal

import (
	"encoding/json"
	"fmt"
	"strings"
)

// TreeNodeType определяет тип узла в дереве
type TreeNodeType int

const (
	NodeTypeFunction TreeNodeType = iota
	NodeTypeClass
	NodeTypeRoot
)

// TreeNode представляет узел в дереве функций
type TreeNode struct {
	Name     string
	Type     TreeNodeType
	Start    int
	End      int
	Children []*TreeNode
	Depth    int
	IsLast   bool
	Lines    []string
}

// BuildTree строит дерево функций и классов
func BuildTree(result *FindResult) []*TreeNode {
	var rootNodes []*TreeNode

	// Если есть классы, строим дерево с классами
	if len(result.Classes) > 0 {
		rootNodes = buildClassTree(result)
	} else {
		// Иначе просто показываем функции
		rootNodes = buildFunctionTree(result.Functions)
	}

	// Устанавливаем флаг IsLast
	setLastFlags(rootNodes)

	return rootNodes
}

// buildClassTree строит дерево с классами как родительскими узлами
func buildClassTree(result *FindResult) []*TreeNode {
	var rootNodes []*TreeNode

	// Сначала создаем узлы классов
	for _, class := range result.Classes {
		classNode := &TreeNode{
			Name:     class.Name,
			Type:     NodeTypeClass,
			Start:    class.Start,
			End:      class.End,
			Children: []*TreeNode{},
		}

		// Находим методы этого класса
		var methods []FunctionBounds
		for _, fn := range result.Functions {
			if fn.ClassName == class.Name {
				methods = append(methods, fn)
			}
		}

		// Добавляем методы как детей класса
		if len(methods) > 0 {
			classNode.Children = buildFunctionTree(methods)
		}

		rootNodes = append(rootNodes, classNode)
	}

	// Добавляем функции, не принадлежащие никакому классу
	var topLevelFuncs []FunctionBounds
	for _, fn := range result.Functions {
		if fn.ClassName == "" {
			topLevelFuncs = append(topLevelFuncs, fn)
		}
	}

	if len(topLevelFuncs) > 0 {
		rootNodes = append(rootNodes, buildFunctionTree(topLevelFuncs)...)
	}

	return rootNodes
}

// buildFunctionTree строит дерево из списка функций
func buildFunctionTree(functions []FunctionBounds) []*TreeNode {
	if len(functions) == 0 {
		return nil
	}

	// Сортируем функции по начальной строке
	sorted := make([]FunctionBounds, len(functions))
	copy(sorted, functions)
	for i := 0; i < len(sorted)-1; i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i].Start > sorted[j].Start {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	var rootNodes []*TreeNode
	var allNodes []*TreeNode

	for _, fn := range sorted {
		node := &TreeNode{
			Name:     fn.Name,
			Type:     NodeTypeFunction,
			Start:    fn.Start,
			End:      fn.End,
			Children: []*TreeNode{},
			Depth:    0,
			IsLast:   false,
			Lines:    fn.Lines,
		}
		allNodes = append(allNodes, node)
	}

	// Проверяем, все ли функции на одном уровне (нет вложенных)
	allOnSameLevel := true
	for _, node := range allNodes {
		parent := findParent(node, allNodes)
		if parent != nil {
			allOnSameLevel = false
			parent.Children = append(parent.Children, node)
		}
	}

	if !allOnSameLevel {
		// Есть вложенные функции, возвращаем только корневые
		for _, node := range allNodes {
			if findParent(node, allNodes) == nil {
				rootNodes = append(rootNodes, node)
			}
		}
	} else {
		// Все на одном уровне
		rootNodes = allNodes
	}

	return rootNodes
}

// findParent находит родительскую функцию для узла
func findParent(node *TreeNode, allNodes []*TreeNode) *TreeNode {
	for _, candidate := range allNodes {
		if candidate == node {
			continue
		}
		if candidate.Start < node.Start && candidate.End > node.End {
			return candidate
		}
	}
	return nil
}

// setLastFlags устанавливает флаг IsLast для последних элементов
func setLastFlags(nodes []*TreeNode) {
	for i := range nodes {
		if i == len(nodes)-1 {
			nodes[i].IsLast = true
		}
		for _, child := range nodes[i].Children {
			child.Depth = nodes[i].Depth + 1
		}
		setLastFlags(nodes[i].Children)
	}
}

// FormatTree форматирует результат в древовидном формате
func FormatTree(result *FindResult, showTypes bool) string {
	treeNodes := BuildTree(result)
	if len(treeNodes) == 0 {
		return ""
	}

	// Устанавливаем флаги IsLast
	setLastFlags(treeNodes)

	var lines []string
	for _, node := range treeNodes {
		lines = append(lines, formatNode(node, showTypes, []bool{}))
		// Рекурсивно добавляем детей
		lines = append(lines, formatChildren(node.Children, showTypes, []bool{})...)
	}

	return strings.Join(lines, "\n")
}

// formatChildren рекурсивно форматирует детей узла
func formatChildren(children []*TreeNode, showTypes bool, ancestorLast []bool) []string {
	var lines []string
	for i, child := range children {
		childAncestorLast := append([]bool{}, ancestorLast...)
		if i == len(children)-1 {
			childAncestorLast = append(childAncestorLast, true)
		} else {
			childAncestorLast = append(childAncestorLast, false)
		}

		lines = append(lines, formatNode(child, showTypes, childAncestorLast))
		lines = append(lines, formatChildren(child.Children, showTypes, childAncestorLast)...)
	}
	return lines
}

// formatNode форматирует один узел дерева
func formatNode(node *TreeNode, showTypes bool, ancestorLast []bool) string {
	var builder strings.Builder

	// Строим префикс
	for i := 0; i < len(ancestorLast)-1; i++ {
		if ancestorLast[i] {
			builder.WriteString("    ")
		} else {
			builder.WriteString("│   ")
		}
	}

	// Добавляем ветку
	if len(ancestorLast) > 0 {
		if node.IsLast {
			builder.WriteString("└── ")
		} else {
			builder.WriteString("├── ")
		}
	}

	// Добавляем тип для классов
	var prefix string
	switch node.Type {
	case NodeTypeClass:
		prefix = "class "
	case NodeTypeFunction:
		if node.Depth > 0 {
			prefix = "method "
		}
	}

	// Форматируем строку функции/класса
	funcLine := formatFunctionLine(node, showTypes)
	builder.WriteString(prefix)
	builder.WriteString(funcLine)

	return builder.String()
}

// formatFunctionLine форматирует строку с информацией о функции или классе
func formatFunctionLine(node *TreeNode, showTypes bool) string {
	if showTypes && node.Type == NodeTypeFunction {
		signature := extractSignatureFromLines(node.Lines)
		if signature != "" {
			return fmt.Sprintf("%s (%d-%d)", signature, node.Start, node.End)
		}
	}
	return fmt.Sprintf("%s (%d-%d)", node.Name, node.Start, node.End)
}

// extractSignatureFromLines извлекает сигнатуру из тела функции
func extractSignatureFromLines(lines []string) string {
	if len(lines) == 0 {
		return ""
	}

	var fullSignature string
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Skip empty lines and decorators
		if trimmed == "" || strings.HasPrefix(trimmed, "@") {
			continue
		}

		// Go functions
		if strings.HasPrefix(trimmed, "func ") {
			idx := strings.Index(trimmed, "{")
			if idx > 0 {
				sig := trimmed[:idx]
				return strings.TrimPrefix(sig, "func ")
			}
			return strings.TrimPrefix(trimmed, "func ")
		}

		// Python functions
		if strings.HasPrefix(trimmed, "def ") || strings.HasPrefix(trimmed, "async def ") {
			// Collect multiline signature
			signature := trimmed
			for j := i + 1; j < len(lines) && !strings.Contains(signature, ":"); j++ {
				signature += " " + strings.TrimSpace(lines[j])
			}
			idx := strings.Index(signature, ":")
			if idx > 0 {
				return strings.TrimSpace(signature[:idx])
			}
			return signature
		}

		// JavaScript/TypeScript functions
		if strings.Contains(trimmed, "function ") {
			idx := strings.Index(trimmed, "{")
			if idx > 0 {
				return strings.TrimSpace(trimmed[:idx])
			}
			return trimmed
		}

		// Java/C#/C++/D methods (public int add(...), void method(...), etc.)
		// Look for pattern: words...identifier(...)
		if strings.Contains(trimmed, "(") && (strings.Contains(trimmed, "{") || i+1 < len(lines)) {
			// Collect multiline signature if needed
			signature := trimmed
			openBraceIdx := strings.Index(signature, "{")

			// If no opening brace yet, might be multiline
			if openBraceIdx < 0 {
				for j := i + 1; j < len(lines) && j < i+5; j++ {
					nextLine := strings.TrimSpace(lines[j])
					signature += " " + nextLine
					if strings.Contains(nextLine, "{") {
						break
					}
				}
			}

			// Extract signature before opening brace
			openBraceIdx = strings.Index(signature, "{")
			if openBraceIdx > 0 {
				fullSignature = strings.TrimSpace(signature[:openBraceIdx])
			} else {
				fullSignature = signature
			}

			// Clean up extra whitespace
			fullSignature = strings.Join(strings.Fields(fullSignature), " ")

			return fullSignature
		}
	}

	return fullSignature
}

// FormatTreeCompact компактный формат дерева без типов
func FormatTreeCompact(result *FindResult) string {
	return FormatTree(result, false)
}

// FormatTreeFull формат дерева с типами возврата
func FormatTreeFull(result *FindResult) string {
	return FormatTree(result, true)
}

// TreeOutput представляет структурированный вывод дерева для JSON
type TreeOutput struct {
	Functions []TreeFunctionNode `json:"functions"`
	Classes   []TreeClassNode    `json:"classes,omitempty"`
	Summary   TreeSummary        `json:"summary"`
}

// TreeFunctionNode представляет узел функции в дереве
type TreeFunctionNode struct {
	Name      string             `json:"name"`
	Start     int                `json:"start"`
	End       int                `json:"end"`
	Children  []TreeFunctionNode `json:"children,omitempty"`
	ClassName string             `json:"class_name,omitempty"`
	Signature string             `json:"signature,omitempty"`
}

// TreeClassNode представляет узел класса в дереве
type TreeClassNode struct {
	Name    string             `json:"name"`
	Start   int                `json:"start"`
	End     int                `json:"end"`
	Methods []TreeFunctionNode `json:"methods"`
}

// TreeSummary содержит сводку по дереву
type TreeSummary struct {
	TotalFunctions int `json:"total_functions"`
	TotalClasses   int `json:"total_classes"`
	MaxDepth       int `json:"max_depth"`
	TotalLines     int `json:"total_lines"`
}

// TreeToJSON преобразует результат в JSON-дерево
func TreeToJSON(result *FindResult, showSignature bool) (string, error) {
	treeNodes := BuildTree(result)

	output := TreeOutput{
		Functions: []TreeFunctionNode{},
		Classes:   []TreeClassNode{},
		Summary: TreeSummary{
			TotalFunctions: len(result.Functions),
			TotalClasses:   len(result.Classes),
			TotalLines:     calculateTotalLines(result.Functions),
		},
	}

	maxDepth := 0

	// Обрабатываем классы
	for _, class := range result.Classes {
		classNode := TreeClassNode{
			Name:    class.Name,
			Start:   class.Start,
			End:     class.End,
			Methods: []TreeFunctionNode{},
		}

		// Добавляем методы класса
		for _, fn := range result.Functions {
			if fn.ClassName == class.Name {
				methodNode := TreeFunctionNode{
					Name:      fn.Name,
					Start:     fn.Start,
					End:       fn.End,
					ClassName: class.Name,
				}
				if showSignature {
					methodNode.Signature = extractSignatureFromLines(fn.Lines)
				}
				classNode.Methods = append(classNode.Methods, methodNode)
			}
		}

		output.Classes = append(output.Classes, classNode)
	}

	// Обрабатываем функции верхнего уровня
	for _, fn := range result.Functions {
		if fn.ClassName == "" {
			fnNode := TreeFunctionNode{
				Name:  fn.Name,
				Start: fn.Start,
				End:   fn.End,
			}
			if showSignature {
				fnNode.Signature = extractSignatureFromLines(fn.Lines)
			}
			output.Functions = append(output.Functions, fnNode)
		}
	}

	// Считаем максимальную глубину
	for _, node := range treeNodes {
		calcDepth(node, 1, &maxDepth)
	}
	output.Summary.MaxDepth = maxDepth

	jsonBytes, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return "", err
	}

	return string(jsonBytes), nil
}

// calcDepth вычисляет глубину дерева
func calcDepth(node *TreeNode, currentDepth int, maxDepth *int) {
	if currentDepth > *maxDepth {
		*maxDepth = currentDepth
	}
	for _, child := range node.Children {
		calcDepth(child, currentDepth+1, maxDepth)
	}
}

// calculateTotalLines вычисляет общее количество строк
func calculateTotalLines(functions []FunctionBounds) int {
	total := 0
	for _, fn := range functions {
		total += fn.End - fn.Start + 1
	}
	return total
}

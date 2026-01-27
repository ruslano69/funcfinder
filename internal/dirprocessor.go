// dirprocessor.go - Directory processing with parallel execution and .gitignore support
package internal

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
)

// Job represents a file to be processed
type Job struct {
	Path      string
	Extension string
	LangKey   string
}

// DirResult represents the outcome of processing a single file
type DirResult struct {
	Path      string
	Functions []FunctionBounds
	Classes   []ClassBounds
	Error     error
}

// DirProcessor handles directory traversal and parallel file processing
type DirProcessor struct {
	config       Config
	workers      int
	recursive    bool
	useGitignore bool
	workMode     string // "functions", "structs", or "all"
}

// TreeNode represents a node in the directory tree for tree output
type DirTreeNode struct {
	Path      string
	Functions []FunctionBounds
	Classes   []ClassBounds
	Children  map[string]*DirTreeNode
}

// NewDirProcessor creates a new directory processor
func NewDirProcessor(config Config, workers int, recursive, useGitignore bool, workMode string) *DirProcessor {
	if workers <= 0 {
		workers = runtime.NumCPU()
	}
	return &DirProcessor{
		config:       config,
		workers:      workers,
		recursive:    recursive,
		useGitignore: useGitignore,
		workMode:     workMode,
	}
}

// ProcessDirectory processes all supported files in a directory
func (dp *DirProcessor) ProcessDirectory(rootPath string) ([]DirResult, error) {
	// Collect all files first
	files, err := dp.collectFiles(rootPath)
	if err != nil {
		return nil, err
	}

	if len(files) == 0 {
		return []DirResult{}, nil
	}

	// Process files in parallel
	return dp.processFilesParallel(files)
}

// collectFiles walks the directory and collects all supported files
func (dp *DirProcessor) collectFiles(rootPath string) ([]Job, error) {
	var jobs []Job
	var mu sync.Mutex

	// Load gitignore patterns if enabled
	var ignoreMatcher *IgnoreMatcher
	if dp.useGitignore {
		ignoreMatcher = NewIgnoreMatcher(rootPath)
	}

	err := filepath.Walk(rootPath, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			// Skip files/directories that can't be accessed
			return nil
		}

		relPath, err := filepath.Rel(rootPath, path)
		if err != nil {
			return nil
		}

		// Check if path matches gitignore patterns
		if ignoreMatcher != nil && ignoreMatcher.Matches(relPath, info.IsDir()) {
			// If it's a directory and we should skip it entirely
			if info.IsDir() {
				return filepath.SkipDir
			}
			// Skip the file
			return nil
		}

		// Skip hidden files and directories (starting with .)
		// except for .gitignore itself
		base := filepath.Base(path)
		if len(base) > 0 && base[0] == '.' && base != ".gitignore" {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip directories if not recursive
		if info.IsDir() {
			if !dp.recursive && path != rootPath {
				return filepath.SkipDir
			}
			return nil
		}

		// Check if file extension is supported
		langConfig := dp.config.GetLanguageByExtension(path)
		if langConfig == nil {
			return nil
		}

		mu.Lock()
		jobs = append(jobs, Job{
			Path:      path,
			Extension: filepath.Ext(path),
			LangKey:   langConfig.LangKey,
		})
		mu.Unlock()

		return nil
	})

	if err != nil {
		return nil, err
	}

	return jobs, nil
}

// processFilesParallel processes files using a worker pool
func (dp *DirProcessor) processFilesParallel(jobs []Job) ([]DirResult, error) {
	jobsChan := make(chan Job, len(jobs))
	resultsChan := make(chan DirResult, dp.workers*2)

	var wg sync.WaitGroup

	// Start worker goroutines
	for i := 0; i < dp.workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			dp.worker(jobsChan, resultsChan)
		}(i)
	}

	// Send all jobs
	for _, job := range jobs {
		jobsChan <- job
	}
	close(jobsChan)

	// Close results channel when all workers are done
	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	// Collect results
	var results []DirResult
	for result := range resultsChan {
		results = append(results, result)
	}

	return results, nil
}

// worker processes jobs from the channel
func (dp *DirProcessor) worker(jobsChan <-chan Job, resultsChan chan<- DirResult) {
	for job := range jobsChan {
		result := dp.processFile(job)
		resultsChan <- result
	}
}

// processFile processes a single file
func (dp *DirProcessor) processFile(job Job) DirResult {
	result := DirResult{
		Path: job.Path,
	}

	langConfig, err := dp.config.GetLanguageConfig(job.LangKey)
	if err != nil {
		result.Error = err
		return result
	}

	switch dp.workMode {
	case "functions":
		// Find only functions
		finder := CreateFinder(langConfig, "", "map", false, false)
		findResult, err := finder.FindFunctions(job.Path)
		if err != nil {
			result.Error = err
			return result
		}
		result.Functions = findResult.Functions
		result.Classes = findResult.Classes

	case "structs":
		// Find only structs/classes/types
		if !langConfig.HasStructSupport() {
			// Skip languages without struct support
			return result
		}
		factory := NewStructFinderFactory()
		structFinder := factory.CreateStructFinder(langConfig, "", true, false)
		structResult, err := structFinder.FindStructures(job.Path)
		if err != nil {
			result.Error = err
			return result
		}
		// For structs mode, put types in Classes field
		for _, typ := range structResult.Types {
			result.Classes = append(result.Classes, ClassBounds{
				Name:  typ.Name,
				Start: typ.Start,
				End:   typ.End,
			})
		}

	case "all":
		// Find both functions and structs
		finder := CreateFinder(langConfig, "", "map", false, false)
		findResult, err := finder.FindFunctions(job.Path)
		if err != nil {
			result.Error = err
			return result
		}
		result.Functions = findResult.Functions
		result.Classes = findResult.Classes

		// Also find structs if language supports it
		if langConfig.HasStructSupport() {
			factory := NewStructFinderFactory()
			structFinder := factory.CreateStructFinder(langConfig, "", true, false)
			structResult, err := structFinder.FindStructures(job.Path)
			if err == nil {
				// Dedup: only add types not already in Classes (from class_pattern)
				seen := make(map[string]bool, len(result.Classes))
				for _, c := range result.Classes {
					seen[c.Name+":"+strconv.Itoa(c.Start)] = true
				}
				for _, typ := range structResult.Types {
					key := typ.Name + ":" + strconv.Itoa(typ.Start)
					if !seen[key] {
						result.Classes = append(result.Classes, ClassBounds{
							Name:  typ.Name,
							Start: typ.Start,
							End:   typ.End,
						})
					}
				}
			}
		}
	}

	return result
}

// AggregateDirResults aggregates results from multiple files
func AggregateDirResults(results []DirResult, jsonOut, treeMode, treeFull bool) string {
	if jsonOut {
		return formatDirResultsJSON(results)
	}

	if treeMode || treeFull {
		return formatDirResultsTree(results, treeFull)
	}

	return formatDirResultsGrep(results)
}

func formatDirResultsJSON(results []DirResult) string {
	totalFuncs := 0
	totalClasses := 0

	type FileResult struct {
		Path      string           `json:"path"`
		Functions []FunctionBounds `json:"functions"`
		Classes   []ClassBounds    `json:"classes,omitempty"`
	}

	files := make([]FileResult, 0, len(results))
	for _, r := range results {
		if len(r.Functions) > 0 || len(r.Classes) > 0 {
			files = append(files, FileResult{
				Path:      r.Path,
				Functions: r.Functions,
				Classes:   r.Classes,
			})
			totalFuncs += len(r.Functions)
			totalClasses += len(r.Classes)
		}
	}

	// Simple JSON formatting without external dependency
	jsonStr := "{\n"
	jsonStr += "  \"files\": [\n"
	for i, f := range files {
		jsonStr += "    {\n"
		jsonStr += "      \"path\": \"" + escapeJSON(f.Path) + "\",\n"
		jsonStr += "      \"functions\": ["
		for j, fn := range f.Functions {
			if j > 0 {
				jsonStr += ", "
			}
			jsonStr += "{\"name\": \"" + escapeJSON(fn.Name) + "\", \"line\": " + itoa(fn.Start) + "}"
		}
		jsonStr += "],\n"
		jsonStr += "      \"classes\": ["
		for j, c := range f.Classes {
			if j > 0 {
				jsonStr += ", "
			}
			jsonStr += "{\"name\": \"" + escapeJSON(c.Name) + "\", \"line\": " + itoa(c.Start) + "}"
		}
		jsonStr += "]\n"
		jsonStr += "    }"
		if i < len(files)-1 {
			jsonStr += ","
		}
		jsonStr += "\n"
	}
	jsonStr += "  ],\n"
	jsonStr += "  \"total_files\": " + itoa(len(files)) + ",\n"
	jsonStr += "  \"total_functions\": " + itoa(totalFuncs) + ",\n"
	jsonStr += "  \"total_classes\": " + itoa(totalClasses) + "\n"
	jsonStr += "}\n"

	return jsonStr
}

func formatDirResultsTree(results []DirResult, full bool) string {
	if len(results) == 0 {
		return "No functions found"
	}

	root := &DirTreeNode{Children: make(map[string]*DirTreeNode)}

	for _, r := range results {
		if len(r.Functions) == 0 && len(r.Classes) == 0 {
			continue
		}

		relPath, err := filepath.Rel(".", r.Path)
		if err != nil {
			relPath = r.Path
		}

		// Build tree structure
		parts := strings.Split(relPath, string(filepath.Separator))
		current := root

		for i := 0; i < len(parts)-1; i++ {
			part := parts[i]
			if current.Children[part] == nil {
				current.Children[part] = &DirTreeNode{
					Path:     filepath.Join(current.Path, part),
					Children: make(map[string]*DirTreeNode),
				}
			}
			current = current.Children[part]
		}

		// Add file to current directory
		filename := parts[len(parts)-1]
		current.Children[filename] = &DirTreeNode{
			Path:      relPath,
			Functions: r.Functions,
			Classes:   r.Classes,
		}
	}

	// Build tree output
	return buildTreeOutput(root, "", true)
}

func buildTreeOutput(node *DirTreeNode, prefix string, isLast bool) string {
	var output string

	// Determine connector
	connector := "├── "
	if isLast {
		connector = "└── "
	}

	if node.Path != "" {
		output += prefix + connector + filepath.Base(node.Path) + "\n"
		newPrefix := prefix
		if isLast {
			newPrefix += "    "
		} else {
			newPrefix += "│   "
		}

		// Add functions and classes for files
		if len(node.Functions) > 0 || len(node.Classes) > 0 {
			for i, fn := range node.Functions {
				funcPrefix := newPrefix + "├── "
				if i == len(node.Functions)+len(node.Classes)-1 {
					funcPrefix = newPrefix + "└── "
				}
				output += funcPrefix + "def " + fn.Name + " (line " + itoa(fn.Start) + ")\n"
			}
			for i, c := range node.Classes {
				classPrefix := newPrefix + "├── "
				if i == len(node.Classes)-1 && len(node.Functions) == 0 {
					classPrefix = newPrefix + "└── "
				}
				output += classPrefix + "class " + c.Name + " (line " + itoa(c.Start) + ")\n"
			}
		}
	}

	// Process children
	children := make([]*DirTreeNode, 0, len(node.Children))
	for _, child := range node.Children {
		children = append(children, child)
	}

	for i, child := range children {
		output += buildTreeOutput(child, prefix, i == len(children)-1)
	}

	return output
}

func formatDirResultsGrep(results []DirResult) string {
	var output string
	for _, r := range results {
		for _, fn := range r.Functions {
			output += r.Path + ":" + itoa(fn.Start) + ": " + fn.Name + "\n"
		}
	}
	return output
}

// IgnoreMatcher handles .gitignore pattern matching
type IgnoreMatcher struct {
	patterns []ignorePattern
	root     string
}

type ignorePattern struct {
	regex     *regexp.Regexp
	directory bool // pattern ends with /
}

func NewIgnoreMatcher(root string) *IgnoreMatcher {
	m := &IgnoreMatcher{
		root:     root,
		patterns: make([]ignorePattern, 0),
	}

	// Load .gitignore from root
	gitignorePath := filepath.Join(root, ".gitignore")
	data, err := os.ReadFile(gitignorePath)
	if err != nil {
		return m
	}

	// Parse patterns
	m.parsePatterns(string(data))
	return m
}

func (m *IgnoreMatcher) parsePatterns(content string) {
	lines := regexp.MustCompile(`\r?\n`).Split(content, -1)
	for _, line := range lines {
		line = regexp.MustCompile(`#.*`).ReplaceAllString(line, "")
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Handle negation
		negate := false
		if strings.HasPrefix(line, "!") {
			negate = true
			line = strings.TrimSpace(line[1:])
		}

		if line == "" {
			continue
		}

		// Handle directory patterns
		isDir := strings.HasSuffix(line, "/")
		if isDir {
			line = strings.TrimSuffix(line, "/")
		}

		// Convert gitignore pattern to regex
		regexStr := m.patternToRegex(line)
		re, err := regexp.Compile(regexStr)
		if err != nil {
			continue
		}

		if !negate {
			m.patterns = append(m.patterns, ignorePattern{
				regex:     re,
				directory: isDir,
			})
		}
	}
}

func (m *IgnoreMatcher) patternToRegex(pattern string) string {
	// Escape regex special characters except * and ?
	regex := regexp.MustCompile(`([.+?^${}()|[\]\\])`).ReplaceAllStringFunc(pattern, func(match string) string {
		return "\\" + match
	})

	// Handle ** glob pattern
	regex = strings.ReplaceAll(regex, `\*\*`, ".*")

	// Handle single * glob pattern
	regex = strings.ReplaceAll(regex, `\*`, "[^/]*")

	// Handle ? glob pattern
	regex = strings.ReplaceAll(regex, `\?`, ".")

	// Match full path or as subdirectory
	if strings.HasPrefix(pattern, "/") {
		return "^" + strings.TrimPrefix(regex, "/") + "$"
	}

	return "(^|/)" + regex + "($|/)"
}

func (m *IgnoreMatcher) Matches(path string, isDir bool) bool {
	for _, p := range m.patterns {
		if p.directory && !isDir {
			continue
		}
		if p.regex.MatchString(path) {
			return true
		}
	}
	return false
}

// Helper functions for JSON formatting
func escapeJSON(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, "\r", "\\r")
	s = strings.ReplaceAll(s, "\t", "\\t")
	return s
}

func itoa(n int) string {
	return strconv.Itoa(n)
}

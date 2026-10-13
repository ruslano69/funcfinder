// dirprocessor.go - Directory processing with parallel execution and .gitignore support
package internal

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
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
	LangKey   string // language key the file was processed with (as in languages.json)
	Functions []FunctionBounds
	Classes   []ClassBounds
	Aliases   []AliasBounds // Go type aliases: names of existing types, not types of their own
	Error     error
}

// DirProcessor handles directory traversal and parallel file processing
type DirProcessor struct {
	config       Config
	workers      int
	recursive    bool
	useGitignore bool
	workMode     string         // "functions", "structs", or "all"
	skipped      map[string]int // extensions of the files the last walk skipped: no language is configured for them
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
	reportSkipped(dp.skipped)

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
	dp.skipped = make(map[string]int)

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
		// except for .gitignore itself and the root path itself
		base := filepath.Base(path)
		if len(base) > 0 && base[0] == '.' && base != ".gitignore" && path != rootPath {
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
			dp.skipped[strings.ToLower(filepath.Ext(path))]++
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

// skippedNotice names the files a walk skipped because no language is configured for their extension: the most files
// first, at most six kinds. It returns "" when nothing was skipped.
func skippedNotice(skipped map[string]int) string {
	if len(skipped) == 0 {
		return ""
	}
	type kind struct {
		ext string
		n   int
	}
	var kinds []kind
	total := 0
	for ext, n := range skipped {
		kinds = append(kinds, kind{ext, n})
		total += n
	}
	sort.Slice(kinds, func(i, j int) bool {
		if kinds[i].n != kinds[j].n {
			return kinds[i].n > kinds[j].n
		}
		return kinds[i].ext < kinds[j].ext
	})

	const maxKinds = 6
	var parts []string
	for i, k := range kinds {
		if i == maxKinds {
			parts = append(parts, "...")
			break
		}
		name := k.ext
		if name == "" {
			name = "(no extension)"
		}
		parts = append(parts, fmt.Sprintf("%s %d", name, k.n))
	}
	return fmt.Sprintf("skipped %d files with no language for their extension: %s", total, strings.Join(parts, ", "))
}

// reportSkipped prints skippedNotice to stderr, as an INFO line, when a walk skipped anything.
func reportSkipped(skipped map[string]int) {
	if msg := skippedNotice(skipped); msg != "" {
		InfoMessage("%s", msg)
	}
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
	reportFileErrors(os.Stderr, results)

	return results, nil
}

// reportFileErrors prints one warning per file that could not be processed. The other files' results are kept:
// a file that fails must not disappear without a word.
func reportFileErrors(w io.Writer, results []DirResult) {
	for _, r := range results {
		if r.Error != nil {
			fmt.Fprintf(w, "Warning: %s: %v\n", r.Path, r.Error)
		}
	}
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
		Path:    job.Path,
		LangKey: job.LangKey,
	}

	langConfig, err := dp.config.GetLanguageConfig(job.LangKey)
	if err != nil {
		result.Error = err
		return result
	}

	// The file is read at most once per job: the finders that take lines and the struct finder share it.
	fileLines := lazyLines(job.Path)

	switch dp.workMode {
	case "functions":
		// Find only functions
		finder := CreateFinder(langConfig, "", "map", false, false)
		findResult, err := findFunctionsOf(finder, job.Path, fileLines)
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
		structResult, err := findStructuresOf(structFinder, job.Path, fileLines)
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
				Kind:  typ.Kind,
			})
		}

	case "all":
		// Find both functions and structs
		finder := CreateFinder(langConfig, "", "map", false, false)
		findResult, err := findFunctionsOf(finder, job.Path, fileLines)
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
			structResult, err := findStructuresOf(structFinder, job.Path, fileLines)
			if err == nil {
				// Dedup: a type already in Classes (from class_pattern) keeps its entry and takes the struct finder's kind
				index := make(map[string]int, len(result.Classes))
				for i, c := range result.Classes {
					index[c.Name+":"+strconv.Itoa(c.Start)] = i
				}
				for _, typ := range structResult.Types {
					key := typ.Name + ":" + strconv.Itoa(typ.Start)
					if i, ok := index[key]; ok {
						result.Classes[i].Kind = typ.Kind
						continue
					}
					result.Classes = append(result.Classes, ClassBounds{
						Name:  typ.Name,
						Start: typ.Start,
						End:   typ.End,
						Kind:  typ.Kind,
					})
				}
			}
		}
	}

	// Go type aliases are names for existing types: they are listed apart from the types.
	if job.LangKey == "go" && dp.workMode != "functions" {
		lines, err := fileLines()
		if err != nil {
			result.Error = err
			return result
		}
		// Cleaned first: a line inside a string or a comment is not a declaration.
		cleaned := NewSanitizer(langConfig, false).CleanLines(lines)
		result.Aliases = findGoAliases(cleaned)
	}

	return result
}

// lazyLines returns a reader of the file's lines: the file is read on the first call, and the result is kept.
func lazyLines(path string) func() ([]string, error) {
	var lines []string
	var err error
	loaded := false
	return func() ([]string, error) {
		if !loaded {
			lines, err = readFileLines(path)
			loaded = true
		}
		return lines, err
	}
}

// linesFinder is a finder that can run on lines the caller has already read.
type linesFinder interface {
	FindFunctionsInLines(lines []string, startLine int, filename string) (*FindResult, error)
}

// findFunctionsOf runs a function finder on a file. A finder that takes lines gets the shared lines; the others
// (Python) read the file themselves, as before.
func findFunctionsOf(finder LanguageFinder, path string, fileLines func() ([]string, error)) (*FindResult, error) {
	lf, ok := finder.(linesFinder)
	if !ok {
		return finder.FindFunctions(path)
	}
	lines, err := fileLines()
	if err != nil {
		return nil, err
	}
	return lf.FindFunctionsInLines(lines, 1, path)
}

// findStructuresOf runs a struct finder on the shared lines of a file.
func findStructuresOf(sf StructFinderInterface, path string, fileLines func() ([]string, error)) (*StructFindResult, error) {
	lines, err := fileLines()
	if err != nil {
		return nil, err
	}
	return sf.FindStructuresInLines(lines, 1, path)
}

// AggregateDirResults aggregates results from multiple files
func AggregateDirResults(results []DirResult, jsonOut, treeMode, treeFull bool) string {
	// Files are reported in path order: workers finish in any order, and the output must not depend on that.
	results = append([]DirResult(nil), results...)
	sort.SliceStable(results, func(i, j int) bool { return results[i].Path < results[j].Path })

	if jsonOut {
		return formatDirResultsJSON(results)
	}

	if treeMode || treeFull {
		return formatDirResultsTree(results, treeFull)
	}

	return formatDirResultsGrep(results)
}

// jsonSymbol is the on-disk shape for a mapped function or type: just its name
// and starting line, not the internal Start/End/Lines/Decorators fields.
type jsonSymbol struct {
	Name string `json:"name"`
	Line int    `json:"line"`
}

// jsonAlias is a Go type alias: a name, the line it is declared on, and the type it names.
type jsonAlias struct {
	Name   string `json:"name"`
	Line   int    `json:"line"`
	Target string `json:"target"`
}

type jsonFile struct {
	Path      string       `json:"path"`
	Functions []jsonSymbol `json:"functions"`
	Classes   []jsonSymbol `json:"classes"`
	Aliases   []jsonAlias  `json:"aliases,omitempty"`
}

type jsonDirResults struct {
	Files          []jsonFile `json:"files"`
	TotalFiles     int        `json:"total_files"`
	TotalFunctions int        `json:"total_functions"`
	TotalClasses   int        `json:"total_classes"`
}

func formatDirResultsJSON(results []DirResult) string {
	out := jsonDirResults{Files: []jsonFile{}}
	for _, r := range results {
		if len(r.Functions) == 0 && len(r.Classes) == 0 && len(r.Aliases) == 0 {
			continue
		}
		jf := jsonFile{
			Path:      r.Path,
			Functions: make([]jsonSymbol, 0, len(r.Functions)),
			Classes:   make([]jsonSymbol, 0, len(r.Classes)),
		}
		for _, fn := range r.Functions {
			jf.Functions = append(jf.Functions, jsonSymbol{Name: fn.Name, Line: fn.Start})
		}
		for _, a := range r.Aliases {
			jf.Aliases = append(jf.Aliases, jsonAlias{Name: a.Name, Line: a.Start, Target: a.Target})
		}
		for _, c := range r.Classes {
			jf.Classes = append(jf.Classes, jsonSymbol{Name: c.Name, Line: c.Start})
		}
		out.Files = append(out.Files, jf)
		out.TotalFiles++
		out.TotalFunctions += len(r.Functions)
		out.TotalClasses += len(r.Classes)
	}

	b, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return "{}\n"
	}
	return string(b) + "\n"
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
				output += funcPrefix + "def " + fn.Name + " (line " + strconv.Itoa(fn.Start) + ")\n"
			}
			for i, c := range node.Classes {
				classPrefix := newPrefix + "├── "
				if i == len(node.Classes)-1 && len(node.Functions) == 0 {
					classPrefix = newPrefix + "└── "
				}
				output += classPrefix + "class " + c.Name + " (line " + strconv.Itoa(c.Start) + ")\n"
			}
		}
	}

	// Process children in name order: a map's iteration order is random, and the tree must not change between runs.
	names := make([]string, 0, len(node.Children))
	for name := range node.Children {
		names = append(names, name)
	}
	sort.Strings(names)
	children := make([]*DirTreeNode, 0, len(names))
	for _, name := range names {
		children = append(children, node.Children[name])
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
			output += r.Path + ":" + strconv.Itoa(fn.Start) + ": " + fn.Name + "\n"
		}
		for _, cl := range r.Classes {
			output += r.Path + ":" + strconv.Itoa(cl.Start) + ": " + cl.Name + "\n"
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

// CollectSourceFiles finds all source files matching langConfig in rootPath.
// It optionally respects .gitignore, skips hidden files/dirs, and honours recursive.
// If langConfig is nil every supported extension is accepted.
func CollectSourceFiles(rootPath string, langConfig *LanguageConfig, recursive bool, useGitignore ...bool) ([]string, error) {
	respectGitignore := len(useGitignore) == 0 || useGitignore[0]
	var ignoreMatcher *IgnoreMatcher
	if respectGitignore {
		ignoreMatcher = NewIgnoreMatcher(rootPath)
	}

	extSet := make(map[string]bool)
	if langConfig != nil {
		for _, ext := range langConfig.Extensions {
			extSet[ext] = true
		}
	}

	var files []string
	err := filepath.Walk(rootPath, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		relPath, err := filepath.Rel(rootPath, path)
		if err != nil {
			return nil
		}

		if ignoreMatcher != nil && ignoreMatcher.Matches(relPath, info.IsDir()) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		base := filepath.Base(path)
		if len(base) > 0 && base[0] == '.' && base != ".gitignore" && path != rootPath {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if info.IsDir() {
			if !recursive && path != rootPath {
				return filepath.SkipDir
			}
			return nil
		}

		if langConfig == nil {
			files = append(files, path)
			return nil
		}
		if extSet[filepath.Ext(path)] {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

// ShardInfo represents metadata about a single shard file
type ShardInfo struct {
	Path           string   `json:"path"`
	Files          int      `json:"files"`
	TotalFunctions int      `json:"total_functions"`
	TotalClasses   int      `json:"total_classes"`
	Checksum       string   `json:"checksum,omitempty"`
	DependsOn      []string `json:"depends_on,omitempty"`
}

// Manifest represents the index of all shards
type Manifest struct {
	Version        string      `json:"version"`
	RootDir        string      `json:"root_dir"`
	SplitBy        string      `json:"split_by"`
	Shards         []ShardInfo `json:"shards"`
	TotalFiles     int         `json:"total_files"`
	TotalFunctions int         `json:"total_functions"`
	TotalClasses   int         `json:"total_classes"`
	// EmptyShards maps the name of each shard whose files hold no declaration to its checksum. Such a shard has no shard
	// file and no entry in Shards; the checksum lets an incremental run see it as unchanged.
	EmptyShards map[string]string `json:"empty_shards,omitempty"`
}

// emptyShardChecksums returns the checksum of each shard in allShardPaths that has no shard file: the shards that
// written does not name.
func emptyShardChecksums(allShardPaths map[string][]string, written map[string]bool) map[string]string {
	var empty map[string]string
	for name, paths := range allShardPaths {
		if written[name] {
			continue
		}
		if empty == nil {
			empty = make(map[string]string)
		}
		empty[name] = computeShardChecksum(paths)
	}
	return empty
}

// pathToShardName is an alias for the exported PathToShardName in shardutil.go.
func pathToShardName(path string) string { return PathToShardName(path) }

// WriteSplitOutput writes results to split shard files with a manifest
func WriteSplitOutput(results []DirResult, outDir, rootDir, splitBy string) (string, error) {
	// Create output directory
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return "", fmt.Errorf("creating output directory: %w", err)
	}

	// Group results by shard key.
	// allShardPaths includes ALL files (for checksum), shards includes only non-empty (for output).
	shards := make(map[string][]DirResult)
	allShardPaths := make(map[string][]string)

	for _, r := range results {
		relPath, err := filepath.Rel(rootDir, r.Path)
		if err != nil {
			relPath = r.Path
		}

		var shardKey string
		if splitBy == "file" {
			shardKey = relPath
		} else {
			shardKey = filepath.Dir(relPath)
			if shardKey == "." {
				shardKey = ""
			}
		}

		// ALL files contribute to checksum
		shardName := pathToShardName(shardKey)
		allShardPaths[shardName] = append(allShardPaths[shardName], r.Path)

		// Only files with results go into shard content
		if len(r.Functions) > 0 || len(r.Classes) > 0 {
			shards[shardKey] = append(shards[shardKey], r)
		}
	}

	// Write each shard and collect manifest info
	var manifest Manifest
	manifest.Version = "1.0"
	manifest.RootDir = rootDir
	manifest.SplitBy = splitBy

	for shardKey, shardResults := range shards {
		shardName := pathToShardName(shardKey)
		shardPath := filepath.Join(outDir, shardName)

		// Generate JSON content for this shard
		content := formatDirResultsJSON(shardResults)

		// Write shard file
		if err := os.WriteFile(shardPath, []byte(content), 0644); err != nil {
			return "", fmt.Errorf("writing shard %s: %w", shardName, err)
		}

		// Count totals; checksum covers ALL files in the shard directory
		totalFuncs := 0
		totalClasses := 0
		for _, r := range shardResults {
			totalFuncs += len(r.Functions)
			totalClasses += len(r.Classes)
		}
		checksum := computeShardChecksum(allShardPaths[shardName])

		manifest.Shards = append(manifest.Shards, ShardInfo{
			Path:           shardName,
			Files:          len(shardResults),
			TotalFunctions: totalFuncs,
			TotalClasses:   totalClasses,
			Checksum:       checksum,
		})

		manifest.TotalFiles += len(shardResults)
		manifest.TotalFunctions += totalFuncs
		manifest.TotalClasses += totalClasses
	}

	// Shards of files with no declaration have no shard file: their checksum goes into the manifest instead.
	written := make(map[string]bool, len(shards))
	for shardKey := range shards {
		written[pathToShardName(shardKey)] = true
	}
	manifest.EmptyShards = emptyShardChecksums(allShardPaths, written)

	// Write manifest.json
	manifestJSON := formatManifestJSON(&manifest)
	manifestPath := filepath.Join(outDir, "manifest.json")
	if err := os.WriteFile(manifestPath, []byte(manifestJSON), 0644); err != nil {
		return "", fmt.Errorf("writing manifest: %w", err)
	}

	// Return summary for display
	return fmt.Sprintf("Split output written to %s/\n  Manifest: manifest.json\n  Shards: %d files\n  Total: %d files, %d functions, %d classes",
		outDir, len(manifest.Shards), manifest.TotalFiles, manifest.TotalFunctions, manifest.TotalClasses), nil
}

// formatManifestJSON serialises the manifest with encoding/json. This matches
// byte-for-byte what `deps --update-manifest` writes (cmd/deps applyGraphToManifest
// also uses json.MarshalIndent with the same indent), so the manifest format is
// now identical no matter which tool wrote it last.
func formatManifestJSON(m *Manifest) string {
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return "{}\n"
	}
	return string(b) + "\n"
}

// loadManifest loads existing manifest from outDir
func loadManifest(outDir string) (*Manifest, error) {
	manifestPath := filepath.Join(outDir, "manifest.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, err
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

// ProcessDirectoryIncremental processes only changed files based on shard checksums
func (dp *DirProcessor) ProcessDirectoryIncremental(rootPath, outDir, splitBy string) ([]DirResult, error) {
	// Load existing manifest if present
	oldManifest, _ := loadManifest(outDir)
	oldChecksums := make(map[string]string)
	if oldManifest != nil {
		for _, s := range oldManifest.Shards {
			oldChecksums[s.Path] = s.Checksum
		}
		for name, sum := range oldManifest.EmptyShards {
			oldChecksums[name] = sum
		}
	}

	// Collect all files first
	files, err := dp.collectFiles(rootPath)
	if err != nil {
		return nil, err
	}
	reportSkipped(dp.skipped)

	if len(files) == 0 {
		return []DirResult{}, nil
	}

	// Group files by shard key
	shardFiles := make(map[string][]Job)
	for _, job := range files {
		relPath, err := filepath.Rel(rootPath, job.Path)
		if err != nil {
			relPath = job.Path
		}

		var shardKey string
		if splitBy == "file" {
			shardKey = relPath
		} else {
			shardKey = filepath.Dir(relPath)
			if shardKey == "." {
				shardKey = ""
			}
		}
		shardFiles[shardKey] = append(shardFiles[shardKey], job)
	}

	// Compute checksums and filter changed shards
	var changedJobs []Job
	changedShards := 0
	unchangedShards := 0

	for shardKey, jobs := range shardFiles {
		shardName := pathToShardName(shardKey)

		// Collect file paths for checksum
		var paths []string
		for _, j := range jobs {
			paths = append(paths, j.Path)
		}
		newChecksum := computeShardChecksum(paths)

		// Compare with old checksum
		if oldChecksum, exists := oldChecksums[shardName]; exists && oldChecksum == newChecksum {
			unchangedShards++
			continue
		}

		changedShards++
		changedJobs = append(changedJobs, jobs...)
	}

	InfoMessage("Incremental: %d shards changed, %d unchanged", changedShards, unchangedShards)

	if len(changedJobs) == 0 {
		// Nothing changed, return empty but load existing results
		return []DirResult{}, nil
	}

	// Process only changed files
	return dp.processFilesParallel(changedJobs)
}

// WriteSplitOutputIncremental writes results merging with unchanged shards from old manifest
func WriteSplitOutputIncremental(results []DirResult, outDir, rootDir, splitBy string) (string, error) {
	// Load old manifest
	oldManifest, _ := loadManifest(outDir)

	// Create output directory
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return "", fmt.Errorf("creating output directory: %w", err)
	}

	// Group new results by shard key.
	// allShardPaths tracks ALL files per shard (for checksum), newShards only non-empty (for output).
	newShards := make(map[string][]DirResult)
	allShardPaths := make(map[string][]string)
	for _, r := range results {
		relPath, err := filepath.Rel(rootDir, r.Path)
		if err != nil {
			relPath = r.Path
		}

		var shardKey string
		if splitBy == "file" {
			shardKey = relPath
		} else {
			shardKey = filepath.Dir(relPath)
			if shardKey == "." {
				shardKey = ""
			}
		}
		shardName := pathToShardName(shardKey)
		allShardPaths[shardName] = append(allShardPaths[shardName], r.Path)
		if len(r.Functions) > 0 || len(r.Classes) > 0 {
			newShards[shardKey] = append(newShards[shardKey], r)
		}
	}

	// Build manifest with both updated and unchanged shards
	var manifest Manifest
	manifest.Version = "1.0"
	manifest.RootDir = rootDir
	manifest.SplitBy = splitBy

	// Every re-processed shard is updated: rewritten if it still holds a declaration, kept as a checksum otherwise.
	updatedShardNames := make(map[string]bool, len(allShardPaths))
	for name := range allShardPaths {
		updatedShardNames[name] = true
	}

	// Write updated shards
	for shardKey, shardResults := range newShards {
		shardName := pathToShardName(shardKey)
		shardPath := filepath.Join(outDir, shardName)

		content := formatDirResultsJSON(shardResults)
		if err := os.WriteFile(shardPath, []byte(content), 0644); err != nil {
			return "", fmt.Errorf("writing shard %s: %w", shardName, err)
		}

		totalFuncs := 0
		totalClasses := 0
		for _, r := range shardResults {
			totalFuncs += len(r.Functions)
			totalClasses += len(r.Classes)
		}
		checksum := computeShardChecksum(allShardPaths[shardName])

		manifest.Shards = append(manifest.Shards, ShardInfo{
			Path:           shardName,
			Files:          len(shardResults),
			TotalFunctions: totalFuncs,
			TotalClasses:   totalClasses,
			Checksum:       checksum,
		})

		manifest.TotalFiles += len(shardResults)
		manifest.TotalFunctions += totalFuncs
		manifest.TotalClasses += totalClasses
	}

	// A re-processed shard that no longer holds a declaration loses its shard file from an earlier run.
	written := make(map[string]bool, len(newShards))
	for shardKey := range newShards {
		written[pathToShardName(shardKey)] = true
	}
	if oldManifest != nil {
		for _, oldShard := range oldManifest.Shards {
			if updatedShardNames[oldShard.Path] && !written[oldShard.Path] {
				if err := os.Remove(filepath.Join(outDir, oldShard.Path)); err != nil && !os.IsNotExist(err) {
					return "", fmt.Errorf("removing shard %s: %w", oldShard.Path, err)
				}
			}
		}
	}
	manifest.EmptyShards = emptyShardChecksums(allShardPaths, written)
	if oldManifest != nil {
		for name, sum := range oldManifest.EmptyShards {
			if !updatedShardNames[name] {
				if manifest.EmptyShards == nil {
					manifest.EmptyShards = make(map[string]string)
				}
				manifest.EmptyShards[name] = sum
			}
		}
	}

	// Copy unchanged shards from old manifest
	if oldManifest != nil {
		for _, oldShard := range oldManifest.Shards {
			if !updatedShardNames[oldShard.Path] {
				manifest.Shards = append(manifest.Shards, oldShard)
				manifest.TotalFiles += oldShard.Files
				manifest.TotalFunctions += oldShard.TotalFunctions
				manifest.TotalClasses += oldShard.TotalClasses
			}
		}
	}

	// Write manifest
	manifestJSON := formatManifestJSON(&manifest)
	manifestPath := filepath.Join(outDir, "manifest.json")
	if err := os.WriteFile(manifestPath, []byte(manifestJSON), 0644); err != nil {
		return "", fmt.Errorf("writing manifest: %w", err)
	}

	updatedCount := len(newShards)
	unchangedCount := len(manifest.Shards) - updatedCount

	return fmt.Sprintf("Split output written to %s/\n  Manifest: manifest.json\n  Shards: %d updated, %d unchanged\n  Total: %d files, %d functions, %d classes",
		outDir, updatedCount, unchangedCount, manifest.TotalFiles, manifest.TotalFunctions, manifest.TotalClasses), nil
}

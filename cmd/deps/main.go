// deps.go - Module dependency analyzer
// Uses shared configuration for multiple languages
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/ruslano69/funcfinder/internal"
)

type DepInfo struct {
	Module string   `json:"module"`
	Count  int      `json:"count"`
	Files  []string `json:"files"`
}

type DepResult struct {
	Language           string         `json:"language"`
	TotalImports       int            `json:"total_imports"`
	UniqueModules      int            `json:"unique_modules"`
	Dependencies       []DepInfo      `json:"dependencies"`
	ExternalVsInternal map[string]int `json:"external_vs_internal"`
}

type fileSet map[string]bool

// Stdlib detection for common languages
var stdlibPrefixes = map[string][]string{
	"go":    {"fmt", "os", "io", "strings", "math", "regexp", "encoding/json", "testing", "bytes", "errors"},
	"rs":    {"std::", "core::"},
	"js":    {"assert", "buffer", "crypto", "fs", "http", "path", "url"},
	"ts":    {"assert", "buffer", "crypto", "fs", "http", "path", "url"},
	"java":  {"java.", "javax."},
	"cs":    {"System.", "Microsoft."},
	"c":     {"stdio", "stdlib", "string", "math"},
	"cpp":   {"iostream", "vector", "string", "algorithm"},
	"d":     {"std."},
	"swift": {"Swift", "Foundation"},
}

// pythonStdlibModules is the set of top-level Python 3 standard library
// module/package names. Matched against the leading dotted component of an
// import (e.g. "os.path" -> "os"), never as a raw substring/prefix — a naive
// prefix check would previously misclassify every import as stdlib because
// of a stray "" entry, and would still falsely match names like "osprey"
// against "os" if done as a plain HasPrefix.
var pythonStdlibModules = map[string]bool{
	"__future__": true, "__main__": true, "_thread": true,
	"abc": true, "aifc": true, "argparse": true, "array": true, "ast": true,
	"asyncio": true, "atexit": true, "base64": true, "bdb": true,
	"binascii": true, "bisect": true, "builtins": true, "bz2": true,
	"calendar": true, "cgi": true, "cgitb": true, "chunk": true, "cmath": true,
	"cmd": true, "code": true, "codecs": true, "codeop": true,
	"collections": true, "colorsys": true, "compileall": true,
	"concurrent": true, "configparser": true, "contextlib": true,
	"contextvars": true, "copy": true, "copyreg": true, "cProfile": true,
	"csv": true, "ctypes": true, "curses": true, "dataclasses": true,
	"datetime": true, "dbm": true, "decimal": true, "difflib": true,
	"dis": true, "distutils": true, "doctest": true, "email": true,
	"encodings": true, "ensurepip": true, "enum": true, "errno": true,
	"faulthandler": true, "fcntl": true, "filecmp": true, "fileinput": true,
	"fnmatch": true, "fractions": true, "ftplib": true, "functools": true,
	"gc": true, "getopt": true, "getpass": true, "gettext": true, "glob": true,
	"graphlib": true, "grp": true, "gzip": true, "hashlib": true, "heapq": true,
	"hmac": true, "html": true, "http": true, "imaplib": true, "imghdr": true,
	"imp": true, "importlib": true, "inspect": true, "io": true,
	"ipaddress": true, "itertools": true, "json": true, "keyword": true,
	"linecache": true, "locale": true, "logging": true, "lzma": true,
	"mailbox": true, "mailcap": true, "marshal": true, "math": true,
	"mimetypes": true, "mmap": true, "modulefinder": true, "multiprocessing": true,
	"netrc": true, "nntplib": true, "numbers": true, "operator": true,
	"optparse": true, "os": true, "pathlib": true, "pdb": true, "pickle": true,
	"pickletools": true, "pipes": true, "pkgutil": true, "platform": true,
	"plistlib": true, "poplib": true, "posix": true, "posixpath": true,
	"pprint": true, "profile": true, "pstats": true, "pty": true, "pwd": true,
	"py_compile": true, "pyclbr": true, "pydoc": true, "queue": true,
	"quopri": true, "random": true, "re": true, "readline": true,
	"reprlib": true, "resource": true, "rlcompleter": true, "runpy": true,
	"sched": true, "secrets": true, "select": true, "selectors": true,
	"shelve": true, "shlex": true, "shutil": true, "signal": true, "site": true,
	"smtplib": true, "sndhdr": true, "socket": true, "socketserver": true,
	"spwd": true, "sqlite3": true, "ssl": true, "stat": true,
	"statistics": true, "string": true, "stringprep": true, "struct": true,
	"subprocess": true, "sunau": true, "symtable": true, "sys": true,
	"sysconfig": true, "syslog": true, "tabnanny": true, "tarfile": true,
	"telnetlib": true, "tempfile": true, "termios": true, "textwrap": true,
	"threading": true, "time": true, "timeit": true, "tkinter": true,
	"token": true, "tokenize": true, "tomllib": true, "trace": true,
	"traceback": true, "tracemalloc": true, "tty": true, "turtle": true,
	"types": true, "typing": true, "unicodedata": true, "unittest": true,
	"urllib": true, "uu": true, "uuid": true, "venv": true, "warnings": true,
	"wave": true, "weakref": true, "webbrowser": true, "winreg": true,
	"winsound": true, "wsgiref": true, "xdrlib": true, "xml": true,
	"xmlrpc": true, "zipapp": true, "zipfile": true, "zipimport": true,
	"zlib": true, "zoneinfo": true,
}

func isStdlib(module, langKey string) bool {
	if langKey == "py" {
		top := module
		if idx := strings.IndexByte(module, '.'); idx >= 0 {
			top = module[:idx]
		}
		return pythonStdlibModules[top]
	}
	prefixes := stdlibPrefixes[langKey]
	for _, p := range prefixes {
		if strings.HasPrefix(module, p) {
			return true
		}
	}
	return false
}

// detectPythonLocalPackages scans the top level of the analyzed directory
// for first-party Python packages (a subdirectory containing __init__.py)
// or modules (a top-level *.py file), so that imports like
// "django.db.models" are recognized as internal to the project being
// scanned rather than lumped in with external third-party dependencies.
func detectPythonLocalPackages(dir string) map[string]bool {
	names := make(map[string]bool)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return names
	}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() {
			if _, err := os.Stat(filepath.Join(dir, name, "__init__.py")); err == nil {
				names[name] = true
			}
		} else if strings.HasSuffix(name, ".py") {
			names[strings.TrimSuffix(name, ".py")] = true
		}
	}
	return names
}

// classifyDep buckets a single import into "std", "int" (internal/local),
// or "ext" (external/third-party), for both the summary counts and the
// per-line annotation printed for each dependency.
func classifyDep(dep, langKey string, localPkgs map[string]bool) string {
	if isStdlib(dep, langKey) {
		return "std"
	}
	if langKey == "py" {
		// Python has no "internal/" or "vendor/" path convention to key
		// off of, and bare (dot-less) imports are just as often a
		// third-party package (import requests, import numpy) as they
		// are Go-style local packages — so the only reliable "internal"
		// signal is a match against what's actually on disk under the
		// scanned directory. Everything else, dotted or bare, is external.
		top := dep
		if idx := strings.IndexByte(dep, '.'); idx >= 0 {
			top = dep[:idx]
		}
		if localPkgs[top] {
			return "int"
		}
		return "ext"
	}
	if strings.Contains(dep, "internal/") || strings.Contains(dep, "vendor/") {
		return "int"
	}
	if strings.Contains(dep, "/") || strings.Contains(dep, ".") {
		return "ext"
	}
	return "int"
}

// collectFileImports returns all imports per file as map[absPath][]importedModule
func collectFileImports(filename string, config *internal.LanguageConfig, excludeREs []*regexp.Regexp) []string {
	var imports []string

	file, err := os.Open(filename)
	if err != nil {
		return imports
	}
	defer file.Close()

	importRe := config.ImportRegex()
	if importRe == nil {
		return imports
	}

	blockImportRe := regexp.MustCompile(`^\s*"([^"]+)"`)
	inBlock := false

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		// Handle multi-line import blocks (Go-style)
		if config.MultiLineBlock != "" && strings.HasPrefix(trimmed, config.MultiLineBlock) {
			inBlock = true
			continue
		}
		if inBlock {
			if trimmed == ")" {
				inBlock = false
			} else if match := blockImportRe.FindStringSubmatch(line); len(match) >= 2 {
				dep := match[1]
				if dep != "" && !strings.Contains(dep, "://") {
					imports = append(imports, dep)
				}
			}
			continue
		}

		skip := false
		for _, re := range excludeREs {
			if re.MatchString(trimmed) {
				skip = true
				break
			}
		}
		if skip {
			continue
		}

		if match := importRe.FindStringSubmatch(line); len(match) >= 2 {
			for i := 1; i < len(match); i++ {
				if match[i] != "" && !strings.Contains(match[i], "://") {
					dep := match[i]
					excluded := false
					for _, re := range excludeREs {
						if re.MatchString(dep) {
							excluded = true
							break
						}
					}
					if excluded {
						continue
					}
					if !strings.HasSuffix(dep, "/") {
						imports = append(imports, dep)
					}
				}
			}
		}
	}
	return imports
}

// analyzeDeps returns aggregated deps map (used by standard flat mode).
func analyzeDeps(filename string, config *internal.LanguageConfig, excludeREs []*regexp.Regexp) map[string]fileSet {
	deps := make(map[string]fileSet)
	for _, imp := range collectFileImports(filename, config, excludeREs) {
		if deps[imp] == nil {
			deps[imp] = make(fileSet)
		}
		deps[imp][filename] = true
	}
	return deps
}

func main() {
	showVersion := false
	dir := "."
	lang := ""
	topN := 0
	jsonOut := false
	shardsMode := false
	splitBy := "dir"
	updateManifest := ""
	noGitignore := false

	for i := 1; i < len(os.Args); i++ {
		arg := os.Args[i]
		switch {
		case arg == "-h" || arg == "--help":
			fmt.Println("Usage: deps [OPTIONS] <dir>")
			fmt.Println("  --version              Show version and exit")
			fmt.Println("  -l <lang>              Force language (py, go, rs, js, ts, java, cs, swift, c, cpp, d)")
			fmt.Println("  -n <num>               Show top N dependencies")
			fmt.Println("  -j, --json             Output JSON")
			fmt.Println("  --shards               Output inter-shard dependency graph")
			fmt.Println("  --split-by dir|file    Shard granularity (default: dir)")
			fmt.Println("  --update-manifest <p>  Write depends_on into existing manifest.json")
			fmt.Println("  --no-gitignore         Do not respect .gitignore rules")
			return
		case arg == "--version":
			showVersion = true
		case arg == "-l" && i+1 < len(os.Args):
			lang = os.Args[i+1]
			i++
		case arg == "-n" && i+1 < len(os.Args):
			fmt.Sscanf(os.Args[i+1], "%d", &topN)
			i++
		case arg == "-j" || arg == "--json":
			jsonOut = true
		case arg == "--shards":
			shardsMode = true
		case arg == "--split-by" && i+1 < len(os.Args):
			splitBy = os.Args[i+1]
			i++
		case arg == "--update-manifest" && i+1 < len(os.Args):
			updateManifest = os.Args[i+1]
			i++
		case arg == "--no-gitignore":
			noGitignore = true
		case !strings.HasPrefix(arg, "-"):
			dir = arg
		}
	}

	if showVersion {
		internal.PrintVersion("deps")
	}

	// Load shared configuration
	config, err := internal.LoadConfig()
	if err != nil {
		internal.FatalError("loading config: %v", err)
	}

	var langConfig *internal.LanguageConfig
	if lang != "" {
		langConfig, err = config.GetLanguageConfig(lang)
		if err != nil {
			internal.FatalError("%v\nSupported languages: %s", err, strings.Join(config.GetSupportedLanguages(), ", "))
		}
	} else {
		for _, lc := range config {
			for _, ext := range lc.Extensions {
				files, _ := filepath.Glob(filepath.Join(dir, "*"+ext))
				if len(files) > 0 {
					langConfig = lc
					break
				}
			}
			if langConfig != nil {
				break
			}
		}
	}

	if langConfig == nil {
		internal.FatalError("no supported files found in directory\nSupported languages: %s", strings.Join(config.GetSupportedLanguages(), ", "))
	}

	// Pre-compile ExcludePatterns
	var excludeREs []*regexp.Regexp
	for _, pattern := range langConfig.ExcludePatterns {
		excludeREs = append(excludeREs, regexp.MustCompile(pattern))
	}

	dirFiles, walkErr := internal.CollectSourceFiles(dir, langConfig, true, !noGitignore)
	if walkErr != nil {
		internal.FatalError("walking directory: %v", walkErr)
	}

	// ── Shard dependency graph mode ─────────────────────────────────────────
	if shardsMode || updateManifest != "" {
		absDir, err := filepath.Abs(dir)
		if err != nil {
			internal.FatalError("resolving directory: %v", err)
		}

		// Collect per-file imports
		fileImports := make(map[string][]string, len(dirFiles))
		for _, path := range dirFiles {
			abs, _ := filepath.Abs(path)
			fileImports[abs] = collectFileImports(path, langConfig, excludeREs)
		}

		// Auto-detect module prefix / aliases per language
		modulePrefix := ""
		var aliases map[string]string
		switch langConfig.LangKey {
		case "go":
			modulePrefix = internal.DetectModulePrefix(absDir)
		case "ts", "js":
			aliases = internal.DetectTSAliases(absDir)
			if len(aliases) == 0 {
				if tscPath := internal.DetectTSConfigAbove(absDir); tscPath != "" {
					fmt.Fprintf(os.Stderr,
						"WARNING: found %s above %s, but --shards only looks in the analyzed "+
							"root and one level below — path aliases from it won't resolve. "+
							"Re-run --shards from its directory.\n", tscPath, absDir)
				}
			}
		}

		graph, stats := internal.BuildShardGraph(absDir, splitBy, modulePrefix, fileImports, aliases)
		list := internal.ShardGraphToList(graph)

		if warning := stats.Warning(); warning != "" {
			fmt.Fprintf(os.Stderr, "WARNING: %s\n", warning)
		}

		if updateManifest != "" {
			if err := applyGraphToManifest(updateManifest, graph); err != nil {
				internal.FatalError("updating manifest: %v", err)
			}
			fmt.Fprintf(os.Stderr, "INFO: Updated %s with depends_on for %d shards\n", updateManifest, len(graph))
			if !jsonOut {
				return
			}
		}

		if jsonOut || shardsMode {
			out, _ := json.MarshalIndent(map[string]any{"shards": list}, "", "  ")
			fmt.Println(string(out))
			return
		}

		// Plain text fallback
		for _, sd := range list {
			if len(sd.DependsOn) == 0 {
				continue
			}
			fmt.Printf("%s → %s\n", sd.Shard, strings.Join(sd.DependsOn, ", "))
		}
		return
	}

	// ── Standard flat deps mode ──────────────────────────────────────────────
	allDeps := make(map[string]fileSet)
	for _, path := range dirFiles {
		fileDeps := analyzeDeps(path, langConfig, excludeREs)
		for dep, files := range fileDeps {
			if allDeps[dep] == nil {
				allDeps[dep] = make(fileSet)
			}
			for f := range files {
				allDeps[dep][f] = true
			}
		}
	}

	// For Python, "django.db.models"-style imports of the project's own
	// first-party packages must not be counted as external dependencies —
	// detect which top-level names under the scanned directory are actually
	// local packages/modules so classifyDep can tell them apart.
	var localPkgs map[string]bool
	if langConfig.LangKey == "py" {
		localPkgs = detectPythonLocalPackages(dir)
	}

	var deps []DepInfo
	stdlib, external, internalCount := 0, 0, 0

	for dep, files := range allDeps {
		fileList := make([]string, 0, len(files))
		for f := range files {
			fileList = append(fileList, f)
		}
		info := DepInfo{Module: dep, Count: len(fileList), Files: fileList}
		deps = append(deps, info)

		switch classifyDep(dep, langConfig.LangKey, localPkgs) {
		case "std":
			stdlib++
		case "int":
			internalCount++
		default:
			external++
		}
	}

	sort.Slice(deps, func(i, j int) bool { return deps[i].Count > deps[j].Count })

	if jsonOut {
		result := DepResult{
			Language:      langConfig.Name,
			TotalImports:  len(allDeps),
			UniqueModules: len(deps),
			Dependencies:  deps,
			ExternalVsInternal: map[string]int{
				"stdlib":   stdlib,
				"external": external,
				"internal": internalCount,
			},
		}
		jsonBytes, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(jsonBytes))
		return
	}

	fmt.Printf("Language: %s\n", langConfig.Name)
	fmt.Printf("Total imports: %d\n", len(allDeps))
	fmt.Printf("Unique modules: %d\n", len(deps))
	fmt.Println(strings.Repeat("-", 35))
	fmt.Printf("stdlib: %d, external: %d, internal: %d\n", stdlib, external, internalCount)
	fmt.Println(strings.Repeat("-", 35))

	printCount := len(deps)
	if topN > 0 && topN < printCount {
		printCount = topN
	}
	for i := 0; i < printCount; i++ {
		kind := classifyDep(deps[i].Module, langConfig.LangKey, localPkgs)
		fmt.Printf("%-30s %3d (%s)\n", deps[i].Module, deps[i].Count, kind)
	}
}

// applyGraphToManifest reads manifest.json, sets DependsOn per shard, rewrites.
func applyGraphToManifest(manifestPath string, graph internal.ShardGraph) error {
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("reading manifest: %w", err)
	}

	var m internal.Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return fmt.Errorf("parsing manifest: %w", err)
	}

	for i, s := range m.Shards {
		if deps, ok := graph[s.Path]; ok {
			list := make([]string, 0, len(deps))
			for d := range deps {
				list = append(list, d)
			}
			sort.Strings(list)
			m.Shards[i].DependsOn = list
		}
	}

	out, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling manifest: %w", err)
	}
	return os.WriteFile(manifestPath, append(out, '\n'), 0644)
}

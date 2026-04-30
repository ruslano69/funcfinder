package internal

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ShardGraph maps shard name → set of shard names it depends on.
type ShardGraph map[string]map[string]struct{}

// ShardDep is one entry in the JSON output.
type ShardDep struct {
	Shard      string   `json:"shard"`
	DependsOn  []string `json:"depends_on"`
	DependedBy []string `json:"depended_by"`
}

// BuildShardGraph resolves per-file imports to shard names and returns the
// dependency graph. Only intra-project imports are tracked.
//
// modulePrefix is stripped from Go-style absolute import paths to yield a
// relative path inside the project (e.g. "github.com/foo/bar" → "").
// Pass an empty string for languages that use relative imports (py, js, ts).
//
// aliases maps import prefix → directory relative to rootDir
// e.g. {"@/": "frontend/src/"} for a Next.js project using tsconfig paths.
func BuildShardGraph(
	rootDir string,
	splitBy string,
	modulePrefix string,
	fileImports map[string][]string, // absFilePath → []importedModule
	aliases ...map[string]string,
) ShardGraph {
	var aliasMap map[string]string
	if len(aliases) > 0 {
		aliasMap = aliases[0]
	}

	graph := make(ShardGraph)

	// Build a lookup: relative file path (slash) → shard name
	// so we can resolve relative imports to shards.
	relToShard := make(map[string]string)
	for absPath := range fileImports {
		key := ShardKeyForPath(absPath, rootDir, splitBy)
		shard := PathToShardName(key)
		rel, err := filepath.Rel(rootDir, absPath)
		if err == nil {
			relToShard[filepath.ToSlash(rel)] = shard
		}
	}

	for absPath, imports := range fileImports {
		srcKey := ShardKeyForPath(absPath, rootDir, splitBy)
		srcShard := PathToShardName(srcKey)

		if _, ok := graph[srcShard]; !ok {
			graph[srcShard] = make(map[string]struct{})
		}

		for _, imp := range imports {
			dstShard := resolveImportToShard(imp, absPath, rootDir, splitBy, modulePrefix, aliasMap, relToShard)
			if dstShard == "" || dstShard == srcShard {
				continue
			}
			graph[srcShard][dstShard] = struct{}{}
		}
	}
	return graph
}

// resolveImportToShard maps one import string to a shard name, or "" if
// the import is external / cannot be resolved.
func resolveImportToShard(
	imp, srcFile, rootDir, splitBy, modulePrefix string,
	aliasMap map[string]string,
	relToShard map[string]string,
) string {
	// --- Go-style: strip module prefix ---
	if modulePrefix != "" && strings.HasPrefix(imp, modulePrefix) {
		rel := strings.TrimPrefix(imp, modulePrefix)
		rel = strings.TrimPrefix(rel, "/")
		shard := shardForDir(rel, splitBy, relToShard)
		if shard != "" {
			return shard
		}
	}

	// --- Path alias (e.g. @/ → src/) ---
	if aliasMap != nil {
		for prefix, target := range aliasMap {
			if strings.HasPrefix(imp, prefix) {
				rel := filepath.ToSlash(target) + strings.TrimPrefix(imp, prefix)
				rel = strings.TrimSuffix(rel, "/")
				for _, candidate := range candidatePaths(filepath.Join(rootDir, filepath.FromSlash(rel))) {
					r, err := filepath.Rel(rootDir, candidate)
					if err == nil {
						if shard, ok := relToShard[filepath.ToSlash(r)]; ok {
							return shard
						}
					}
				}
				shard := shardForDir(rel, splitBy, relToShard)
				if shard != "" {
					return shard
				}
			}
		}
	}

	// --- Relative import (Python / JS / TS) ---
	if strings.HasPrefix(imp, ".") {
		srcDir := filepath.Dir(srcFile)
		// Normalise Python "from .foo import bar" → "foo"
		cleaned := strings.TrimLeft(imp, ".")
		cleaned = strings.ReplaceAll(cleaned, ".", string(filepath.Separator))
		abs := filepath.Join(srcDir, cleaned)
		// Try exact match and with common extensions
		for _, candidate := range candidatePaths(abs) {
			rel, err := filepath.Rel(rootDir, candidate)
			if err != nil {
				continue
			}
			relSlash := filepath.ToSlash(rel)
			if shard, ok := relToShard[relSlash]; ok {
				return shard
			}
		}
		// Fallback: treat cleaned path as a directory
		rel, err := filepath.Rel(rootDir, abs)
		if err == nil {
			return shardForDir(filepath.ToSlash(rel), splitBy, relToShard)
		}
	}

	return ""
}

// shardForDir returns the shard name for any file whose relative path starts
// with dirRel (slash-separated).
func shardForDir(dirRel, splitBy string, relToShard map[string]string) string {
	dirRel = filepath.ToSlash(dirRel)
	prefix := dirRel + "/"
	for rel, shard := range relToShard {
		if rel == dirRel || strings.HasPrefix(rel, prefix) {
			if splitBy == "file" {
				return shard
			}
			// For dir-mode all files in the dir share the same shard.
			return shard
		}
	}
	return ""
}

// candidatePaths returns likely file paths for an import without extension.
func candidatePaths(base string) []string {
	exts := []string{".go", ".py", ".js", ".ts", ".java", ".cs", ".rs", ".swift", ".kt", ".rb", ".php", ".scala"}
	paths := []string{base}
	for _, ext := range exts {
		paths = append(paths, base+ext)
	}
	// Also try base/index.{js,ts}
	paths = append(paths, filepath.Join(base, "index.js"), filepath.Join(base, "index.ts"))
	return paths
}

// ShardGraphToList converts the graph to a sorted slice for JSON output,
// including reverse "depended_by" edges.
func ShardGraphToList(graph ShardGraph) []ShardDep {
	// Build reverse index
	dependedBy := make(map[string]map[string]struct{})
	for src, dsts := range graph {
		for dst := range dsts {
			if dependedBy[dst] == nil {
				dependedBy[dst] = make(map[string]struct{})
			}
			dependedBy[dst][src] = struct{}{}
		}
	}

	// Collect all known shards
	all := make(map[string]struct{})
	for s, dsts := range graph {
		all[s] = struct{}{}
		for d := range dsts {
			all[d] = struct{}{}
		}
	}

	list := make([]ShardDep, 0, len(all))
	for shard := range all {
		dep := ShardDep{Shard: shard}
		for d := range graph[shard] {
			dep.DependsOn = append(dep.DependsOn, d)
		}
		for d := range dependedBy[shard] {
			dep.DependedBy = append(dep.DependedBy, d)
		}
		sort.Strings(dep.DependsOn)
		sort.Strings(dep.DependedBy)
		list = append(list, dep)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Shard < list[j].Shard })
	return list
}

// DetectTSAliases reads tsconfig.json paths and returns alias→dir mapping.
// Searches rootDir and common sub-paths (src/, frontend/).
// e.g. "@/*" → ["./src/*"] becomes {"@/": "src/"}
func DetectTSAliases(rootDir string) map[string]string {
	candidates := []string{
		filepath.Join(rootDir, "tsconfig.json"),
		filepath.Join(rootDir, "tsconfig.base.json"),
	}
	// Also search one level deep
	entries, _ := os.ReadDir(rootDir)
	for _, e := range entries {
		if e.IsDir() {
			candidates = append(candidates, filepath.Join(rootDir, e.Name(), "tsconfig.json"))
		}
	}

	aliases := make(map[string]string)
	for _, tscPath := range candidates {
		data, err := os.ReadFile(tscPath)
		if err != nil {
			continue
		}
		tscDir := filepath.Dir(tscPath)
		relDir, err := filepath.Rel(rootDir, tscDir)
		if err != nil {
			relDir = "."
		}
		parseTSConfigPaths(data, relDir, aliases)
	}
	return aliases
}

// parseTSConfigPaths is a minimal JSON parser for the "paths" section of tsconfig.
// It avoids bringing in encoding/json to keep the parser simple and fast.
func parseTSConfigPaths(data []byte, tscDir string, aliases map[string]string) {
	src := string(data)
	// Find "paths": {
	idx := strings.Index(src, `"paths"`)
	if idx < 0 {
		return
	}
	start := strings.Index(src[idx:], "{")
	if start < 0 {
		return
	}
	// Extract the object body (naive but sufficient for flat paths objects)
	body := src[idx+start+1:]
	end := strings.Index(body, "}")
	if end < 0 {
		return
	}
	body = body[:end]

	// Parse "alias": ["target", ...]
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if !strings.Contains(line, ":") {
			continue
		}
		kv := strings.SplitN(line, ":", 2)
		alias := strings.Trim(strings.TrimSpace(kv[0]), `"`)
		// Strip trailing /* from alias: "@/*" → "@/"
		alias = strings.TrimSuffix(alias, "*")

		val := strings.TrimSpace(kv[1])
		// Extract first string from array ["./src/*"]
		q1 := strings.Index(val, `"`)
		if q1 < 0 {
			continue
		}
		q2 := strings.Index(val[q1+1:], `"`)
		if q2 < 0 {
			continue
		}
		target := val[q1+1 : q1+1+q2]
		// Strip trailing /* from target: "./src/*" → "./src/"
		target = strings.TrimSuffix(target, "*")
		target = strings.TrimPrefix(target, "./")
		target = strings.TrimSuffix(target, "/")

		// Resolve relative to tsconfig location
		if tscDir != "." && tscDir != "" {
			target = filepath.ToSlash(filepath.Join(tscDir, target))
		}
		target = strings.TrimSuffix(target, "/") + "/"
		aliases[alias] = target
	}
}

// DetectModulePrefix tries to find the Go module path from go.mod in rootDir.
func DetectModulePrefix(rootDir string) string {
	data, err := os.ReadFile(filepath.Join(rootDir, "go.mod"))
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module "))
		}
	}
	return ""
}

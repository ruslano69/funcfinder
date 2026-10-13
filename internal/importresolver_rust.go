// importresolver_rust.go - Rust imports for the shard graph: a crate is imported by its name, and a module path inside
// it names a directory of the crate's src/ tree.
package internal

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// RustCrate is a crate of the analyzed tree: the name its code imports it by, and its root directory relative to the
// analyzed root, slash-separated ("" for the root itself).
type RustCrate struct {
	Name string
	Dir  string
}

// rustNameLine matches a `name = "..."` key; cargoCrateName reads it only inside [package] and [lib].
var rustNameLine = regexp.MustCompile(`^\s*name\s*=\s*"([^"]+)"`)

// rustIdent is an identifier: one module segment of an import path.
var rustIdent = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// DetectRustCrates finds the crates under rootDir from their Cargo.toml files. A Cargo.toml that names no crate (a
// workspace root, say) is skipped. The result is sorted by name, so it does not depend on the walk order.
func DetectRustCrates(rootDir string) []RustCrate {
	var crates []RustCrate
	_ = filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // an unreadable entry holds no crate
		}
		if d.IsDir() {
			if path != rootDir && skipCrateDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Name() != "Cargo.toml" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		name := cargoCrateName(string(data))
		if name == "" {
			return nil
		}
		rel, err := filepath.Rel(rootDir, filepath.Dir(path))
		if err != nil {
			return nil
		}
		dir := filepath.ToSlash(rel)
		if dir == "." {
			dir = ""
		}
		crates = append(crates, RustCrate{Name: name, Dir: dir})
		return nil
	})
	sort.Slice(crates, func(i, j int) bool {
		if crates[i].Name != crates[j].Name {
			return crates[i].Name < crates[j].Name
		}
		return crates[i].Dir < crates[j].Dir
	})
	return crates
}

// skipCrateDir reports the directories that never hold a crate of the project: build output, dependencies, VCS.
func skipCrateDir(name string) bool {
	return name == "target" || name == "node_modules" || strings.HasPrefix(name, ".")
}

// cargoCrateName is the name a crate is imported by: the [lib] name when the file sets one, otherwise the [package]
// name with "-" read as "_". It returns "" when the file names no crate.
func cargoCrateName(text string) string {
	var pkg, lib, section string
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") {
			section = strings.Trim(trimmed, "[] \t\r")
			continue
		}
		m := rustNameLine.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		switch section {
		case "package":
			if pkg == "" {
				pkg = m[1]
			}
		case "lib":
			if lib == "" {
				lib = m[1]
			}
		}
	}
	name := pkg
	if lib != "" {
		name = lib
	}
	return strings.ReplaceAll(name, "-", "_")
}

// resolveRustImport maps a Rust import to a shard. `crate_name::module::item` names a crate, and `crate::module::item`
// names the importing file's own crate. The module path is walked through the crate's src/ directories while they hold
// files of the analyzed tree, and the deepest one reached gives the shard; the crate's src/ shard is the fallback. The
// second result is false for paths that are not intra-project (std, external crates, super::, self::).
func resolveRustImport(imp, srcFile, rootDir string, crates []RustCrate, relToShard map[string]string) (string, bool) {
	segments := strings.Split(strings.TrimSpace(imp), "::")
	head := strings.TrimSpace(segments[0])

	owner := crateNamed(head, crates)
	if head == "crate" {
		owner = crateContaining(srcFile, rootDir, crates)
	}
	if owner == nil {
		return "", false
	}

	dir := "src"
	if owner.Dir != "" {
		dir = owner.Dir + "/src"
	}
	shard := shardForDir(dir, relToShard)
	for _, seg := range segments[1:] {
		seg = strings.TrimSpace(seg)
		if !rustIdent.MatchString(seg) {
			break // a brace group or a glob ends the module path
		}
		next := dir + "/" + seg
		s := shardForDir(next, relToShard)
		if s == "" {
			break // the path leaves the module tree: the item is in the module reached so far
		}
		shard, dir = s, next
	}
	return shard, true
}

// crateNamed returns the crate that code imports by the given name, or nil. Crates come sorted by name and then by
// directory, so the root crate wins over a nested one of the same name.
func crateNamed(name string, crates []RustCrate) *RustCrate {
	for i := range crates {
		if crates[i].Name == name {
			return &crates[i]
		}
	}
	return nil
}

// crateContaining returns the crate that holds srcFile: the one whose directory is the longest prefix of the file's
// path, so a nested crate wins over the crate around it.
func crateContaining(srcFile, rootDir string, crates []RustCrate) *RustCrate {
	rel, err := filepath.Rel(rootDir, srcFile)
	if err != nil {
		return nil
	}
	rel = filepath.ToSlash(rel)
	var best *RustCrate
	for i := range crates {
		c := &crates[i]
		inside := c.Dir == "" || strings.HasPrefix(rel, c.Dir+"/")
		if inside && (best == nil || len(c.Dir) > len(best.Dir)) {
			best = c
		}
	}
	return best
}

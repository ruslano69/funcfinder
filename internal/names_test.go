package internal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Each name has its kind: a function, a method (it has a class), a type with its own kind, a type the class finder
// found (no kind: "class"), and a Go alias. Within a file the names are in line order; files are in path order.
func TestFormatNames_KindsAndLineOrder(t *testing.T) {
	results := []DirResult{
		{
			Path:      "b.go",
			Functions: []FunctionBounds{{Name: "Run", Start: 9}},
			Classes:   []ClassBounds{{Name: "Server", Start: 3, Kind: "struct"}, {Name: "Plain", Start: 1}},
			Aliases:   []AliasBounds{{Name: "Handler", Start: 5}},
		},
		{
			Path:      "a.go",
			Functions: []FunctionBounds{{Name: "Start", Start: 2, ClassName: "Server"}},
		},
	}
	got := FormatNames(results)
	want := "method\tStart\ta.go:2\n" +
		"class\tPlain\tb.go:1\n" +
		"struct\tServer\tb.go:3\n" +
		"alias\tHandler\tb.go:5\n" +
		"function\tRun\tb.go:9\n"
	if got != want {
		t.Errorf("FormatNames =\n%s\nwant\n%s", got, want)
	}
}

// The names of one Go file in all mode: a struct, an alias and a function. A Go receiver method is not listed as a
// method yet: the function finder sets no class name for it, so it would come out as a function.
func TestNamesOfGoFile(t *testing.T) {
	src := "package sample\n\ntype Server struct {\n\tPort int\n}\n\ntype Handler = func(int)\n\nfunc Run() {}\n"
	path := createTempFile(t, src, "names_*.go")
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}

	got := FormatNames([]DirResult{ProcessSingleFile(cfg, path, "go", "all")})
	want := "struct\tServer\t" + path + ":3\n" +
		"alias\tHandler\t" + path + ":7\n" +
		"function\tRun\t" + path + ":9\n"
	if got != want {
		t.Errorf("names =\n%s\nwant\n%s", got, want)
	}
}

// A dataclass is listed with its kind: the decorator above the class sets it.
func TestNamesOfPythonDataclass(t *testing.T) {
	src := "from dataclasses import dataclass\n\n@dataclass\nclass Point:\n    x: int\n"
	path := createTempFile(t, src, "names_*.py")
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}

	got := FormatNames([]DirResult{ProcessSingleFile(cfg, path, "py", "all")})
	want := "dataclass\tPoint\t" + path + ":4\n"
	if got != want {
		t.Errorf("names = %q, want %q", got, want)
	}
}

// A file and a directory that holds it give the same names: both run the same job. The paths are compared apart, since
// a directory gives the file a different path spelling.
func TestNamesFileAndDirectoryAgree(t *testing.T) {
	src := "package sample\n\ntype Handler = func(int)\n\nfunc Run() {}\n"
	dir := t.TempDir()
	file := filepath.Join(dir, "sample.go")
	if err := os.WriteFile(file, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}

	fromFile := namesWithoutPath(FormatNames([]DirResult{ProcessSingleFile(cfg, file, "go", "all")}))
	results, err := NewDirProcessor(cfg, 1, true, false, "all").ProcessDirectory(dir)
	if err != nil {
		t.Fatal(err)
	}
	fromDir := namesWithoutPath(FormatNames(results))
	if fromFile == "" {
		t.Fatal("the file gave no names")
	}
	if fromFile != fromDir {
		t.Errorf("file names =\n%s\ndirectory names =\n%s", fromFile, fromDir)
	}
}

// namesWithoutPath keeps the kind, the name and the line of each line of a names listing.
func namesWithoutPath(s string) string {
	var out []string
	for _, line := range strings.Split(strings.TrimSpace(s), "\n") {
		if line == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		loc := fields[len(fields)-1]
		out = append(out, fields[0]+"\t"+fields[1]+"\t"+loc[strings.LastIndex(loc, ":")+1:])
	}
	return strings.Join(out, "\n")
}

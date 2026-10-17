package internal

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type csCompareFile struct {
	path  string
	lines []string
}

// The caller supplies pinned clones; do not accidentally benchmark generated
// build output or untracked source files from a directory walk.
func csCompareFiles(t testing.TB, root string) ([]csCompareFile, int64, int) {
	return braceCompareFiles(t, root, "*.cs")
}

func braceCompareFiles(t testing.TB, root, pattern string) ([]csCompareFile, int64, int) {
	t.Helper()
	output, err := exec.Command("git", "-C", root, "ls-files", "-z", "--", pattern).Output()
	if err != nil {
		t.Fatal(err)
	}
	var files []csCompareFile
	var bytes int64
	var lineCount int
	for _, rel := range strings.Split(string(output), "\x00") {
		if rel == "" {
			continue
		}
		path := filepath.Join(root, rel)
		lines, err := readFileLines(path)
		if err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		bytes += info.Size()
		lineCount += len(lines)
		files = append(files, csCompareFile{path, lines})
	}
	if len(files) == 0 {
		t.Fatalf("no tracked %s files in %s", pattern, root)
	}
	return files, bytes, lineCount
}

func TestCSharpTraversalParity(t *testing.T) {
	config, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	files := []csCompareFile{
		{"nested.cs", strings.Split(`class C {
    C() { }
    void Outer() {
        if (true) { int Local() => 1; }
        var f = () => { int LambdaLocal() { return 2; } return LambdaLocal(); };
        var v = value is {} provider ? provider(1) : 0;
    }
    class Inner { Inner() { } }
    int ICount.Count { get { int GetterLocal() => 3; return GetterLocal(); } }
    void After() => After();
}`, "\n")},
		{"deep.cs", strings.Split("class Deep { void Outer() {\n"+strings.Repeat("if (true) {\n", 1024)+"int Local() => 1;\n"+strings.Repeat("}\n", 1024)+"} }", "\n")},
	}
	for _, root := range filepath.SplitList(os.Getenv("CSHARP_COMPARE_ROOTS")) {
		corpus, _, _ := csCompareFiles(t, root)
		files = append(files, corpus...)
	}
	for _, file := range files {
		for _, mode := range []struct {
			offset  int
			selects []string
			extract bool
		}{{1, nil, false}, {71, nil, true}, {1, []string{"Local", "Get", "Execute"}, true}} {
			linear := NewFinder(config["cs"], mode.selects, mode.selects == nil, mode.extract, false)
			recursive := NewFinder(config["cs"], mode.selects, mode.selects == nil, mode.extract, false)
			want, err := linear.findCSharpFunctions(file.lines, mode.offset, file.path)
			if err != nil {
				t.Fatal(err)
			}
			got, err := recursive.findCSharpFunctionsRecursive(file.lines, mode.offset, file.path)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("%s (offset %d, selected %v):\nrecursive=%+v\nlinear=%+v", file.path, mode.offset, mode.selects, got, want)
			}
			shared := NewFinder(config["cs"], mode.selects, mode.selects == nil, mode.extract, false)
			generic, err := shared.findBraceFunctionsRecursive(file.lines, mode.offset, file.path)
			if err != nil || !reflect.DeepEqual(generic, want) {
				t.Fatalf("%s (offset %d, selected %v): shared traversal differs: %+v; want %+v; error %v", file.path, mode.offset, mode.selects, generic, want, err)
			}
		}
	}
	t.Logf("identical functions, classes, signature metadata and extraction: %d files, 3 modes", len(files))
}

var csCompareSink int

func BenchmarkCSharpTraversal(b *testing.B) {
	roots := filepath.SplitList(os.Getenv("CSHARP_COMPARE_ROOTS"))
	if len(roots) == 0 {
		b.Skip("set CSHARP_COMPARE_ROOTS to pinned C# repositories")
	}
	config, err := LoadConfig()
	if err != nil {
		b.Fatal(err)
	}
	for _, root := range roots {
		files, bytes, lines := csCompareFiles(b, root)
		b.Run(filepath.Base(root), func(b *testing.B) {
			variants := []struct {
				name string
				find func(*Finder, []string, int, string) (*FindResult, error)
			}{{"linear", (*Finder).findCSharpFunctions}, {"recursive", (*Finder).findCSharpFunctionsRecursive}}
			if os.Getenv("CSHARP_COMPARE_REVERSE") == "1" {
				variants[0], variants[1] = variants[1], variants[0]
			}
			for _, variant := range variants {
				b.Run(variant.name, func(b *testing.B) {
					b.ReportAllocs()
					b.SetBytes(bytes)
					b.ResetTimer()
					for n := 0; n < b.N; n++ {
						count := 0
						for _, file := range files {
							finder := NewFinder(config["cs"], nil, true, false, false)
							result, err := variant.find(finder, file.lines, 1, file.path)
							if err != nil {
								b.Fatal(err)
							}
							count += len(result.Functions)
						}
						csCompareSink = count
					}
					b.ReportMetric(float64(lines)*float64(b.N)/b.Elapsed().Seconds(), "lines/s")
				})
			}
		})
	}
}

package internal

import (
	"os"
	"path/filepath"
	"testing"
)

var dTraversalSink int

// Compare the complete production D path with the shared recursive pilot.
// Both include D preparation and class/function discovery, without disk I/O.
func BenchmarkDTraversal(b *testing.B) {
	roots := filepath.SplitList(os.Getenv("D_COMPARE_ROOTS"))
	if len(roots) == 0 {
		b.Skip("set D_COMPARE_ROOTS to pinned D repositories")
	}
	config, err := LoadConfig()
	if err != nil {
		b.Fatal(err)
	}
	for _, root := range roots {
		files, bytes, lines := braceCompareFiles(b, root, "*.d")
		b.Run(filepath.Base(root), func(b *testing.B) {
			variants := []struct {
				name string
				find func(*Finder, []string, int, string) (*FindResult, error)
			}{{"baseline", (*Finder).FindFunctionsInLines}, {"recursive", (*Finder).findBraceFunctionsRecursive}}
			if os.Getenv("D_COMPARE_REVERSE") == "1" {
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
							finder := NewFinder(config["d"], nil, true, false, false)
							result, err := variant.find(finder, file.lines, 1, file.path)
							if err != nil {
								b.Fatal(err)
							}
							count += len(result.Functions)
						}
						dTraversalSink = count
					}
					b.ReportMetric(float64(lines)*float64(b.N)/b.Elapsed().Seconds(), "lines/s")
				})
			}
		})
	}
}

package internal

import (
	"runtime"
	"sync"
	"sync/atomic"
)

// ForEachParallel calls fn(i) for every i in [0, n) on up to runtime.NumCPU() goroutines and returns when all calls
// are done. Each call must write only to its own index of the caller's result slice; the caller then reads the results
// in index order, so the output does not depend on scheduling.
func ForEachParallel(n int, fn func(i int)) {
	workers := runtime.NumCPU()
	if workers > n {
		workers = n
	}
	var next atomic.Int64
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				i := int(next.Add(1) - 1)
				if i >= n {
					return
				}
				fn(i)
			}
		}()
	}
	wg.Wait()
}

// FindFunctionsInFile runs a function finder on a file whose lines the caller has already read: a finder that takes
// lines gets them, so the file is not read a second time; the others (Python) read the file themselves.
func FindFunctionsInFile(finder LanguageFinder, path string, lines []string) (*FindResult, error) {
	return findFunctionsOf(finder, path, func() ([]string, error) { return lines, nil })
}





package main

import (
	"flag"
	"fmt"
	"os"
	"time"
	"github.com/ruslano69/funcfinder/internal"
)

func main() {
	iterations := flag.Int("n", 1000, "Number of iterations")
	flag.Parse()

	if flag.NArg() < 2 {
		fmt.Fprintf(os.Stderr, "Usage: benchmark -n <iterations> <file> <lang>\n")
		os.Exit(1)
	}

	filename := flag.Arg(0)
	lang := flag.Arg(1)

	// Load config once
	config, err := internal.LoadConfig()
	if err != nil {
		internal.FatalError("loading config: %v", err)
	}

	langConfig, err := config.GetLanguageConfig(lang)
	if err != nil {
		internal.FatalError("language config: %v", err)
	}

	// Warm up
	finder := internal.CreateFinder(langConfig, "", "map", false, false)
	_, err = finder.FindFunctions(filename)
	if err != nil {
		internal.FatalError("warm up: %v", err)
	}

	// Benchmark
	start := time.Now()
	for i := 0; i < *iterations; i++ {
		finder := internal.CreateFinder(langConfig, "", "map", false, false)
		_, err := finder.FindFunctions(filename)
		if err != nil {
			internal.FatalError("iteration %d: %v", i, err)
		}
	}
	elapsed := time.Since(start)

	avgMs := float64(elapsed.Microseconds()) / float64(*iterations) / 1000.0
	throughput := float64(*iterations) / elapsed.Seconds()

	fmt.Printf("Benchmark Results\n")
	fmt.Printf("=================\n")
	fmt.Printf("File:            %s\n", filename)
	fmt.Printf("Iterations:      %d\n", *iterations)
	fmt.Printf("Total time:      %v\n", elapsed)
	fmt.Printf("Avg per iter:    %.3f ms\n", avgMs)
	fmt.Printf("Throughput:      %.1f files/sec\n", throughput)
}

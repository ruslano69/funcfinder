// uncalled.go - the output of funcstats --uncalled: the functions no code line names, as text or JSON.
package main

import (
	"encoding/json"
	"fmt"

	"github.com/ruslano69/funcfinder/internal"
)

// printUncalled prints the candidates for deletion: a count, one per line, and what the list does not cover.
func printUncalled(report internal.UncalledReport, asJSON bool) {
	if asJSON {
		data, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			internal.FatalError("encoding JSON: %v", err)
		}
		fmt.Println(string(data))
		return
	}

	fmt.Printf("Uncalled: %d of %d functions have no use in the tree (candidates to check before deleting)\n",
		len(report.Candidates), report.Considered)
	for _, c := range report.Candidates {
		note := ""
		if c.Exported {
			note = "  (exported: code outside this tree may call it)"
		}
		fmt.Printf("  %s:%d  %s%s\n", c.File, c.Line, c.Name, note)
	}
	fmt.Println("Not counted: functions in test files, main and init, and Go interface methods (String, Error, Len, ...).")
	fmt.Println("A name used only in strings, through reflection, or from another repository looks uncalled.")
}

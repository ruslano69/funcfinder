package main

import "fmt"

func main() {
	// Regular string
	regular := "Hello World"

	// Raw string with comment-like content
	query := `SELECT * FROM users WHERE id = 1 // this is NOT a comment`

	// Raw string with backslashes
	path := `C:\Users\Documents\file.txt`

	// Raw string multiline
	template := `
		Line 1
		Line 2 // still NOT a comment
		Line 3
	`

	// These ARE real code
	fmt.Println(regular)
	fmt.Println(query)
	fmt.Println(path)
	fmt.Println(template)
}

// Expected:
// Code lines: ~15 (not 26!)
// - Multiline raw string content should NOT count as separate code lines
// Function calls: 7 unique (main, Println x4, not counting query/regular/path)

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"
)

// ts_declaration_vectors.jsonl is what the D scanners are tested against: it must be what Go's patterns return now.
func TestVectorsAreCurrent(t *testing.T) {
	want, err := generate()
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile("../ts_declaration_vectors.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	norm := func(b []byte) []byte { return bytes.ReplaceAll(b, []byte("\r\n"), []byte("\n")) }
	if !bytes.Equal(norm(got), norm(want)) {
		t.Fatal("benchmarks/ffd/ts_declaration_vectors.jsonl is out of date: run `go run ./benchmarks/ffd/genvectors > benchmarks/ffd/ts_declaration_vectors.jsonl`")
	}
}

// The vectors must reach both declaration alternatives, the declarator, and lines that match neither.
func TestVectorsExerciseEveryAlternative(t *testing.T) {
	b, err := generate()
	if err != nil {
		t.Fatal(err)
	}
	var first, second, declarators, neither int
	for _, line := range bytes.Split(bytes.TrimSpace(b), []byte("\n")) {
		var v vector
		if err := json.Unmarshal(line, &v); err != nil {
			t.Fatal(err)
		}
		if v.Decl == nil && v.Decr == nil {
			neither++
		}
		if len(v.Decl) == 3 && v.Decl[1] != "" {
			first++
		}
		if len(v.Decl) == 3 && v.Decl[2] != "" {
			second++
		}
		if v.Decr != nil {
			declarators++
		}
	}
	if first < 100 || second < 100 || declarators < 100 || neither < 1000 {
		t.Fatalf("vectors reach too little: first alternative %d, second %d, declarator %d, neither %d", first, second, declarators, neither)
	}
}

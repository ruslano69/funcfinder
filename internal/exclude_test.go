package internal

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// A pattern without a slash matches a base name anywhere; one with a slash matches the path under the analysed root.
func TestFilterExcluded_NameAndPath(t *testing.T) {
	root := t.TempDir()
	paths := []string{
		filepath.Join(root, "a.go"),
		filepath.Join(root, "a_test.go"),
		filepath.Join(root, "gen", "x.go"),
		filepath.Join(root, "internal", "gen", "y.go"),
	}

	got, err := FilterExcluded(paths, root, []string{"*_test.go", "gen/*.go"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{filepath.Join(root, "a.go"), filepath.Join(root, "internal", "gen", "y.go")}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("FilterExcluded = %v, want %v", got, want)
	}
}

// With no pattern the paths come back unchanged.
func TestFilterExcluded_NoPatterns(t *testing.T) {
	paths := []string{"a.go", "b_test.go"}
	got, err := FilterExcluded(paths, ".", nil)
	if err != nil || !reflect.DeepEqual(got, paths) {
		t.Errorf("FilterExcluded = %v, %v, want the paths unchanged", got, err)
	}
}

// A single file is taken relative to its directory, so its base name is what a pattern meets.
func TestFilterExcluded_SingleFile(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "lib_test.rs")
	if err := os.WriteFile(file, []byte("fn f() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := FilterExcluded([]string{file}, file, []string{"*_test.rs"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("FilterExcluded = %v, want the single file excluded", got)
	}
}

// A malformed pattern is reported, not silently matched against nothing.
func TestFilterExcluded_BadPattern(t *testing.T) {
	if _, err := FilterExcluded([]string{"a.go"}, ".", []string{"["}); err == nil {
		t.Error("FilterExcluded with pattern [ = no error, want one")
	}
}

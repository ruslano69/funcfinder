package internal

import "testing"

// A one-line Go struct (type X struct{}) is a struct: the catch-all "named" pattern (type X Y) matches it too, so it must
// not name it. A real named type stays "named".
func TestGoOneLineStructIsStruct(t *testing.T) {
	src := "package sample\n\ntype Server struct{}\n\ntype ID int\n"
	path := createTempFile(t, src, "oneline_*.go")
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}

	got := FormatNames([]DirResult{ProcessSingleFile(cfg, path, "go", "all")})
	want := "struct\tServer\t" + path + ":3\n" +
		"named\tID\t" + path + ":5\n"
	if got != want {
		t.Errorf("names =\n%s\nwant\n%s", got, want)
	}
}

package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

// ffd_config.json is generated from languages.json by this package. The D port embeds it, so a languages.json change
// that is not regenerated would leave the D tools on the old patterns without any other sign.
func TestConfigIsCurrent(t *testing.T) {
	want, err := generate()
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile("../ffd_config.json")
	if err != nil {
		t.Fatal(err)
	}
	norm := func(b []byte) []byte { return bytes.TrimSpace(bytes.ReplaceAll(b, []byte("\r\n"), []byte("\n"))) }
	if !bytes.Equal(norm(got), norm(want)) {
		t.Fatal("benchmarks/ffd/ffd_config.json is out of date: run `go run ./benchmarks/ffd/genconfig > benchmarks/ffd/ffd_config.json`")
	}
}

func TestEveryLanguageIsExported(t *testing.T) {
	b, err := generate()
	if err != nil {
		t.Fatal(err)
	}
	for _, lang := range []string{"c", "cpp", "cs", "d", "go", "java", "js", "kotlin", "php", "py", "ruby", "rust", "scala", "swift", "ts"} {
		if !strings.Contains(string(b), "\"lang\": \""+lang+"\"") {
			t.Errorf("language %q is missing from the generated config", lang)
		}
	}
}

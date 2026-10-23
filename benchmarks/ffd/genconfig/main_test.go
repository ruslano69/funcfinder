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
	for _, lang := range []string{"c", "cpp", "cs", "d", "go", "goose", "java", "js", "kotlin", "php", "py", "ruby", "rust", "scala", "swift", "ts"} {
		if !strings.Contains(string(b), "\"lang\": \""+lang+"\"") {
			t.Errorf("language %q is missing from the generated config", lang)
		}
	}
}

func TestDConfigIsCurrent(t *testing.T) {
	want, err := generateD()
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile("../ffd_config_data.d")
	if err != nil {
		t.Fatal(err)
	}
	norm := func(b []byte) []byte { return bytes.TrimSpace(bytes.ReplaceAll(b, []byte("\r\n"), []byte("\n"))) }
	if !bytes.Equal(norm(got), norm(want)) {
		t.Fatal("ffd_config_data.d is out of date: run `go run ./benchmarks/ffd/genconfig -d > benchmarks/ffd/ffd_config_data.d`")
	}
}

func TestConfigUsesPinnedDUnicode(t *testing.T) {
	table, err := os.ReadFile("../../brace_port_unicode.d")
	if err != nil {
		t.Fatal(err)
	}
	marker := []byte("enum unicodeVersion=\"" + dUnicodeVersion + "\";")
	if !bytes.Contains(table, marker) {
		t.Fatal("D config version must match the frozen Unicode table")
	}
	data, err := generate()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte("\"unicode_version\": \""+dUnicodeVersion+"\"")) {
		t.Fatal("generated config lost the pinned D Unicode version")
	}
}

func TestDConfigRejectsUnsupportedSchema(t *testing.T) {
	for _, v := range []any{float64(1), []any{"text", float64(1)}} {
		var b bytes.Buffer
		if err := emitD(&b, "new_field", v, 0); err == nil {
			t.Fatalf("unsupported value %v was silently accepted", v)
		}
	}
}

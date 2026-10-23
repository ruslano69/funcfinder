// Command genconfig writes resolved language settings as JSON or immutable D tables (-d).
//
// The D finders were written against Go's resolved patterns ({IDENT} and {JS_IDENT} expanded), not the raw text of
// languages.json, so the file is produced by the Go package itself. Run it after changing languages.json:
//
//	go run ./benchmarks/ffd/genconfig > benchmarks/ffd/ffd_config.json
//	go run ./benchmarks/ffd/genconfig -d > benchmarks/ffd/ffd_config_data.d
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"regexp"

	ff "github.com/ruslano69/funcfinder/internal"
)

// D scanners use brace_port_unicode.d's frozen categories, not the generator's
// host Go Unicode tables. Keep the metadata stable across Go toolchains.
const dUnicodeVersion = "15.0.0"

// generate returns the JSON text: {"unicode_version": ..., "languages": {"<key>": {...}, ...}}. Maps are written with
// sorted keys, so the output is stable.
func generate() ([]byte, error) {
	cfg, err := ff.LoadConfig()
	if err != nil {
		return nil, err
	}
	languages := map[string]any{}
	for _, lang := range cfg.GetSupportedLanguages() {
		c, err := cfg.GetLanguageConfig(lang)
		if err != nil {
			return nil, err
		}
		raw, err := json.Marshal(c)
		if err != nil {
			return nil, err
		}
		var out map[string]any
		if err := json.Unmarshal(raw, &out); err != nil {
			return nil, err
		}
		out["lang"] = lang
		out["unicode_version"] = dUnicodeVersion
		out["func_pattern"] = c.FuncRegex().String()
		patterns := []struct {
			key string
			re  *regexp.Regexp
		}{
			{"class_pattern", c.ClassRegex()},
			{"class_method_open", c.ClassMethodOpenRegex()},
			{"header_open_pattern", c.HeaderOpenRegex()},
			{"open_type_header_pattern", c.OpenTypeHeaderRegex()},
			{"declaration_pattern", c.DeclarationRegex()},
			{"declarator_pattern", c.DeclaratorRegex()},
			{"quoted_method_pattern", c.QuotedMethodRegex()},
			{"constructor_pattern", c.ConstructorRegex()},
			{"member_declaration_pattern", c.MemberDeclarationRegex()},
			{"conversion_operator_pattern", c.ConversionOperatorRegex()},
		}
		for _, p := range patterns {
			if p.re != nil {
				out[p.key] = p.re.String()
			}
		}
		// The patterns the struct finders read, with their identifier classes resolved, by kind.
		structPatterns := map[string]string{}
		for kind, re := range c.GetStructPatterns() {
			structPatterns[kind] = re.String()
		}
		out["struct_patterns"] = structPatterns
		languages[lang] = out
	}
	return json.MarshalIndent(map[string]any{"unicode_version": dUnicodeVersion, "languages": languages}, "", " ")
}

func main() {
	staticD := flag.Bool("d", false, "write immutable D configuration tables instead of JSON")
	flag.Parse()
	b, err := generate()
	if *staticD {
		b, err = generateD()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "genconfig:", err)
		os.Exit(1)
	}
	os.Stdout.Write(append(b, '\n'))
}

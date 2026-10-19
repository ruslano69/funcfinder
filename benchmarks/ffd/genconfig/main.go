// Command genconfig writes ffd_config.json: the resolved configuration of every language, as the D port reads it.
//
// The D finders were written against Go's resolved patterns ({IDENT} and {JS_IDENT} expanded), not the raw text of
// languages.json, so the file is produced by the Go package itself. Run it after changing languages.json:
//
//	go run ./benchmarks/ffd/genconfig > benchmarks/ffd/ffd_config.json
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"unicode"

	ff "github.com/ruslano69/funcfinder/internal"
)

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
		out["unicode_version"] = unicode.Version
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
	return json.MarshalIndent(map[string]any{"unicode_version": unicode.Version, "languages": languages}, "", " ")
}

func main() {
	b, err := generate()
	if err != nil {
		fmt.Fprintln(os.Stderr, "genconfig:", err)
		os.Exit(1)
	}
	os.Stdout.Write(append(b, '\n'))
}

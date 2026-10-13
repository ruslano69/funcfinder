package internal

import "testing"

// Regular expressions are compiled on first use, so nothing else would notice a pattern in languages.json that does
// not compile until a run reads that language. This test compiles every language.
func TestEveryLanguageCompiles(t *testing.T) {
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	for _, lang := range cfg.GetSupportedLanguages() {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("%s: compiling its patterns failed: %v", lang, r)
				}
			}()
			cfg[lang].ensure()
		}()
	}
	if cfg["go"].FuncRegex() == nil {
		t.Error("go: the function regex was not compiled")
	}
}

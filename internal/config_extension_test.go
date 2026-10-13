package internal

import "testing"

// `.h` is claimed by both C and C++. The language a header gets must not depend on map iteration order:
// repeated lookups must all return the C++ configuration, and `.c` must stay C.
func TestGetLanguageByExtension_HeaderIsStable(t *testing.T) {
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 100; i++ {
		if got := cfg.GetLanguageByExtension("include/fmt/core.h"); got != cfg["cpp"] {
			t.Fatalf("lookup %d: .h -> %p, want the cpp configuration %p", i, got, cfg["cpp"])
		}
	}
	if got := cfg.GetLanguageByExtension("lua.c"); got != cfg["c"] {
		t.Errorf(".c -> %p, want the c configuration %p", got, cfg["c"])
	}
	if got := cfg.GetLanguageByExtension("notes.unknownext"); got != nil {
		t.Errorf("unknown extension -> %p, want nil", got)
	}
}

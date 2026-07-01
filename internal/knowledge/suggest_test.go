package knowledge

import (
	"strings"
	"testing"
)

func TestNormalizeForIndex(t *testing.T) {
	// U+FFFD and C1 controls become spaces; runs collapse; newline/tab kept.
	got := NormalizeForIndex("a�bc\nd\te", "")
	if got != "a b c\nd\te" {
		t.Fatalf("garbage strip = %q", got)
	}
	// stripRunes handles a corpus-specific artifact and unglues words.
	got = NormalizeForIndex("theΩstatementsΩΩwithin", "Ω")
	if got != "the statements within" {
		t.Fatalf("omega unglue = %q", got)
	}
	// omega is preserved when NOT listed (legit elsewhere, e.g. ohms).
	if out := NormalizeForIndex("10Ω resistor", ""); !strings.Contains(out, "Ω") {
		t.Fatalf("omega should survive when not stripped: %q", out)
	}
	// legit typography (em dash, guillemets) is untouched.
	if out := NormalizeForIndex("раздел — «память»", ""); out != "раздел — «память»" {
		t.Fatalf("typography clobbered: %q", out)
	}
}

func TestSuggest(t *testing.T) {
	db, err := Open(t.TempDir() + "/s.sqlite")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	docs := []string{
		"SORT records ascending",
		"SORT and MERGE files",
		"the SORT statement is powerful",
		"PERFORM a paragraph",
	}
	for i, d := range docs {
		if _, err := Add(db, "doc", d, "spec", "{}", nil); err != nil {
			t.Fatalf("add %d: %v", i, err)
		}
	}

	terms, err := Suggest(db, "sort", 10)
	if err != nil {
		t.Fatalf("suggest: %v", err)
	}
	if len(terms) == 0 || terms[0].Term != "sort" {
		t.Fatalf("expected 'sort' top term, got %+v", terms)
	}
	// "sort" appears in 3 of 4 docs, 3 times total.
	if terms[0].Docs != 3 || terms[0].Count != 3 {
		t.Fatalf("sort stats = docs %d count %d, want 3/3", terms[0].Docs, terms[0].Count)
	}
	// case-insensitive prefix (vocab is lowercased).
	if up, _ := Suggest(db, "SORT", 10); len(up) == 0 || up[0].Term != "sort" {
		t.Fatalf("uppercase prefix should match lowercased vocab: %+v", up)
	}
	// unknown prefix -> empty, no error.
	if none, err := Suggest(db, "zzz", 10); err != nil || len(none) != 0 {
		t.Fatalf("unknown prefix: err=%v n=%d", err, len(none))
	}
}

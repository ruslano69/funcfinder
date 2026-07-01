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

	// 20-doc corpus with a real frequency spread: "sort" is common (weak key),
	// "merge" is rare (sharp key).
	for i := 0; i < 20; i++ {
		body := "sort records" // "sort" in all 20 docs → weak
		if i < 2 {
			body += " merge files" // "merge" in only 2 → sharp
		}
		if _, err := Add(db, "doc", body, "spec", "{}", nil); err != nil {
			t.Fatalf("add %d: %v", i, err)
		}
	}

	terms, err := Suggest(db, "sort", 10)
	if err != nil {
		t.Fatalf("suggest: %v", err)
	}
	if len(terms) == 0 || terms[0].Term != "sort" || terms[0].Docs != 20 {
		t.Fatalf("expected 'sort' in all 20 docs, got %+v", terms)
	}
	// "sort" in 20/20 docs → IDF=0, a weak key.
	if terms[0].IDF != 0 || !terms[0].Weak() {
		t.Fatalf("sort IDF=%.3f Weak=%v, want 0 and weak", terms[0].IDF, terms[0].Weak())
	}
	// "merge" in 2/20 (10%) → IDF=ln(10)≈2.30, a sharp key.
	m, _ := Suggest(db, "merge", 5)
	if len(m) == 0 || m[0].Weak() || m[0].IDF <= terms[0].IDF {
		t.Fatalf("merge should be a sharper (non-weak) key than sort: %+v", m)
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

// TestSuggestRelativeTo pins the mixed-corpus correction: a term common within
// its own partition but diluted by another partition gets a lower (more honest)
// IDF when computed relative to the partition it actually lives in.
func TestSuggestRelativeTo(t *testing.T) {
	db, err := Open(t.TempDir() + "/r.sqlite")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	// Partition "en" (4 docs): "данных" absent.
	for i := 0; i < 4; i++ {
		Add(db, "a", "unrelated english content here", "en", "{}", nil)
	}
	// Partition "ru" (6 docs): "данных" in 4 of the 6 → weak within ru.
	for i := 0; i < 6; i++ {
		body := "прочие сведения"
		if i < 4 {
			body = "описание данных раздела"
		}
		Add(db, "b", body, "ru", "{}", nil)
	}

	rel, err := SuggestRelativeTo(db, "данных", "ru", 10)
	if err != nil {
		t.Fatalf("relative: %v", err)
	}
	if len(rel) == 0 || rel[0].Term != "данных" {
		t.Fatalf("expected данных, got %+v", rel)
	}
	// Partition-relative: 4 of 6 ru docs → IDF=ln(6/4)=0.405, weak.
	if rel[0].Docs != 4 || !rel[0].Weak() {
		t.Fatalf("relative: docs=%d idf=%.3f weak=%v, want 4 docs and weak", rel[0].Docs, rel[0].IDF, rel[0].Weak())
	}
	// Global counts all 10 docs → IDF=ln(10/4)=0.916 > partition IDF: the
	// dilution correction lowers the score toward the honest value.
	glob, _ := Suggest(db, "данных", 10)
	if len(glob) == 0 || glob[0].IDF <= rel[0].IDF {
		t.Fatalf("global IDF %.3f should exceed partition IDF %.3f (dilution)", glob[0].IDF, rel[0].IDF)
	}

	// Unknown partition type → error.
	if _, err := SuggestRelativeTo(db, "данных", "nope", 10); err == nil {
		t.Fatal("expected error for unknown partition type")
	}
}

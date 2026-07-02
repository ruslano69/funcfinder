package knowledge

import "testing"

func TestEnumerate(t *testing.T) {
	db, err := Open(t.TempDir() + "/e.sqlite")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	docs := []string{
		"uses PB_Cipher_MD5 and PB_Cipher_SHA1 for hashing",
		"PB_Cipher_MD5 appears twice here: PB_Cipher_MD5 again",
		"a doc that mentions PB_Cipher_HMAC only",
		"no constants in this one at all",
	}
	for i, body := range docs {
		if _, err := Add(db, "doc", body, "spec", "{}", nil); err != nil {
			t.Fatalf("add %d: %v", i, err)
		}
	}

	matches, err := Enumerate(db, `PB_Cipher_[A-Za-z0-9]+`, 0)
	if err != nil {
		t.Fatalf("Enumerate: %v", err)
	}

	byValue := map[string]Match{}
	for _, m := range matches {
		byValue[m.Value] = m
	}

	// PB_Cipher_MD5: 1 occurrence in doc0, 2 occurrences in doc1 -> count=3, docs=2.
	if m := byValue["PB_Cipher_MD5"]; m.Count != 3 || m.Docs != 2 {
		t.Errorf("PB_Cipher_MD5 = %+v, want Count=3 Docs=2", m)
	}
	if m := byValue["PB_Cipher_SHA1"]; m.Count != 1 || m.Docs != 1 {
		t.Errorf("PB_Cipher_SHA1 = %+v, want Count=1 Docs=1", m)
	}
	// The completeness-audit payoff: a constant not in anyone's guess list is
	// still found, because Enumerate scans the corpus rather than checking a
	// candidate list.
	if m := byValue["PB_Cipher_HMAC"]; m.Count != 1 || m.Docs != 1 {
		t.Errorf("PB_Cipher_HMAC = %+v, want Count=1 Docs=1 (found by scanning, not guessing)", m)
	}

	// Sorted by Count desc: MD5 (3) must come before SHA1/HMAC (1 each).
	if matches[0].Value != "PB_Cipher_MD5" {
		t.Errorf("matches[0] = %q, want PB_Cipher_MD5 (highest count first)", matches[0].Value)
	}

	// limit caps the result.
	limited, err := Enumerate(db, `PB_Cipher_[A-Za-z0-9]+`, 1)
	if err != nil {
		t.Fatalf("Enumerate limit=1: %v", err)
	}
	if len(limited) != 1 {
		t.Fatalf("len(limited) = %d, want 1", len(limited))
	}

	// A pattern matching nothing returns an empty (not nil-error) slice.
	none, err := Enumerate(db, `NoSuchConstant_[0-9]+`, 0)
	if err != nil {
		t.Fatalf("Enumerate no-match: %v", err)
	}
	if len(none) != 0 {
		t.Fatalf("len(none) = %d, want 0", len(none))
	}

	// Invalid regex surfaces a compile error, not a panic.
	if _, err := Enumerate(db, `[unterminated`, 0); err == nil {
		t.Fatal("Enumerate with invalid regex: want error, got nil")
	}
}

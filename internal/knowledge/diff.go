package knowledge

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"sort"
)

// DiffEntry summarizes one document's diff status between two releases.
type DiffEntry struct {
	ID    int64  `json:"id"`
	Title string `json:"title"`
	Type  string `json:"type"`
}

// Diff is the result of comparing two releases' docs tables (TZ FR-18):
// which documents were added, removed, or changed going from one release to
// another.
type Diff struct {
	Added   []DiffEntry `json:"added"`
	Removed []DiffEntry `json:"removed"`
	Changed []DiffEntry `json:"changed"`
}

// DiffDocs compares the docs tables of two release databases (from → to) and
// reports additions, removals, and content/title/type/metadata changes.
// Comparison is by document id: ids are stable across releases (VACUUM INTO
// preserves rowids, and publish never mutates a prior release), so an id
// present in both is the same logical document, not a coincidence.
func DiffDocs(from, to *sql.DB) (Diff, error) {
	a, err := loadFingerprints(from)
	if err != nil {
		return Diff{}, err
	}
	b, err := loadFingerprints(to)
	if err != nil {
		return Diff{}, err
	}

	var d Diff
	for id, fb := range b {
		fa, ok := a[id]
		if !ok {
			d.Added = append(d.Added, DiffEntry{ID: id, Title: fb.title, Type: fb.docType})
			continue
		}
		if fa.hash != fb.hash {
			d.Changed = append(d.Changed, DiffEntry{ID: id, Title: fb.title, Type: fb.docType})
		}
	}
	for id, fa := range a {
		if _, ok := b[id]; !ok {
			d.Removed = append(d.Removed, DiffEntry{ID: id, Title: fa.title, Type: fa.docType})
		}
	}

	byID := func(s []DiffEntry) func(i, j int) bool {
		return func(i, j int) bool { return s[i].ID < s[j].ID }
	}
	sort.Slice(d.Added, byID(d.Added))
	sort.Slice(d.Removed, byID(d.Removed))
	sort.Slice(d.Changed, byID(d.Changed))
	return d, nil
}

type fingerprint struct {
	title   string
	docType string
	hash    string
}

// loadFingerprints hashes (title, content, type, metadata) per document so
// DiffDocs can detect a change without holding every doc body in memory
// twice or diffing field-by-field.
func loadFingerprints(db *sql.DB) (map[int64]fingerprint, error) {
	rows, err := db.Query(`SELECT id, title, content, type, metadata FROM docs`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[int64]fingerprint{}
	for rows.Next() {
		var id int64
		var title, content, docType, metadata string
		if err = rows.Scan(&id, &title, &content, &docType, &metadata); err != nil {
			return nil, err
		}
		h := sha256.New()
		h.Write([]byte(title))
		h.Write([]byte{0})
		h.Write([]byte(content))
		h.Write([]byte{0})
		h.Write([]byte(docType))
		h.Write([]byte{0})
		h.Write([]byte(metadata))
		out[id] = fingerprint{title: title, docType: docType, hash: hex.EncodeToString(h.Sum(nil))}
	}
	return out, rows.Err()
}

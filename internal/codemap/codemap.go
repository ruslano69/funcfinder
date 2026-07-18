// Package codemap implements TZ FR-22 ("funcfinder as a code → truth
// compiler"): baking a deterministic structural map of a source tree
// (functions and types per file) into a docsearch-server release at publish
// time. "Where is X defined" is then answered by an exact, versioned index
// instead of an approximate vector search over source blobs — funcfinder
// gives the skeleton (structure), not the semantics ("why"); that half of
// truth still comes from ingested human docs (specs/decisions/docstrings).
package codemap

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/ruslano69/funcfinder/analyze"
)

// DocType tags every document Ingest writes, so callers can find (or a
// future publish can replace) exactly the code-map slice of the corpus
// without touching human-authored docs.
const DocType = "code_map"

// Stats reports what Ingest did.
type Stats struct {
	CodeDir   string
	Files     int    // source files that contributed a document
	Replaced  int64  // stale code_map documents deleted before re-ingesting
	CommitSHA string // "" if codeDir isn't a git checkout (or git is unavailable)
	Dirty     bool   // true if codeDir has uncommitted changes
}

// Warning returns a diagnostic when the code map's provenance is
// questionable, or "" when it looks trustworthy.
func (s Stats) Warning() string {
	switch {
	case s.Files == 0 && s.Replaced > 0:
		return fmt.Sprintf("%s produced no code-map documents — the previous code map (%d documents) was replaced with an empty one; check that the path is correct", s.CodeDir, s.Replaced)
	case s.CommitSHA == "":
		return fmt.Sprintf("could not determine a git commit for %s — code_map documents will carry no source_version provenance", s.CodeDir)
	case s.Dirty:
		return fmt.Sprintf("%s has uncommitted changes — the code map may not exactly match commit %s", s.CodeDir, s.CommitSHA)
	default:
		return ""
	}
}

type meta struct {
	SourceVersion string `json:"source_version"`
	Path          string `json:"path"`
}

// Ingest walks codeDir with funcfinder's own analysis core (all 15
// supported languages, auto-detected per file, respecting .gitignore) and
// writes one compact function/type map document per source file to db (a
// docsearch-server write-log), replacing any code_map documents from a
// previous Ingest call first — the code map is a regenerated build
// artifact, not something that should accumulate stale copies release over
// release. Each document's metadata.source_version is codeDir's git commit
// SHA (best effort), tying the map to the exact commit it was compiled
// from.
func Ingest(db *sql.DB, codeDir string) (Stats, error) {
	stats := Stats{CodeDir: codeDir}

	if info, err := os.Stat(codeDir); err != nil || !info.IsDir() {
		return stats, fmt.Errorf("code-dir %q is not a directory: %v", codeDir, err)
	}
	stats.CommitSHA, stats.Dirty = gitState(codeDir)

	config, err := analyze.LoadConfig()
	if err != nil {
		return stats, fmt.Errorf("load language config: %w", err)
	}

	processor := analyze.NewDirProcessor(config, 0, true, true, "all")
	results, err := processor.ProcessDirectory(codeDir)
	if err != nil {
		return stats, fmt.Errorf("scan %s: %w", codeDir, err)
	}

	// The delete-then-reinsert runs as one transaction so a failure partway
	// through (a bad file, a closed DB) leaves the previous code map intact
	// instead of the write-log stuck half-replaced.
	tx, err := db.Begin()
	if err != nil {
		return stats, fmt.Errorf("begin code-map transaction: %w", err)
	}
	defer tx.Rollback()

	replaced, err := deleteDocsByType(tx, DocType)
	if err != nil {
		return stats, fmt.Errorf("clear previous code map: %w", err)
	}
	stats.Replaced = replaced

	for _, r := range results {
		if r.Error != nil || (len(r.Functions) == 0 && len(r.Classes) == 0) {
			continue
		}
		rel, relErr := filepath.Rel(codeDir, r.Path)
		if relErr != nil {
			rel = r.Path
		}
		rel = filepath.ToSlash(rel)

		metaJSON, err := json.Marshal(meta{SourceVersion: stats.CommitSHA, Path: rel})
		if err != nil {
			return stats, fmt.Errorf("marshal metadata for %s: %w", rel, err)
		}
		if err := insertDoc(tx, rel, formatSkeleton(r), DocType, string(metaJSON)); err != nil {
			return stats, fmt.Errorf("ingest %s: %w", rel, err)
		}
		stats.Files++
	}

	if err := tx.Commit(); err != nil {
		return stats, fmt.Errorf("commit code map: %w", err)
	}
	return stats, nil
}

// deleteDocsByType and insertDoc run the delete-then-reinsert against an
// already-open *sql.Tx, so the whole replace-the-code-map operation commits
// or rolls back as one unit.
func deleteDocsByType(tx *sql.Tx, docType string) (int64, error) {
	res, err := tx.Exec(`DELETE FROM docs WHERE type = ?`, docType)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func insertDoc(tx *sql.Tx, title, content, docType, metadata string) error {
	_, err := tx.Exec(
		`INSERT INTO docs (title, content, type, metadata) VALUES (?, ?, ?, ?)`,
		title, content, docType, metadata,
	)
	return err
}

// formatSkeleton renders one file's functions and types as a compact,
// grep-style map — the same "name: start-end" shape funcfinder's own CLI
// prints, chosen for token efficiency (this is what rides FTS/search
// results) and because it's already a format agents in this project are
// primed to expect.
func formatSkeleton(r analyze.DirResult) string {
	var b strings.Builder
	if len(r.Functions) > 0 {
		b.WriteString("functions:\n")
		for _, f := range r.Functions {
			fmt.Fprintf(&b, "  %s: %d-%d\n", f.Name, f.Start, f.End)
		}
	}
	if len(r.Classes) > 0 {
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString("types:\n")
		for _, c := range r.Classes {
			fmt.Fprintf(&b, "  %s: %d-%d\n", c.Name, c.Start, c.End)
		}
	}
	return b.String()
}

// gitState returns dir's current commit SHA ("" if dir isn't a git
// checkout, or git isn't available) and whether the working tree has
// uncommitted changes relative to that commit.
func gitState(dir string) (sha string, dirty bool) {
	out, err := exec.Command("git", "-C", dir, "rev-parse", "HEAD").Output()
	if err != nil {
		return "", false
	}
	sha = strings.TrimSpace(string(out))

	statusOut, err := exec.Command("git", "-C", dir, "status", "--porcelain").Output()
	if err == nil && len(bytes.TrimSpace(statusOut)) > 0 {
		dirty = true
	}
	return sha, dirty
}

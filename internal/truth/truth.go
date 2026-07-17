// Package truth implements the release/channel backbone of docsearch-server
// (see docs/docsearch-server/TZ.md §7.1, §7.3, §8).
//
// It realizes the CQRS split at the storage layer: a single mutable *write-log*
// knowledge DB where truth flows in, and immutable, named *release* files
// snapshotted from it via SQLite VACUUM INTO. Channels (stable/testing/unstable)
// are pointers to releases, so consumers pin to a reproducible version of truth.
//
// This package owns only the release lifecycle and pointer resolution; the
// actual document storage and hybrid search live in internal/knowledge, which
// operates on both the write-log and the (read-only) release files unchanged.
package truth

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// Channel names. unstable always resolves to the live write-log; stable/testing
// point at published releases (or are empty until first publish).
const (
	ChannelStable   = "stable"
	ChannelTesting  = "testing"
	ChannelUnstable = "unstable"
)

// ErrNoRelease is returned when a channel has no release assigned yet.
var ErrNoRelease = errors.New("channel points at no release")

const controlSchema = `
CREATE TABLE IF NOT EXISTS releases (
    version    TEXT PRIMARY KEY,
    created_at INTEGER NOT NULL,
    frozen_at  INTEGER,
    parent     TEXT,
    notes      TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS channels (
    name            TEXT PRIMARY KEY,
    release_version TEXT REFERENCES releases(version)
);

CREATE TABLE IF NOT EXISTS provenance (
    record_id      INTEGER,
    author         TEXT,
    ts             INTEGER NOT NULL,
    target_release TEXT,
    source_ref     TEXT
);
`

// Store is a docsearch-server data root: one control DB, one live write-log
// knowledge DB, and a releases/ directory of immutable snapshots.
type Store struct {
	root    string
	control *sql.DB
}

// Release describes a published, immutable snapshot of truth.
type Release struct {
	Version   string
	CreatedAt int64
	FrozenAt  int64 // 0 if never frozen
	Parent    string
	Notes     string
}

// Channel is a named pointer at a release (empty Release = points nowhere yet;
// unstable is reported with Release == "unstable" to mean "the live write-log").
type Channel struct {
	Name    string
	Release string
}

// Open opens (creating if needed) a Store rooted at dir. It creates the control
// DB, ensures the three standard channels exist, and returns a handle. The
// write-log knowledge DB is created lazily by WriteLogPath's first user.
func Open(dir string) (*Store, error) {
	if err := os.MkdirAll(filepath.Join(dir, "releases"), 0o755); err != nil {
		return nil, fmt.Errorf("mkdir root: %w", err)
	}
	control, err := sql.Open("sqlite", filepath.Join(dir, "control.sqlite"))
	if err != nil {
		return nil, err
	}
	if _, err = control.Exec(controlSchema); err != nil {
		control.Close()
		return nil, fmt.Errorf("control schema: %w", err)
	}
	// Seed the standard channels once (pointing nowhere until first publish).
	for _, ch := range []string{ChannelStable, ChannelTesting, ChannelUnstable} {
		if _, err = control.Exec(
			`INSERT OR IGNORE INTO channels(name, release_version) VALUES (?, NULL)`, ch,
		); err != nil {
			control.Close()
			return nil, fmt.Errorf("seed channel %s: %w", ch, err)
		}
	}
	return &Store{root: dir, control: control}, nil
}

// Close releases the control DB handle.
func (s *Store) Close() error { return s.control.Close() }

// WriteLogPath is the path to the live, mutable write-log knowledge DB — the
// rewrite side where ingest and record accumulate before a publish.
func (s *Store) WriteLogPath() string {
	return filepath.Join(s.root, "writelog.sqlite")
}

// ReleasePath is the on-disk path of a named release snapshot.
func (s *Store) ReleasePath(version string) string {
	return filepath.Join(s.root, "releases", "truth-"+version+".sqlite")
}

// Publish snapshots the current write-log into an immutable release file via
// VACUUM INTO, records it in the control DB, and returns it. A release version
// must be unique; re-publishing an existing version is refused.
func (s *Store) Publish(version, notes string) (Release, error) {
	if strings.TrimSpace(version) == "" {
		return Release{}, errors.New("empty release version")
	}
	dst := s.ReleasePath(version)
	if _, err := os.Stat(dst); err == nil {
		return Release{}, fmt.Errorf("release %s already exists (immutable)", version)
	}

	// VACUUM INTO reads the source write-log and writes a fully self-contained
	// copy (including FTS5 shadow tables and vector blobs) to dst.
	wl, err := sql.Open("sqlite", s.WriteLogPath())
	if err != nil {
		return Release{}, fmt.Errorf("open write-log: %w", err)
	}
	defer wl.Close()
	// dst path is interpolated as a quoted SQL string literal (single quotes
	// doubled); VACUUM INTO does not accept a bound parameter on all builds.
	lit := "'" + strings.ReplaceAll(dst, "'", "''") + "'"
	if _, err = wl.Exec("VACUUM INTO " + lit); err != nil {
		return Release{}, fmt.Errorf("vacuum into %s: %w", dst, err)
	}

	rel := Release{Version: version, CreatedAt: time.Now().Unix(), Notes: notes}
	if _, err = s.control.Exec(
		`INSERT INTO releases(version, created_at, notes) VALUES (?, ?, ?)`,
		rel.Version, rel.CreatedAt, rel.Notes,
	); err != nil {
		os.Remove(dst) // roll back the orphaned file so retry is clean
		return Release{}, fmt.Errorf("record release: %w", err)
	}
	return rel, nil
}

// ListReleases returns all published releases, newest first.
func (s *Store) ListReleases() ([]Release, error) {
	// rowid tiebreaks releases published within the same second so ordering is
	// deterministic (insertion order = publish order).
	rows, err := s.control.Query(
		`SELECT version, created_at, COALESCE(frozen_at,0), COALESCE(parent,''), notes
		   FROM releases ORDER BY created_at DESC, rowid DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Release
	for rows.Next() {
		var r Release
		if err = rows.Scan(&r.Version, &r.CreatedAt, &r.FrozenAt, &r.Parent, &r.Notes); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// PruneReleases keeps the newest `keep` releases and deletes the rest — both
// the release file (ReleasePath) and its control-DB row — implementing TZ
// FR-15's retention policy. A release currently pinned by a channel
// (stable/testing) is never pruned regardless of age: breaking what's
// actually being served is worse than exceeding the retention count by one.
// keep <= 0 is a no-op (nothing pruned) rather than "prune everything" —
// FR-15 is about bounding growth, not a footgun for clearing history.
// Returns the versions actually pruned, oldest first.
func (s *Store) PruneReleases(keep int) ([]string, error) {
	if keep <= 0 {
		return nil, nil
	}
	releases, err := s.ListReleases() // newest first
	if err != nil {
		return nil, err
	}
	if len(releases) <= keep {
		return nil, nil
	}

	channels, err := s.Channels()
	if err != nil {
		return nil, err
	}
	pinned := make(map[string]bool, len(channels))
	for _, c := range channels {
		if c.Release != "" && c.Release != ChannelUnstable {
			pinned[c.Release] = true
		}
	}

	var candidates []Release
	for i, r := range releases {
		if i < keep || pinned[r.Version] {
			continue // within the retention window, or pinned — keep
		}
		candidates = append(candidates, r)
	}

	pruned := make([]string, 0, len(candidates))
	// Delete oldest first so a failure partway through leaves the more
	// recent (more likely to still matter) releases intact.
	for i := len(candidates) - 1; i >= 0; i-- {
		v := candidates[i].Version
		if err := s.deleteRelease(v); err != nil {
			return pruned, fmt.Errorf("prune release %s: %w", v, err)
		}
		pruned = append(pruned, v)
	}
	return pruned, nil
}

// deleteRelease removes one release's on-disk file and control-DB row. The
// file may already be gone (a prior partial prune, manual cleanup); that's
// not an error, only a failed delete of an existing file is.
func (s *Store) deleteRelease(version string) error {
	if err := os.Remove(s.ReleasePath(version)); err != nil && !os.IsNotExist(err) {
		return err
	}
	_, err := s.control.Exec(`DELETE FROM releases WHERE version = ?`, version)
	return err
}

// SetChannel repoints a channel at a published release. The pointer swap is a
// single control-DB write ("release day"). unstable is reserved for the live
// write-log and cannot be repointed.
func (s *Store) SetChannel(name, version string) error {
	if name == ChannelUnstable {
		return errors.New("unstable always tracks the live write-log")
	}
	var exists int
	if err := s.control.QueryRow(
		`SELECT COUNT(*) FROM releases WHERE version = ?`, version).Scan(&exists); err != nil {
		return err
	}
	if exists == 0 {
		return fmt.Errorf("no such release %q", version)
	}
	_, err := s.control.Exec(
		`UPDATE channels SET release_version = ? WHERE name = ?`, version, name)
	return err
}

// ChannelRelease returns the release version a channel currently points at.
// For unstable it returns "" (the live write-log, not a release).
func (s *Store) ChannelRelease(name string) (string, error) {
	if name == ChannelUnstable {
		return "", nil
	}
	var v sql.NullString
	err := s.control.QueryRow(
		`SELECT release_version FROM channels WHERE name = ?`, name).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("no such channel %q", name)
	}
	if err != nil {
		return "", err
	}
	if !v.Valid {
		return "", ErrNoRelease
	}
	return v.String, nil
}

// Channels lists the standard channels and what they point at. unstable is
// reported as tracking the live write-log; an unassigned channel has Release "".
func (s *Store) Channels() ([]Channel, error) {
	rows, err := s.control.Query(
		`SELECT name, COALESCE(release_version,'') FROM channels ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Channel
	for rows.Next() {
		var c Channel
		if err = rows.Scan(&c.Name, &c.Release); err != nil {
			return nil, err
		}
		if c.Name == ChannelUnstable {
			c.Release = ChannelUnstable // marker: the live write-log
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// RecordProvenance stores the feedback-loop metadata for a recorded result
// (TZ FR-5/FR-17): who produced it, when, and against which source task/spec.
// target_release is left empty — the record rides into the NEXT publish.
func (s *Store) RecordProvenance(recordID int64, author, sourceRef string) error {
	_, err := s.control.Exec(
		`INSERT INTO provenance(record_id, author, ts, target_release, source_ref)
		 VALUES (?, ?, ?, '', ?)`,
		recordID, author, time.Now().Unix(), sourceRef)
	return err
}

// ProvenanceEntry answers "who produced this, when, against what" for a
// recorded document (TZ FR-17).
type ProvenanceEntry struct {
	RecordID      int64  `json:"record_id"`
	Author        string `json:"author"`
	Ts            int64  `json:"ts"`
	TargetRelease string `json:"target_release"` // empty until the record's next publish lands
	SourceRef     string `json:"source_ref"`
}

// Provenance returns the provenance trail for a recorded document id, oldest
// first (TZ FR-17).
func (s *Store) Provenance(recordID int64) ([]ProvenanceEntry, error) {
	rows, err := s.control.Query(
		`SELECT record_id, author, ts, target_release, source_ref
		   FROM provenance WHERE record_id = ? ORDER BY ts`, recordID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ProvenanceEntry
	for rows.Next() {
		var p ProvenanceEntry
		if err = rows.Scan(&p.RecordID, &p.Author, &p.Ts, &p.TargetRelease, &p.SourceRef); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// Freeze marks a published release as frozen (TZ FR-11, §5 glossary): a
// stabilization-window signal that this release is now the testing candidate
// and further fixes should land in unstable rather than repointing it.
// Freezing an unknown or already-frozen release is refused.
func (s *Store) Freeze(version string) (int64, error) {
	var frozenAt sql.NullInt64
	err := s.control.QueryRow(
		`SELECT frozen_at FROM releases WHERE version = ?`, version).Scan(&frozenAt)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, fmt.Errorf("no such release %q", version)
	}
	if err != nil {
		return 0, err
	}
	if frozenAt.Valid {
		return 0, fmt.Errorf("release %s already frozen at %d", version, frozenAt.Int64)
	}
	ts := time.Now().Unix()
	if _, err = s.control.Exec(
		`UPDATE releases SET frozen_at = ? WHERE version = ?`, ts, version); err != nil {
		return 0, err
	}
	return ts, nil
}

// Resolve maps a reference (a channel name or a raw release version) to the
// on-disk DB path a reader should open. "unstable" resolves to the live
// write-log; a channel resolves to its release file; anything else is treated
// as a release version.
func (s *Store) Resolve(ref string) (string, error) {
	switch ref {
	case ChannelUnstable:
		return s.WriteLogPath(), nil
	case ChannelStable, ChannelTesting:
		version, err := s.ChannelRelease(ref)
		if err != nil {
			return "", err
		}
		return s.ReleasePath(version), nil
	default:
		// Treat as an explicit release version.
		path := s.ReleasePath(ref)
		if _, err := os.Stat(path); err != nil {
			return "", fmt.Errorf("no such release or channel %q", ref)
		}
		return path, nil
	}
}

// OpenRelease opens a resolved DB path read-only for grounding queries. The
// handle is safe to share across a pool of readers; the file is immutable.
func OpenRelease(path string) (*sql.DB, error) {
	dsn := "file:" + path + "?_pragma=query_only(true)&_pragma=busy_timeout(5000)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	if err = db.Ping(); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

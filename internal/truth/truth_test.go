package truth

import (
	"path/filepath"
	"testing"

	"github.com/ruslano69/funcfinder/internal/knowledge"
)

// seedWriteLog adds documents to the store's live write-log via the real
// knowledge layer, so the publish path is exercised end-to-end.
func seedWriteLog(t *testing.T, s *Store, docs ...[2]string) {
	t.Helper()
	db, err := knowledge.Open(s.WriteLogPath())
	if err != nil {
		t.Fatalf("open write-log: %v", err)
	}
	defer db.Close()
	for _, d := range docs {
		if _, err := knowledge.Add(db, d[0], d[1], "spec", "{}", nil); err != nil {
			t.Fatalf("add %q: %v", d[0], err)
		}
	}
}

func TestPublishChannelResolveGround(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer s.Close()

	// Ingest v1 truth and publish it.
	seedWriteLog(t, s, [2]string{"Auth spec", "login uses OAuth2 device flow"})
	rel, err := s.Publish("2026.07", "first release")
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	if rel.Version != "2026.07" {
		t.Fatalf("version = %q, want 2026.07", rel.Version)
	}

	// Point stable at it and resolve the channel.
	if err := s.SetChannel(ChannelStable, "2026.07"); err != nil {
		t.Fatalf("set channel: %v", err)
	}
	path, err := s.Resolve(ChannelStable)
	if err != nil {
		t.Fatalf("resolve stable: %v", err)
	}
	if path != s.ReleasePath("2026.07") {
		t.Fatalf("resolved %q, want release path", path)
	}

	// Ground a query against the immutable release.
	rdb, err := OpenRelease(path)
	if err != nil {
		t.Fatalf("open release: %v", err)
	}
	defer rdb.Close()
	res, err := knowledge.SearchFTS(rdb, "OAuth2", 5, true)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(res) != 1 || res[0].Title != "Auth spec" {
		t.Fatalf("grounding miss: got %d results %+v", len(res), res)
	}
}

func TestReproducibility_ReleaseFrozenAfterMoreWrites(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()

	seedWriteLog(t, s, [2]string{"Doc A", "alpha content"})
	if _, err := s.Publish("2026.07", ""); err != nil {
		t.Fatalf("publish v1: %v", err)
	}

	// Truth keeps flowing into the write-log AFTER publish...
	seedWriteLog(t, s, [2]string{"Doc B", "beta content"})

	// ...but the published release must NOT see the new doc (immutability).
	rel1, _ := s.Resolve("2026.07")
	rdb, err := OpenRelease(rel1)
	if err != nil {
		t.Fatalf("open release: %v", err)
	}
	defer rdb.Close()
	n, err := knowledge.Count(rdb)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Fatalf("release should be frozen at 1 doc, has %d", n)
	}

	// unstable (live write-log) DOES see both.
	live, _ := s.Resolve(ChannelUnstable)
	ldb, err := knowledge.Open(live)
	if err != nil {
		t.Fatalf("open live: %v", err)
	}
	defer ldb.Close()
	if n, _ := knowledge.Count(ldb); n != 2 {
		t.Fatalf("write-log should have 2 docs, has %d", n)
	}
}

func TestPublishRefusesDuplicateVersion(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()

	seedWriteLog(t, s, [2]string{"Doc", "content"})
	if _, err := s.Publish("2026.07", ""); err != nil {
		t.Fatalf("publish: %v", err)
	}
	if _, err := s.Publish("2026.07", ""); err == nil {
		t.Fatal("expected duplicate publish to be refused")
	}
}

func TestSetChannelUnknownReleaseRejected(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()
	if err := s.SetChannel(ChannelStable, "2099.01"); err == nil {
		t.Fatal("expected set-channel to unknown release to fail")
	}
	if err := s.SetChannel(ChannelUnstable, "2026.07"); err == nil {
		t.Fatal("expected repointing unstable to fail")
	}
}

func TestListReleases(t *testing.T) {
	dir := t.TempDir()
	s, _ := Open(dir)
	defer s.Close()
	seedWriteLog(t, s, [2]string{"D", "c"})
	s.Publish("2026.06", "a")
	s.Publish("2026.07", "b")
	rels, err := s.ListReleases()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(rels) != 2 {
		t.Fatalf("want 2 releases, got %d", len(rels))
	}
	// newest first
	if rels[0].Version != "2026.07" {
		t.Fatalf("want 2026.07 first, got %q", rels[0].Version)
	}
	_ = filepath.Base(dir)
}

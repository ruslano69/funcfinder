package truth

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
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

func TestFreeze(t *testing.T) {
	dir := t.TempDir()
	s, _ := Open(dir)
	defer s.Close()
	seedWriteLog(t, s, [2]string{"D", "c"})
	if _, err := s.Publish("2026.07", ""); err != nil {
		t.Fatalf("publish: %v", err)
	}

	if _, err := s.Freeze("2099.01"); err == nil {
		t.Fatal("expected freeze of unknown release to fail")
	}

	ts, err := s.Freeze("2026.07")
	if err != nil {
		t.Fatalf("freeze: %v", err)
	}
	if ts == 0 {
		t.Fatal("want non-zero frozen_at")
	}
	if _, err := s.Freeze("2026.07"); err == nil {
		t.Fatal("expected re-freezing an already-frozen release to fail")
	}

	rels, err := s.ListReleases()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if rels[0].FrozenAt != ts {
		t.Fatalf("want FrozenAt %d reflected in ListReleases, got %d", ts, rels[0].FrozenAt)
	}
}

func TestProvenance(t *testing.T) {
	dir := t.TempDir()
	s, _ := Open(dir)
	defer s.Close()

	if err := s.RecordProvenance(1, "ruslan", "task-42"); err != nil {
		t.Fatalf("record provenance: %v", err)
	}
	if err := s.RecordProvenance(1, "ruslan", "task-42-followup"); err != nil {
		t.Fatalf("record provenance: %v", err)
	}

	entries, err := s.Provenance(1)
	if err != nil {
		t.Fatalf("provenance: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("want 2 provenance entries, got %d", len(entries))
	}
	if entries[0].Author != "ruslan" || entries[0].SourceRef != "task-42" {
		t.Fatalf("unexpected first entry: %+v", entries[0])
	}

	none, err := s.Provenance(999)
	if err != nil {
		t.Fatalf("provenance of unknown record: %v", err)
	}
	if len(none) != 0 {
		t.Fatalf("want no entries for unknown record, got %d", len(none))
	}
}

func TestPruneReleases_KeepsNewestK(t *testing.T) {
	dir := t.TempDir()
	s, _ := Open(dir)
	defer s.Close()

	var versions []string
	for i := 1; i <= 5; i++ {
		seedWriteLog(t, s, [2]string{"D", "c"})
		v := fmt.Sprintf("2026.0%d", i)
		if _, err := s.Publish(v, ""); err != nil {
			t.Fatalf("publish %s: %v", v, err)
		}
		versions = append(versions, v)
	}
	// versions = [2026.01 .. 2026.05], oldest to newest

	pruned, err := s.PruneReleases(2)
	if err != nil {
		t.Fatalf("PruneReleases: %v", err)
	}
	wantPruned := []string{"2026.01", "2026.02", "2026.03"}
	if !reflect.DeepEqual(pruned, wantPruned) {
		t.Fatalf("pruned = %v, want %v (oldest first)", pruned, wantPruned)
	}

	remaining, err := s.ListReleases()
	if err != nil {
		t.Fatalf("ListReleases: %v", err)
	}
	if len(remaining) != 2 {
		t.Fatalf("want 2 releases remaining, got %d: %+v", len(remaining), remaining)
	}
	got := map[string]bool{}
	for _, r := range remaining {
		got[r.Version] = true
	}
	if !got["2026.04"] || !got["2026.05"] {
		t.Fatalf("want 2026.04 and 2026.05 to survive, got %+v", remaining)
	}

	// The pruned releases' files must actually be gone from disk, not just
	// the control-DB rows.
	for _, v := range wantPruned {
		if _, err := os.Stat(s.ReleasePath(v)); !os.IsNotExist(err) {
			t.Errorf("release file for %s still exists on disk", v)
		}
	}
	// The survivors' files must still be there.
	for _, v := range []string{"2026.04", "2026.05"} {
		if _, err := os.Stat(s.ReleasePath(v)); err != nil {
			t.Errorf("release file for %s missing: %v", v, err)
		}
	}
}

func TestPruneReleases_NeverPrunesChannelPinnedRelease(t *testing.T) {
	dir := t.TempDir()
	s, _ := Open(dir)
	defer s.Close()

	for i := 1; i <= 5; i++ {
		seedWriteLog(t, s, [2]string{"D", "c"})
		if _, err := s.Publish(fmt.Sprintf("2026.0%d", i), ""); err != nil {
			t.Fatalf("publish: %v", err)
		}
	}
	// Point stable at the oldest release — it would normally be pruned at
	// keep=2, but must survive because a channel depends on it.
	if err := s.SetChannel(ChannelStable, "2026.01"); err != nil {
		t.Fatalf("set-channel: %v", err)
	}

	pruned, err := s.PruneReleases(2)
	if err != nil {
		t.Fatalf("PruneReleases: %v", err)
	}
	for _, v := range pruned {
		if v == "2026.01" {
			t.Fatalf("pinned release 2026.01 was pruned; pruned=%v", pruned)
		}
	}
	if _, err := os.Stat(s.ReleasePath("2026.01")); err != nil {
		t.Fatalf("pinned release's file should still exist: %v", err)
	}
	// stable must still resolve.
	if _, err := s.Resolve(ChannelStable); err != nil {
		t.Fatalf("stable channel should still resolve after prune: %v", err)
	}
}

func TestPruneReleases_KeepZeroOrFewerIsNoop(t *testing.T) {
	dir := t.TempDir()
	s, _ := Open(dir)
	defer s.Close()

	for i := 1; i <= 3; i++ {
		seedWriteLog(t, s, [2]string{"D", "c"})
		if _, err := s.Publish(fmt.Sprintf("2026.0%d", i), ""); err != nil {
			t.Fatalf("publish: %v", err)
		}
	}

	for _, keep := range []int{0, -1} {
		pruned, err := s.PruneReleases(keep)
		if err != nil {
			t.Fatalf("PruneReleases(%d): %v", keep, err)
		}
		if len(pruned) != 0 {
			t.Fatalf("PruneReleases(%d): want no-op, pruned %v", keep, pruned)
		}
	}

	remaining, _ := s.ListReleases()
	if len(remaining) != 3 {
		t.Fatalf("want all 3 releases still present, got %d", len(remaining))
	}
}

func TestPruneReleases_FewerReleasesThanKeepIsNoop(t *testing.T) {
	dir := t.TempDir()
	s, _ := Open(dir)
	defer s.Close()

	seedWriteLog(t, s, [2]string{"D", "c"})
	if _, err := s.Publish("2026.01", ""); err != nil {
		t.Fatalf("publish: %v", err)
	}

	pruned, err := s.PruneReleases(5)
	if err != nil {
		t.Fatalf("PruneReleases: %v", err)
	}
	if len(pruned) != 0 {
		t.Fatalf("want no-op when releases < keep, pruned %v", pruned)
	}
}

package main

import (
	"testing"

	"github.com/ruslano69/funcfinder/internal/knowledge"
	"github.com/ruslano69/funcfinder/internal/truth"
)

// TestMetaJSON pins the provenance blob shape that ingest/record attach to docs.
func TestMetaJSON(t *testing.T) {
	got := metaJSON("ruslan", "backend,api", "v3", "")
	// order-independent membership check via a fresh parse would be ideal, but
	// json.Marshal of a map sorts keys, so this is stable.
	want := `{"author":"ruslan","role_tags":"backend,api","source_version":"v3"}`
	if got != want {
		t.Fatalf("metaJSON = %s, want %s", got, want)
	}
	if empty := metaJSON("", "", "", ""); empty != "{}" {
		t.Fatalf("empty metaJSON = %s, want {}", empty)
	}
}

// TestParseEmbedding covers the BYO-embedding CLI parsing (bracketed + bare).
func TestParseEmbedding(t *testing.T) {
	for _, in := range []string{"0.1,0.2,0.3", "[0.1, 0.2, 0.3]", " 0.1 , 0.2 , 0.3 "} {
		v := parseEmbedding(in)
		if len(v) != 3 || v[0] != 0.1 {
			t.Fatalf("parseEmbedding(%q) = %v", in, v)
		}
	}
	if v := parseEmbedding(""); v != nil {
		t.Fatalf("empty embedding = %v, want nil", v)
	}
}

// TestLifecycle exercises the rewrite→publish→readonly path the CLI drives,
// through the same library calls, asserting immutability of a published release.
func TestLifecycle(t *testing.T) {
	dir := t.TempDir()
	s, err := truth.Open(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()

	// ingest
	wl, err := knowledge.Open(s.WriteLogPath())
	if err != nil {
		t.Fatalf("open write-log: %v", err)
	}
	id, err := knowledge.Add(wl, "Auth spec", "OAuth2 device flow", "spec",
		metaJSON("ruslan", "backend", "v1", ""), nil)
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	// record with provenance
	rid, err := knowledge.Add(wl, "Decision", "use WAL", "decision",
		metaJSON("ruslan", "", "", "spec:Auth"), nil)
	if err != nil {
		t.Fatalf("record add: %v", err)
	}
	wl.Close()
	if err := s.RecordProvenance(rid, "ruslan", "spec:Auth"); err != nil {
		t.Fatalf("provenance: %v", err)
	}

	// publish + point stable
	if _, err := s.Publish("2026.07", "first"); err != nil {
		t.Fatalf("publish: %v", err)
	}
	if err := s.SetChannel(truth.ChannelStable, "2026.07"); err != nil {
		t.Fatalf("set channel: %v", err)
	}

	// grounding against stable finds the spec
	path, _ := s.Resolve(truth.ChannelStable)
	rdb, err := truth.OpenRelease(path)
	if err != nil {
		t.Fatalf("open release: %v", err)
	}
	res, err := knowledge.SearchFTS(rdb, "OAuth2", 5, true)
	rdb.Close()
	if err != nil || len(res) != 1 || res[0].ID != id {
		t.Fatalf("grounding miss: err=%v res=%+v", err, res)
	}

	// post-publish write is invisible to the frozen release
	wl2, _ := knowledge.Open(s.WriteLogPath())
	knowledge.Add(wl2, "New", "post-publish content", "spec", "{}", nil)
	wl2.Close()
	rdb2, _ := truth.OpenRelease(path)
	n, _ := knowledge.Count(rdb2)
	rdb2.Close()
	if n != 2 {
		t.Fatalf("frozen release should stay at 2 docs, has %d", n)
	}
}

package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ruslano69/funcfinder/internal/embed"
	"github.com/ruslano69/funcfinder/internal/knowledge"
	"github.com/ruslano69/funcfinder/internal/truth"
)

// newTestReadServer stands up a store with one published release pointed at by
// stable, and returns an httptest.Server wrapping the HTTP read-server's routes
// (no real listener, no goroutines) so handlers can be exercised directly.
func newTestReadServer(t *testing.T) *httptest.Server {
	t.Helper()
	dir := t.TempDir()
	store, err := truth.Open(dir)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	wl, err := knowledge.Open(store.WriteLogPath())
	if err != nil {
		t.Fatalf("open write-log: %v", err)
	}
	if _, err := knowledge.Add(wl, "Auth spec", "login uses OAuth2 device flow", "spec",
		metaJSON("ruslan", "backend", "v1", ""), nil); err != nil {
		t.Fatalf("add doc: %v", err)
	}
	if _, err := knowledge.Add(wl, "Deploy runbook", "kubectl rollout restart", "spec",
		metaJSON("ruslan", "ops", "v1", ""), nil); err != nil {
		t.Fatalf("add doc: %v", err)
	}
	wl.Close()

	if _, err := store.Publish("2026.07", "first"); err != nil {
		t.Fatalf("publish: %v", err)
	}
	if err := store.SetChannel(truth.ChannelStable, "2026.07"); err != nil {
		t.Fatalf("set channel: %v", err)
	}

	srv := &server{store: store, channel: truth.ChannelStable, poolSize: 4, embc: embed.New("", "")}
	rel, err := srv.openRelease()
	if err != nil {
		t.Fatalf("open release: %v", err)
	}
	srv.cur.Store(rel)
	// Close the release handle so Windows can delete the temp file (an open
	// handle blocks TempDir's RemoveAll cleanup).
	t.Cleanup(func() { rel.db.Close() })

	ts := httptest.NewServer(srv.routes())
	t.Cleanup(ts.Close)
	return ts
}

// getJSON issues a GET and decodes the JSON body into v, returning the status.
func getJSON(t *testing.T, url string, v any) int {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	if v != nil {
		if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
			t.Fatalf("decode %s: %v", url, err)
		}
	}
	return resp.StatusCode
}

func TestHTTPHealthz(t *testing.T) {
	ts := newTestReadServer(t)
	var body map[string]any
	if code := getJSON(t, ts.URL+"/healthz", &body); code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}
	if body["status"] != "ok" {
		t.Errorf("status field = %v, want ok", body["status"])
	}
	if body["release"] != "2026.07" {
		t.Errorf("release = %v, want 2026.07", body["release"])
	}
	if body["channel"] != truth.ChannelStable {
		t.Errorf("channel = %v, want %s", body["channel"], truth.ChannelStable)
	}
}

func TestHTTPSearch(t *testing.T) {
	ts := newTestReadServer(t)
	var results []knowledge.Result
	if code := getJSON(t, ts.URL+"/search?q=OAuth2", &results); code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}
	if len(results) != 1 || results[0].Title != "Auth spec" {
		t.Fatalf("grounding miss: got %d results %+v", len(results), results)
	}
}

func TestHTTPSearchModes(t *testing.T) {
	ts := newTestReadServer(t)
	// fts mode works without an embedder; hybrid degrades to FTS the same way.
	for _, mode := range []string{"fts", "hybrid"} {
		var results []knowledge.Result
		code := getJSON(t, ts.URL+"/search?q=kubectl&mode="+mode, &results)
		if code != http.StatusOK {
			t.Fatalf("mode %s: status = %d, want 200", mode, code)
		}
		if len(results) != 1 || results[0].Title != "Deploy runbook" {
			t.Fatalf("mode %s: grounding miss: %+v", mode, results)
		}
	}
}

func TestHTTPSearchValidation(t *testing.T) {
	ts := newTestReadServer(t)
	cases := []struct {
		url  string
		want int
	}{
		{"/search", http.StatusBadRequest},                 // no q
		{"/search?q=x&limit=abc", http.StatusBadRequest},   // bad limit
		{"/search?q=x&limit=0", http.StatusBadRequest},     // limit < 1
		{"/search?q=x&mode=bogus", http.StatusBadRequest},  // unknown mode
		{"/search?q=x&mode=vec", http.StatusBadRequest},    // vec with no embedding/embedder
	}
	for _, c := range cases {
		if code := getJSON(t, ts.URL+c.url, nil); code != c.want {
			t.Errorf("GET %s: status = %d, want %d", c.url, code, c.want)
		}
	}
}

func TestHTTPSearchLimitClamped(t *testing.T) {
	ts := newTestReadServer(t)
	// An over-cap limit must be accepted (clamped), not rejected.
	if code := getJSON(t, ts.URL+"/search?q=spec&limit=99999", nil); code != http.StatusOK {
		t.Fatalf("over-cap limit: status = %d, want 200 (clamped)", code)
	}
}

func TestHTTPRead(t *testing.T) {
	ts := newTestReadServer(t)
	var docs []knowledge.Doc
	if code := getJSON(t, ts.URL+"/read?id=1&context=0", &docs); code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}
	if len(docs) != 1 || docs[0].ID != 1 {
		t.Fatalf("want exactly doc id=1, got %+v", docs)
	}

	if code := getJSON(t, ts.URL+"/read", nil); code != http.StatusBadRequest {
		t.Errorf("read without id: status = %d, want 400", code)
	}
	if code := getJSON(t, ts.URL+"/read?id=notanint", nil); code != http.StatusBadRequest {
		t.Errorf("read with bad id: status = %d, want 400", code)
	}
}

func TestHTTPContext(t *testing.T) {
	ts := newTestReadServer(t)
	var docs []knowledge.Doc
	if code := getJSON(t, ts.URL+"/context?role=ops", &docs); code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}
	if len(docs) != 1 || docs[0].Title != "Deploy runbook" {
		t.Fatalf("role filter miss: got %+v", docs)
	}
	if code := getJSON(t, ts.URL+"/context", nil); code != http.StatusBadRequest {
		t.Errorf("context without role: status = %d, want 400", code)
	}
}

func TestHTTPReleasesAndChannels(t *testing.T) {
	ts := newTestReadServer(t)
	var rels []truth.Release
	if code := getJSON(t, ts.URL+"/releases", &rels); code != http.StatusOK {
		t.Fatalf("releases status = %d, want 200", code)
	}
	if len(rels) != 1 || rels[0].Version != "2026.07" {
		t.Fatalf("want 1 release 2026.07, got %+v", rels)
	}

	var chans []truth.Channel
	if code := getJSON(t, ts.URL+"/channels", &chans); code != http.StatusOK {
		t.Fatalf("channels status = %d, want 200", code)
	}
	if len(chans) == 0 {
		t.Fatal("want at least one channel")
	}
}

// TestHTTPReadonlyContract pins the CQRS guarantee: a read node answers no
// write verb on any route. Every mutation lives on the writer/CLI/MCP side.
func TestHTTPReadonlyContract(t *testing.T) {
	ts := newTestReadServer(t)
	for _, path := range []string{"/search", "/read", "/context", "/releases", "/channels", "/healthz"} {
		resp, err := http.Post(ts.URL+path, "application/json", nil)
		if err != nil {
			t.Fatalf("POST %s: %v", path, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusMethodNotAllowed {
			t.Errorf("POST %s: status = %d, want 405", path, resp.StatusCode)
		}
	}
}

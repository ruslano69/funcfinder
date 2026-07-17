package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/ruslano69/funcfinder/internal/embed"
	"github.com/ruslano69/funcfinder/internal/knowledge"
	"github.com/ruslano69/funcfinder/internal/truth"
)

// The HTTP read-server (TZ FR-20) is the same stateless read node as the TCP
// serve command — same release model, same channel hot-swap — exposed over
// HTTP/JSON instead of the line protocol, so ordinary clients (curl, a
// browser, a service mesh) can ground without speaking the ###END### framing.
// It reuses the `server` struct wholesale: the release pointer, the pool, the
// embedder, and watchChannel all behave identically; only the transport and
// the response encoding differ.
//
// Every route is readonly by construction. A read-server sits on the readonly
// side of the CQRS split (§7.1) and must never accept ingest/publish — those
// live on the writer daemon, the CLI, and the MCP server. There is
// deliberately no POST /ingest here.

// maxHTTPLimit caps the per-request result count so a client can't ask one
// read node for an unbounded scan of a release.
const maxHTTPLimit = 100

// httpError writes a JSON error body with the given status. Keeping the shape
// uniform ({"error": ...}) means a client parses success and failure the same
// way regardless of which endpoint produced it.
func httpError(w http.ResponseWriter, status int, format string, args ...any) {
	writeJSON(w, status, map[string]string{"error": fmt.Sprintf(format, args...)})
}

// writeJSON encodes v as the response body. An encode failure after the header
// is already committed can't be recovered into a clean status, so it's logged
// rather than swallowed.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		fmt.Fprintf(os.Stderr, "http: encode response: %v\n", err)
	}
}

// clampLimit parses a ?limit= value, applying def when absent/blank and
// bounding it to [1, maxHTTPLimit]. A malformed value is an explicit client
// error rather than a silent fallback, so a typo'd limit doesn't quietly
// return the default and mislead.
func clampLimit(raw string, def int) (int, error) {
	if raw == "" {
		return def, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("limit %q is not an integer", raw)
	}
	if n < 1 {
		return 0, fmt.Errorf("limit must be >= 1, got %d", n)
	}
	if n > maxHTTPLimit {
		n = maxHTTPLimit
	}
	return n, nil
}

// handleHealthz reports liveness plus which release the node is currently
// serving — the readiness signal a load balancer or the hot-swap-aware caller
// polls. It never touches the release DB, so it stays green even mid-swap.
func (s *server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	rel := s.cur.Load()
	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"channel": s.channel,
		"release": rel.version,
		"served":  s.served.Load(),
	})
}

// handleSearch grounds a query against the current release, mirroring the CLI
// `search` command's mode switch (fts|vec|hybrid|regex) and its live-embedding
// behavior, so the HTTP and CLI grounding paths can't drift.
func (s *server) handleSearch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	query := q.Get("q")
	if query == "" {
		httpError(w, http.StatusBadRequest, "q (query) is required")
		return
	}
	limit, err := clampLimit(q.Get("limit"), 10)
	if err != nil {
		httpError(w, http.StatusBadRequest, "%v", err)
		return
	}
	mode := q.Get("mode")
	if mode == "" {
		mode = "hybrid"
	}
	prefix := q.Get("prefix") != "false" // default on, matching CLI

	// Query embedding: explicit ?embedding= wins; else a configured embedder
	// embeds the query live for vec/hybrid. Same precedence as runSearch.
	emb := parseEmbedding(q.Get("embedding"))
	if len(emb) == 0 && s.embc.Enabled() && (mode == "vec" || mode == "hybrid") {
		if emb, err = s.embc.Embed(query); err != nil {
			httpError(w, http.StatusBadGateway, "embed query: %v", err)
			return
		}
	}

	db := s.cur.Load().db
	var results []knowledge.Result
	switch mode {
	case "fts":
		results, err = knowledge.SearchFTS(db, query, limit, prefix)
	case "vec":
		if len(emb) == 0 {
			httpError(w, http.StatusBadRequest, "vec mode needs ?embedding= (no embedder configured)")
			return
		}
		results, err = knowledge.SearchVec(db, emb, limit, knowledge.MetricCosine, "")
	case "regex":
		results, err = knowledge.SearchRegex(db, query, limit, "")
	case "hybrid":
		results, err = knowledge.SearchHybrid(db, query, emb, limit, knowledge.MetricCosine, "", prefix)
	default:
		httpError(w, http.StatusBadRequest, "unknown mode %q (want fts|vec|hybrid|regex)", mode)
		return
	}
	if err != nil {
		httpError(w, http.StatusInternalServerError, "search: %v", err)
		return
	}
	s.served.Add(1)
	writeJSON(w, http.StatusOK, results)
}

// handleRead returns the contiguous id-context..id+context neighborhood of a
// chunk — the HTTP face of the CLI `read --id --context` primitive.
func (s *server) handleRead(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	idRaw := q.Get("id")
	if idRaw == "" {
		httpError(w, http.StatusBadRequest, "id is required")
		return
	}
	id, err := strconv.ParseInt(idRaw, 10, 64)
	if err != nil {
		httpError(w, http.StatusBadRequest, "id %q is not an integer", idRaw)
		return
	}
	context := 2
	if raw := q.Get("context"); raw != "" {
		if context, err = strconv.Atoi(raw); err != nil || context < 0 {
			httpError(w, http.StatusBadRequest, "context must be a non-negative integer")
			return
		}
	}
	docs, err := knowledge.ReadRange(s.cur.Load().db, id, context)
	if err != nil {
		httpError(w, http.StatusInternalServerError, "read: %v", err)
		return
	}
	s.served.Add(1)
	writeJSON(w, http.StatusOK, docs)
}

// handleContext returns the documents tagged for a role — the truth-under-role
// view (FR-9), matching the CLI `context --role` command.
func (s *server) handleContext(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	role := q.Get("role")
	if role == "" {
		httpError(w, http.StatusBadRequest, "role is required")
		return
	}
	limit, err := clampLimit(q.Get("limit"), 20)
	if err != nil {
		httpError(w, http.StatusBadRequest, "%v", err)
		return
	}
	docs, err := knowledge.ByRole(s.cur.Load().db, role, limit)
	if err != nil {
		httpError(w, http.StatusInternalServerError, "context: %v", err)
		return
	}
	s.served.Add(1)
	writeJSON(w, http.StatusOK, docs)
}

// handleReleases and handleChannels expose the control-plane listings — these
// read the control DB (release/channel metadata), not the served release, so
// they answer regardless of which release is currently swapped in.
func (s *server) handleReleases(w http.ResponseWriter, r *http.Request) {
	rels, err := s.store.ListReleases()
	if err != nil {
		httpError(w, http.StatusInternalServerError, "releases: %v", err)
		return
	}
	writeJSON(w, http.StatusOK, rels)
}

func (s *server) handleChannels(w http.ResponseWriter, r *http.Request) {
	chans, err := s.store.Channels()
	if err != nil {
		httpError(w, http.StatusInternalServerError, "channels: %v", err)
		return
	}
	writeJSON(w, http.StatusOK, chans)
}

// routes builds the mux. A GET-only wrapper rejects write verbs uniformly:
// nothing on a read node mutates, so POST/PUT/DELETE are 405 everywhere, which
// also makes the readonly contract legible from the outside.
func (s *server) routes() http.Handler {
	mux := http.NewServeMux()
	get := func(path string, h http.HandlerFunc) {
		mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				httpError(w, http.StatusMethodNotAllowed, "%s only supports GET (read-server is readonly)", path)
				return
			}
			h(w, r)
		})
	}
	get("/healthz", s.handleHealthz)
	get("/search", s.handleSearch)
	get("/read", s.handleRead)
	get("/context", s.handleContext)
	get("/releases", s.handleReleases)
	get("/channels", s.handleChannels)
	return mux
}

func runServeHTTP(s *truth.Store, embc *embed.Client, args []string) {
	fs := flag.NewFlagSet("serve-http", flag.ExitOnError)
	addr := fs.String("addr", "127.0.0.1:9100", "listen address")
	channel := fs.String("channel", truth.ChannelStable, "channel to serve (stable|testing)")
	pool := fs.Int("pool", 16, "read connection pool size")
	swapMs := fs.Int("swap-check-ms", 500, "channel repoint poll interval")
	fs.Parse(args)

	srv := &server{store: s, channel: *channel, poolSize: *pool, embc: embc}

	rel, err := srv.openRelease()
	if err != nil {
		fatalf("serve-http: channel %q not ready (publish + set-channel first): %v", *channel, err)
	}
	srv.cur.Store(rel)

	go srv.watchChannel(time.Duration(*swapMs)*time.Millisecond, 2*time.Second)

	httpSrv := &http.Server{
		Addr:    *addr,
		Handler: srv.routes(),
		// ReadHeaderTimeout bounds slow-header (Slowloris) clients; the read
		// path is fast so generous overall timeouts are still safe.
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second, // keep-alive reuse
	}
	fmt.Fprintf(os.Stderr, "http read-server on %s serving channel %q @ release %s (pool=%d)\n",
		*addr, *channel, rel.version, *pool)
	fmt.Fprintf(os.Stderr, "  GET /search?q=...&mode=hybrid&limit=10  /read?id=N&context=K  /context?role=R  /releases  /channels  /healthz\n")

	if err := httpSrv.ListenAndServe(); err != nil {
		fatalf("http serve: %v", err)
	}
}

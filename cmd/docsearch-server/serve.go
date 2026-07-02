package main

import (
	"bufio"
	"database/sql"
	"flag"
	"fmt"
	"net"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/ruslano69/funcfinder/internal/embed"
	"github.com/ruslano69/funcfinder/internal/knowledge"
	"github.com/ruslano69/funcfinder/internal/truth"
)

// activeRelease is the read-only handle a reader serves against, tagged with
// the version it materializes so the swapper can detect channel moves.
type activeRelease struct {
	version string
	db      *sql.DB
}

// server is a stateless read node: it grounds queries against whatever release
// the channel currently points at, and hot-swaps that release atomically when
// the channel moves — the "release day" pointer flip, with zero dropped
// requests, expressed as a single atomic.Pointer store.
type server struct {
	store    *truth.Store
	channel  string
	lite     bool
	poolSize int
	embc     *embed.Client // enables semantic/hybrid grounding when configured
	cur      atomic.Pointer[activeRelease]
	served   atomic.Int64
}

// openRelease resolves the channel's current version and opens it read-only
// with a reader pool sized for concurrency.
func (s *server) openRelease() (*activeRelease, error) {
	version, err := s.store.ChannelRelease(s.channel)
	if err != nil {
		return nil, err
	}
	db, err := truth.OpenRelease(s.store.ReleasePath(version))
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(s.poolSize)
	db.SetMaxIdleConns(s.poolSize)
	return &activeRelease{version: version, db: db}, nil
}

// watchChannel polls the control DB and hot-swaps the active release when the
// channel is repointed. The old handle is closed after a grace delay so any
// in-flight query on it finishes first (immutable files never conflict).
func (s *server) watchChannel(interval, grace time.Duration) {
	for range time.Tick(interval) {
		version, err := s.store.ChannelRelease(s.channel)
		if err != nil || version == s.cur.Load().version {
			continue
		}
		next, err := s.openRelease()
		if err != nil {
			fmt.Fprintf(os.Stderr, "hot-swap: failed to open %s: %v\n", version, err)
			continue
		}
		old := s.cur.Swap(next)
		fmt.Fprintf(os.Stderr, ">> hot-swapped %s → %s (in-flight requests drain on old)\n",
			old.version, next.version)
		go func(o *activeRelease) {
			time.Sleep(grace)
			o.db.Close()
		}(old)
	}
}

// handle serves one keep-alive connection: newline-delimited queries in,
// result block terminated by ###END### out. One goroutine per connection —
// the scheduler multiplexes thousands of these over the OS threads for free.
func (s *server) handle(conn net.Conn) {
	defer conn.Close()
	if tc, ok := conn.(*net.TCPConn); ok {
		tc.SetNoDelay(true)
	}
	r := bufio.NewReader(conn)
	w := bufio.NewWriter(conn)
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		query := strings.TrimSpace(line)
		if query == "" {
			continue
		}
		rel := s.cur.Load()
		if _, err := w.WriteString(s.search(rel.db, query)); err != nil {
			return
		}
		if w.Flush() != nil {
			return
		}
		s.served.Add(1)
	}
}

// search runs the grounding query. lite mirrors the bare-rowid path used in the
// PureBasic comparison; the default is the real hybrid/FTS product query.
func (s *server) search(db *sql.DB, query string) string {
	if s.lite {
		fts := knowledge.BuildFTSQuery(query, true)
		rows, err := db.Query(`SELECT rowid FROM docs_fts WHERE docs_fts MATCH ? LIMIT 20`, fts)
		if err != nil {
			return "count=0\n###END###\n"
		}
		n := 0
		for rows.Next() {
			n++
		}
		rows.Close()
		return fmt.Sprintf("count=%d\n###END###\n", n)
	}

	// Semantic depth: when an embedder is configured, ground via hybrid
	// (FTS + vector); otherwise pure FTS.
	var res []knowledge.Result
	var err error
	if s.embc.Enabled() {
		if qv, e := s.embc.Embed(query); e == nil {
			res, err = knowledge.SearchHybrid(db, query, qv, 5, knowledge.MetricCosine, "", true)
		} else {
			res, err = knowledge.SearchFTS(db, query, 5, true)
		}
	} else {
		res, err = knowledge.SearchFTS(db, query, 5, true)
	}
	if err != nil {
		return "count=0\n###END###\n"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "count=%d\n", len(res))
	for _, r := range res {
		fmt.Fprintf(&b, "[%d] %s :: %s\n", r.ID, r.Title, r.Preview(80))
	}
	b.WriteString("###END###\n")
	return b.String()
}

func runServe(s *truth.Store, embc *embed.Client, args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	addr := fs.String("addr", "127.0.0.1:9099", "listen address")
	channel := fs.String("channel", truth.ChannelStable, "channel to serve (stable|testing)")
	pool := fs.Int("pool", 16, "read connection pool size")
	lite := fs.Bool("lite", false, "bare-rowid query path (for benchmarking parity)")
	swapMs := fs.Int("swap-check-ms", 500, "channel repoint poll interval")
	fs.Parse(args)

	srv := &server{store: s, channel: *channel, lite: *lite, poolSize: *pool, embc: embc}

	rel, err := srv.openRelease()
	if err != nil {
		fatalf("serve: channel %q not ready (publish + set-channel first): %v", *channel, err)
	}
	srv.cur.Store(rel)

	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		fatalf("listen %s: %v", *addr, err)
	}
	fmt.Fprintf(os.Stderr, "read-server on %s serving channel %q @ release %s (pool=%d, lite=%v)\n",
		*addr, *channel, rel.version, *pool, *lite)

	go srv.watchChannel(time.Duration(*swapMs)*time.Millisecond, 2*time.Second)
	go func() {
		var prev int64
		for range time.Tick(time.Second) {
			cur := srv.served.Load()
			fmt.Fprintf(os.Stderr, "  %d req/s (release %s)\n", cur-prev, srv.cur.Load().version)
			prev = cur
		}
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			continue
		}
		go srv.handle(conn)
	}
}

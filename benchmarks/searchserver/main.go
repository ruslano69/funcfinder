// searchserver — a real-async Go FTS5 search server + load harness.
//
// Built to answer one question honestly: how fast is an FTS5 search server on
// Go's *native* concurrency (goroutines + channels, blocking parks the
// goroutine, instant wake on send — no poll loop, no Delay(1)) vs a hand-rolled
// dispatcher? It uses modernc.org/sqlite (cgo-free, same SQLite C engine
// transpiled to Go), WAL, and a prepared statement reused per request.
//
// Subcommands:
//
//	seed  --db f.sqlite --docs 50000            build an FTS5 corpus
//	serve --db f.sqlite --addr :9000 --workers 8
//	bench --addr 127.0.0.1:9000 --clients 64 --duration 5s
//	query --db f.sqlite --n 200000              in-process pure-query timing
//
// The worker pool reads from a buffered channel: `job := <-jobs` is the Go-native
// equivalent of WaitSemaphore over a queue — zero CPU when idle, no latency on
// wake. That is the whole point of "real async, not emulated".
package main

import (
	"bufio"
	"database/sql"
	"flag"
	"fmt"
	"math/rand"
	"net"
	"os"
	"runtime"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	_ "modernc.org/sqlite"
)

// Real PureBasic manual vocabulary so generated docs and queries actually
// match content in the funcfinder docsearch knowledge base (docs_fts).
var vocab = []string{
	"While", "Wend", "Repeat", "Until", "Forever", "For", "Next", "ForEach",
	"Break", "Continue", "Select", "Case", "Default", "If", "Else", "EndIf",
	"Structure", "EndStructure", "Procedure", "EndProcedure", "ProcedureReturn",
	"Macro", "EndMacro", "Interface", "EndInterface", "Import", "EndImport",
	"Database", "OpenDatabase", "DatabaseQuery", "DatabaseUpdate", "SQLite",
	"UseSQLiteDatabase", "GetDatabaseString", "GetDatabaseLong", "GetDatabaseBlob",
	"Thread", "CreateThread", "Mutex", "CreateMutex", "LockMutex", "Semaphore",
	"CreateSemaphore", "WaitSemaphore", "SignalSemaphore", "CountCPUs",
	"Network", "CreateNetworkServer", "NetworkServerEvent", "ReceiveNetworkData",
	"SendNetworkData", "OpenNetworkConnection", "HTTPRequest", "HTTPInfo",
	"ReceiveHTTPMemory", "ParseJSON", "GetJSONMember", "GetJSONElement",
	"ExtractJSONArray", "SortList", "SortArray", "CustomSortList", "AddElement",
	"DeleteElement", "FirstElement", "NextElement", "NewList", "NewMap",
	"Pointer", "AllocateMemory", "FreeMemory", "PeekF", "PeekS", "CopyMemory",
	"Array", "Dim", "ReDim", "String", "StringField", "ReplaceString", "Trim",
	"Window", "OpenWindow", "Gadget", "ButtonGadget", "EventGadget",
	"WaitWindowEvent", "Debug", "OpenConsole", "PrintN", "Delay",
	"ElapsedMilliseconds", "RunProgram", "KillProgram", "CountProgramParameters",
	"OpenFile", "ReadFile", "WriteFile", "CreateFile", "CloseFile",
	"Sprite", "OpenScreen", "InitSprite", "InitEngine3D", "Camera3D",
	"Compiler", "CompilerIf", "EnableExplicit", "Protected", "Global", "Shared",
	"Sort", "Regular", "Expression", "PCRE", "MatchRegularExpression",
	"Date", "AddDate", "FormatDate", "Random", "Sin", "Cos", "Sqr",
}

func openDB(path string, maxConns int) (*sql.DB, error) {
	// Pragmas via DSN so every pooled connection gets them (WAL = concurrent
	// readers don't block; busy_timeout = retry instead of SQLITE_BUSY).
	dsn := "file:" + path + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(maxConns)
	db.SetMaxIdleConns(maxConns)
	return db, nil
}

func cmdSeed(args []string) {
	fs := flag.NewFlagSet("seed", flag.ExitOnError)
	dbPath := fs.String("db", "search.sqlite", "sqlite file")
	docs := fs.Int("docs", 50000, "number of documents")
	wordsPer := fs.Int("words", 40, "words per document")
	fs.Parse(args)

	_ = os.Remove(*dbPath)
	_ = os.Remove(*dbPath + "-wal")
	_ = os.Remove(*dbPath + "-shm")

	db, err := openDB(*dbPath, 1)
	must(err)
	defer db.Close()

	_, err = db.Exec(`CREATE VIRTUAL TABLE docs_fts USING fts5(body)`)
	must(err)

	rng := rand.New(rand.NewSource(42))
	tx, err := db.Begin()
	must(err)
	stmt, err := tx.Prepare(`INSERT INTO docs_fts(body) VALUES (?)`)
	must(err)

	start := time.Now()
	buf := make([]byte, 0, *wordsPer*8)
	for i := 0; i < *docs; i++ {
		buf = buf[:0]
		for w := 0; w < *wordsPer; w++ {
			if w > 0 {
				buf = append(buf, ' ')
			}
			buf = append(buf, vocab[rng.Intn(len(vocab))]...)
		}
		_, err = stmt.Exec(string(buf))
		must(err)
		if i%20000 == 0 && i > 0 {
			must(tx.Commit())
			tx, err = db.Begin()
			must(err)
			stmt, err = tx.Prepare(`INSERT INTO docs_fts(body) VALUES (?)`)
			must(err)
		}
	}
	must(tx.Commit())
	fmt.Printf("seeded %d docs into %s in %v\n", *docs, *dbPath, time.Since(start).Round(time.Millisecond))
}

// runQuery executes one FTS5 MATCH and drains the rowids (realistic: we touch
// every result column the way a real handler would).
func runQuery(stmt *sql.Stmt, term string) (int, error) {
	rows, err := stmt.Query(term)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	n := 0
	var id int64
	for rows.Next() {
		if err := rows.Scan(&id); err != nil {
			return n, err
		}
		n++
	}
	return n, rows.Err()
}

type job struct {
	term  string
	reply chan int
}

func cmdServe(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	dbPath := fs.String("db", "search.sqlite", "sqlite file")
	addr := fs.String("addr", "127.0.0.1:9000", "listen address")
	workers := fs.Int("workers", runtime.NumCPU(), "worker goroutines (== db conns)")
	queue := fs.Int("queue", 1024, "job queue depth")
	fs.Parse(args)

	db, err := openDB(*dbPath, *workers)
	must(err)
	defer db.Close()

	// One prepared statement, reused for the whole server lifetime. database/sql
	// re-binds it to whichever pooled connection a worker gets.
	stmt, err := db.Prepare(`SELECT rowid FROM docs_fts WHERE docs_fts MATCH ? LIMIT 20`)
	must(err)
	defer stmt.Close()

	jobs := make(chan job, *queue)
	var served atomic.Int64

	// Worker pool. `job := <-jobs` blocks with zero CPU and wakes instantly —
	// the native version of Signal/WaitSemaphore, no Delay(1) anywhere.
	var wg sync.WaitGroup
	for i := 0; i < *workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				n, _ := runQuery(stmt, j.term)
				served.Add(1)
				j.reply <- n
			}
		}()
	}

	ln, err := net.Listen("tcp", *addr)
	must(err)
	fmt.Printf("serving %s on %s with %d workers (queue %d)\n", *dbPath, *addr, *workers, *queue)

	// Light throughput ticker so we can watch it live.
	go func() {
		prev := int64(0)
		for range time.Tick(time.Second) {
			cur := served.Load()
			fmt.Printf("  %d req/s (total %d)\n", cur-prev, cur)
			prev = cur
		}
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			continue
		}
		go handleConn(conn, jobs)
	}
}

// handleConn: newline-delimited requests. One request in flight per connection
// (the client drives concurrency by opening many connections). Each request is
// dispatched to the worker pool via the channel and the reply written back.
func handleConn(conn net.Conn, jobs chan<- job) {
	defer conn.Close()
	if tc, ok := conn.(*net.TCPConn); ok {
		_ = tc.SetNoDelay(true) // request/response: disable Nagle
	}
	r := bufio.NewReader(conn)
	w := bufio.NewWriter(conn)
	reply := make(chan int, 1) // reused per connection
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		term := line[:len(line)-1]
		jobs <- job{term: term, reply: reply}
		n := <-reply
		fmt.Fprintf(w, "%d\n", n)
		if w.Flush() != nil {
			return
		}
	}
}

// persistentConn is a pre-dialed, kept-open client connection reused across all
// ramp phases. We dial the pool once and never slam the port between steps.
type persistentConn struct {
	conn net.Conn
	r    *bufio.Reader
	w    *bufio.Writer
	rng  *rand.Rand
}

func cmdBench(args []string) {
	fs := flag.NewFlagSet("bench", flag.ExitOnError)
	addr := fs.String("addr", "127.0.0.1:9000", "server address")
	clients := fs.Int("clients", 64, "concurrent connections (pool size / single-phase load)")
	ramp := fs.String("ramp", "", "comma list of active-client counts to sweep, e.g. 1,8,32,128 (reuses the pool)")
	dur := fs.Duration("duration", 3*time.Second, "per-phase duration")
	fs.Parse(args)

	// Determine the phases and the pool size (max of them).
	phases := []int{*clients}
	if *ramp != "" {
		phases = phases[:0]
		for _, p := range splitInts(*ramp) {
			phases = append(phases, p)
		}
	}
	pool := 0
	for _, p := range phases {
		if p > pool {
			pool = p
		}
	}

	// Dial the whole pool ONCE and keep it open for the entire session.
	conns := make([]*persistentConn, 0, pool)
	for i := 0; i < pool; i++ {
		conn, err := net.Dial("tcp", *addr)
		if err != nil {
			must(fmt.Errorf("dial %d: %w", i, err))
		}
		if tc, ok := conn.(*net.TCPConn); ok {
			_ = tc.SetNoDelay(true)
		}
		conns = append(conns, &persistentConn{
			conn: conn,
			r:    bufio.NewReader(conn),
			w:    bufio.NewWriter(conn),
			rng:  rand.New(rand.NewSource(int64(i) + 1)),
		})
	}
	defer func() {
		for _, c := range conns {
			c.conn.Close()
		}
	}()
	fmt.Printf("pool of %d persistent connections; sweeping %v\n", pool, phases)

	for _, active := range phases {
		runPhase(conns[:active], *dur)
	}
}

// runPhase drives `active` already-open connections for `dur`, reusing them.
func runPhase(conns []*persistentConn, dur time.Duration) {
	var total atomic.Int64
	var wg sync.WaitGroup
	stop := make(chan struct{})
	latCh := make(chan []time.Duration, len(conns))

	for _, pc := range conns {
		wg.Add(1)
		go func(pc *persistentConn) {
			defer wg.Done()
			lats := make([]time.Duration, 0, 200000)
			for {
				select {
				case <-stop:
					latCh <- lats
					return
				default:
				}
				term := vocab[pc.rng.Intn(len(vocab))]
				t0 := time.Now()
				if _, err := pc.w.WriteString(term + "\n"); err != nil {
					latCh <- lats
					return
				}
				if pc.w.Flush() != nil {
					latCh <- lats
					return
				}
				if _, err := pc.r.ReadString('\n'); err != nil {
					latCh <- lats
					return
				}
				lats = append(lats, time.Since(t0))
				total.Add(1)
			}
		}(pc)
	}

	time.Sleep(dur)
	close(stop)
	wg.Wait()
	close(latCh)

	var all []time.Duration
	for l := range latCh {
		all = append(all, l...)
	}
	sort.Slice(all, func(i, j int) bool { return all[i] < all[j] })

	n := total.Load()
	rps := float64(n) / dur.Seconds()
	line := fmt.Sprintf("  clients=%-4d  %8.0f req/s", len(conns), rps)
	if len(all) > 0 {
		line += fmt.Sprintf("  p50=%-9v p99=%-9v max=%v",
			all[len(all)*50/100].Round(time.Microsecond),
			all[len(all)*99/100].Round(time.Microsecond),
			all[len(all)-1].Round(time.Microsecond))
	}
	fmt.Println(line)
}

func splitInts(s string) []int {
	var out []int
	cur := 0
	has := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= '0' && c <= '9' {
			cur = cur*10 + int(c-'0')
			has = true
		} else if has {
			out = append(out, cur)
			cur, has = 0, false
		}
	}
	if has {
		out = append(out, cur)
	}
	return out
}

// cmdQuery isolates the pure in-process query cost — the Go analogue of the
// "43 ns" number, with no TCP, no dispatch, no serialization.
func cmdQuery(args []string) {
	fs := flag.NewFlagSet("query", flag.ExitOnError)
	dbPath := fs.String("db", "search.sqlite", "sqlite file")
	n := fs.Int("n", 200000, "iterations")
	fs.Parse(args)

	db, err := openDB(*dbPath, 1)
	must(err)
	defer db.Close()
	stmt, err := db.Prepare(`SELECT rowid FROM docs_fts WHERE docs_fts MATCH ? LIMIT 20`)
	must(err)
	defer stmt.Close()

	rng := rand.New(rand.NewSource(7))
	// warmup
	for i := 0; i < 1000; i++ {
		_, _ = runQuery(stmt, vocab[rng.Intn(len(vocab))])
	}
	start := time.Now()
	hits := 0
	for i := 0; i < *n; i++ {
		h, err := runQuery(stmt, vocab[rng.Intn(len(vocab))])
		must(err)
		hits += h
	}
	el := time.Since(start)
	fmt.Printf("pure query: %d iters in %v => %.0f ns/query, %.0f q/s (avg hits %.1f)\n",
		*n, el.Round(time.Millisecond), float64(el.Nanoseconds())/float64(*n),
		float64(*n)/el.Seconds(), float64(hits)/float64(*n))
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("usage: searchserver <seed|serve|bench|query> [flags]")
		os.Exit(2)
	}
	switch os.Args[1] {
	case "seed":
		cmdSeed(os.Args[2:])
	case "serve":
		cmdServe(os.Args[2:])
	case "bench":
		cmdBench(os.Args[2:])
	case "query":
		cmdQuery(os.Args[2:])
	default:
		fmt.Println("unknown command:", os.Args[1])
		os.Exit(2)
	}
}

package knowledge

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"golang.org/x/net/html"
)

// --- pure helpers (no network) ---

func TestNormalizeURL(t *testing.T) {
	cases := []struct{ in, want string }{
		// @version segments are stripped so versioned/unversioned collapse.
		{"https://pkg.go.dev/net/http@go1.21.0", "https://pkg.go.dev/net/http"},
		{"https://example.com/mod@v1.2.3/sub", "https://example.com/mod/sub"},
		// fragments are dropped.
		{"https://example.com/a/b#section", "https://example.com/a/b"},
		{"https://example.com/a/b#section@go1.21", "https://example.com/a/b"},
		// plain URLs pass through unchanged.
		{"https://example.com/docs/page", "https://example.com/docs/page"},
	}
	for _, c := range cases {
		if got := normalizeURL(c.in); got != c.want {
			t.Errorf("normalizeURL(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestIsSameSite(t *testing.T) {
	base := mustParseURL(t, "https://pkg.go.dev/net/http")
	cases := []struct {
		link string
		want bool
	}{
		{"https://pkg.go.dev/net/http/pprof", true},  // under the path prefix
		{"https://pkg.go.dev/net/http", true},        // the root itself
		{"https://pkg.go.dev/os", false},             // same host, different path
		{"https://example.com/net/http", false},      // different host
		{"https://pkg.go.dev/net/http?tab=doc", false}, // query variant, base has none
	}
	for _, c := range cases {
		if got := isSameSite(base, c.link); got != c.want {
			t.Errorf("isSameSite(%q) = %v, want %v", c.link, got, c.want)
		}
	}

	// When the root URL itself carries a query, query variants are allowed.
	baseQ := mustParseURL(t, "https://example.com/search?q=x")
	if !isSameSite(baseQ, "https://example.com/search?q=y") {
		t.Error("query variant should be allowed when base has a query")
	}
}

func TestNodeText_CollapsesWhitespace(t *testing.T) {
	doc := mustParseHTML(t, "<p>hello\n\t  world   again</p>")
	if got := nodeText(doc); got != "hello world again" {
		t.Errorf("nodeText = %q, want %q", got, "hello world again")
	}
}

func TestFindMainContent(t *testing.T) {
	if findMainContent(mustParseHTML(t, "<body><main><p>x</p></main></body>")) == nil {
		t.Error("want <main> found")
	}
	if findMainContent(mustParseHTML(t, "<body><article><p>x</p></article></body>")) == nil {
		t.Error("want <article> found")
	}
	if findMainContent(mustParseHTML(t, `<body><div role="main"><p>x</p></div></body>`)) == nil {
		t.Error("want role=main found")
	}
	if findMainContent(mustParseHTML(t, "<body><div><p>x</p></div></body>")) != nil {
		t.Error("want nil when no main/article/role=main present")
	}
}

func TestExtractSections_SkipsChromeAndBuildsSections(t *testing.T) {
	page := `<html><head><title>Doc</title></head><body>
<nav><a href="/x">skip me</a>navigation junk</nav>
<main>
  <h1>Install</h1>
  <p>Run the installer.</p>
  <h2>Verify</h2>
  <p>Check the version.</p>
  <footer>copyright noise</footer>
</main></body></html>`
	title, sections := extractSections(mustParseHTML(t, page), "src")
	if title != "Doc" {
		t.Errorf("title = %q, want Doc", title)
	}
	if len(sections) != 2 {
		t.Fatalf("want 2 sections (Install, Verify), got %d: %+v", len(sections), sections)
	}
	if sections[0].title != "Install" || sections[1].title != "Verify" {
		t.Errorf("section titles = %q/%q, want Install/Verify", sections[0].title, sections[1].title)
	}
	joined := strings.Join(append(sections[0].paras, sections[1].paras...), " ")
	for _, noise := range []string{"navigation junk", "copyright noise", "skip me"} {
		if strings.Contains(joined, noise) {
			t.Errorf("chrome text %q leaked into sections: %q", noise, joined)
		}
	}
}

func TestExtractLinks_ResolvesRelativeAndStripsFragment(t *testing.T) {
	base := mustParseURL(t, "https://example.com/docs/")
	doc := mustParseHTML(t, `<a href="page2.html">rel</a><a href="/abs">abs</a>`+
		`<a href="https://other.com/x#frag">ext</a><a href="mailto:a@b.c">mail</a>`)
	links := extractLinks(doc, base)
	want := map[string]bool{
		"https://example.com/docs/page2.html": true,
		"https://example.com/abs":             true,
		"https://other.com/x":                 true, // fragment stripped
	}
	if len(links) != len(want) {
		t.Fatalf("want %d links, got %d: %v", len(want), len(links), links)
	}
	for _, l := range links {
		if !want[l] {
			t.Errorf("unexpected link %q (mailto: should be dropped, fragments stripped)", l)
		}
	}
}

// --- integration via httptest ---

func TestIngestURL_CrawlsLinkedPages(t *testing.T) {
	ts := httptest.NewServer(htmlSite(map[string]string{
		"/": `<title>Home</title><main><h1>Home</h1><p>The authenticate call validates a token.</p>
			<a href="/page2">next</a></main>`,
		"/page2": `<title>Deploy</title><article><h1>Deploy</h1><p>Run kubectl rollout restart now.</p></article>`,
	}))
	defer ts.Close()

	chunks, err := IngestURL(ts.URL+"/", CrawlOpts{}, nil)
	if err != nil {
		t.Fatalf("IngestURL: %v", err)
	}
	all := chunkText(chunks)
	if !strings.Contains(all, "authenticate call validates") {
		t.Errorf("root page content missing: %q", all)
	}
	if !strings.Contains(all, "kubectl rollout restart") {
		t.Errorf("linked page content missing (link not followed?): %q", all)
	}
}

func TestIngestURL_StaysSameSite(t *testing.T) {
	ts := httptest.NewServer(htmlSite(map[string]string{
		// Link out to a different host; the crawler must not follow it.
		"/": `<title>Home</title><main><p>only this page indexed here.</p>
			<a href="https://example.com/secret">external</a></main>`,
	}))
	defer ts.Close()

	chunks, err := IngestURL(ts.URL+"/", CrawlOpts{}, nil)
	if err != nil {
		t.Fatalf("IngestURL: %v", err)
	}
	if len(chunks) != 1 {
		t.Fatalf("want exactly 1 chunk (external link not followed), got %d: %q", len(chunks), chunkText(chunks))
	}
}

func TestIngestURL_StaysWithinPathPrefix(t *testing.T) {
	ts := httptest.NewServer(htmlSite(map[string]string{
		"/docs/":  `<title>Docs</title><main><p>docs root content here.</p><a href="/docs/sub">in</a><a href="/blog/post">out</a></main>`,
		"/docs/sub": `<title>Sub</title><main><p>nested docs content here.</p></main>`,
		"/blog/post": `<title>Blog</title><main><p>blog content should be skipped.</p></main>`,
	}))
	defer ts.Close()

	chunks, err := IngestURL(ts.URL+"/docs/", CrawlOpts{}, nil)
	if err != nil {
		t.Fatalf("IngestURL: %v", err)
	}
	all := chunkText(chunks)
	if !strings.Contains(all, "nested docs content") {
		t.Errorf("in-prefix page not crawled: %q", all)
	}
	if strings.Contains(all, "blog content") {
		t.Errorf("out-of-prefix /blog page was crawled: %q", all)
	}
}

func TestIngestURL_DedupsIdenticalContent(t *testing.T) {
	dup := `<title>Dup</title><main><p>this exact body appears at two urls.</p></main>`
	ts := httptest.NewServer(htmlSite(map[string]string{
		"/":  `<title>Home</title><main><p>unique root body.</p><a href="/x">x</a><a href="/y">y</a></main>`,
		"/x": dup,
		"/y": dup, // byte-identical to /x → deduped by content hash
	}))
	defer ts.Close()

	chunks, err := IngestURL(ts.URL+"/", CrawlOpts{}, nil)
	if err != nil {
		t.Fatalf("IngestURL: %v", err)
	}
	all := chunkText(chunks)
	if n := strings.Count(all, "this exact body appears"); n != 1 {
		t.Errorf("identical content indexed %d times, want 1 (dedup by hash)", n)
	}
}

func TestIngestURL_RespectsMaxPages(t *testing.T) {
	ts := httptest.NewServer(htmlSite(map[string]string{
		"/":   `<title>0</title><main><p>page zero body.</p><a href="/p1">1</a></main>`,
		"/p1": `<title>1</title><main><p>page one body.</p><a href="/p2">2</a></main>`,
		"/p2": `<title>2</title><main><p>page two body should not be reached.</p></main>`,
	}))
	defer ts.Close()

	chunks, err := IngestURL(ts.URL+"/", CrawlOpts{MaxPages: 2}, nil)
	if err != nil {
		t.Fatalf("IngestURL: %v", err)
	}
	all := chunkText(chunks)
	if strings.Contains(all, "page two body") {
		t.Errorf("MaxPages=2 exceeded — /p2 was crawled: %q", all)
	}
}

func TestIngestURL_SkipsNonHTML(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<title>Home</title><main><p>the only indexable page.</p><a href="/data.json">data</a></main>`))
	})
	mux.HandleFunc("/data.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"note":"should not be indexed"}`))
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	chunks, err := IngestURL(ts.URL+"/", CrawlOpts{}, nil)
	if err != nil {
		t.Fatalf("IngestURL: %v", err)
	}
	if strings.Contains(chunkText(chunks), "should not be indexed") {
		t.Error("non-HTML content-type was indexed")
	}
}

func TestIngestURL_NoContentError(t *testing.T) {
	ts := httptest.NewServer(htmlSite(map[string]string{
		"/": `<title>Empty</title><body></body>`, // no main/article, no paragraphs
	}))
	defer ts.Close()

	if _, err := IngestURL(ts.URL+"/", CrawlOpts{}, nil); err == nil {
		t.Fatal("want an error when a page yields no extractable content")
	}
}

func TestIngestURL_ProgressCallbackInvoked(t *testing.T) {
	ts := httptest.NewServer(htmlSite(map[string]string{
		"/":   `<title>0</title><main><p>root body content.</p><a href="/p1">1</a></main>`,
		"/p1": `<title>1</title><main><p>second page content.</p></main>`,
	}))
	defer ts.Close()

	var calls int
	_, err := IngestURL(ts.URL+"/", CrawlOpts{}, func(fetched, queued int, pageURL string) {
		calls++
	})
	if err != nil {
		t.Fatalf("IngestURL: %v", err)
	}
	if calls != 2 {
		t.Errorf("progress callback invoked %d times, want 2 (one per page)", calls)
	}
}

// --- test helpers ---

func mustParseURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse %q: %v", raw, err)
	}
	return u
}

func mustParseHTML(t *testing.T, s string) *html.Node {
	t.Helper()
	doc, err := html.Parse(strings.NewReader(s))
	if err != nil {
		t.Fatalf("parse html: %v", err)
	}
	return doc
}

// htmlSite returns a handler that serves each path's body as a full HTML
// document with Content-Type text/html.
func htmlSite(pages map[string]string) http.Handler {
	mux := http.NewServeMux()
	for path, body := range pages {
		p, b := path, body
		mux.HandleFunc(p, func(w http.ResponseWriter, r *http.Request) {
			// Only serve the exact registered path (ServeMux "/" would else
			// catch every unregistered path and skew same-site tests).
			if r.URL.Path != p {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write([]byte("<html>" + b + "</html>"))
		})
	}
	return mux
}

func chunkText(chunks []Chunk) string {
	var b strings.Builder
	for _, c := range chunks {
		b.WriteString(c.Content)
		b.WriteString("\n")
	}
	return b.String()
}

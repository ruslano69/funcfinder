package knowledge

import (
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode"

	"golang.org/x/net/html"
)

// CrawlOpts controls the web crawler behaviour.
type CrawlOpts struct {
	MaxPages   int           // hard cap on pages fetched; default 200
	Timeout    time.Duration // per-request timeout; default 10s
	UserAgent  string        // default "docsearch-crawler/1.0"
	ChunkOpts  ChunkOpts
}

func (o *CrawlOpts) withDefaults() CrawlOpts {
	out := *o
	if out.MaxPages == 0 {
		out.MaxPages = 200
	}
	if out.Timeout == 0 {
		out.Timeout = 10 * time.Second
	}
	if out.UserAgent == "" {
		out.UserAgent = "docsearch-crawler/1.0"
	}
	out.ChunkOpts = out.ChunkOpts.withDefaults()
	return out
}

// CrawlProgress is called after each page is fetched.
// fetched = pages done so far, total = estimated (may grow).
type CrawlProgress func(fetched, queued int, pageURL string)

// IngestURL crawls the given URL (staying within the same host+path prefix)
// and returns all indexable chunks. progress may be nil.
func IngestURL(rootURL string, opts CrawlOpts, progress CrawlProgress) ([]Chunk, error) {
	opts = opts.withDefaults()

	base, err := url.Parse(rootURL)
	if err != nil {
		return nil, fmt.Errorf("invalid url %q: %w", rootURL, err)
	}
	// Normalise: strip fragment, ensure path ends consistently.
	base.Fragment = ""

	client := &http.Client{Timeout: opts.Timeout}

	visited := map[string]bool{}
	seenContent := map[[32]byte]bool{} // dedup pages with identical content
	queue := []string{base.String()}
	var allChunks []Chunk

	for len(queue) > 0 && len(visited) < opts.MaxPages {
		pageURL := queue[0]
		queue = queue[1:]

		if visited[pageURL] {
			continue
		}
		visited[pageURL] = true

		if progress != nil {
			progress(len(visited), len(queue), pageURL)
		}

		chunks, links, rawBody, err := fetchPage(client, pageURL, opts.UserAgent, opts.ChunkOpts)
		if err != nil {
			// Non-fatal: skip this page and continue crawling.
			continue
		}

		// Skip pages whose body is identical to one we already indexed.
		h := sha256.Sum256(rawBody)
		if seenContent[h] {
			continue
		}
		seenContent[h] = true

		allChunks = append(allChunks, chunks...)

		for _, link := range links {
			norm := normalizeURL(link)
			if !visited[norm] && isSameSite(base, norm) {
				queue = append(queue, norm)
			}
		}
	}

	if len(allChunks) == 0 {
		return nil, fmt.Errorf("no content extracted from %s", rootURL)
	}
	return allChunks, nil
}

// isSameSite returns true when link shares the same host and has rootURL's
// path as a prefix — so crawling pkg.go.dev/net/http stays within that package.
// Links with query parameters are allowed only if the root URL itself has
// query parameters (opt-in); otherwise we strip query strings to avoid
// crawling tab variants, pagination, etc.
func isSameSite(base *url.URL, link string) bool {
	u, err := url.Parse(link)
	if err != nil {
		return false
	}
	if u.Host != base.Host {
		return false
	}
	// Reject query-param variants unless root URL also has query params.
	if u.RawQuery != "" && base.RawQuery == "" {
		return false
	}
	rootPath := strings.TrimRight(base.Path, "/")
	return rootPath == "" || strings.HasPrefix(u.Path, rootPath)
}

// normalizeURL strips version suffixes (@go1.x.x, @v1.x.x) from URL paths
// so that versioned and unversioned variants are treated as the same page.
func normalizeURL(link string) string {
	u, err := url.Parse(link)
	if err != nil {
		return link
	}
	// Strip @version from each path segment.
	segments := strings.Split(u.Path, "/")
	for i, seg := range segments {
		if idx := strings.Index(seg, "@"); idx >= 0 {
			segments[i] = seg[:idx]
		}
	}
	u.Path = strings.Join(segments, "/")
	u.Fragment = ""
	return u.String()
}

// fetchPage downloads one page and returns its chunks, discovered links, and raw body bytes.
func fetchPage(client *http.Client, pageURL, ua string, opts ChunkOpts) ([]Chunk, []string, []byte, error) {
	req, err := http.NewRequest("GET", pageURL, nil)
	if err != nil {
		return nil, nil, nil, err
	}
	req.Header.Set("User-Agent", ua)
	req.Header.Set("Accept", "text/html")

	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, nil, nil, fmt.Errorf("HTTP %d for %s", resp.StatusCode, pageURL)
	}
	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "text/html") {
		return nil, nil, nil, fmt.Errorf("skipping non-HTML content-type %q", ct)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20)) // 4 MB cap
	if err != nil {
		return nil, nil, nil, err
	}

	doc, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		return nil, nil, nil, err
	}

	base, _ := url.Parse(pageURL)
	links := extractLinks(doc, base)
	title, sections := extractSections(doc, pageURL)
	_ = title

	chunks := sectionsToChunks(sections, pageURL, opts)
	return chunks, links, body, nil
}

// extractLinks returns all absolute in-document href values.
func extractLinks(doc *html.Node, base *url.URL) []string {
	var links []string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			for _, a := range n.Attr {
				if a.Key == "href" {
					u, err := base.Parse(a.Val)
					if err != nil {
						break
					}
					u.Fragment = ""
					if u.Scheme == "http" || u.Scheme == "https" {
						links = append(links, u.String())
					}
					break
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return links
}

// skipTags are HTML elements whose subtree we skip entirely.
var skipTags = map[string]bool{
	"script": true, "style": true, "noscript": true,
	"nav": true, "header": true, "footer": true, "aside": true,
}

// extractSections walks the HTML tree and returns a page title and a list of
// docSections built from heading/paragraph structure.
func extractSections(doc *html.Node, source string) (string, []docSection) {
	// Find <title> for fallback section title.
	pageTitle := extractTitle(doc)

	var sections []docSection
	cur := docSection{title: pageTitle}

	flushSection := func() {
		if len(cur.paras) > 0 {
			sections = append(sections, cur)
		}
		cur = docSection{title: pageTitle}
		cur.paras = nil
	}

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			tag := n.Data
			if skipTags[tag] {
				return
			}
			switch tag {
			case "h1", "h2", "h3", "h4", "h5", "h6":
				text := strings.TrimSpace(nodeText(n))
				if text != "" {
					flushSection()
					cur.title = text
				}
				return // don't recurse into heading — we already grabbed text
			case "p", "pre", "code", "li", "dd", "blockquote", "td", "th":
				text := strings.TrimSpace(nodeText(n))
				if text != "" {
					cur.paras = append(cur.paras, text)
				}
				return // text collected, don't double-recurse
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}

	// Find <main>, <article>, or <div role="main"> to focus extraction.
	main := findMainContent(doc)
	if main == nil {
		main = doc
	}
	walk(main)
	flushSection()

	return pageTitle, sections
}

// findMainContent returns the first <main>, <article>, or role="main" element.
func findMainContent(doc *html.Node) *html.Node {
	var found *html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if found != nil {
			return
		}
		if n.Type == html.ElementNode {
			tag := n.Data
			if tag == "main" || tag == "article" {
				found = n
				return
			}
			for _, a := range n.Attr {
				if a.Key == "role" && a.Val == "main" {
					found = n
					return
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return found
}

// extractTitle returns the text of the first <title> element.
func extractTitle(doc *html.Node) string {
	var found string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if found != "" {
			return
		}
		if n.Type == html.ElementNode && n.Data == "title" {
			found = strings.TrimSpace(nodeText(n))
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return found
}

// nodeText returns the concatenated text content of a node and its descendants,
// collapsing whitespace runs to single spaces.
func nodeText(n *html.Node) string {
	var sb strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			sb.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	// Collapse whitespace.
	fields := strings.FieldsFunc(sb.String(), unicode.IsSpace)
	return strings.Join(fields, " ")
}

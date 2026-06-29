package knowledge

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/ledongthuc/pdf"
)

func ingestPDF(path string, opts ChunkOpts) ([]Chunk, error) {
	f, r, err := pdf.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open pdf %s: %w", path, err)
	}
	defer f.Close()

	var sections []docSection
	total := r.NumPage()
	// Shared font cache used only by the GetPlainText fallback path.
	fonts := map[string]*pdf.Font{}

	for i := 1; i <= total; i++ {
		page := r.Page(i)
		if page.V.IsNull() {
			continue
		}
		text := extractPageText(page)
		// Fall back to GetPlainText when position-based extraction looks glued
		// (average word length > 15 chars typically means W=0 in the PDF).
		if looksGlued(text) {
			if plain, err := page.GetPlainText(fonts); err == nil {
				text = plain
			}
		}
		if strings.TrimSpace(text) == "" {
			continue
		}
		paras := splitParagraphs(normalizeWhitespace(text))
		if len(paras) == 0 {
			continue
		}
		sections = append(sections, docSection{
			title: fmt.Sprintf("Page %d", i),
			paras: paras,
			page:  i,
		})
	}

	if len(sections) == 0 {
		return nil, fmt.Errorf("no extractable text found in %s", path)
	}
	return sectionsToChunks(sections, path, opts), nil
}

// extractPageText rebuilds page text from individual positioned text elements.
//
// ledongthuc/pdf's GetPlainText concatenates elements without spaces, producing
// "LeftStr()" as "LeftStr()". This function instead uses page.Content() to get
// each element's X/Y position, groups elements into lines by Y coordinate, and
// inserts a space wherever the gap between adjacent elements exceeds a threshold.
func extractPageText(page pdf.Page) string {
	content := page.Content()
	if len(content.Text) == 0 {
		return ""
	}

	type elem struct {
		x, y, w float64
		s       string
	}

	elems := make([]elem, 0, len(content.Text))
	for _, t := range content.Text {
		s := strings.TrimSpace(t.S)
		if s == "" {
			continue
		}
		elems = append(elems, elem{t.X, t.Y, t.W, s})
	}
	if len(elems) == 0 {
		return ""
	}

	// Sort top-to-bottom (higher Y first in PDF coordinates), then left-to-right.
	sort.Slice(elems, func(i, j int) bool {
		dy := elems[i].y - elems[j].y
		if math.Abs(dy) > 2 {
			return dy > 0
		}
		return elems[i].x < elems[j].x
	})

	// Group into lines: elements within yTol points share a line.
	const yTol = 2.0
	var lines [][]elem
	var cur []elem
	for _, e := range elems {
		if len(cur) == 0 || math.Abs(e.y-cur[0].y) <= yTol {
			cur = append(cur, e)
		} else {
			lines = append(lines, cur)
			cur = []elem{e}
		}
	}
	if len(cur) > 0 {
		lines = append(lines, cur)
	}

	var sb strings.Builder
	for _, line := range lines {
		sort.Slice(line, func(i, j int) bool { return line[i].x < line[j].x })
		for i, e := range line {
			if i > 0 {
				// Insert a space when the gap between end of previous element
				// and start of current exceeds 1.5 pts.
				gap := e.x - (line[i-1].x + line[i-1].w)
				if gap > 1.5 {
					sb.WriteByte(' ')
				}
			}
			sb.WriteString(e.s)
		}
		sb.WriteByte('\n')
	}
	return sb.String()
}

// looksGlued returns true when the text looks like words were concatenated
// without spaces — a sign that all text elements had W=0 in the PDF and the
// gap-based space insertion produced nothing useful.
// Heuristic: fewer than 3 words OR average word length > 15 characters.
func looksGlued(s string) bool {
	words := strings.Fields(s)
	if len(words) < 3 {
		return strings.TrimSpace(s) != ""
	}
	total := 0
	for _, w := range words {
		total += len(w)
	}
	return total/len(words) > 15
}

// normalizeWhitespace collapses runs of spaces and trims lines.
func normalizeWhitespace(s string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = strings.Join(strings.Fields(l), " ")
	}
	return strings.Join(lines, "\n")
}

package knowledge

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"unicode"

	"github.com/ledongthuc/pdf"
)

// OCRQualityError is returned when the PDF appears to contain bad OCR output
// that would pollute the knowledge base with unreadable text.
// The caller can decide whether to skip the file or log a warning.
type OCRQualityError struct {
	Path  string
	Score float64 // 0.0 = garbage, 1.0 = perfect
}

func (e *OCRQualityError) Error() string {
	return fmt.Sprintf("bad OCR quality in %s (score %.2f): text is likely unreadable", e.Path, e.Score)
}

// minOCRQuality is the threshold below which a PDF is rejected as bad OCR.
// Score is averaged over sampled pages; tune if needed.
const minOCRQuality = 0.45

func ingestPDF(path string, opts ChunkOpts) ([]Chunk, error) {
	f, r, err := pdf.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open pdf %s: %w", path, err)
	}
	defer f.Close()

	total := r.NumPage()
	// Shared font cache used only by the GetPlainText fallback path.
	fonts := map[string]*pdf.Font{}

	// Sample up to 10 evenly-spaced pages to estimate OCR quality before
	// committing to a full parse. Skip the first and last few pages
	// (often covers/indices with little prose).
	if score := sampleOCRQuality(r, total, fonts); score < minOCRQuality {
		return nil, &OCRQualityError{Path: path, Score: score}
	}

	var sections []docSection

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

// sampleOCRQuality reads up to 10 evenly-spaced pages (skipping first 5% and
// last 5% which are often covers/indices) and returns the average pageTextQuality.
func sampleOCRQuality(r *pdf.Reader, total int, fonts map[string]*pdf.Font) float64 {
	if total == 0 {
		return 1.0
	}

	skip := total / 20 // skip ~5% at each end
	if skip < 1 {
		skip = 1
	}
	start, end := skip+1, total-skip
	if start > end {
		start, end = 1, total
	}

	const maxSamples = 10
	span := end - start + 1
	step := span / maxSamples
	if step < 1 {
		step = 1
	}

	var sum float64
	var n int
	for i := start; i <= end; i += step {
		page := r.Page(i)
		if page.V.IsNull() {
			continue
		}
		text := extractPageText(page)
		if looksGlued(text) {
			if plain, err := page.GetPlainText(fonts); err == nil {
				text = plain
			}
		}
		if strings.TrimSpace(text) == "" {
			continue
		}
		sum += pageTextQuality(text)
		n++
		if n >= maxSamples {
			break
		}
	}
	if n == 0 {
		return 1.0 // all pages empty — let the main loop handle it
	}
	return sum / float64(n)
}

// pageTextQuality returns a quality score in [0.0, 1.0] for a page's text.
// It combines three signals:
//
//  1. letterRatio  — fraction of non-space runes that are letters.
//     Bad OCR often has many garbage symbols (□■ÃÂ©) pulling this down.
//
//  2. wordRatio    — fraction of whitespace-separated tokens that look like
//     real words (≥2 letters, ≤30 chars). Broken OCR splits text into
//     single-char tokens or produces unrecognisable symbol runs.
//
//  3. singleRatio  — fraction of tokens that are exactly 1 character.
//     Spaced-out OCR ("T e x t") produces almost all length-1 tokens.
//     A high value is penalised.
func pageTextQuality(text string) float64 {
	if strings.TrimSpace(text) == "" {
		return 0.0
	}

	// --- letter ratio ---
	var letters, nonSpaceRunes int
	for _, r := range text {
		if unicode.IsSpace(r) {
			continue
		}
		nonSpaceRunes++
		if unicode.IsLetter(r) {
			letters++
		}
	}
	var letterRatio float64
	if nonSpaceRunes > 0 {
		letterRatio = float64(letters) / float64(nonSpaceRunes)
	}

	// --- word ratio and single-char ratio ---
	tokens := strings.Fields(text)
	if len(tokens) == 0 {
		return 0.0
	}
	var wordLike, singleChar int
	for _, tok := range tokens {
		runes := []rune(tok)
		if len(runes) == 1 {
			singleChar++
		}
		// "word-like": between 2 and 30 chars, at least half are letters
		if len(runes) >= 2 && len(runes) <= 30 {
			var lc int
			for _, r := range runes {
				if unicode.IsLetter(r) {
					lc++
				}
			}
			if float64(lc)/float64(len(runes)) >= 0.5 {
				wordLike++
			}
		}
	}
	wordRatio := float64(wordLike) / float64(len(tokens))
	singleRatio := float64(singleChar) / float64(len(tokens))

	// Penalise heavily when most tokens are single chars (spaced-out OCR).
	singlePenalty := 0.0
	if singleRatio > 0.4 {
		singlePenalty = (singleRatio - 0.4) * 2.0 // up to ~1.2 extra penalty
	}

	score := 0.4*letterRatio + 0.6*wordRatio - singlePenalty
	if score < 0 {
		score = 0
	}
	if score > 1 {
		score = 1
	}
	return score
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

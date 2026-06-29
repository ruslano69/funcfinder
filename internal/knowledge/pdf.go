package knowledge

import (
	"fmt"
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
	// Shared font cache across pages for performance.
	fonts := map[string]*pdf.Font{}

	for i := 1; i <= total; i++ {
		page := r.Page(i)
		if page.V.IsNull() {
			continue
		}
		text, err := page.GetPlainText(fonts)
		if err != nil || strings.TrimSpace(text) == "" {
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

// normalizeWhitespace collapses runs of spaces and trims lines so that PDF
// extraction artefacts (extra spaces, soft hyphens) don't pollute paragraphs.
func normalizeWhitespace(s string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		// Collapse internal spaces
		fields := strings.Fields(l)
		lines[i] = strings.Join(fields, " ")
	}
	return strings.Join(lines, "\n")
}

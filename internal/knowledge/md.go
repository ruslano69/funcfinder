package knowledge

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var mdHeaderRe = regexp.MustCompile(`^(#{1,6})\s+(.+)$`)

func ingestMD(path string, opts ChunkOpts) ([]Chunk, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	// MD files must be UTF-8; strip BOM just in case.
	text := strings.TrimPrefix(string(raw), "\xef\xbb\xbf")
	sections := splitMDSections(text, path)
	return sectionsToChunks(sections, path, opts), nil
}

// splitMDSections splits Markdown text into sections delimited by ATX headers.
// Content before the first header becomes a section titled with the filename.
// Each section accumulates its paragraphs via splitParagraphsMD so that fenced
// code blocks are never broken across paragraph boundaries.
func splitMDSections(text, source string) []docSection {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")

	defaultTitle := strings.TrimSuffix(filepath.Base(source), filepath.Ext(source))
	var sections []docSection
	cur := docSection{title: defaultTitle}
	var bodyLines []string

	flushSection := func() {
		body := strings.TrimSpace(strings.Join(bodyLines, "\n"))
		if body != "" {
			cur.paras = splitParagraphsMD(body)
		}
		if len(cur.paras) > 0 {
			sections = append(sections, cur)
		}
		bodyLines = nil
	}

	for _, line := range lines {
		if m := mdHeaderRe.FindStringSubmatch(line); m != nil {
			flushSection()
			cur = docSection{title: m[2]}
			// Keep the header line in the body so FTS can match it.
			bodyLines = []string{line}
		} else {
			bodyLines = append(bodyLines, line)
		}
	}
	flushSection()
	return sections
}

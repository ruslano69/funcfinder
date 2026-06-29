package knowledge

import (
	"strings"
	"unicode/utf8"
)

// docSection is an intermediate unit: a titled block with a list of paragraphs.
// Chunks never cross section boundaries — each section produces its own chunk(s).
type docSection struct {
	title string
	paras []string
	page  int // PDF page, 0 otherwise
}

// sectionsToChunks converts sections into Chunks.
// Paragraphs are merged greedily up to opts.MaxRunes. When a section is too
// long to fit in one chunk, it is split at paragraph boundaries with overlap.
func sectionsToChunks(sections []docSection, source string, opts ChunkOpts) []Chunk {
	var out []Chunk
	for _, sec := range sections {
		out = append(out, chunkSection(sec, source, opts)...)
	}
	return out
}

func chunkSection(sec docSection, source string, opts ChunkOpts) []Chunk {
	var chunks []Chunk
	var acc []string
	accRunes := 0
	var overlap string

	flush := func() {
		if len(acc) == 0 {
			return
		}
		var sb strings.Builder
		if overlap != "" {
			sb.WriteString(overlap)
			sb.WriteString("\n\n")
		}
		sb.WriteString(strings.Join(acc, "\n\n"))
		chunks = append(chunks, Chunk{
			Title:   sec.title,
			Content: sb.String(),
			Source:  source,
			Section: sec.title,
			Page:    sec.page,
		})
	}

	for _, para := range sec.paras {
		paraRunes := utf8.RuneCountInString(para)
		// If adding this paragraph overflows AND we already have content, flush first.
		if accRunes+paraRunes > opts.MaxRunes && accRunes > 0 {
			flush()
			// Carry the tail of the last paragraph as overlap for continuity.
			overlap = tailRunes(acc[len(acc)-1], opts.OverlapRunes)
			acc = nil
			accRunes = 0
		}
		acc = append(acc, para)
		accRunes += paraRunes
	}
	flush()
	return chunks
}

// tailRunes returns the last n runes of s, trimmed to a word boundary if possible.
func tailRunes(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	tail := string(runes[len(runes)-n:])
	// Trim to the first space so we don't start mid-word.
	if idx := strings.IndexByte(tail, ' '); idx >= 0 {
		tail = tail[idx+1:]
	}
	return tail
}

// splitParagraphs splits text on blank lines, dropping empty results.
func splitParagraphs(text string) []string {
	parts := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n\n")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// splitParagraphsMD is like splitParagraphs but treats fenced code blocks as
// atomic units — blank lines inside ``` ... ``` do not create new paragraphs.
func splitParagraphsMD(text string) []string {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	var paras []string
	var cur []string
	inFence := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			inFence = !inFence
			cur = append(cur, line)
			continue
		}
		if !inFence && trimmed == "" {
			if len(cur) > 0 {
				paras = append(paras, strings.Join(cur, "\n"))
				cur = nil
			}
		} else {
			cur = append(cur, line)
		}
	}
	if len(cur) > 0 {
		paras = append(paras, strings.Join(cur, "\n"))
	}
	return paras
}

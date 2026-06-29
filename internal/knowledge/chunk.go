package knowledge

import (
	"strings"
	"unicode"
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
		for _, ch := range chunkSection(sec, source, opts) {
			if !hasRepetitiveRuns(ch.Content) {
				out = append(out, ch)
			}
		}
	}
	return out
}

// hasRepetitiveRuns returns true when a chunk looks like a table of contents or
// other low-value filler — detected by a high density of runs of the same
// non-alphanumeric character appearing 4+ times consecutively or separated by
// single spaces (e.g. ". . . . 374" or "--------").
//
// Rule: if more than 25% of the non-space runes in the content belong to such
// repetitive runs, the chunk is considered noise and should be skipped.
func hasRepetitiveRuns(s string) bool {
	runes := []rune(strings.TrimSpace(s))
	if len(runes) == 0 {
		return false
	}

	// Count runes that are part of a repetitive run.
	// A run is: the same non-alphanumeric rune appearing at positions i, i+1, i+2, i+3
	// (consecutive) OR at i, i+2, i+4, i+6 (separated by single spaces).
	inRun := make([]bool, len(runes))

	markRun := func(start, step, minLen int) {
		r := runes[start]
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return
		}
		count := 1
		pos := start + step
		for pos < len(runes) {
			if step == 2 && pos-1 >= 0 && runes[pos-1] != ' ' {
				break
			}
			if runes[pos] == r {
				count++
				pos += step
			} else {
				break
			}
		}
		if count >= minLen {
			for p := start; p < pos && p < len(runes); p += step {
				inRun[p] = true
				if step == 2 && p+1 < len(runes) {
					inRun[p+1] = true // include the space between
				}
			}
		}
	}

	for i := range runes {
		markRun(i, 1, 4) // consecutive: ....
		if i+1 < len(runes) && runes[i+1] == ' ' {
			markRun(i, 2, 4) // spaced: . . . .
		}
	}

	runRunes := 0
	nonSpace := 0
	for i, r := range runes {
		if r != ' ' {
			nonSpace++
			if inRun[i] {
				runRunes++
			}
		}
	}
	if nonSpace == 0 {
		return false
	}
	return float64(runRunes)/float64(nonSpace) > 0.25
}

func chunkSection(sec docSection, source string, opts ChunkOpts) []Chunk {
	// Expand paragraphs that individually exceed MaxRunes into word-sized pieces
	// so that e.g. a PDF page extracted as one long text block is still split.
	expandedParas := make([]string, 0, len(sec.paras))
	for _, p := range sec.paras {
		expandedParas = append(expandedParas, splitLongParagraph(p, opts.MaxRunes)...)
	}

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

	for _, para := range expandedParas {
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

// splitLongParagraph splits a single paragraph that exceeds maxRunes into
// smaller pieces at whitespace boundaries (spaces or newlines). Used to
// handle e.g. PDF pages extracted as one large text block.
func splitLongParagraph(para string, maxRunes int) []string {
	if maxRunes <= 0 {
		return []string{para}
	}
	runes := []rune(para)
	if len(runes) <= maxRunes {
		return []string{para}
	}
	var parts []string
	start := 0
	for start < len(runes) {
		end := start + maxRunes
		if end >= len(runes) {
			parts = append(parts, string(runes[start:]))
			break
		}
		// Find last whitespace in [start, end) to avoid mid-word splits.
		split := -1
		for i := end - 1; i >= start; i-- {
			if runes[i] == ' ' || runes[i] == '\n' {
				split = i
				break
			}
		}
		if split == -1 {
			// No whitespace in window — hard split at maxRunes.
			split = end
			parts = append(parts, string(runes[start:split]))
			start = split
		} else {
			parts = append(parts, string(runes[start:split]))
			start = split + 1
		}
	}
	return parts
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

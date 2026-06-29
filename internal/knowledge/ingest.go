package knowledge

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Chunk is a text segment ready for indexing.
type Chunk struct {
	Title   string // section header or filename
	Content string // plain UTF-8 text
	Source  string // original file path
	Section string // section path (e.g. "Installation > Quick Start")
	Page    int    // PDF page number, 0 if not applicable
}

// ChunkOpts controls how text is split into chunks.
type ChunkOpts struct {
	MaxRunes     int // max chunk size in runes; default 800
	OverlapRunes int // runes repeated at start of next chunk; default 80
}

func (o *ChunkOpts) withDefaults() ChunkOpts {
	out := *o
	if out.MaxRunes == 0 {
		out.MaxRunes = 800
	}
	if out.OverlapRunes == 0 {
		out.OverlapRunes = 80
	}
	return out
}

// IngestFile reads a .txt, .md/.markdown, or .pdf file and returns indexable chunks.
func IngestFile(path string, opts ChunkOpts) ([]Chunk, error) {
	opts = opts.withDefaults()
	switch strings.ToLower(filepath.Ext(path)) {
	case ".txt":
		return ingestTXT(path, opts)
	case ".md", ".markdown":
		return ingestMD(path, opts)
	case ".pdf":
		return ingestPDF(path, opts)
	default:
		return nil, fmt.Errorf("unsupported file type %q (supported: .txt .md .pdf)", filepath.Ext(path))
	}
}

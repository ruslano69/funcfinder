package knowledge

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"golang.org/x/net/html/charset"
	"golang.org/x/text/transform"
)

func ingestTXT(path string, opts ChunkOpts) ([]Chunk, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	text, err := toUTF8(raw)
	if err != nil {
		return nil, fmt.Errorf("decode %s: %w", path, err)
	}
	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	sec := docSection{title: name, paras: splitParagraphs(text)}
	return sectionsToChunks([]docSection{sec}, path, opts), nil
}

// toUTF8 converts arbitrary-encoded bytes to a UTF-8 string.
// It strips the UTF-8 BOM if present, validates the result, and falls back to
// charset detection + re-encoding when the input is not valid UTF-8.
func toUTF8(raw []byte) (string, error) {
	// Strip UTF-8 BOM.
	raw = bytes.TrimPrefix(raw, []byte{0xEF, 0xBB, 0xBF})

	if utf8.Valid(raw) {
		return string(raw), nil
	}

	// Detect encoding from the first 4 KB of content.
	sample := raw
	if len(sample) > 4096 {
		sample = sample[:4096]
	}
	enc, _, _ := charset.DetermineEncoding(sample, "text/plain; charset=")

	decoded, _, err := transform.Bytes(enc.NewDecoder(), raw)
	if err != nil {
		return "", fmt.Errorf("transcode: %w", err)
	}
	return string(decoded), nil
}
